package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/src-hunter/internal/api/dto"
	"github.com/src-hunter/internal/api/response"
	"github.com/src-hunter/internal/model"
	"gorm.io/gorm"
	"strconv"
)

type TaskHandler struct {
	DB *gorm.DB
}

func NewTaskHandler(db *gorm.DB) *TaskHandler {
	return &TaskHandler{DB: db}
}

// GetTasksByProject 分页获取指定项目下的所有父任务
func (h *TaskHandler) GetTasksByProject(c *gin.Context) {
	// 1. 解析项目ID
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		response.BadRequest(c, "无效的项目ID", err)
		return
	}

	// 2. 绑定分页参数
	var req dto.PaginationRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "分页参数错误", err)
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 10
	}

	// 3. 查询总数和当前页数据
	var tasks []model.Task
	var total int64

	// 创建查询构建器，限定项目ID和只看父任务 (ParentTaskID = 0)
	query := h.DB.Model(&model.Task{}).Where("project_id = ? AND parent_task_id = ?", projectID, 0)

	if err := query.Count(&total).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.ServerError(c, err)
		}
		return
	}

	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Order("created_at desc").Find(&tasks).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.ServerError(c, err)
		}
		return
	}

	// 4. 将数据库模型转换为DTO
	var taskDTOs []dto.TaskResponse
	for _, task := range tasks {
		taskDTOs = append(taskDTOs, dto.TaskResponse{
			ID:            task.ID,
			ProjectID:     task.ProjectID,
			ScanProfileID: task.ScanProfileID,
			Status:        task.Status,
			Result:        task.Result,
			FinishedAt:    task.FinishedAt,
			CreatedAt:     task.CreatedAt,
		})
	}

	// 5. 返回分页响应
	response.Ok(c, dto.PaginationResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     taskDTOs,
	})
}
