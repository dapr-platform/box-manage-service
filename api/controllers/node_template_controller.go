/*
 * @module api/controllers/node_template_controller
 * @description 节点模板控制器实现
 * @architecture API层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow HTTP Request -> Controller -> Service -> Repository -> Database
 * @rules 实现节点模板相关的RESTful API接口
 * @dependencies service, models
 * @refs 业务编排引擎需求文档.md 6.3节
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// NodeTemplateController 节点模板控制器
type NodeTemplateController struct {
	templateService service.NodeTemplateService
}

// NewNodeTemplateController 创建节点模板控制器实例
func NewNodeTemplateController(templateService service.NodeTemplateService) *NodeTemplateController {
	return &NodeTemplateController{
		templateService: templateService,
	}
}

// CreateNodeTemplate 创建节点模板
// @Summary 创建节点模板
// @Description 创建新的自定义节点模板，可用于扩展系统功能
// @Tags 节点模板
// @Accept json
// @Produce json
// @Param template body models.NodeTemplate true "节点模板信息，包含key_name、name、type、config_schema等"
// @Success 200 {object} APIResponse{data=models.NodeTemplate} "创建成功，返回节点模板对象"
// @Failure 400 {object} APIResponse "参数错误或key_name已存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/node-templates [post]
func (c *NodeTemplateController) CreateNodeTemplate(w http.ResponseWriter, r *http.Request) {
	var template models.NodeTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "参数错误: " + err.Error()})
		return
	}

	if err := c.templateService.Create(r.Context(), &template); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "创建节点模板失败: " + err.Error()})
		return
	}

	render.JSON(w, r, SuccessResponse("创建节点模板成功", template))
}

// GetNodeTemplate 获取节点模板详情
// @Summary 获取节点模板详情
// @Description 根据ID获取节点模板详情
// @Tags 节点模板
// @Accept json
// @Produce json
// @Param id path int true "模板ID"
// @Success 200 {object} APIResponse{data=models.NodeTemplate}
// @Failure 404 {object} APIResponse
// @Router /api/v1/node-templates/{id} [get]
func (c *NodeTemplateController) GetNodeTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "无效的ID"})
		return
	}

	template, err := c.templateService.GetByID(r.Context(), uint(id))
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, ErrorResponse{Error: "节点模板不存在"})
		return
	}

	render.JSON(w, r, SuccessResponse("获取节点模板成功", template))
}

// UpdateNodeTemplate 更新节点模板
// @Summary 更新节点模板
// @Description 更新节点模板信息
// @Tags 节点模板
// @Accept json
// @Produce json
// @Param id path int true "模板ID"
// @Param template body models.NodeTemplate true "节点模板信息"
// @Success 200 {object} APIResponse{data=models.NodeTemplate}
// @Failure 400 {object} APIResponse
// @Router /api/v1/node-templates/{id} [put]
func (c *NodeTemplateController) UpdateNodeTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "无效的ID"})
		return
	}

	var template models.NodeTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "参数错误: " + err.Error()})
		return
	}

	template.ID = uint(id)
	if err := c.templateService.Update(r.Context(), &template); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "更新节点模板失败: " + err.Error()})
		return
	}

	render.JSON(w, r, SuccessResponse("更新节点模板成功", template))
}

// DeleteNodeTemplate 删除节点模板
// @Summary 删除节点模板
// @Description 删除节点模板（软删除）
// @Tags 节点模板
// @Accept json
// @Produce json
// @Param id path int true "模板ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Router /api/v1/node-templates/{id} [delete]
func (c *NodeTemplateController) DeleteNodeTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "无效的ID"})
		return
	}

	if err := c.templateService.Delete(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "删除节点模板失败: " + err.Error()})
		return
	}

	render.JSON(w, r, SuccessResponse("删除节点模板成功", nil))
}

// GetNodeTemplates 列出节点模板
// @Summary 列出节点模板
// @Description 列出所有可用的节点模板，包括系统预置和自定义模板。支持按类型、分类、启用状态过滤
// @Tags 节点模板
// @Accept json
// @Produce json
// @Param type query string false "节点类型过滤：start/end/python_script/reasoning等"
// @Param category query string false "节点分类过滤：control/script/ai/communication"
// @Param enabled query bool false "是否启用过滤：true只返回启用的模板"
// @Success 200 {object} APIResponse{data=[]models.NodeTemplate} "获取成功，返回节点模板列表"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/node-templates [get]
func (c *NodeTemplateController) GetNodeTemplates(w http.ResponseWriter, r *http.Request) {
	var templates []*models.NodeTemplate
	var err error

	// 根据查询参数过滤
	if nodeType := r.URL.Query().Get("type"); nodeType != "" {
		templates, err = c.templateService.FindByType(r.Context(), nodeType)
	} else if category := r.URL.Query().Get("category"); category != "" {
		templates, err = c.templateService.FindByCategory(r.Context(), category)
	} else if enabledStr := r.URL.Query().Get("enabled"); enabledStr == "true" {
		templates, err = c.templateService.FindEnabled(r.Context())
	} else {
		templates, err = c.templateService.List(r.Context())
	}

	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "获取节点模板列表失败: " + err.Error()})
		return
	}

	render.JSON(w, r, SuccessResponse("获取节点模板列表成功", templates))
}

// GetNodeTemplatesByCategory 根据分类获取节点模板
// @Summary 根据分类获取节点模板
// @Description 根据分类获取节点模板列表，用于前端按分类展示节点
// @Tags 节点模板
// @Accept json
// @Produce json
// @Param category path string true "节点分类：control/script/ai/communication"
// @Success 200 {object} APIResponse{data=[]models.NodeTemplate} "获取成功，返回该分类下的节点模板列表"
// @Failure 400 {object} APIResponse "分类参数无效"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/node-templates/category/{category} [get]
func (c *NodeTemplateController) GetNodeTemplatesByCategory(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")

	templates, err := c.templateService.FindByCategory(r.Context(), category)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "获取节点模板列表失败: " + err.Error()})
		return
	}

	render.JSON(w, r, SuccessResponse("获取节点模板列表成功", templates))
}
