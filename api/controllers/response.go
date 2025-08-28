package controllers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

// 业务状态码定义
const (
	StatusSuccess            = 0   // 成功
	StatusBadRequest         = 400 // 请求参数错误
	StatusUnauthorized       = 401 // 未授权
	StatusForbidden          = 403 // 禁止访问
	StatusNotFound           = 404 // 资源不存在
	StatusConflict           = 409 // 冲突（如资源状态不允许操作）
	StatusInternalError      = 500 // 服务器内部错误
	StatusServiceUnavailable = 503 // 服务不可用
)

// APIResponse 统一API响应结构
type APIResponse struct {
	Code      int         `json:"code" example:"200"`
	Message   string      `json:"message" example:"操作成功"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp string      `json:"timestamp" example:"2025-01-26T12:00:00Z"`
}

// PaginatedResponse 分页响应结构
type PaginatedResponse struct {
	Code      int         `json:"code" example:"200"`
	Message   string      `json:"message" example:"操作成功"`
	Data      interface{} `json:"data"`
	Total     int64       `json:"total" example:"100"`
	Page      int         `json:"page" example:"1"`
	Size      int         `json:"size" example:"10"`
	Timestamp string      `json:"timestamp" example:"2025-01-26T12:00:00Z"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code      int    `json:"code" example:"400"`
	Message   string `json:"message" example:"参数错误"`
	Error     string `json:"error,omitempty" example:"详细错误信息"`
	Timestamp string `json:"timestamp" example:"2025-01-26T12:00:00Z"`
}

// BatchIDsRequest 批量操作请求
type BatchIDsRequest struct {
	IDs []uint `json:"ids" binding:"required" example:"[1,2,3]"` // ID列表
}

// Response 实现render.Renderer接口
func (a *APIResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// 根据业务状态码设置HTTP状态码
	httpStatus := http.StatusOK
	if a.Code >= 400 {
		httpStatus = a.Code
	}
	w.WriteHeader(httpStatus)
	return nil
}

func (e *ErrorResponse) Render(w http.ResponseWriter, r *http.Request) error {
	httpStatus := http.StatusOK
	if e.Code >= 400 {
		httpStatus = e.Code
	}
	w.WriteHeader(httpStatus)
	return nil
}

func (p *PaginatedResponse) Render(w http.ResponseWriter, r *http.Request) error {
	httpStatus := http.StatusOK
	if p.Code >= 400 {
		httpStatus = p.Code
	}
	w.WriteHeader(httpStatus)
	return nil
}

// SuccessResponse 创建成功响应
func SuccessResponse(message string, data interface{}) render.Renderer {
	return &APIResponse{
		Code:      StatusSuccess,
		Message:   message,
		Data:      data,
		Timestamp: getCurrentTimestamp(),
	}
}

// CreateErrorResponse 创建错误响应
func CreateErrorResponse(code int, message string, err error) render.Renderer {
	response := &ErrorResponse{
		Code:      code,
		Message:   message,
		Timestamp: getCurrentTimestamp(),
	}

	if err != nil {
		response.Error = err.Error()
	}

	return response
}

// BadRequestResponse 创建参数错误响应
func BadRequestResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusBadRequest, message, err)
}

// UnauthorizedResponse 创建未授权响应
func UnauthorizedResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusUnauthorized, message, err)
}

// ForbiddenResponse 创建禁止访问响应
func ForbiddenResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusForbidden, message, err)
}

// NotFoundResponse 创建资源不存在响应
func NotFoundResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusNotFound, message, err)
}

// ConflictResponse 创建冲突响应（状态不允许操作）
func ConflictResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusConflict, message, err)
}

// InternalErrorResponse 创建服务器内部错误响应
func InternalErrorResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusInternalError, message, err)
}

// ServiceUnavailableResponse 创建服务不可用响应
func ServiceUnavailableResponse(message string, err error) render.Renderer {
	return CreateErrorResponse(StatusServiceUnavailable, message, err)
}

// PaginatedSuccessResponse 创建分页成功响应
func PaginatedSuccessResponse(message string, data interface{}, total int64, page, size int) render.Renderer {
	return &PaginatedResponse{
		Code:      StatusSuccess,
		Message:   message,
		Data:      data,
		Total:     total,
		Page:      page,
		Size:      size,
		Timestamp: getCurrentTimestamp(),
	}
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() string {
	return time.Now().Format(time.RFC3339)
}
