/*
 * @module api/controllers/model_controller
 * @description 原始模型管理控制器，提供模型上传、管理等功能
 * @architecture MVC架构 - 控制器层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow HTTP请求处理 -> 业务逻辑处理 -> 数据库操作 -> 响应返回
 * @rules 遵循PostgREST RBAC权限验证，所有操作需要相应权限
 * @dependencies box-manage-service/service
 * @refs REQ-002.md, DESIGN-003.md, DESIGN-004.md
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"box-manage-service/service"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ModelController 原始模型管理控制器
type ModelController struct {
	modelService service.ModelService
}

// NewModelController 创建模型控制器实例
func NewModelController(modelService service.ModelService) *ModelController {
	return &ModelController{
		modelService: modelService,
	}
}

// UploadModel 直接上传模型文件
// @Summary 上传模型文件
// @Description 直接上传模型文件，支持.pt、.onnx、.pth等格式
// @Tags 模型管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "模型文件"
// @Param name formData string true "模型名称"
// @Param version formData string true "模型版本"
// @Param model_type formData string true "模型类型" Enums(pt,onnx,pth)
// @Param framework formData string true "框架类型" Enums(pytorch,onnx,tensorflow)
// @Param description formData string false "模型描述"
// @Param tags formData string false "标签列表，逗号分隔"
// @Param task_type formData string false "AI任务类型" Enums(detection,segmentation,classification) default(detection)
// @Param input_width formData int false "输入宽度" default(640)
// @Param input_height formData int false "输入高度" default(640)
// @Param input_channels formData int false "输入通道数" default(3)
// @Success 200 {object} APIResponse{data=models.OriginalModel} "上传成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "内部服务器错误"
// @Router /api/models/upload [post]
func (c *ModelController) UploadModel(w http.ResponseWriter, r *http.Request) {
	// 限制上传文件大小为 500MB
	maxSize := int64(500 << 20) // 500MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	// 解析multipart表单
	if err := r.ParseMultipartForm(maxSize); err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "解析表单失败或文件过大", err))
		return
	}

	// 获取上传的文件
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "获取上传文件失败", err))
		return
	}
	defer file.Close()

	// 获取表单参数
	name := r.FormValue("name")
	version := r.FormValue("version")
	modelType := r.FormValue("model_type")
	framework := r.FormValue("framework")
	description := r.FormValue("description")
	tagsStr := r.FormValue("tags")

	// AI任务相关参数
	taskTypeStr := r.FormValue("task_type")
	inputWidthStr := r.FormValue("input_width")
	inputHeightStr := r.FormValue("input_height")
	inputChannelsStr := r.FormValue("input_channels")

	// 验证必需参数
	if name == "" || version == "" || modelType == "" || framework == "" {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "缺少必需参数: name, version, model_type, framework", nil))
		return
	}

	// 解析标签
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(strings.TrimSpace(tagsStr), ",")
		// 清理标签
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// 解析AI任务相关参数
	var taskType models.ModelTaskType = models.ModelTaskTypeDetection // 默认值
	if taskTypeStr != "" {
		taskType = models.ModelTaskType(taskTypeStr)
	}

	inputWidth := 640 // 默认值
	if inputWidthStr != "" {
		if parsed, err := strconv.Atoi(inputWidthStr); err == nil && parsed > 0 {
			inputWidth = parsed
		}
	}

	inputHeight := 640 // 默认值
	if inputHeightStr != "" {
		if parsed, err := strconv.Atoi(inputHeightStr); err == nil && parsed > 0 {
			inputHeight = parsed
		}
	}

	inputChannels := 3 // 默认值
	if inputChannelsStr != "" {
		if parsed, err := strconv.Atoi(inputChannelsStr); err == nil && parsed > 0 {
			inputChannels = parsed
		}
	}

	// 构建上传请求
	req := &service.DirectUploadRequest{
		File:          file,
		FileName:      fileHeader.Filename,
		FileSize:      fileHeader.Size,
		Name:          name,
		Version:       version,
		ModelType:     models.OriginalModelType(modelType),
		Framework:     framework,
		Description:   description,
		Tags:          tags,
		TaskType:      taskType,
		InputWidth:    inputWidth,
		InputHeight:   inputHeight,
		InputChannels: inputChannels,
		UserID:        c.getCurrentUserID(r), // 获取当前用户ID
	}

	// 调用服务层处理上传
	model, err := c.modelService.DirectUpload(r.Context(), req)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "上传模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("模型上传成功", model))
}

// GetModels 获取模型列表
// @Summary 获取模型列表
// @Description 获取模型列表，支持分页和筛选
// @Tags 模型管理
// @Produce json
// @Param user_id query int false "用户ID"
// @Param model_type query string false "模型类型" Enums(pt,onnx,pth)
// @Param status query string false "模型状态"
// @Param tags query []string false "标签列表"
// @Param name query string false "模型名称搜索关键词"
// @Param framework query string false "框架类型" Enums(pytorch,onnx,tensorflow)
// @Param task_type query string false "AI任务类型" Enums(detection,segmentation,classification)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param sort_field query string false "排序字段" default(created_at)
// @Param sort_order query string false "排序方向" Enums(asc,desc) default(desc)
// @Success 200 {object} APIResponse{data=service.GetModelListResponse}
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models [get]
func (c *ModelController) GetModels(w http.ResponseWriter, r *http.Request) {
	req := &service.GetModelListRequest{}

	// 解析查询参数
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			req.UserID = uint(userID)
		}
	}

	if modelType := r.URL.Query().Get("model_type"); modelType != "" {
		req.ModelType = models.OriginalModelType(modelType)
	}

	if status := r.URL.Query().Get("status"); status != "" {
		req.Status = models.OriginalModelStatus(status)
	}

	if tags := r.URL.Query()["tags"]; len(tags) > 0 {
		req.Tags = tags
	}

	// TODO: 需要在service.GetModelListRequest中添加以下字段的支持
	// if name := r.URL.Query().Get("name"); name != "" {
	// 	req.Name = name
	// }

	// if framework := r.URL.Query().Get("framework"); framework != "" {
	// 	req.Framework = framework
	// }

	// if taskType := r.URL.Query().Get("task_type"); taskType != "" {
	// 	req.TaskType = taskType
	// }

	// 分页参数
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 20
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	req.Pagination = &repository.PaginationOptions{
		Page:     page,
		PageSize: pageSize,
	}

	// 排序参数
	sortField := r.URL.Query().Get("sort_field")
	if sortField == "" {
		sortField = "created_at"
	}

	sortOrder := r.URL.Query().Get("sort_order")
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	req.Sort = &repository.SortOptions{
		Field: sortField,
		Order: sortOrder,
	}

	response, err := c.modelService.GetModelList(r.Context(), req)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取模型列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型列表成功", response))
}

// GetModel 获取模型详情
// @Summary 获取模型详情
// @Description 根据模型ID获取详细信息
// @Tags 模型管理
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} APIResponse{data=models.OriginalModel}
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/{id} [get]
func (c *ModelController) GetModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的模型ID", err))
		return
	}

	model, err := c.modelService.GetModelDetail(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取模型详情失败", err))
		return
	}

	if model == nil {
		render.Render(w, r, CreateErrorResponse(http.StatusNotFound, "模型不存在", errors.New("model not found")))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型详情成功", model))
}

// UpdateModel 更新模型信息
// @Summary 更新模型信息
// @Description 更新模型的基本信息，包括AI任务相关参数
// @Tags 模型管理
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Param request body service.UpdateModelRequest true "更新请求"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/{id} [put]
func (c *ModelController) UpdateModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的模型ID", err))
		return
	}

	var req service.UpdateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的请求参数", err))
		return
	}

	if err := c.modelService.UpdateModel(r.Context(), uint(id), &req); err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "更新模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("模型更新成功", nil))
}

// DeleteModel 删除模型
// @Summary 删除模型
// @Description 软删除模型记录
// @Tags 模型管理
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/{id} [delete]
func (c *ModelController) DeleteModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的模型ID", err))
		return
	}

	if err := c.modelService.DeleteModel(r.Context(), uint(id)); err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "删除模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("模型删除成功", nil))
}

// DownloadModel 下载模型文件
// @Summary 下载模型文件
// @Description 下载模型文件，支持大文件和错误恢复
// @Tags 模型管理
// @Produce application/octet-stream
// @Param id path int true "模型ID"
// @Success 200 {file} file
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/{id}/download [get]
func (c *ModelController) DownloadModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的模型ID", err))
		return
	}

	downloadInfo, err := c.modelService.DownloadModel(r.Context(), uint(id))
	if err != nil {
		// 根据错误类型返回不同的HTTP状态码
		if err.Error() == "model not found" {
			render.Render(w, r, CreateErrorResponse(http.StatusNotFound, "模型不存在", err))
		} else if err.Error() == "model file not found" {
			render.Render(w, r, CreateErrorResponse(http.StatusNotFound, "模型文件不存在", err))
		} else {
			render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "下载模型失败", err))
		}
		return
	}

	// 验证文件大小
	if downloadInfo.FileSize <= 0 {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "模型文件无效", errors.New("file size is zero")))
		return
	}

	// 打开文件
	file, err := os.Open(downloadInfo.FilePath)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "打开文件失败", err))
		return
	}
	defer file.Close()

	// 获取文件信息以验证文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取文件信息失败", err))
		return
	}

	// 验证文件大小是否与数据库记录一致
	if fileInfo.Size() != downloadInfo.FileSize {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "文件大小不一致",
			fmt.Errorf("expected %d bytes, got %d bytes", downloadInfo.FileSize, fileInfo.Size())))
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", downloadInfo.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadInfo.FileName))
	w.Header().Set("Content-Length", strconv.FormatInt(downloadInfo.FileSize, 10))
	w.Header().Set("Accept-Ranges", "bytes") // 支持断点续传
	w.Header().Set("Cache-Control", "no-cache")

	// 复制文件内容到响应，使用缓冲区提高性能
	buffer := make([]byte, 32*1024) // 32KB 缓冲区
	written, err := io.CopyBuffer(w, file, buffer)
	if err != nil {
		// 使用日志记录而不是fmt.Printf
		fmt.Printf("[ModelController] Error copying file during download - ModelID: %d, Error: %v", id, err)
		// 此时响应头已经发送，无法返回错误响应
		return
	}

	// 验证传输的字节数
	if written != downloadInfo.FileSize {
		fmt.Printf("[ModelController] Download incomplete - ModelID: %d, Expected: %d bytes, Transferred: %d bytes",
			id, downloadInfo.FileSize, written)
	}
}

// SearchModels 搜索模型
// @Summary 搜索模型
// @Description 根据关键词搜索模型
// @Tags 模型管理
// @Produce json
// @Param keyword query string true "搜索关键词"
// @Param user_id query int false "用户ID"
// @Param model_type query string false "模型类型"
// @Param tags query []string false "标签列表"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} APIResponse{data=service.SearchModelResponse}
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/search [get]
func (c *ModelController) SearchModels(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "搜索关键词不能为空", errors.New("keyword is required")))
		return
	}

	req := &service.SearchModelRequest{
		Keyword: keyword,
	}

	// 解析查询参数
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			req.UserID = uint(userID)
		}
	}

	if modelType := r.URL.Query().Get("model_type"); modelType != "" {
		req.ModelType = models.OriginalModelType(modelType)
	}

	if tags := r.URL.Query()["tags"]; len(tags) > 0 {
		req.Tags = tags
	}

	// 分页参数
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 20
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	req.Pagination = &repository.PaginationOptions{
		Page:     page,
		PageSize: pageSize,
	}

	response, err := c.modelService.SearchModels(r.Context(), req)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "搜索模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("搜索成功", response))
}

// GetModelVersions 获取模型版本列表
// @Summary 获取模型版本列表
// @Description 根据模型名称获取所有版本
// @Tags 模型管理
// @Produce json
// @Param name query string true "模型名称"
// @Success 200 {object} APIResponse{data=[]models.OriginalModel}
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/versions [get]
func (c *ModelController) GetModelVersions(w http.ResponseWriter, r *http.Request) {
	modelName := r.URL.Query().Get("name")
	if modelName == "" {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "模型名称不能为空", errors.New("model name is required")))
		return
	}

	versions, err := c.modelService.GetModelVersions(r.Context(), modelName)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取模型版本失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型版本成功", versions))
}

// ValidateModel 验证模型
// @Summary 验证模型
// @Description 验证模型文件的完整性和格式
// @Tags 模型管理
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /api/models/{id}/validate [post]
func (c *ModelController) ValidateModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的模型ID", err))
		return
	}

	if err := c.modelService.ValidateModel(r.Context(), uint(id)); err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "验证模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("模型验证成功", nil))
}

// GetModelStatistics 获取模型统计信息
// @Summary 获取模型统计信息
// @Description 获取模型的各种统计数据
// @Tags 模型管理
// @Produce json
// @Success 200 {object} APIResponse{data=service.ModelStatisticsResponse}
// @Failure 500 {object} APIResponse
// @Router /api/models/statistics [get]
func (c *ModelController) GetModelStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := c.modelService.GetModelStatistics(r.Context())
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取统计信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取统计信息成功", stats))
}

// GetStorageStatistics 获取存储统计信息
// @Summary 获取存储统计信息
// @Description 获取存储空间使用情况
// @Tags 模型管理
// @Produce json
// @Success 200 {object} APIResponse{data=service.StorageStatisticsResponse}
// @Failure 500 {object} APIResponse
// @Router /api/models/storage/statistics [get]
func (c *ModelController) GetStorageStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := c.modelService.GetStorageStatistics(r.Context())
	if err != nil {
		render.Render(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取存储统计失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取存储统计成功", stats))
}

// getCurrentUserID 获取当前用户ID
func (c *ModelController) getCurrentUserID(r *http.Request) uint {
	// TODO: 从JWT token或session中获取用户ID
	// 这里返回默认值，实际应该从认证中间件中获取
	return 1
}
