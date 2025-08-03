package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/src-hunter/internal/model"
	"github.com/src-hunter/internal/worker/parser"
	"github.com/src-hunter/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"text/template"
)

type TaskProcessor struct {
	DB          *gorm.DB
	AsynqClient *asynq.Client
	Executor    Executor
}

func NewTaskProcessor(db *gorm.DB, client *asynq.Client) *TaskProcessor {
	return &TaskProcessor{
		DB:          db,
		AsynqClient: client,
		Executor:    NewLocalExecutor(),
	}
}

type Payload struct {
	ParentTaskID    uint   `json:"parent_task_id"`
	ScanProfileID   uint   `json:"scan_profile_id"`
	CurrentStepName string `json:"current_step_name"`
	Input           string `json:"input"`
}

func (p *TaskProcessor) HandleWorkflowTask(ctx context.Context, t *asynq.Task) error {
	var payload Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("解析任务载荷失败: %v", err)
	}

	var profile model.ScanProfile
	if err := p.DB.First(&profile, payload.ScanProfileID).Error; err != nil {
		return fmt.Errorf("查找扫描模板ID %d 失败: %w", payload.ScanProfileID, err)
	}
	step, ok := findStepByName(profile.WorkflowSteps, payload.CurrentStepName)
	if !ok {
		return fmt.Errorf("在模板 %s 中未找到步骤 '%s'", profile.Name, payload.CurrentStepName)
	}

	childTask := model.Task{
		ParentTaskID:  payload.ParentTaskID,
		ScanProfileID: payload.ScanProfileID, // <-- 修正点 2: 保存 ScanProfileID
		Status:        "running",
		Type:          t.Type(),
		WorkflowStep:  payload.CurrentStepName,
	}
	if err := p.DB.Create(&childTask).Error; err != nil {
		return fmt.Errorf("创建子任务数据库记录失败: %w", err)
	}

	inputData, err := p.getInputForTask(&payload, &step)
	if err != nil {
		return p.failTask(&childTask, fmt.Sprintf("获取任务输入失败: %v", err))
	}
	cmdString, err := renderTemplate(step.CommandTemplate, inputData)
	if err != nil {
		return p.failTask(&childTask, fmt.Sprintf("渲染命令模板失败: %v", err))
	}
	logger.Logger.Info("即将执行任务命令",
		zap.Uint("task_id", childTask.ID),
		zap.String("step_name", step.Name),
		zap.String("command", cmdString),
	)
	cmdParts := strings.Fields(cmdString)
	cmdResult, err := p.Executor.Run(ctx, cmdParts[0], cmdParts[1:]...)
	if err != nil {
		errorMsg := fmt.Sprintf("执行步骤 '%s' 失败: %v. Stderr: %s", step.Name, err, string(cmdResult.Stderr))
		return p.failTask(&childTask, errorMsg)
	}
	outputRecord := model.TaskOutput{
		TaskID:       childTask.ID,
		ParentTaskID: payload.ParentTaskID,
		OutputType:   step.OutputParserType,
		Data:         model.JSONB(cmdResult.Stdout),
	}

	if step.OutputParserType != "" {
		registeredParser, err := parser.Get(step.OutputParserType)
		if err != nil {
			// 如果找不到解析器，可以选择仅记录警告而不使任务失败
			p.DB.Model(&childTask).Update("result", fmt.Sprintf("警告：找不到解析器 %s", step.OutputParserType))
		} else {
			parseResult, err := registeredParser.Parse(cmdResult.Stdout)
			if err != nil {
				return p.failTask(&childTask, fmt.Sprintf("使用解析器 '%s' 解析输出失败: %v", step.OutputParserType, err))
			}

			// 3. 将解析后的结构化数据存入相应的数据表
			// 这里需要获取父任务信息来关联 ProjectID
			var parentTask model.Task
			if p.DB.First(&parentTask, payload.ParentTaskID).Error == nil {
				if len(parseResult.Domains) > 0 {
					for i := range parseResult.Domains {
						parseResult.Domains[i].ProjectID = parentTask.ProjectID
						if step.InputFrom == "initial" {
							parseResult.Domains[i].RootDomain = payload.Input
						}
					}
					// 使用 gorm 的批量插入，并忽略冲突
					p.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&parseResult.Domains)
				}
				// (未来可以在此添加对 Assets 的处理)
				// if len(parseResult.Assets) > 0 { ... }
			}
		}
	}

	if err := p.triggerNextStep(&childTask, &payload, &profile, &outputRecord); err != nil {
		return p.failTask(&childTask, fmt.Sprintf("触发下一步任务失败: %v", err))
	}
	if err := p.finalizeParallelSubtask(&childTask); err != nil {
		return p.failTask(&childTask, fmt.Sprintf("终结并行子任务失败: %v", err))
	}
	return nil
}

func (p *TaskProcessor) failTask(task *model.Task, reason string) error {
	task.Status = "failed"
	task.Result = reason
	p.DB.Save(task)
	return fmt.Errorf("task failed: %s", reason)
}

