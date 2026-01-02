/*
 * @module api/controllers/dependency_controller
 * @description 模型依赖控制器 - 管理任务的模型依赖和盒子兼容性检查
 * @architecture RESTful API Controller
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow HTTP请求 -> 依赖服务调用 -> 响应返回
 * @rules 确保任务所需的模型在目标盒子上可用
 * @dependencies service.ModelDependencyService
 */

package controllers

import (
	"net/http"
	"strconv"

	"box-manage-service/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// DependencyController 模型依赖控制器
type DependencyController struct {
	dependencyService service.ModelDependencyService
}

// NewDependencyController 创建模型依赖控制器实例
func NewDependencyController(dependencyService service.ModelDependencyService) *DependencyController {
	return &DependencyController{
		dependencyService: dependencyService,
	}
}

// CheckTaskDependency 检查任务的模型依赖
// @Summary 检查任务模型依赖
// @Description 检查任务所需的模型是否在目标盒子或任何兼容盒子上可用
// @Tags 模型依赖
// @Produce json
// @Param taskId path int true "任务ID"
// @Success 200 {object} APIResponse{data=service.DependencyCheckResult}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/tasks/{taskId}/check [get]
func (c *DependencyController) CheckTaskDependency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取任务ID
	taskIDStr := chi.URLParam(r, "taskId")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	// 调用依赖检查服务
	result, err := c.dependencyService.CheckTaskModelDependency(ctx, uint(taskID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("检查任务依赖失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("检查任务依赖成功", result))
}

// CheckCompatibility 检查盒子与模型的兼容性
// @Summary 检查盒子模型兼容性
// @Description 检查指定盒子是否兼容指定的模型
// @Tags 模型依赖
// @Produce json
// @Param boxId path int true "盒子ID"
// @Param modelKey path string true "模型Key"
// @Success 200 {object} APIResponse{data=service.CompatibilityResult}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/boxes/{boxId}/models/{modelKey}/compatibility [get]
func (c *DependencyController) CheckCompatibility(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取参数
	boxIDStr := chi.URLParam(r, "boxId")
	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	modelKey := chi.URLParam(r, "modelKey")
	if modelKey == "" {
		render.Render(w, r, BadRequestResponse("模型Key不能为空", nil))
		return
	}

	// 调用兼容性检查服务
	result, err := c.dependencyService.CheckBoxModelCompatibility(ctx, uint(boxID), modelKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("检查兼容性失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("检查兼容性成功", result))
}

// GetDeploymentStatus 获取模型在盒子上的部署状态
// @Summary 获取模型部署状态
// @Description 获取指定模型在指定盒子上的部署状态
// @Tags 模型依赖
// @Produce json
// @Param boxId path int true "盒子ID"
// @Param modelKey path string true "模型Key"
// @Success 200 {object} APIResponse{data=service.DeploymentStatus}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/boxes/{boxId}/models/{modelKey}/status [get]
func (c *DependencyController) GetDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取参数
	boxIDStr := chi.URLParam(r, "boxId")
	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	modelKey := chi.URLParam(r, "modelKey")
	if modelKey == "" {
		render.Render(w, r, BadRequestResponse("模型Key不能为空", nil))
		return
	}

	// 调用部署状态查询服务
	result, err := c.dependencyService.GetModelDeploymentStatus(ctx, uint(boxID), modelKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署状态失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署状态成功", result))
}

// DeployModel 将模型部署到盒子
// @Summary 部署模型到盒子
// @Description 将指定模型部署到指定盒子
// @Tags 模型依赖
// @Produce json
// @Param boxId path int true "盒子ID"
// @Param modelKey path string true "模型Key"
// @Success 200 {object} APIResponse{data=service.DeploymentResult}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/boxes/{boxId}/models/{modelKey}/deploy [post]
func (c *DependencyController) DeployModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取参数
	boxIDStr := chi.URLParam(r, "boxId")
	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	modelKey := chi.URLParam(r, "modelKey")
	if modelKey == "" {
		render.Render(w, r, BadRequestResponse("模型Key不能为空", nil))
		return
	}

	// 调用模型部署服务
	result, err := c.dependencyService.DeployModelToBox(ctx, uint(boxID), modelKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("部署模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("部署模型成功", result))
}

// EnsureModelAvailability 确保模型在盒子上可用
// @Summary 确保模型可用
// @Description 确保指定模型在指定盒子上可用，如果未部署则自动部署
// @Tags 模型依赖
// @Produce json
// @Param boxId path int true "盒子ID"
// @Param modelKey path string true "模型Key"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/boxes/{boxId}/models/{modelKey}/ensure [post]
func (c *DependencyController) EnsureModelAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取参数
	boxIDStr := chi.URLParam(r, "boxId")
	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	modelKey := chi.URLParam(r, "modelKey")
	if modelKey == "" {
		render.Render(w, r, BadRequestResponse("模型Key不能为空", nil))
		return
	}

	// 调用确保模型可用服务
	if err := c.dependencyService.EnsureModelAvailability(ctx, uint(boxID), modelKey); err != nil {
		render.Render(w, r, InternalErrorResponse("确保模型可用失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("模型已确保可用", map[string]interface{}{
		"model_key": modelKey,
		"box_id":    boxID,
		"status":    "available",
	}))
}

// GetBoxModels 获取盒子上的模型列表
// @Summary 获取盒子模型列表
// @Description 获取指定盒子上所有已部署的模型
// @Tags 模型依赖
// @Produce json
// @Param boxId path int true "盒子ID"
// @Success 200 {object} APIResponse{data=[]service.BoxModel}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/boxes/{boxId}/models [get]
func (c *DependencyController) GetBoxModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取盒子ID
	boxIDStr := chi.URLParam(r, "boxId")
	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	// 获取盒子模型列表
	models, err := c.dependencyService.GetBoxModels(ctx, uint(boxID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取盒子模型列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取盒子模型列表成功", models))
}

// GetModelUsage 获取模型使用情况
// @Summary 获取模型使用情况
// @Description 获取指定模型的使用情况统计
// @Tags 模型依赖
// @Produce json
// @Param modelKey path string true "模型Key"
// @Success 200 {object} APIResponse{data=service.ModelUsageStats}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/dependencies/models/{modelKey}/usage [get]
func (c *DependencyController) GetModelUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取模型Key
	modelKey := chi.URLParam(r, "modelKey")
	if modelKey == "" {
		render.Render(w, r, BadRequestResponse("模型Key不能为空", nil))
		return
	}

	// 获取模型使用情况
	stats, err := c.dependencyService.GetModelUsage(ctx, modelKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取模型使用情况失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型使用情况成功", stats))
}
