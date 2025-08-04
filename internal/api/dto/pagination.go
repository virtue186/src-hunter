package dto

// PaginationRequest 定义了分页请求的通用参数
type PaginationRequest struct {
	Page     int `form:"page,default=1"`
	PageSize int `form:"pageSize,default=10"`
}

// PaginationResponse 定义了分页响应的通用结构
type PaginationResponse struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	List     interface{} `json:"list"`
}
