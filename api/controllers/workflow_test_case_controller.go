/*
 * @module api/controllers/workflow_test_case_controller
 * @description 编排测试用例接口：增删改查 + AI 生成用例（只返回草稿，不保存）。
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowTestCaseController 编排测试用例控制器
type WorkflowTestCaseController struct {
	testCaseService service.WorkflowTestCaseService
	aiService       service.WorkflowTestCaseAIService
}

// NewWorkflowTestCaseController 创建控制器
func NewWorkflowTestCaseController(testCaseService service.WorkflowTestCaseService, aiService service.WorkflowTestCaseAIService) *WorkflowTestCaseController {
	return &WorkflowTestCaseController{testCaseService: testCaseService, aiService: aiService}
}

// ListTestCases 查询某编排的测试用例
// @Summary 查询编排测试用例
// @Description 按编排的 key_name 查询全部测试用例（跨版本复用）
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param id path int true "编排ID"
// @Success 200 {object} APIResponse{data=[]models.WorkflowTestCase}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/workflows/{id}/test-cases [get]
func (c *WorkflowTestCaseController) ListTestCases(w http.ResponseWriter, r *http.Request) {
	if c.testCaseService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("测试用例服务未初始化", nil))
		return
	}
	workflowID, err := parseUintParam(r, "id")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("无效的编排ID", err))
		return
	}
	cases, err := c.testCaseService.ListByWorkflowID(r.Context(), workflowID)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("查询测试用例失败", err))
		return
	}
	if cases == nil {
		cases = []*models.WorkflowTestCase{}
	}
	render.JSON(w, r, SuccessResponse("查询测试用例成功", cases))
}

// CreateTestCase 新建测试用例
// @Summary 新建测试用例
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param id path int true "编排ID"
// @Param testCase body models.WorkflowTestCase true "用例"
// @Success 200 {object} APIResponse{data=models.WorkflowTestCase}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/{id}/test-cases [post]
func (c *WorkflowTestCaseController) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	if c.testCaseService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("测试用例服务未初始化", nil))
		return
	}
	workflowID, err := parseUintParam(r, "id")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("无效的编排ID", err))
		return
	}
	testCase, err := decodeTestCase(r)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	created, err := c.testCaseService.Create(r.Context(), workflowID, testCase)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("创建测试用例失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("创建测试用例成功", created))
}

// UpdateTestCase 修改测试用例
// @Summary 修改测试用例
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param caseId path int true "用例ID"
// @Param testCase body models.WorkflowTestCase true "用例"
// @Success 200 {object} APIResponse{data=models.WorkflowTestCase}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflow-test-cases/{caseId} [put]
func (c *WorkflowTestCaseController) UpdateTestCase(w http.ResponseWriter, r *http.Request) {
	if c.testCaseService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("测试用例服务未初始化", nil))
		return
	}
	caseID, err := parseUintParam(r, "caseId")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("无效的用例ID", err))
		return
	}
	testCase, err := decodeTestCase(r)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	updated, err := c.testCaseService.Update(r.Context(), caseID, testCase)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("更新测试用例失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("更新测试用例成功", updated))
}

// DeleteTestCase 删除测试用例
// @Summary 删除测试用例
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param caseId path int true "用例ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflow-test-cases/{caseId} [delete]
func (c *WorkflowTestCaseController) DeleteTestCase(w http.ResponseWriter, r *http.Request) {
	if c.testCaseService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("测试用例服务未初始化", nil))
		return
	}
	caseID, err := parseUintParam(r, "caseId")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("无效的用例ID", err))
		return
	}
	if err := c.testCaseService.Delete(r.Context(), caseID); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("删除测试用例失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("删除测试用例成功", nil))
}

// GenerateTestCase 由业务需求生成测试用例（只返回草稿，不保存）
// @Summary AI 生成测试用例
// @Description 从业务需求出发生成逐帧用例与断言；只给模型节点/输出名称，不给脚本内容，避免「照着实现写期望值」
// @Tags 工作流api-编排测试
// @Accept json
// @Produce json
// @Param id path int true "编排ID"
// @Param request body models.WorkflowTestCaseAIRequest true "生成请求"
// @Success 200 {object} APIResponse{data=models.WorkflowTestCaseAIDraft}
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "LLM未配置或不可用"
// @Router /api/v1/workflows/{id}/test-cases/ai-generate [post]
func (c *WorkflowTestCaseController) GenerateTestCase(w http.ResponseWriter, r *http.Request) {
	if c.aiService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("AI用例生成服务未初始化", nil))
		return
	}
	workflowID, err := parseUintParam(r, "id")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("无效的编排ID", err))
		return
	}
	var req models.WorkflowTestCaseAIRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2*1024*1024))
	if err := decoder.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	// 路径中的编排 ID 优先，避免前端漏传
	req.WorkflowID = workflowID
	if strings.TrimSpace(req.Description) == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("业务需求不能为空", errors.New("description is required")))
		return
	}
	draft, err := c.aiService.Generate(r.Context(), &req)
	if err != nil {
		status := http.StatusBadGateway
		switch {
		case strings.Contains(err.Error(), "not configured"), strings.Contains(err.Error(), "call LLM API"):
			status = http.StatusServiceUnavailable
		case strings.Contains(err.Error(), "exceeds"):
			status = http.StatusBadRequest
		}
		render.Status(r, status)
		render.JSON(w, r, CreateErrorResponse(status, "AI生成测试用例失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse("AI生成测试用例成功", draft))
}

// parseUintParam 解析路径中的无符号整数参数。
func parseUintParam(r *http.Request, name string) (uint, error) {
	raw := chi.URLParam(r, name)
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

// decodeTestCase 解析用例请求体。
func decodeTestCase(r *http.Request) (*models.WorkflowTestCase, error) {
	var testCase models.WorkflowTestCase
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4*1024*1024))
	if err := decoder.Decode(&testCase); err != nil {
		return nil, err
	}
	return &testCase, nil
}
