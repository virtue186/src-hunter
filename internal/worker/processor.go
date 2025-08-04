package worker

import (
	"bytes"
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
	"time"
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
	ProjectID       uint   `json:"project_id"`
	DomainID        uint   `json:"domain_id,omitempty"`
	ParentTaskID    uint   `json:"parent_task_id"`
	ScanProfileID   uint   `json:"scan_profile_id"`
	CurrentStepName string `json:"current_step_name"`
	Input           string `json:"input"`
}

func (p *TaskProcessor) HandleWorkflowTask(ctx context.Context, t *asynq.Task) error {
	logger.Logger.Info("开始处理任务",
		zap.String("task_type", t.Type()),
		zap.ByteString("payload", t.Payload()),
	)

	var payload Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		logger.Logger.Error("解析任务载荷失败",
			zap.Error(err),
			zap.ByteString("payload", t.Payload()),
		)
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
		ProjectID:     payload.ProjectID,
		ParentTaskID:  payload.ParentTaskID,
		ScanProfileID: payload.ScanProfileID,
		Status:        "running",
		Type:          t.Type(),
		WorkflowStep:  payload.CurrentStepName,
		StartedAt:     time.Now(),
	}
	if err := p.DB.Create(&childTask).Error; err != nil {
		return fmt.Errorf("创建子任务数据库记录失败: %w", err)
	}

	if _, err := p.getInputForTask(&payload, &step); err != nil {
		return p.failTask(&childTask, fmt.Sprintf("获取任务输入失败: %v", err))
	}

	cmdString, err := renderTemplate(step.CommandTemplate, payload)
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

	// 准备输出记录，但先不保存
	outputRecord := model.TaskOutput{
		TaskID:       childTask.ID,
		ParentTaskID: payload.ParentTaskID,
		OutputType:   step.OutputParserType,
		Data:         model.JSONB(cmdResult.Stdout), // 默认使用原始输出
	}

	// 如果是 subfinder，将其输出格式化为合法的 JSON 数组
	if step.OutputParserType == "subfinder_json_list" {
		lines := bytes.Split(bytes.TrimSpace(cmdResult.Stdout), []byte("\n"))
		var nonEmptyLines [][]byte
		for _, line := range lines {
			if len(bytes.TrimSpace(line)) > 0 {
				nonEmptyLines = append(nonEmptyLines, line)
			}
		}
		joined := bytes.Join(nonEmptyLines, []byte(","))
		outputRecord.Data = model.JSONB(append(append([]byte{'['}, joined...), ']'))
	}

	// 保存格式化后的输出结果
	if err := p.DB.Create(&outputRecord).Error; err != nil {
		return p.failTask(&childTask, fmt.Sprintf("保存任务输出结果失败: %v", err))
	}

	if step.OutputParserType != "" {
		registeredParser, err := parser.Get(step.OutputParserType)
		if err != nil {
			p.DB.Model(&childTask).Update("result", fmt.Sprintf("警告：找不到解析器 %s", step.OutputParserType))
		} else {
			//确保解析器处理的是格式化后的数据
			parseResult, err := registeredParser.Parse(outputRecord.Data)
			if err != nil {
				return p.failTask(&childTask, fmt.Sprintf("使用解析器 '%s' 解析输出失败: %v", step.OutputParserType, err))
			}

			// --- 数据持久化逻辑 ---

			// 1. 处理域名 (Domains)
			if len(parseResult.Domains) > 0 {
				for i := range parseResult.Domains {
					parseResult.Domains[i].ProjectID = childTask.ProjectID
					parseResult.Domains[i].LastSeenAt = time.Now()
					if step.InputFrom == "initial" {
						parseResult.Domains[i].RootDomain = payload.Input
					}
				}
				// 为了获取新创建域名的ID，我们需要先将它们插入数据库
				p.DB.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "project_id"}, {Name: "fqdn"}},
					DoNothing: true,
				}).Create(&parseResult.Domains)

				// 重新查询，以确保GORM将数据库生成的ID填充回模型切片中
				var fqdns []string
				for _, d := range parseResult.Domains {
					fqdns = append(fqdns, d.FQDN)
				}
				p.DB.Where("project_id = ? AND fqdn IN ?", childTask.ProjectID, fqdns).Find(&parseResult.Domains)

				// 将带有ID的域名列表重新序列化，作为下一步的输入
				updatedDataBytes, _ := json.Marshal(parseResult.Domains)
				outputRecord.Data = updatedDataBytes // 覆盖旧的输出
			}

			// 2. 处理资产 (Assets)
			if len(parseResult.Assets) > 0 {
				for i := range parseResult.Assets {
					parseResult.Assets[i].ProjectID = childTask.ProjectID
					parseResult.Assets[i].LastSeenAt = time.Now()
				}
				// 批量插入/更新资产
				conflictColumns := []clause.Column{{Name: "project_id"}, {Name: "ip"}, {Name: "port"}}
				updateColumns := []string{"last_seen_at", "title", "web_server", "technologies", "updated_at"}
				if err := p.DB.Clauses(clause.OnConflict{
					Columns:   conflictColumns,
					DoUpdates: clause.AssignmentColumns(updateColumns),
				}).Create(&parseResult.Assets).Error; err != nil {
					logger.Logger.Error("批量保存资产记录失败", zap.Error(err))
				}

				// 3. 处理资产与域名的关联 (AssetDomainMapping)
				if payload.DomainID != 0 {
					// 重新查询刚创建/更新的资产，以获取它们的ID
					var createdOrUpdatedAssets []model.Asset
					var ips []string
					for _, a := range parseResult.Assets {
						ips = append(ips, a.IP)
					}
					p.DB.Where("project_id = ? AND ip IN ?", childTask.ProjectID, ips).Find(&createdOrUpdatedAssets)

					// 批量创建关联
					var mappings []model.AssetDomainMapping
					for _, asset := range createdOrUpdatedAssets {
						mappings = append(mappings, model.AssetDomainMapping{
							AssetID:  asset.ID,
							DomainID: payload.DomainID,
						})
					}
					if len(mappings) > 0 {
						p.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&mappings)
					}
				}
			}
		}
	}

	fannedOut, err := p.triggerNextStep(&childTask, &payload, &profile, &outputRecord)
	if err != nil {
		return p.failTask(&childTask, fmt.Sprintf("触发下一步任务失败: %v", err))
	}

	if fannedOut {
		childTask.Status = "success"
		childTask.Result = "已成功派发所有并行子任务"
		childTask.FinishedAt = time.Now()
		return p.DB.Save(&childTask).Error
	}

	if err := p.finalizeParallelSubtask(&childTask); err != nil {
		return p.failTask(&childTask, fmt.Sprintf("终结并行子任务失败: %v", err))
	}

	return nil
}

