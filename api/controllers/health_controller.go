/*
 * @module api/controllers/health_controller
 * @description 健康检查控制器，提供服务健康状态检查
 * @architecture MVC架构 - 控制器层
 * @documentReference DESIGN-000.md
 * @stateFlow HTTP请求处理流程
 * @rules 提供简单的健康检查接口，用于容器健康检查和负载均衡
 * @dependencies net/http
 * @refs DESIGN-000.md
 */

package controllers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

// HealthController 健康检查控制器
type HealthController struct{}

// NewHealthController 创建健康检查控制器实例
func NewHealthController() *HealthController {
	return &HealthController{}
}

// HealthResponse 健康检查响应结构
type HealthResponse struct {
	Status    string    `json:"status" example:"ok"`
	Timestamp time.Time `json:"timestamp" example:"2024-01-01T00:00:00Z"`
	Version   string    `json:"version" example:"1.0.0"`
	Service   string    `json:"service" example:"box-manage-service"`
}

// Health 健康检查
// @Summary 健康检查
// @Description 检查服务健康状态
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (c *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Service:   "box-manage-service",
	}

	render.Render(w, r, SuccessResponse("服务健康", response))
}

// Ready 就绪检查
// @Summary 就绪检查
// @Description 检查服务是否就绪
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /ready [get]
func (c *HealthController) Ready(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Service:   "box-manage-service",
	}

	render.Render(w, r, SuccessResponse("服务就绪", response))
}
