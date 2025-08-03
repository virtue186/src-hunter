package handler

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/src-hunter/internal/api/dto"
	"github.com/src-hunter/internal/api/response"
	"github.com/src-hunter/internal/model"
	"gorm.io/gorm"
)

type ScanHandler struct {
	DB          *gorm.DB
	AsynqClient *asynq.Client
}

func NewScanHandler(db *gorm.DB, asynqClient *asynq.Client) *ScanHandler {
	return &ScanHandler{
		DB:          db,
		AsynqClient: asynqClient,
	}
}

func (h *ScanHandler) CreateScan(c *gin.Context) {
	var req dto.CreateScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误", err)
		return
	}

	var profile model.ScanProfile
	var parentTask model.Task

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 获取并验证扫描模板
		if err := tx.First(&profile, req.ScanProfileID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("扫描模板不存在")
			}
			return err
		}

		// *** 核心修正点在这里 ***
		// 2. 将完整的请求体序列化为JSON，作为父任务的Payload
		payloadBytes, err := json.Marshal(req)
		if err != nil {
			// 如果gin可以绑定成功，这里序列化通常不会失败，但做好防御是好习惯
			return errors.New("无法序列化请求为Payload")
		}

		// 创建父任务，代表整个工作流
		parentTask = model.Task{
			ProjectID:     req.ProjectID,
			ScanProfileID: req.ScanProfileID,
			Type:          "workflow",
			Status:        "pending",
			Payload:       payloadBytes, // 存入序列化后的 []byte
		}
		if err := tx.Create(&parentTask).Error; err != nil {
			return err
		}

		// 3. 找到工作流的第一步
		firstStep, ok := findFirstStep(profile.WorkflowSteps)
		if !ok {
			return errors.New("无法在工作流中找到起始步骤 (input_from: 'initial')")
		}

		// 4. 为每个初始输入派发第一步的任务
		for _, input := range req.InitialInputs {
			taskPayload, _ := json.Marshal(map[string]interface{}{
				"parent_task_id":    parentTask.ID,
				"scan_profile_id":   profile.ID,
				"current_step_name": firstStep.Name,
				"input":             input,
			})

			task := asynq.NewTask(firstStep.TaskType, taskPayload)
			if _, err := h.AsynqClient.Enqueue(task, asynq.Queue("default")); err != nil { // 明确指定队列
				return err // 事务会回滚
			}
		}

		return nil
	})

	if err != nil {
		// 这里可以根据err的类型返回更具体的HTTP状态码
		if err.Error() == "扫描模板不存在" {
			response.Fail(c, err.Error())
		} else {
			response.ServerError(c, err)
		}
		return
	}

	// 5. 返回父任务ID，让用户可以追踪整个工作流的进度
	response.OkWithMessage(c, "工作流扫描任务已成功创建并启动。", gin.H{
		"parentTaskId": parentTask.ID,
	})
}

func findFirstStep(steps []model.WorkflowStep) (model.WorkflowStep, bool) {
	for _, step := range steps {
		if step.InputFrom == "initial" {
			return step, true
		}
	}
	return model.WorkflowStep{}, false
}