func (p *TaskProcessor) finalizeParallelSubtask(childTask *model.Task) error {
	if childTask.ParentTaskID == 0 {
		// 不是子任务，直接标记成功并返回
		childTask.Status = "success"
		childTask.Result = "任务成功完成"
		return p.DB.Save(childTask).Error
	}

	var parentTask model.Task
	var remaining int64

	err := p.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parentTask, childTask.ParentTaskID).Error; err != nil {
			return err // 找不到父任务，直接返回错误
		}
		if parentTask.PendingSubtasks <= 0 {
			return nil // 父任务无需等待，直接返回
		}
		parentTask.PendingSubtasks--
		remaining = int64(parentTask.PendingSubtasks)
		return tx.Save(&parentTask).Error
	})

	if err != nil {
		return err // 事务失败
	}

	// 标记当前子任务成功
	childTask.Status = "success"
	childTask.Result = "并行子任务成功完成"
	p.DB.Save(childTask)

	// ========== 核心修正区域开始 ==========
	if remaining == 0 {
		fmt.Printf("所有并行子任务 (扇出任务ID: %d) 已全部完成。\n", parentTask.ID)
		parentTask.Status = "success" // 标记扇出任务本身成功
		p.DB.Save(&parentTask)

		var profile model.ScanProfile
		if err := p.DB.First(&profile, parentTask.ScanProfileID).Error; err != nil {
			// 找不到关联的模板，这是一个严重错误
			return p.failTask(&parentTask, fmt.Sprintf("扇入失败：找不到扫描模板ID %d", parentTask.ScanProfileID))
		}

		// 检查这个扇出任务（父任务）之后是否还有步骤
		_, ok := findNextStep(profile.WorkflowSteps, parentTask.WorkflowStep)
		if !ok {
			// 如果没有下一步，说明这是工作流的最后阶段。
			// 我们需要找到最顶层的父任务，并将其标记为 "completed"
			var ultimateParentTask = parentTask
			// 通过循环查找，直到找到 ParentTaskID 为 0 的顶级任务
			for ultimateParentTask.ParentTaskID != 0 {
				if err := p.DB.First(&ultimateParentTask, ultimateParentTask.ParentTaskID).Error; err != nil {
					// 如果中途出错，记录错误并返回
					return p.failTask(&parentTask, fmt.Sprintf("扇入失败：查找顶级父任务时出错: %v", err))
				}
			}
			fmt.Printf("工作流 (顶级任务ID: %d) 已成功完成。\n", ultimateParentTask.ID)
			ultimateParentTask.Status = "completed"
			ultimateParentTask.Result = "工作流成功完成"
			return p.DB.Save(&ultimateParentTask).Error
		}

		parentPayload := Payload{
			ParentTaskID:    parentTask.ParentTaskID, // 父级是爷级
			ScanProfileID:   parentTask.ScanProfileID,
			CurrentStepName: parentTask.WorkflowStep,
		}

		var pseudoOutput model.TaskOutput

		return p.triggerNextStep(&parentTask, &parentPayload, &profile, &pseudoOutput)
	}

	return nil
}

// (getInputForTask, renderTemplate, findStepByName, findNextStep 函数与上个版本相同，此处省略以保持简洁)
// (triggerNextStep 的逻辑也与上一版相同，扇出逻辑已包含在内)
func (p *TaskProcessor) triggerNextStep(currentTask *model.Task, currentPayload *Payload, profile *model.ScanProfile, taskOutput *model.TaskOutput) error {
	nextStep, ok := findNextStep(profile.WorkflowSteps, currentPayload.CurrentStepName)
	if !ok {
		// 这是工作流的最后一步
		return p.DB.Model(&model.Task{}).Where("id = ?", currentPayload.ParentTaskID).Update("status", "completed").Error
	}
	if nextStep.ExecutionMode == "parallel" {
		var results []interface{}
		if err := json.Unmarshal(taskOutput.Data, &results); err == nil && len(results) > 0 {
			p.DB.Model(currentTask).Update("pending_subtasks", len(results))
			for _, item := range results {
				inputBytes, _ := json.Marshal(item)
				nextPayload, _ := json.Marshal(Payload{
					ParentTaskID:    currentTask.ID,
					ScanProfileID:   profile.ID,
					CurrentStepName: nextStep.Name,
					Input:           string(inputBytes),
				})
				task := asynq.NewTask(nextStep.TaskType, nextPayload)
				if _, err := p.AsynqClient.Enqueue(task); err != nil {
					return err
				}
			}
			return nil
		}
	}
	nextPayload, _ := json.Marshal(Payload{
		ParentTaskID:    currentPayload.ParentTaskID,
		ScanProfileID:   profile.ID,
		CurrentStepName: nextStep.Name,
	})
	task := asynq.NewTask(nextStep.TaskType, nextPayload)
	_, err := p.AsynqClient.Enqueue(task)
	return err
}

func (p *TaskProcessor) getInputForTask(payload *Payload, step *model.WorkflowStep) (interface{}, error) {
	if step.InputFrom == "initial" {
		return payload.Input, nil
	}
	var sourceTask model.Task
	if err := p.DB.Where("parent_task_id = ? AND workflow_step = ?", payload.ParentTaskID, step.InputFrom).First(&sourceTask).Error; err != nil {
		return nil, fmt.Errorf("找不到上游任务 '%s' 的记录: %w", step.InputFrom, err)
	}
	var sourceOutput model.TaskOutput
	if err := p.DB.Where("task_id = ?", sourceTask.ID).First(&sourceOutput).Error; err != nil {
		return nil, fmt.Errorf("找不到上游任务 '%s' 的输出结果: %w", step.InputFrom, err)
	}
	var data interface{}
	if err := json.Unmarshal(sourceOutput.Data, &data); err != nil {
		return nil, fmt.Errorf("解析上游任务输出的JSON失败: %w", err)
	}
	return data, nil
}

func renderTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("cmd").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, map[string]interface{}{"Input": data}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func findStepByName(steps []model.WorkflowStep, name string) (model.WorkflowStep, bool) {
	for _, step := range steps {
		if step.Name == name {
			return step, true
		}
	}
	return model.WorkflowStep{}, false
}

func findNextStep(steps []model.WorkflowStep, currentStepName string) (model.WorkflowStep, bool) {
	for _, step := range steps {
		if step.InputFrom == currentStepName {
			return step, true
		}
	}
	return model.WorkflowStep{}, false
}
