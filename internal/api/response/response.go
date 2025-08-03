package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Response 是我们统一的JSON响应结构体
type Response struct {
	Code int         `json:"code"`           // 自定义业务状态码 (0 表示成功)
	Msg  string      `json:"msg"`            // 响应消息
	Data interface{} `json:"data,omitempty"` // 响应数据，omitempty 表示如果数据为nil,则JSON中省略此字段
}

// 定义一些常用的业务状态码
const (
	SuccessCode = 0 // 成功状态码
	ErrorCode   = 1 // 通用错误状态码
)

// success 方法，用于快速返回一个成功的响应
func successResponse(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: SuccessCode,
		Msg:  msg,
		Data: data,
	})
}

// error 方法，用于快速返回一个失败的响应
func errorResponse(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

// --- 对外暴露的辅助函数 ---

// Ok 通常用于成功返回数据
func Ok(c *gin.Context, data interface{}) {
	successResponse(c, "success", data)
}

// OkWithMessage 用于成功时返回自定义消息
func OkWithMessage(c *gin.Context, msg string, data interface{}) {
	successResponse(c, msg, data)
}

// Fail 通常用于业务逻辑错误
func Fail(c *gin.Context, msg string) {
	errorResponse(c, http.StatusOK, ErrorCode, msg)
}

// FailWithCode 用于返回带自定义业务码的业务错误
func FailWithCode(c *gin.Context, code int, msg string) {
	errorResponse(c, http.StatusOK, code, msg)
}

// BadRequest 用于处理参数绑定或请求格式错误的响应 (HTTP 400)
func BadRequest(c *gin.Context, msg string, err error) {
	if msg == "" {
		msg = "请求参数错误"
	}
	if err != nil {
		c.Error(err).SetType(gin.ErrorTypePrivate)
	}
	errorResponse(c, http.StatusBadRequest, ErrorCode, msg)
}

// NotFound 用于处理资源未找到的响应 (HTTP 404)
func NotFound(c *gin.Context) {
	errorResponse(c, http.StatusNotFound, ErrorCode, "资源未找到")
}

// ServerError 用于处理服务器内部错误的响应 (HTTP 500)
func ServerError(c *gin.Context, err error) {
	if err != nil {
		// 在返回响应前，将原始错误添加到上下文中
		c.Error(err).SetType(gin.ErrorTypePrivate)
	}
	errorResponse(c, http.StatusInternalServerError, ErrorCode, "服务器内部错误")
}
