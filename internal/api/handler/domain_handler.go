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

type DomainHandler struct {
	DB *gorm.DB
}

func NewDomainHandler(db *gorm.DB) *DomainHandler {
	return &DomainHandler{DB: db}
}

// GetDomainsByProject 分页获取指定项目下发现的所有域名
func (h *DomainHandler) GetDomainsByProject(c *gin.Context) {
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
	var domains []model.Domain
	var total int64

	// 创建查询构建器，限定项目ID
	query := h.DB.Model(&model.Domain{}).Where("project_id = ?", projectID)

	if err := query.Count(&total).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.ServerError(c, err)
		}
		return
	}

	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Order("created_at desc").Find(&domains).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.ServerError(c, err)
		}
		return
	}

	// 4. 将数据库模型转换为DTO
	var domainDTOs []dto.DomainResponse
	for _, domain := range domains {
		domainDTOs = append(domainDTOs, dto.DomainResponse{
			ID:         domain.ID,
			FQDN:       domain.FQDN,
			RootDomain: domain.RootDomain,
			Source:     domain.Source,
			CreatedAt:  domain.CreatedAt,
		})
	}

	// 5. 返回分页响应
	response.Ok(c, dto.PaginationResponse{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		List:     domainDTOs,
	})
}
