package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/src-hunter/internal/model"
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
	if err := p.DB.Create(&outputRecord).Error; err != nil {
		return p.failTask(&childTask, fmt.Sprintf("存储任务输出失败: %v", err))
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
		// 不是并行子任务，直接标记成功
		childTask.Status = "success"
		childTask.Result = "任务成功完成"
		return p.DB.Save(childTask).Error
	}

	var parentTask model.Task
	var remaining int64

	err := p.DB.Transaction(func(tx *gorm.DB) error {
		// 使用行级锁锁定父任务，防止并发更新问题
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parentTask, childTask.ParentTaskID).Error; err != nil {
			return err
		}
		if parentTask.PendingSubtasks <= 0 {
			// 父任务不是一个扇出节点，或者已经处理完毕
			return nil
		}
		parentTask.PendingSubtasks--
		remaining = int64(parentTask.PendingSubtasks)
		return tx.Save(&parentTask).Error
	})
	if err != nil {
		return err
	}

	// 在事务外标记子任务成功
	childTask.Status = "success"
	childTask.Result = "并行子任务成功完成"
	p.DB.Save(childTask)

	// 修正点 3: 实现了并行任务完成后的“扇入”触发逻辑
	if remaining == 0 {
		fmt.Printf("所有并行子任务 (扇出任务ID: %d) 已全部完成。\n", parentTask.ID)
		parentTask.Status = "success" // 标记扇出任务本身成功
		p.DB.Save(&parentTask)

		// 触发扇出任务的下一步
		var profile model.ScanProfile
		p.DB.First(&profile, parentTask.ScanProfileID) // 使用父任务的ID获取profile

		// 构造一个“伪”payload来触发下一步
		parentPayload := Payload{
			ParentTaskID:    parentTask.ParentTaskID, // 注意，父级是爷级
			ScanProfileID:   parentTask.ScanProfileID,
			CurrentStepName: parentTask.WorkflowStep,
		}

		var parentOutput model.TaskOutput
		// 理论上，扇出任务本身也应该有聚合后的输出，这里暂时简化
		p.DB.Where("task_id = ?", parentTask.ID).First(&parentOutput)

		return p.triggerNextStep(&parentTask, &parentPayload, &profile, &parentOutput)
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
