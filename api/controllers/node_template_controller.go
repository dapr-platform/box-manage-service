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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
// @Tags 工作流api-节点模板
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
// @Tags 工作流api-节点模板
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
// @Tags 工作流api-节点模板
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
// @Tags 工作流api-节点模板
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
// @Description 列出所有可用的节点模板，包括系统预置和自定义模板。支持按类型、分类、启用状态过滤，多个条件可组合使用，支持分页
// @Tags 工作流api-节点模板
// @Accept json
// @Produce json
// @Param type query string false "节点类型过滤（模糊匹配）：start/end/python_script/reasoning等"
// @Param category query string false "节点分类过滤：logic/business"
// @Param class_type query string false "类型管理分类类型"
// @Param class_name query string false "类型管理分类名称"
// @Param enabled query bool false "是否启用过滤：true 只返回启用的模板，false 只返回禁用的模板"
// @Success 200 {object} APIResponse{data=[]models.NodeTemplate} "获取成功，返回节点模板列表（包含 Variables）"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/node-templates [get]
func (c *NodeTemplateController) GetNodeTemplates(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	nodeType := r.URL.Query().Get("type")
	category := r.URL.Query().Get("category")
	classType := r.URL.Query().Get("class_type")
	className := r.URL.Query().Get("class_name")
	enabledStr := r.URL.Query().Get("enabled")

	// 先获取所有模板（包含 Variables）
	templates, err := c.templateService.List(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取节点模板列表失败", err))
		return
	}

	// 应用过滤条件
	var filtered []*models.NodeTemplate
	for _, template := range templates {
		// 类型过滤（模糊匹配，不区分大小写）
		if nodeType != "" {
			lowerType := strings.ToLower(nodeType)
			lowerTypeKey := strings.ToLower(template.TypeKey)
			lowerTypeName := strings.ToLower(template.TypeName)

			if !strings.Contains(lowerTypeKey, lowerType) && !strings.Contains(lowerTypeName, lowerType) {
				continue
			}
		}

		// 分类过滤（精确匹配）
		if category != "" && template.Category != category {
			continue
		}

		if classType != "" && template.ClassType != classType {
			continue
		}

		if className != "" && template.ClassName != className {
			continue
		}

		// 启用状态过滤
		if enabledStr != "" {
			enabled := enabledStr == "true"
			if template.IsEnabled != enabled {
				continue
			}
		}

		filtered = append(filtered, template)
	}

	render.Render(w, r, SuccessResponse("获取节点模板列表成功", filtered))
}

// GetNodeTemplatesByCategory 根据分类获取节点模板
// @Summary 根据分类获取节点模板
// @Description 根据分类获取节点模板列表，用于前端按分类展示节点
// @Tags 工作流api-节点模板
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

// ExportNodeTemplates 导出节点模板为SQL文件
// @Summary 导出节点模板
// @Description 导出节点模板及其关联的变量定义为SQL文件，用于共享给其他现场使用
// @Tags 工作流api-节点模板
// @Produce application/sql
// @Param category query string false "节点分类过滤：logic/business，为空则导出全部"
// @Success 200 {file} file "返回Sql文件"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/node-templates/export [get]
func (c *NodeTemplateController) ExportNodeTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	sqlContent, err := c.templateService.ExportSQL(r.Context(), category)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "导出失败: " + err.Error()})
		return
	}

	// 设置响应头，触发文件下载
	filename := fmt.Sprintf("node_templates_export_%s.sql", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/sql")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(sqlContent)))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(sqlContent))
}

// ImportNodeTemplates 导入节点模板SQL文件
// @Summary 导入节点模板
// @Description 上传SQL文件导入节点模板及其关联的变量定义
// @Tags 工作流api-节点模板
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "要导入的SQL文件"
// @Success 200 {object} APIResponse "导入成功"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/node-templates/import [post]
func (c *NodeTemplateController) ImportNodeTemplates(w http.ResponseWriter, r *http.Request) {
	// 解析multipart表单，最大 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "解析表单失败: " + err.Error()})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "获取上传文件失败: " + err.Error()})
		return
	}
	defer file.Close()

	// 验证文件后缀
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".sql") {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "只支持 .sql 文件"})
		return
	}

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "读取文件失败: " + err.Error()})
		return
	}

	sqlContent := string(content)
	if strings.TrimSpace(sqlContent) == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Error: "SQL文件内容为空"})
		return
	}

	// 执行导入
	if err := c.templateService.ImportSQL(r.Context(), sqlContent); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Error: "导入失败: " + err.Error()})
		return
	}

	render.JSON(w, r, SuccessResponse("导入节点模板成功", nil))
}
