package dto

import "time"

type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// AddTargetsRequest 定义了为项目添加目标的API请求体结构
type AddTargetsRequest struct {
	Targets []struct {
		Value       string `json:"value" binding:"required"`
		Type        string `json:"type" binding:"required,oneof=domain ip cidr"` // 增加类型校验
		Description string `json:"description"`
	} `json:"targets" binding:"required,dive"` // dive关键字会深入到数组内部进行校验
}

type ProjectResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
