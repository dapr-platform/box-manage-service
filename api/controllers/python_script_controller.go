/*
 * @module api/controllers/python_script_controller
 * @description Python 节点脚本生成 API，仅返回未落库的脚本草稿
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

// PythonScriptController Python 节点脚本生成控制器。
type PythonScriptController struct {
	generator service.PythonScriptGeneratorService
}

// NewPythonScriptController 创建 Python 脚本生成控制器。
func NewPythonScriptController(generator service.PythonScriptGeneratorService) *PythonScriptController {
	return &PythonScriptController{generator: generator}
}

// GenerateScript 根据节点描述和 DAG 上下文生成 Python 脚本草稿。
// @Summary 生成 Python 节点脚本
// @Description 调用配置的 OpenAI 兼容大模型，根据自然语言描述、上下游契约和样例生成可预览脚本。接口不会执行脚本，也不会修改工作流或节点数据。
// @Tags 工作流api-Python节点
// @Accept json
// @Produce json
// @Param request body service.PythonScriptGenerateRequest true "Python节点脚本生成上下文"
// @Success 200 {object} APIResponse{data=service.PythonScriptDraft}
// @Failure 400 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse "LLM未配置或不可用"
// @Failure 502 {object} ErrorResponse "LLM返回结果无效"
// @Router /api/v1/python-script/generate [post]
// @Router /api/v1/node-templates/python-script/generate [post]
func (c *PythonScriptController) GenerateScript(w http.ResponseWriter, r *http.Request) {
	if c.generator == nil {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, ServiceUnavailableResponse("Python脚本生成服务未初始化", nil))
		return
	}

	var req service.PythonScriptGenerateRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2*1024*1024))
	if err := decoder.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("请求参数解析失败", err))
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, BadRequestResponse("description不能为空", errors.New("description is required")))
		return
	}

	draft, err := c.generator.Generate(r.Context(), &req)
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "not configured") || strings.Contains(err.Error(), "call LLM API") {
			status = http.StatusServiceUnavailable
		} else if strings.Contains(err.Error(), "context exceeds") {
			status = http.StatusBadRequest
		}
		render.Status(r, status)
		render.JSON(w, r, CreateErrorResponse(status, "生成Python脚本失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("生成Python脚本成功", draft))
}
