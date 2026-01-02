/*
 * @module api/controllers/box_client_controller
 * @description 盒子客户端控制器，处理盒子端发送的心跳和状态上报
 * @architecture MVC架构 - 控制器层
 * @documentReference REQ-001: 盒子管理功能, req_1222.md
 * @stateFlow HTTP请求接收 -> 业务逻辑处理 -> 响应返回
 * @rules 处理盒子端的心跳上报，触发任务同步
 * @dependencies box-manage-service/service
 * @refs req_1222.md
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
)

// BoxClientController 盒子客户端控制器
type BoxClientController struct {
	boxClientService *service.BoxClientService
}

// NewBoxClientController 创建盒子客户端控制器实例
func NewBoxClientController(boxClientService *service.BoxClientService) *BoxClientController {
	return &BoxClientController{
		boxClientService: boxClientService,
	}
}

// Heartbeat 处理盒子心跳
// @Summary 接收盒子心跳
// @Description 接收盒子端发送的心跳数据，更新盒子状态并触发任务同步
// @Tags 盒子客户端
// @Accept json
// @Produce json
// @Param request body models.BoxHeartbeatRequest true "心跳数据"
// @Success 200 {object} APIResponse{data=models.BoxHeartbeatResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/box-client/heartbeat [post]
func (c *BoxClientController) Heartbeat(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// 解析请求体
	var heartbeat models.BoxHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
		log.Printf("[BoxClientController] 解析心跳数据失败: %v", err)
		render.Render(w, r, BadRequestResponse("心跳数据格式错误", err))
		return
	}

	// 获取客户端IP地址
	clientIP := getClientIP(r)
	heartbeat.IPAddress = clientIP

	log.Printf("[BoxClientController] 收到心跳请求: IP=%s, DeviceFingerprint=%s, Port=%d",
		clientIP, heartbeat.Device.DeviceFingerprint, heartbeat.Port)

	// 处理心跳
	ctx := r.Context()
	response, err := c.boxClientService.ProcessHeartbeat(ctx, &heartbeat)
	if err != nil {
		log.Printf("[BoxClientController] 处理心跳失败: %v", err)
		render.Render(w, r, InternalErrorResponse("处理心跳失败", err))
		return
	}

	// 记录处理时间
	processingTime := time.Since(startTime)
	log.Printf("[BoxClientController] 心跳处理完成: BoxID=%d, TasksToSync=%d, SyncTriggered=%v, 耗时=%v",
		response.BoxID, response.TasksToSync, response.SyncTriggered, processingTime)

	render.Render(w, r, SuccessResponse("心跳处理成功", response))
}

// HealthCheck 健康检查端点（供盒子确认管理端在线）
// @Summary 健康检查
// @Description 供盒子端检查管理端是否在线
// @Tags 盒子客户端
// @Produce json
// @Success 200 {object} APIResponse
// @Router /api/v1/box-client/health [get]
func (c *BoxClientController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UnixMilli(),
		"service":   "box-manage-service",
	}
	render.Render(w, r, SuccessResponse("服务正常", response))
}

// GetServerTime 获取服务器时间（供盒子同步时间）
// @Summary 获取服务器时间
// @Description 获取管理端服务器当前时间，供盒子同步
// @Tags 盒子客户端
// @Produce json
// @Success 200 {object} APIResponse
// @Router /api/v1/box-client/time [get]
func (c *BoxClientController) GetServerTime(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	response := map[string]interface{}{
		"timestamp":    now.UnixMilli(),
		"timestamp_ns": now.UnixNano(),
		"formatted":    now.Format(time.RFC3339),
		"timezone":     now.Location().String(),
	}
	render.Render(w, r, SuccessResponse("获取服务器时间成功", response))
}

// getClientIP 获取客户端真实IP地址
func getClientIP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取（可能经过代理）
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For 可能包含多个IP，取第一个
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 尝试从 X-Real-IP 获取
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 从 RemoteAddr 获取
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// 如果解析失败，直接返回 RemoteAddr
		return r.RemoteAddr
	}

	return ip
}



