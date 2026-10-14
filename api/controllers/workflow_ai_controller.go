/*
 * @module api/controllers/workflow_ai_controller
 * @description AI 快速编排、编排补全（导入）、编排校验三个接口。
 *              均为无副作用的计算型接口：不写数据库，不修改已有编排。
 */

package controllers

import (
	"box-manage-service/service"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/render"
)

// WorkflowAIController AI 快速编排与编排图处理控制器。
type WorkflowAIController struct {
	aiService    service.WorkflowAIService
	graphService service.WorkflowGraphService
}

// NewWorkflowAIController 创建控制器。
func NewWorkflowAIController(aiService service.WorkflowAIService, graphService service.WorkflowGraphService) *WorkflowAIController {
	return &WorkflowAIController{aiService: aiService, graphService: graphService}
}

// GenerateWorkflow 根据参考模板与业务需求生成一份可导入的编排。
// @Summary AI 快速编排（单轮生成）
// @Description 将参考模板编排、业务需求、基础节点说明和编排规则一起发给大模型，返回可直接导入画布的 LogicFlow JSON 和校验报告。需求不明确时返回问题列表。接口不落库，多轮修复由前端驱动。
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param request body service.WorkflowAIGenerateRequest true "生成请求"
// @Success 200 {object} APIResponse{data=service.WorkflowAIGenerateResult}
// @Failure 400 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse "大模型返回内容不可用"
// @Failure 503 {object} ErrorResponse "大模型未配置或不可用"
// @Router /api/v1/workflows/ai-generate [post]
func (c *WorkflowAIController) GenerateWorkflow(w http.ResponseWriter, r *http.Request) {
	if c.aiService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("AI快速编排服务未初始化", nil))
		return
	}
	var req service.WorkflowAIGenerateRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4*1024*1024))
	if err := decoder.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("业务需求不能为空", errors.New("description is required")))
		return
	}

	result, err := c.aiService.Generate(r.Context(), &req)
	if err != nil {
		status := http.StatusBadGateway
		switch {
		case strings.Contains(err.Error(), "not configured"), strings.Contains(err.Error(), "call LLM API"):
			status = http.StatusServiceUnavailable
		case strings.Contains(err.Error(), "exceeds"):
			status = http.StatusBadRequest
		case strings.Contains(err.Error(), "不存在"), strings.Contains(err.Error(), "required"):
			status = http.StatusBadRequest
		}
		render.Status(r, status)
		render.JSON(w, r, CreateErrorResponse(status, "AI生成编排失败", err))
		return
	}

	message := "AI生成编排成功"
	if result.Status == "need_clarification" {
		message = "AI需要确认以下信息"
	}
	render.JSON(w, r, SuccessResponse(message, result))
}

// NormalizeWorkflow 补全并校验编排 JSON，供导入使用。
// @Summary 补全编排 JSON
// @Description 接收完整或精简的 LogicFlow JSON，补全 node_template_id、节点文本、坐标与连线样式后返回可导入的 JSON 及校验报告。
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param request body object true "请求体，字段 graph 为编排 JSON"
// @Success 200 {object} APIResponse{data=service.WorkflowNormalizeResult}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/normalize [post]
func (c *WorkflowAIController) NormalizeWorkflow(w http.ResponseWriter, r *http.Request) {
	c.handleGraphRequest(w, r, "补全编排成功", func(graph json.RawMessage) (interface{}, error) {
		return c.graphService.Normalize(r.Context(), graph)
	})
}

// ValidateWorkflowGraph 校验编排 JSON。
// @Summary 校验编排 JSON
// @Description 对编排 JSON 执行结构、节点模板、连通性、引用、必填项与 Python 静态检查，返回带节点/连线定位的报告。
// @Tags 工作流api-AI编排
// @Accept json
// @Produce json
// @Param request body object true "请求体，字段 graph 为编排 JSON"
// @Success 200 {object} APIResponse{data=service.ValidationReport}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/workflows/validate [post]
func (c *WorkflowAIController) ValidateWorkflowGraph(w http.ResponseWriter, r *http.Request) {
	c.handleGraphRequest(w, r, "校验编排成功", func(graph json.RawMessage) (interface{}, error) {
		return c.graphService.Validate(r.Context(), graph)
	})
}

// handleGraphRequest 解析 { graph: ... } 请求体并交给具体处理函数。
func (c *WorkflowAIController) handleGraphRequest(w http.ResponseWriter, r *http.Request, message string, handler func(json.RawMessage) (interface{}, error)) {
	if c.graphService == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("编排图处理服务未初始化", nil))
		return
	}
	var req struct {
		Graph json.RawMessage `json:"graph"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8*1024*1024))
	if err := decoder.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if len(req.Graph) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("graph不能为空", errors.New("graph is required")))
		return
	}
	result, err := handler(req.Graph)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("编排处理失败", err))
		return
	}
	render.JSON(w, r, SuccessResponse(message, result))
}
