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

// HandleConversionEvents 处理转换事件流
// @Summary 转换事件流
// @Description 建立转换相关事件的SSE连接，实时接收转换开始、完成、失败等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/conversion [get]
func (c *SSEController) HandleConversionEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleConversionEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleExtractTaskEvents 处理抽帧任务事件流
// @Summary 抽帧任务事件流
// @Description 建立抽帧任务状态变化的SSE连接，实时接收抽帧任务创建、开始、完成、失败等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/extract-tasks [get]
func (c *SSEController) HandleExtractTaskEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleExtractTaskEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleRecordTaskEvents 处理录制任务事件流
// @Summary 录制任务事件流
// @Description 建立录制任务状态变化的SSE连接，实时接收录制任务创建、开始、完成、失败等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/record-tasks [get]
func (c *SSEController) HandleRecordTaskEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleRecordTaskEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleDeploymentTaskEvents 处理部署任务事件流
// @Summary 部署任务事件流
// @Description 建立AI推理任务部署状态变化的SSE连接，实时接收任务创建、部署、完成、失败等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/deployment-tasks [get]
func (c *SSEController) HandleDeploymentTaskEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleDeploymentTaskEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleBatchDeploymentEvents 处理批量部署事件流
// @Summary 批量部署事件流
// @Description 建立批量部署任务状态变化的SSE连接，实时接收批量部署进度、完成、失败等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/batch-deployments [get]
func (c *SSEController) HandleBatchDeploymentEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleBatchDeploymentEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleModelDeploymentEvents 处理模型部署任务事件流
// @Summary 模型部署任务事件流
// @Description 建立模型部署任务状态变化的SSE连接，实时接收模型部署进度、完成、失败等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/model-deployments [get]
func (c *SSEController) HandleModelDeploymentEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleModelDeploymentEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleBoxEvents 处理盒子事件流
// @Summary 盒子事件流
// @Description 建立盒子状态变化的SSE连接，实时接收盒子上线、离线等事件
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

// HandleModelEvents 处理模型事件流
// @Summary 模型事件流
// @Description 建立模型相关事件的SSE连接，实时接收模型部署等事件
// @Tags SSE事件流
// @Accept text/event-stream
// @Produce text/event-stream
// @Param user_id query string false "用户ID，用于过滤用户相关事件"
// @Success 200 {string} string "SSE事件流"
// @Failure 500 {object} ErrorResponse
// @Router /api/sse/models [get]
func (c *SSEController) HandleModelEvents(w http.ResponseWriter, r *http.Request) {
	if err := c.sseService.HandleModelEvents(w, r); err != nil {
		render.Render(w, r, InternalErrorResponse("SSE连接失败", err))
		return
	}
}

// HandleSystemEvents 处理系统事件流
// @Summary 系统事件流
// @Description 建立系统错误事件的SSE连接，实时接收系统级错误通知
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