func (p *TaskProcessor) failTask(task *model.Task, reason string) error {
	task.Status = "failed"
	task.Result = reason
	task.FinishedAt = time.Now()
	p.DB.Save(task)
	return fmt.Errorf("task failed: %s", reason)
}

func (p *TaskProcessor) finalizeParallelSubtask(childTask *model.Task) error {
	if childTask.ParentTaskID == 0 {
		// 对于没有父任务的线性任务，如果能走到这里，说明它可能是最后一步
		// 它的状态将由 triggerNextStep 在判断无下一步时处理
		return nil
	}

	var parentTask model.Task
	var remaining int64

	err := p.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parentTask, childTask.ParentTaskID).Error; err != nil {
			return err
		}
		if parentTask.PendingSubtasks <= 0 {
			return nil
		}
		parentTask.PendingSubtasks--
		remaining = int64(parentTask.PendingSubtasks)
		return tx.Save(&parentTask).Error
	})

	if err != nil {
		return err
	}

	childTask.Status = "success"
	childTask.Result = "并行子任务成功完成"
	childTask.FinishedAt = time.Now()
	p.DB.Save(childTask)

	if remaining == 0 {
		logger.Logger.Info("所有并行子任务已全部完成", zap.Uint("fanOutTaskId", parentTask.ID))
		parentTask.Status = "success"
		parentTask.FinishedAt = time.Now()
		p.DB.Save(&parentTask)

		var profile model.ScanProfile
		if err := p.DB.First(&profile, parentTask.ScanProfileID).Error; err != nil {
			return p.failTask(&parentTask, fmt.Sprintf("扇入失败：找不到扫描模板ID %d", parentTask.ScanProfileID))
		}

		// 检查这个已完成的扇出任务之后是否还有步骤
		_, ok := findNextStep(profile.WorkflowSteps, parentTask.WorkflowStep)
		if !ok {
			// 如果没有下一步，说明工作流的这个分支已经结束，需要检查是否整个工作流都完成了
			var ultimateParentTask = parentTask
			for ultimateParentTask.ParentTaskID != 0 {
				if err := p.DB.First(&ultimateParentTask, ultimateParentTask.ParentTaskID).Error; err != nil {
					return p.failTask(&parentTask, fmt.Sprintf("扇入失败：查找顶级父任务时出错: %v", err))
				}
			}
			logger.Logger.Info("工作流已成功完成", zap.Uint("workflowTaskId", ultimateParentTask.ID))
			ultimateParentTask.Status = "completed"
			ultimateParentTask.Result = "工作流成功完成"
			ultimateParentTask.FinishedAt = time.Now()
			return p.DB.Save(&ultimateParentTask).Error
		}

		// 如果还有下一步，则触发
		parentPayload := Payload{
			ProjectID:       parentTask.ProjectID,
			ParentTaskID:    parentTask.ParentTaskID,
			ScanProfileID:   parentTask.ScanProfileID,
			CurrentStepName: parentTask.WorkflowStep,
		}

		// 扇入后的下一步，其输入是所有子任务结果的聚合，这是一个复杂逻辑
		// 目前我们用一个伪输出触发，意味着下一步必须从数据库自行拉取所有结果
		var pseudoOutput model.TaskOutput
		_, err := p.triggerNextStep(&parentTask, &parentPayload, &profile, &pseudoOutput)
		return err
	}

	return nil
}

