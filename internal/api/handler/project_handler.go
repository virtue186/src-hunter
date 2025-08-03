package handler

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/src-hunter/internal/api/dto"
	"github.com/src-hunter/internal/api/response"
	"github.com/src-hunter/internal/api/validator"
	"github.com/src-hunter/internal/model"
	"gorm.io/gorm"
	"strconv"
)

type ProjectHandler struct {
	DB *gorm.DB
}

func NewProjectHandler(db *gorm.DB) *ProjectHandler {
	return &ProjectHandler{
		DB: db,
	}
}

func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req dto.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "", err)
		return
	}
	project := model.Project{
		Name:        req.Name,
		Description: req.Description,
	}
	if result := h.DB.Create(&project); result.Error != nil {
		response.ServerError(c, result.Error)
		return
	}
	response.OkWithMessage(c, "创建项目成功", project)
}

func (h *ProjectHandler) AddTargetsToProject(c *gin.Context) {
	projectIDStr := c.Param("projectId")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		response.BadRequest(c, "无效的项目ID", err)
		return
	}
	var req dto.AddTargetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "", err)
		return
	}

	for _, t := range req.Targets {
		if err := validator.ValidateTargetValue(t.Type, t.Value); err != nil {
			// 如果有任何一个目标格式不合法，则直接拒绝整个请求
			response.BadRequest(c, "存在非法目标", err)
			return
		}
	}

	var createdTargets []model.ProjectTarget
	err = h.DB.Transaction(func(tx *gorm.DB) error {
		// 3.1 检查项目是否存在
		var project model.Project
		if err := tx.First(&project, uint(projectID)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 返回一个特殊的错误，以便在事务外识别
				return fmt.Errorf("project_not_found")
			}
			return err
		}

		// 3.2 遍历并创建目标
		for _, t := range req.Targets {
			target := model.ProjectTarget{
				ProjectID:   uint(projectID),
				Value:       t.Value,
				Type:        t.Type,
				Description: t.Description,
			}
			if err := tx.Create(&target).Error; err != nil {
				// 如果发生错误（比如唯一性冲突），则回滚整个事务
				return fmt.Errorf("创建目标 '%s' 失败: %w", t.Value, err)
			}
			createdTargets = append(createdTargets, target)
		}

		// 事务将在函数成功返回时自动提交
		return nil
	})

	if err != nil {
		if err.Error() == "project_not_found" {
			response.BadRequest(c, "项目ID不存在", err)
		} else {
			// 其他所有错误（包括目标重复）都视为内部错误或冲突
			response.ServerError(c, err)
		}
		return
	}
	response.OkWithMessage(c, "添加目标成功", createdTargets)

}
