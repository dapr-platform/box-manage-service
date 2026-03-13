/*
 * @module api/controllers/workflow_log_controller
 * @description 工作流日志控制器实现
 * @architecture API层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow HTTP Request -> Controller -> Service -> Repository -> Database
 * @rules 实现工作流日志相关的RESTful API接口
 * @dependencies repository, models
 * @refs 业务编排引擎需求文档.md 6.6节
 */

package controllers

import (
	"box-manage-service/repository"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
)

// WorkflowLogController 工作流日志控制器
type WorkflowLogController struct {
	logRepo repository.WorkflowLogRepository
}

// NewWorkflowLogController 创建工作流日志控制器实例
func NewWorkflowLogController(repoManager repository.RepositoryManager) *WorkflowLogController {
	return &WorkflowLogController{
		logRepo: repository.NewWorkflowLogRepository(repoManager.DB()),
	}
}

// GetLogs 获取工作流日志
// @Summary 获取工作流日志
// @Description 获取指定工作流实例的执行日志，按时间倒序排列
// @Tags 工作流日志
// @Accept json
// @Produce json
// @Param workflow_instance_id query int true "工作流实例ID，必填"
// @Param limit query int false "限制返回数量，默认100，最大1000" default(100)
// @Param level query string false "日志级别过滤：info/warn/error"
// @Success 200 {object} APIResponse{data=[]models.WorkflowLog} "获取成功，返回日志列表"
// @Failure 400 {object} APIResponse "工作流实例ID无效"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-logs [get]
func (c *WorkflowLogController) GetLogs(w http.ResponseWriter, r *http.Request) {
	workflowInstanceIDStr := r.URL.Query().Get("workflow_instance_id")
	workflowInstanceID, err := strconv.ParseUint(workflowInstanceIDStr, 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的工作流实例ID", err))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "100"
	}
	limit, _ := strconv.Atoi(limitStr)

	logs, err := c.logRepo.FindByWorkflowInstanceID(r.Context(), uint(workflowInstanceID), limit)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取日志失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取日志成功", logs))
}

// GetNodeLogs 获取节点日志
// @Summary 获取节点日志
// @Description 获取指定节点实例的执行日志，用于查看单个节点的详细执行过程
// @Tags 工作流日志
// @Accept json
// @Produce json
// @Param node_instance_id query int true "节点实例ID，必填"
// @Param limit query int false "限制返回数量，默认100，最大1000" default(100)
// @Param level query string false "日志级别过滤：info/warn/error"
// @Success 200 {object} APIResponse{data=[]models.WorkflowLog} "获取成功，返回日志列表"
// @Failure 400 {object} APIResponse "节点实例ID无效"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-logs/node [get]
func (c *WorkflowLogController) GetNodeLogs(w http.ResponseWriter, r *http.Request) {
	nodeInstanceIDStr := r.URL.Query().Get("node_instance_id")
	nodeInstanceID, err := strconv.ParseUint(nodeInstanceIDStr, 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的节点实例ID", err))
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "100"
	}
	limit, _ := strconv.Atoi(limitStr)

	logs, err := c.logRepo.FindByNodeInstanceID(r.Context(), uint(nodeInstanceID), limit)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取日志失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取日志成功", logs))
}
