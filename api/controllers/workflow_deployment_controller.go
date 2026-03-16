/*
 * @module api/controllers/workflow_deployment_controller
 * @description 工作流部署控制器实现
 * @architecture API层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow HTTP Request -> Controller -> Service -> Repository -> Database
 * @rules 实现工作流部署相关的RESTful API接口
 * @dependencies service, models
 * @refs 业务编排引擎需求文档.md 6.5节
 */

package controllers

import (
	"box-manage-service/service"
	"net/http"
	"strconv"

	"encoding/json"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowDeploymentController 工作流部署控制器
type WorkflowDeploymentController struct {
	deploymentService service.WorkflowDeploymentService
}

// NewWorkflowDeploymentController 创建工作流部署控制器实例
func NewWorkflowDeploymentController(deploymentService service.WorkflowDeploymentService) *WorkflowDeploymentController {
	return &WorkflowDeploymentController{
		deploymentService: deploymentService,
	}
}

// DeployRequest 部署请求
type DeployRequest struct {
	WorkflowID uint `json:"workflow_id" binding:"required"`
	BoxID      uint `json:"box_id" binding:"required"`
}

// WorkflowBatchDeployRequest 工作流批量部署请求
type WorkflowBatchDeployRequest struct {
	WorkflowID uint   `json:"workflow_id" binding:"required"`
	BoxIDs     []uint `json:"box_ids" binding:"required"`
}

// Deploy 部署工作流
// @Summary 部署工作流到盒子
// @Description 将工作流部署到指定盒子
// @Tags 工作流部署
// @Accept json
// @Produce json
// @Param request body DeployRequest true "部署请求"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Router /api/v1/workflow-deployments/deploy [post]
func (c *WorkflowDeploymentController) Deploy(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	if err := c.deploymentService.Deploy(r.Context(), req.WorkflowID, req.BoxID); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "部署工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("部署工作流成功", nil))
}

// BatchDeploy 批量部署工作流
// @Summary 批量部署工作流
// @Description 将工作流批量部署到多个盒子
// @Tags 工作流部署
// @Accept json
// @Produce json
// @Param request body BatchDeployRequest true "批量部署请求"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Router /api/v1/workflow-deployments/batch-deploy [post]
func (c *WorkflowDeploymentController) BatchDeploy(w http.ResponseWriter, r *http.Request) {
	var req WorkflowBatchDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	if err := c.deploymentService.DeployToMultipleBoxes(r.Context(), req.WorkflowID, req.BoxIDs); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "批量部署工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("批量部署工作流成功", nil))
}

// Rollback 回滚部署
// @Summary 回滚部署
// @Description 回滚工作流部署到上一个版本
// @Tags 工作流部署
// @Accept json
// @Produce json
// @Param id path int true "部署ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Router /api/v1/workflow-deployments/{id}/rollback [post]
func (c *WorkflowDeploymentController) Rollback(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.deploymentService.Rollback(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "回滚部署失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("回滚部署成功", nil))
}

// GetDeployment 获取部署详情
// @Summary 获取部署详情
// @Description 根据ID获取部署详情
// @Tags 工作流部署
// @Accept json
// @Produce json
// @Param id path int true "部署ID"
// @Success 200 {object} APIResponse{data=models.WorkflowDeployment}
// @Failure 404 {object} APIResponse
// @Router /api/v1/workflow-deployments/{id} [get]
func (c *WorkflowDeploymentController) GetDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	deployment, err := c.deploymentService.GetDeployment(r.Context(), uint(id))
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, CreateErrorResponse(http.StatusNotFound, "部署记录不存在", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取部署详情成功", deployment))
}

// ListDeployments 列出部署记录
// @Summary 列出部署记录
// @Description 列出工作流或盒子的部署记录
// @Tags 工作流部署
// @Accept json
// @Produce json
// @Param workflow_id query int false "工作流ID"
// @Param box_id query int false "盒子ID"
// @Success 200 {object} APIResponse{data=[]models.WorkflowDeployment}
// @Router /api/v1/workflow-deployments [get]
func (c *WorkflowDeploymentController) ListDeployments(w http.ResponseWriter, r *http.Request) {
	if workflowIDStr := r.URL.Query().Get("workflow_id"); workflowIDStr != "" {
		workflowID, _ := strconv.ParseUint(workflowIDStr, 10, 32)
		deployments, err := c.deploymentService.ListDeployments(r.Context(), uint(workflowID))
		if err != nil {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取部署记录失败", err))
			return
		}
		render.JSON(w, r, SuccessResponse("获取部署记录成功", deployments))
		return
	}

	if boxIDStr := r.URL.Query().Get("box_id"); boxIDStr != "" {
		boxID, _ := strconv.ParseUint(boxIDStr, 10, 32)
		deployments, err := c.deploymentService.ListDeploymentsByBox(r.Context(), uint(boxID))
		if err != nil {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取部署记录失败", err))
			return
		}
		render.JSON(w, r, SuccessResponse("获取部署记录成功", deployments))
		return
	}

	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "请提供workflow_id或box_id参数", nil))
}
