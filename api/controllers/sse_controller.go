/*
 * @module controllers/sse_controller
 * @description Server-Sent Events (SSE) API控制器，提供实时事件流接口
 * @architecture 控制器层
 * @documentReference REQ-003: 模型转换功能, REQ-005: 任务管理功能
 * @stateFlow HTTP请求 -> 连接建立 -> 事件流推送 -> 连接管理
 * @rules 提供RESTful风格的SSE事件流管理接口
 * @dependencies service.SSEService, go-chi/render
 * @refs REQ-003.md, REQ-005.md
 */

package controllers

import (
	"box-manage-service/service"
	"net/http"

	"github.com/go-chi/render"
)

// SSEController SSE控制器
type SSEController struct {
	sseService service.SSEService
}

// NewSSEController 创建SSE控制器
func NewSSEController(sseService service.SSEService) *SSEController {
	return &SSEController{
		sseService: sseService,
	}
}

// HandleConversionTaskEvents 处理转换任务事件流
// @Summary 转换任务事件流
// @Description 建立转换任务状态变化的SSE连接，实时接收转换任务状态更新
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/conversion-tasks [get]
func (c *SSEController) HandleConversionTaskEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleConversionTaskEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleTaskEvents 处理任务事件流
// @Summary 任务事件流
// @Description 建立任务状态变化的SSE连接，实时接收任务状态更新
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/tasks [get]
func (c *SSEController) HandleTaskEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleTaskEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleBoxEvents 处理盒子事件流
// @Summary 盒子事件流
// @Description 建立盒子状态变化的SSE连接，实时接收盒子状态更新
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/boxes [get]
func (c *SSEController) HandleBoxEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleBoxEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleSystemEvents 处理系统事件流
// @Summary 系统事件流
// @Description 建立系统事件的SSE连接，实时接收系统级事件通知
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/system [get]
func (c *SSEController) HandleSystemEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleSystemEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleDiscoveryEvents 处理盒子扫描发现事件流
// @Summary 盒子扫描发现事件流
// @Description 建立盒子扫描发现的SSE连接，实时接收扫描进度和发现结果
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/discovery [get]
func (c *SSEController) HandleDiscoveryEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleDiscoveryEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// GetConnectionStats 获取SSE连接统计
// @Summary 获取SSE连接统计
// @Description 获取当前活跃的SSE连接统计信息
// @Tags SSE事件流
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=service.ConnectionStats}
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/stats [get]
func (c *SSEController) GetConnectionStats(w http.ResponseWriter, r *http.Request) {
	stats := c.sseService.GetConnectionStats()
	render.Render(w, r, SuccessResponse("获取连接统计成功", stats))
}

// CloseAllConnections 关闭所有SSE连接
// @Summary 关闭所有SSE连接
// @Description 强制关闭所有活跃的SSE连接（管理员功能）
// @Tags SSE事件流
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/close-all [post]
func (c *SSEController) CloseAllConnections(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.CloseAllConnections(); err != nil {
		render.Render(w, r, InternalErrorResponse("关闭连接失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("所有SSE连接已关闭", nil))
}
