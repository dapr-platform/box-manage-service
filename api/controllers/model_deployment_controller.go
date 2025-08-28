/*
 * @module controllers/model_deployment_controller
 * @description 模型部署API控制器，提供模型部署任务的HTTP接口
 * @architecture 控制器层
 * @documentReference REQ-004: 模型部署功能
 * @stateFlow HTTP请求 -> 参数验证 -> Service调用 -> 响应返回
 * @rules 提供RESTful风格的模型部署任务管理接口
 * @dependencies service.ModelDeploymentService, go-chi/render
 * @refs DESIGN-007.md
 */

package controllers

import (
	"box-manage-service/service"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ModelDeploymentController 模型部署控制器
type ModelDeploymentController struct {
	deploymentService service.ModelDeploymentService
}

// NewModelDeploymentController 创建模型部署控制器
func NewModelDeploymentController(deploymentService service.ModelDeploymentService) *ModelDeploymentController {
	return &ModelDeploymentController{
		deploymentService: deploymentService,
	}
}

// CreateModelDeploymentRequest 创建模型部署任务请求
type CreateModelDeploymentRequest struct {
	Name              string `json:"name" binding:"required"`                // 任务名称
	ConvertedModelIDs []uint `json:"converted_model_ids" binding:"required"` // 转换后模型ID列表
	BoxIDs            []uint `json:"box_ids" binding:"required"`             // 目标盒子ID列表
}

// CreateDeploymentTask 创建部署任务
// @Summary 创建模型部署任务
// @Description 创建新的模型部署任务，将转换后模型部署到指定盒子
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param request body CreateModelDeploymentRequest true "部署任务信息"
// @Success 201 {object} APIResponse{data=models.ModelDeploymentTask}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks [post]
func (c *ModelDeploymentController) CreateDeploymentTask(w http.ResponseWriter, r *http.Request) {
	var req CreateModelDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	// 设置创建用户
	userID := c.getCurrentUserID(r)

	serviceReq := &service.CreateDeploymentRequest{
		Name:              req.Name,
		ConvertedModelIDs: req.ConvertedModelIDs,
		BoxIDs:            req.BoxIDs,
		UserID:            userID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	task, err := c.deploymentService.CreateDeploymentTask(ctx, serviceReq)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("创建部署任务失败", err))
		return
	}
	go func() {
		c.deploymentService.StartDeployment(r.Context(), strconv.FormatUint(uint64(task.ID), 10))
	}()
	message := fmt.Sprintf("部署任务创建成功，共 %d 个部署项", task.TotalItems)

	render.Status(r, http.StatusCreated)
	render.Render(w, r, SuccessResponse(message, task))
}

// GetDeploymentTasks 获取部署任务列表
// @Summary 获取部署任务列表
// @Description 获取用户的模型部署任务列表
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param user_id query int false "用户ID"
// @Success 200 {object} APIResponse{data=[]models.ModelDeploymentTask}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks [get]
func (c *ModelDeploymentController) GetDeploymentTasks(w http.ResponseWriter, r *http.Request) {
	// 解析用户ID参数
	userID := c.getCurrentUserID(r)
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			userID = uint(uid)
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tasks, err := c.deploymentService.GetDeploymentTasks(ctx, userID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署任务列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署任务列表成功", tasks))
}

// GetDeploymentTask 获取部署任务详情
// @Summary 获取部署任务详情
// @Description 根据任务ID获取部署任务详情
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.ModelDeploymentTask}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks/{taskId} [get]
func (c *ModelDeploymentController) GetDeploymentTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	task, err := c.deploymentService.GetDeploymentTask(ctx, taskID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("部署任务不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署任务详情成功", task))
}

// StartDeployment 启动部署
// @Summary 启动部署任务
// @Description 启动指定的部署任务
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks/{taskId}/start [post]
func (c *ModelDeploymentController) StartDeployment(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()

	if err := c.deploymentService.StartDeployment(ctx, taskID); err != nil {
		render.Render(w, r, InternalErrorResponse("启动部署失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("部署任务启动成功", nil))
}

// CancelDeployment 取消部署
// @Summary 取消部署任务
// @Description 取消正在运行的部署任务
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks/{taskId}/cancel [post]
func (c *ModelDeploymentController) CancelDeployment(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := c.deploymentService.CancelDeployment(ctx, taskID); err != nil {
		render.Render(w, r, InternalErrorResponse("取消部署失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("部署任务取消成功", nil))
}

// GetDeploymentItems 获取部署项列表
// @Summary 获取部署项列表
// @Description 获取指定任务的部署项列表
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse{data=[]models.ModelDeploymentItem}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks/{taskId}/items [get]
func (c *ModelDeploymentController) GetDeploymentItems(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	items, err := c.deploymentService.GetDeploymentItems(ctx, taskID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署项列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署项列表成功", items))
}

// DeleteDeploymentTask 删除部署任务
// @Summary 删除部署任务
// @Description 删除指定的部署任务
// @Tags 模型部署
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/model-deployment/tasks/{taskId} [delete]
func (c *ModelDeploymentController) DeleteDeploymentTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := c.deploymentService.DeleteDeploymentTask(ctx, taskID); err != nil {
		render.Render(w, r, InternalErrorResponse("删除部署任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("部署任务删除成功", nil))
}

// getCurrentUserID 获取当前用户ID（从请求头或上下文获取）
func (c *ModelDeploymentController) getCurrentUserID(r *http.Request) uint {
	// 这里应该从JWT token或session中获取用户ID
	// 为了简化，返回固定值1
	return 1
}