func (p *TaskProcessor) triggerNextStep(currentTask *model.Task, currentPayload *Payload, profile *model.ScanProfile, taskOutput *model.TaskOutput) (bool, error) {
	nextStep, ok := findNextStep(profile.WorkflowSteps, currentPayload.CurrentStepName)
	if !ok {
		// 没有下一步，这意味着当前任务是工作流的终点
		// 我们将在 finalizeParallelSubtask 或 HandleWorkflowTask 的主逻辑中处理它的最终状态
		return false, nil
	}

	// 模式一：下一步是并行（扇出）
	if nextStep.ExecutionMode == "parallel" {
		var results []map[string]interface{}
		if taskOutput != nil && taskOutput.Data != nil {
			if err := json.Unmarshal(taskOutput.Data, &results); err == nil && len(results) > 0 {
				p.DB.Model(currentTask).Update("pending_subtasks", len(results))

				for _, itemMap := range results {
					// --- 核心修正点在这里 ---
					// 使用与 model.Domain 序列化后一致的、正确的字段名 "FQDN" 和 "ID"
					host, _ := itemMap["FQDN"].(string)
					domainID := uint(0)
					if idVal, ok := itemMap["ID"].(float64); ok { // JSON 数字默认为 float64
						domainID = uint(idVal)
					}
					// --- 修正结束 ---

					if host == "" {
						continue
					}

					nextPayloadBytes, _ := json.Marshal(Payload{
						ProjectID:       currentPayload.ProjectID,
						DomainID:        domainID,
						ParentTaskID:    currentTask.ID,
						ScanProfileID:   profile.ID,
						CurrentStepName: nextStep.Name,
						Input:           host,
					})

					task := asynq.NewTask(nextStep.TaskType, nextPayloadBytes)
					if _, err := p.AsynqClient.Enqueue(task); err != nil {
						return true, err
					}
				}
				return true, nil // 成功扇出
			}
		}
		return false, nil // 没有可供扇出的结果
	}

	// 模式二：下一步是线性任务
	nextPayloadBytes, _ := json.Marshal(Payload{
		ProjectID:       currentPayload.ProjectID,
		ParentTaskID:    currentPayload.ParentTaskID,
		ScanProfileID:   profile.ID,
		CurrentStepName: nextStep.Name,
		Input:           "", // 线性任务的输入由下一步的 getInputForTask 从数据库获取
	})

	task := asynq.NewTask(nextStep.TaskType, nextPayloadBytes)
	_, err := p.AsynqClient.Enqueue(task)
	return false, err
}

func (p *TaskProcessor) getInputForTask(payload *Payload, step *model.WorkflowStep) (interface{}, error) {
	if step.InputFrom == "initial" || payload.Input != "" {
		// 对于初始任务或已在Payload中携带输入的并行任务，无需操作
		return payload.Input, nil
	}

	// 仅当 Input 为空且非初始步骤时（即线性任务），才从数据库查询
	var sourceTask model.Task
	if err := p.DB.Where("parent_task_id = ? AND workflow_step = ?", payload.ParentTaskID, step.InputFrom).First(&sourceTask).Error; err != nil {
		return nil, fmt.Errorf("找不到上游任务 '%s' 的记录: %w", step.InputFrom, err)
	}
	var sourceOutput model.TaskOutput
	if err := p.DB.Where("task_id = ?", sourceTask.ID).First(&sourceOutput).Error; err != nil {
		return nil, fmt.Errorf("找不到上游任务 '%s' 的输出结果: %w", step.InputFrom, err)
	}

	// 将查找到的完整输出结果（JSON数组）作为字符串填充到 Input 字段
	payload.Input = string(sourceOutput.Data)

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
	if err := t.Execute(&buf, data); err != nil {
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
