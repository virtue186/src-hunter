package dto

import "github.com/src-hunter/internal/model"

type CreateScanRequest struct {
	ProjectID uint `json:"projectId" binding:"required"`
	// 使用模板的唯一ID，而不是名字
	ScanProfileID uint `json:"scanProfileId" binding:"required"`
	// 接受原始字符串输入，而不是数据库ID
	InitialInputs []string `json:"initialInputs" binding:"required,min=1,dive,required"`
	Description   string   `json:"description"`
}

// CreateScanProfileRequest 定义了创建扫描模板的请求体结构
type CreateScanProfileRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	// binding:"required,dive" 确保数组不为空，且对数组内每个元素进行校验
	WorkflowSteps []model.WorkflowStep `json:"workflowSteps" binding:"required,min=1,dive"`
}

// UpdateScanProfileRequest 定义了更新扫描模板的请求体结构
type UpdateScanProfileRequest struct {
	Name          string               `json:"name"` // 更新时，字段变为可选
	Description   string               `json:"description"`
	WorkflowSteps []model.WorkflowStep `json:"workflowSteps"`
	IsActive      *bool                `json:"isActive"` // 使用指针来区分 "未提供" 和 "提供的值为false"
}
