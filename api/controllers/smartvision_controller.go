package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"box-manage-service/service"

	"github.com/go-chi/render"
)

// SmartVisionController SmartVision 对接控制器。
type SmartVisionController struct {
	smartVisionService *service.SmartVisionService
}

// NewSmartVisionController 创建 SmartVision 控制器。
func NewSmartVisionController(smartVisionService *service.SmartVisionService) *SmartVisionController {
	return &SmartVisionController{smartVisionService: smartVisionService}
}

// InnerLoginRequest SmartVision 内登请求。
type InnerLoginRequest struct {
	Token string `json:"token" example:"smartvision-token"`
}

// InnerLogin SmartVision token 换取本地登录 token。
// @Summary SmartVision 内登
// @Description 前端传入 SmartVision token，后端校验后使用本地用户密码调用 postgrest.get_token 并返回本地 token
// @Tags SmartVision
// @Accept json
// @Produce json
// @Param body body InnerLoginRequest true "SmartVision token"
// @Success 200 {object} APIResponse{data=service.SmartVisionInnerLoginResult}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/smartvision/inner_login [post]
func (c *SmartVisionController) InnerLogin(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		var req InnerLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			render.Render(w, r, BadRequestResponse("请求参数解析失败", err))
			return
		}
		token = strings.TrimSpace(req.Token)
	}
	if token == "" {
		render.Render(w, r, BadRequestResponse("token 不能为空", nil))
		return
	}

	data, err := c.smartVisionService.InnerLogin(r.Context(), token)
	if err != nil {
		if strings.Contains(err.Error(), "token 校验失败") || strings.Contains(err.Error(), "Unauthorized") {
			render.Render(w, r, UnauthorizedResponse("SmartVision token 校验失败", err))
			return
		}
		render.Render(w, r, InternalErrorResponse("SmartVision 内登失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("SmartVision 内登成功", data))
}

// SyncUsers 手动同步 SmartVision 用户。
// @Summary 同步 SmartVision 用户
// @Description 手动触发 SmartVision 用户列表同步到 postgrest.users
// @Tags SmartVision
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=service.SmartVisionSyncResult}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/smartvision/sync-users [post]
func (c *SmartVisionController) SyncUsers(w http.ResponseWriter, r *http.Request) {
	data, err := c.smartVisionService.SyncUsers(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("同步 SmartVision 用户失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("同步 SmartVision 用户成功", data))
}

// SyncModels 手动同步 SmartVision 模型。
// @Summary 同步 SmartVision 模型
// @Description 手动触发 SmartVision 成功模型列表同步到 original_models
// @Tags SmartVision
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=service.SmartVisionSyncResult}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/smartvision/sync-models [post]
func (c *SmartVisionController) SyncModels(w http.ResponseWriter, r *http.Request) {
	data, err := c.smartVisionService.SyncModels(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("同步 SmartVision 模型失败", err))
		return
	}
	render.Render(w, r, SuccessResponse("同步 SmartVision 模型成功", data))
}
