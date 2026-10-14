/*
 * @module api/controllers/workflow_device_test_controller
 * @description 编排真机测试接口：把编排临时下发到测试盒子，用同一组用例逐帧真实执行。
 */

package controllers

import (
	"box-manage-service/service"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowDeviceTestController 真机测试控制器
type WorkflowDeviceTestController struct {
	testService service.WorkflowDeviceTestService
}

// NewWorkflowDeviceTestController 创建控制器
func NewWorkflowDeviceTestController(testService service.WorkflowDeviceTestService) *WorkflowDeviceTestController {
	return &WorkflowDeviceTestController{testService: testService}
}

// StartRun 启动真机测试
// @Summary 启动编排真机测试
// @Description 把编排临时下发到测试盒子，用同一组用例逐帧真实执行，收集每个节点的输入输出，跑完自动删除部署与测试上下文。后台执行，立即返回 run_id。
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param request body service.WorkflowDeviceTestRunRequest true "测试请求"
// @Success 200 {object} APIResponse{data=service.WorkflowDeviceTestRun}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflow-tests/run [post]
func (c *WorkflowDeviceTestController) StartRun(w http.ResponseWriter, r *http.Request) {
	if c.testService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("真机测试服务未初始化", nil))
		return
	}
	var req service.WorkflowDeviceTestRunRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1024*1024))
	if err := decoder.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	run, err := c.testService.Start(r.Context(), &req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("启动真机测试失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("真机测试已启动", run))
}

// GetRun 查询真机测试进度与结果
// @Summary 查询真机测试进度
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param runId path string true "运行ID"
// @Success 200 {object} APIResponse{data=service.WorkflowDeviceTestRun}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/workflow-tests/runs/{runId} [get]
func (c *WorkflowDeviceTestController) GetRun(w http.ResponseWriter, r *http.Request) {
	if c.testService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("真机测试服务未初始化", nil))
		return
	}
	runID := chi.URLParam(r, "runId")
	run, ok := c.testService.Get(runID)
	if !ok {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, NotFoundResponse("运行记录不存在或已过期", nil))
		return
	}
	render.JSON(w, r, SuccessResponse("查询真机测试成功", run))
}

// StopRun 停止真机测试
// @Summary 停止真机测试
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param runId path string true "运行ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflow-tests/runs/{runId}/stop [post]
func (c *WorkflowDeviceTestController) StopRun(w http.ResponseWriter, r *http.Request) {
	if c.testService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("真机测试服务未初始化", nil))
		return
	}
	runID := chi.URLParam(r, "runId")
	if err := c.testService.Stop(runID); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("停止真机测试失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("已请求停止真机测试", nil))
}
