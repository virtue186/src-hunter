package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/src-hunter/internal/api/dto"
	"github.com/src-hunter/internal/api/response"
	"github.com/src-hunter/internal/model"
	"gorm.io/gorm"
)

type ScanProfileHandler struct {
	DB *gorm.DB
}

func NewScanProfileHandler(db *gorm.DB) *ScanProfileHandler {
	return &ScanProfileHandler{DB: db}
}

// CreateScanProfile 创建一个新的扫描模板
// @Router /scan-profiles [post]
func (h *ScanProfileHandler) CreateScanProfile(c *gin.Context) {
	var req dto.CreateScanProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误", err)
		return
	}

	profile := model.ScanProfile{
		Name:          req.Name,
		Description:   req.Description,
		WorkflowSteps: req.WorkflowSteps,
		IsActive:      true,
	}

	if result := h.DB.Create(&profile); result.Error != nil {
		response.ServerError(c, result.Error)
		return
	}
	response.OkWithMessage(c, "创建成功", profile)
}

// GetScanProfiles 获取所有扫描模板列表
// @Router /scan-profiles [get]
func (h *ScanProfileHandler) GetScanProfiles(c *gin.Context) {
	var profiles []model.ScanProfile
	if result := h.DB.Find(&profiles); result.Error != nil {
		response.ServerError(c, result.Error)
		return
	}
	response.Ok(c, profiles)
}

// GetScanProfileByID 根据ID获取单个扫描模板
// @Router /scan-profiles/{id} [get]
func (h *ScanProfileHandler) GetScanProfileByID(c *gin.Context) {
	var profile model.ScanProfile
	id := c.Param("id")

	if err := h.DB.First(&profile, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c)
			return
		}
		response.ServerError(c, err)
		return
	}
	response.Ok(c, profile)
}

// UpdateScanProfile 更新一个扫描模板
// @Router /scan-profiles/{id} [put]
func (h *ScanProfileHandler) UpdateScanProfile(c *gin.Context) {
	id := c.Param("id")
	var profile model.ScanProfile

	if err := h.DB.First(&profile, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c)
			return
		}
		response.ServerError(c, err)
		return
	}

	var req dto.UpdateScanProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误", err)
		return
	}

	// 按需更新字段
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.WorkflowSteps != nil {
		updates["workflow_steps"] = req.WorkflowSteps
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	// 使用 map 更新可以避免 GORM 的零值问题，更健壮
	if err := h.DB.Model(&profile).Updates(updates).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	response.OkWithMessage(c, "更新成功", profile)
}

// DeleteScanProfile 删除一个扫描模板
// @Router /scan-profiles/{id} [delete]
func (h *ScanProfileHandler) DeleteScanProfile(c *gin.Context) {
	id := c.Param("id")

	if result := h.DB.Unscoped().Delete(&model.ScanProfile{}, id); result.Error != nil {
		response.ServerError(c, result.Error)
		return
	} else if result.RowsAffected == 0 {
		response.NotFound(c)
		return
	}

	response.OkWithMessage(c, "删除成功", nil)
}
