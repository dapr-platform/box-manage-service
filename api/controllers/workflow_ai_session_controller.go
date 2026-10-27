/*
 * @module api/controllers/workflow_ai_session_controller
 * @description AI 编排会话接口：保存对话过程、查询历史会话、继续修改。
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowAISessionController AI 编排会话控制器
type WorkflowAISessionController struct {
	sessionService service.WorkflowAISessionService
}

// NewWorkflowAISessionController 创建控制器
func NewWorkflowAISessionController(sessionService service.WorkflowAISessionService) *WorkflowAISessionController {
	return &WorkflowAISessionController{sessionService: sessionService}
}

// SaveSession 保存 AI 编排会话
// @Summary 保存 AI 编排会话
// @Description 把一次「与 AI 对话式编排」的过程落库，便于关闭弹窗后继续修改。按 workflow_key_name 定位，同一编排的连续对话复用同一条会话。
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param request body models.WorkflowAISession true "会话内容"
// @Success 200 {object} APIResponse{data=models.WorkflowAISession}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/ai-sessions [post]
func (c *WorkflowAISessionController) SaveSession(w http.ResponseWriter, r *http.Request) {
	if c.sessionService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("AI 会话服务未初始化", nil))
		return
	}
	var session models.WorkflowAISession
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4*1024*1024))
	if err := decoder.Decode(&session); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	saved, err := c.sessionService.Save(r.Context(), &session)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("保存 AI 会话失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("保存 AI 会话成功", saved))
}

// ListSessions 查询某编排的全部 AI 会话
// @Summary 查询 AI 编排会话列表
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param keyName path string true "编排标识（key_name）"
// @Success 200 {object} APIResponse{data=[]models.WorkflowAISession}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/by-key/{keyName}/ai-sessions [get]
func (c *WorkflowAISessionController) ListSessions(w http.ResponseWriter, r *http.Request) {
	if c.sessionService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("AI 会话服务未初始化", nil))
		return
	}
	keyName := chi.URLParam(r, "keyName")
	sessions, err := c.sessionService.ListByWorkflowKeyName(r.Context(), keyName)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("查询 AI 会话失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("查询 AI 会话成功", sessions))
}

// GetLatestSession 查询某编排最近一次的 AI 会话（用于「继续修改」）
// @Summary 查询最近一次 AI 编排会话
// @Description 返回最近一次会话，包含完整对话与最近产出的编排图，前端可据此直接接着改，无需重跑大模型。
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param keyName path string true "编排标识（key_name）"
// @Success 200 {object} APIResponse{data=models.WorkflowAISession}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/by-key/{keyName}/ai-sessions/latest [get]
func (c *WorkflowAISessionController) GetLatestSession(w http.ResponseWriter, r *http.Request) {
	if c.sessionService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("AI 会话服务未初始化", nil))
		return
	}
	keyName := chi.URLParam(r, "keyName")
	session, err := c.sessionService.Latest(r.Context(), keyName)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("查询 AI 会话失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("查询 AI 会话成功", session))
}

// DeleteSession 删除 AI 会话
// @Summary 删除 AI 编排会话
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param sessionId path int true "会话 ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/ai-sessions/{sessionId} [delete]
func (c *WorkflowAISessionController) DeleteSession(w http.ResponseWriter, r *http.Request) {
	if c.sessionService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("AI 会话服务未初始化", nil))
		return
	}
	id, err := strconv.ParseUint(chi.URLParam(r, "sessionId"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("会话 ID 无效", err))
		return
	}
	if err := c.sessionService.Delete(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("删除 AI 会话失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("删除 AI 会话成功", nil))
}
