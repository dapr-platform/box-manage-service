/*
 * @module controllers/converted_model_controller
 * @description 转换后模型API控制器，提供转换后模型的HTTP接口
 * @architecture 控制器层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow HTTP请求 -> 参数验证 -> Service调用 -> 响应返回
 * @rules 提供RESTful风格的转换后模型管理接口
 * @dependencies service.ConvertedModelService, go-chi/render
 * @refs REQ-003.md, DESIGN-005.md
 */

package controllers

import (
	"box-manage-service/repository"
	"box-manage-service/service"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ConvertedModelController 转换后模型控制器
type ConvertedModelController struct {
	convertedModelService  service.ConvertedModelService
	modelBoxDeploymentRepo repository.ModelBoxDeploymentRepository
}

// NewConvertedModelController 创建转换后模型控制器
func NewConvertedModelController(
	convertedModelService service.ConvertedModelService,
	modelBoxDeploymentRepo repository.ModelBoxDeploymentRepository,
) *ConvertedModelController {
	return &ConvertedModelController{
		convertedModelService:  convertedModelService,
		modelBoxDeploymentRepo: modelBoxDeploymentRepo,
	}
}

// CreateConvertedModel 创建转换后模型
// @Summary 创建转换后模型
// @Description 创建新的转换后模型记录
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param request body service.CreateConvertedModelRequest true "转换后模型信息"
// @Success 201 {object} APIResponse{data=models.ConvertedModel}
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models [post]
func (c *ConvertedModelController) CreateConvertedModel(w http.ResponseWriter, r *http.Request) {
	log.Printf("[ConvertedModelController] CreateConvertedModel started")

	var req service.CreateConvertedModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ConvertedModelController] Failed to decode request - Error: %v", err)
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	// 设置创建用户
	req.UserID = c.getCurrentUserID(r)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	model, err := c.convertedModelService.CreateConvertedModel(ctx, &req)
	if err != nil {
		log.Printf("[ConvertedModelController] Failed to create converted model - Error: %v", err)
		if strings.Contains(err.Error(), "已存在") {
			render.Render(w, r, ConflictResponse("转换后模型创建失败", err))
		} else {
			render.Render(w, r, InternalErrorResponse("转换后模型创建失败", err))
		}
		return
	}

	log.Printf("[ConvertedModelController] CreateConvertedModel completed - ID: %d", model.ID)
	render.Status(r, http.StatusCreated)
	render.Render(w, r, SuccessResponse("转换后模型创建成功", model))
}

// GetConvertedModels 获取转换后模型列表
// @Summary 获取转换后模型列表
// @Description 分页获取转换后模型列表，支持多条件搜索
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "模型状态" Enums(active,inactive,failed)
// @Param target_chip query string false "目标芯片" Enums(bm1684,bm1684x,bm1688)
// @Param user_id query int false "用户ID"
// @Param original_model_id query int false "原始模型ID"
// @Param tags query string false "标签（逗号分隔）"
// @Param name query string false "模型名称搜索关键词"
// @Param version query string false "版本搜索关键词"
// @Success 200 {object} PaginatedResponse{data=[]models.ConvertedModel}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models [get]
func (c *ConvertedModelController) GetConvertedModels(w http.ResponseWriter, r *http.Request) {
	log.Printf("[ConvertedModelController] GetConvertedModels started")

	// 解析查询参数
	page := parseIntWithDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntWithDefault(r.URL.Query().Get("page_size"), 20)
	status := r.URL.Query().Get("status")
	targetChip := r.URL.Query().Get("target_chip")

	var userID *uint
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	var originalModelID *uint
	if modelIDStr := r.URL.Query().Get("original_model_id"); modelIDStr != "" {
		if mid, err := strconv.ParseUint(modelIDStr, 10, 32); err == nil {
			m := uint(mid)
			originalModelID = &m
		}
	}

	var tags []string
	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	name := r.URL.Query().Get("name")
	version := r.URL.Query().Get("version")

	req := &service.GetConvertedModelListRequest{
		Status:          status,
		TargetChip:      targetChip,
		UserID:          userID,
		OriginalModelID: originalModelID,
		Tags:            tags,
		// TODO: 需要在service.GetConvertedModelListRequest中添加Name和Version字段的支持
		// Name:            name,
		// Version:         version,
		Pagination: &repository.PaginationOptions{
			Page:     page,
			PageSize: pageSize,
		},
	}

	// TODO: 暂时忽略name和version参数，后续需要在service层实现
	_ = name
	_ = version

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := c.convertedModelService.GetConvertedModelList(ctx, req)
	if err != nil {
		log.Printf("[ConvertedModelController] Failed to get converted models - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("获取转换后模型列表失败", err))
		return
	}

	log.Printf("[ConvertedModelController] GetConvertedModels completed - Total: %d", response.Total)
	render.Render(w, r, PaginatedSuccessResponse(
		"获取转换后模型列表成功",
		response.Models,
		response.Total,
		response.Page,
		response.PageSize,
	))
}

// GetConvertedModel 获取转换后模型详情
// @Summary 获取转换后模型详情
// @Description 根据ID获取转换后模型详情
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} APIResponse{data=models.ConvertedModel}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/{id} [get]
func (c *ConvertedModelController) GetConvertedModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的模型ID", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	model, err := c.convertedModelService.GetConvertedModel(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("转换后模型不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换后模型详情成功", model))
}

// UpdateConvertedModel 更新转换后模型
// @Summary 更新转换后模型
// @Description 更新转换后模型信息
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Param request body service.UpdateConvertedModelRequest true "更新信息"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/{id} [put]
func (c *ConvertedModelController) UpdateConvertedModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的模型ID", err))
		return
	}

	var req service.UpdateConvertedModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := c.convertedModelService.UpdateConvertedModel(ctx, uint(id), &req); err != nil {
		render.Render(w, r, InternalErrorResponse("更新转换后模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("更新转换后模型成功", nil))
}

// DeleteConvertedModel 删除转换后模型
// @Summary 删除转换后模型
// @Description 删除指定的转换后模型
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param id path int true "模型ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/{id} [delete]
func (c *ConvertedModelController) DeleteConvertedModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的模型ID", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := c.convertedModelService.DeleteConvertedModel(ctx, uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("删除转换后模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除转换后模型成功", nil))
}

// SearchConvertedModels 搜索转换后模型
// @Summary 搜索转换后模型
// @Description 根据关键词搜索转换后模型
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param keyword query string true "搜索关键词"
// @Param target_chip query string false "目标芯片"
// @Param status query string false "模型状态"
// @Param user_id query int false "用户ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} PaginatedResponse{data=[]models.ConvertedModel}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/search [get]
func (c *ConvertedModelController) SearchConvertedModels(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		render.Render(w, r, BadRequestResponse("搜索关键词不能为空", nil))
		return
	}

	targetChip := r.URL.Query().Get("target_chip")
	status := r.URL.Query().Get("status")
	page := parseIntWithDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntWithDefault(r.URL.Query().Get("page_size"), 20)

	var userID *uint
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	req := &service.SearchConvertedModelRequest{
		Keyword:    keyword,
		TargetChip: targetChip,
		Status:     status,
		UserID:     userID,
		Pagination: &repository.PaginationOptions{
			Page:     page,
			PageSize: pageSize,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := c.convertedModelService.SearchConvertedModels(ctx, req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("搜索转换后模型失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse(
		"搜索转换后模型成功",
		response.Models,
		response.Total,
		response.Page,
		response.PageSize,
	))
}

// DownloadConvertedModel 下载转换后模型
// @Summary 下载转换后模型
// @Description 下载转换后模型文件
// @Tags 转换后模型
// @Accept json
// @Produce application/octet-stream
// @Param id path int true "模型ID"
// @Success 200 {file} binary "模型文件"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/{id}/download [get]
func (c *ConvertedModelController) DownloadConvertedModel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的模型ID", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	downloadInfo, err := c.convertedModelService.DownloadConvertedModel(ctx, uint(id))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			render.Render(w, r, NotFoundResponse("模型文件不存在", err))
		} else {
			render.Render(w, r, InternalErrorResponse("下载转换后模型失败", err))
		}
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", downloadInfo.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadInfo.FileName+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(downloadInfo.FileSize, 10))

	// 流式传输文件
	http.ServeFile(w, r, downloadInfo.FilePath)
}

// GetConvertedModelStatistics 获取转换后模型统计
// @Summary 获取转换后模型统计
// @Description 获取转换后模型的统计信息
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param user_id query int false "用户ID"
// @Success 200 {object} APIResponse{data=service.ConvertedModelStatistics}
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/statistics [get]
func (c *ConvertedModelController) GetConvertedModelStatistics(w http.ResponseWriter, r *http.Request) {
	var userID *uint
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	statistics, err := c.convertedModelService.GetConvertedModelStatistics(ctx, userID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取转换后模型统计失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换后模型统计成功", statistics))
}

// GetConvertedModelsByOriginal 根据原始模型获取转换后模型
// @Summary 根据原始模型获取转换后模型
// @Description 根据原始模型ID获取相关的转换后模型列表
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param originalModelId path int true "原始模型ID"
// @Success 200 {object} APIResponse{data=[]models.ConvertedModel}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/original/{originalModelId} [get]
func (c *ConvertedModelController) GetConvertedModelsByOriginal(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "originalModelId")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的原始模型ID", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	models, err := c.convertedModelService.GetConvertedModelsByOriginal(ctx, uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取转换后模型失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换后模型成功", models))
}

// GetModelDeployments 获取转换后模型的部署信息
// @Summary 获取转换后模型的部署信息
// @Description 根据转换后模型ID获取其在哪些盒子上已部署
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param id path int true "转换后模型ID"
// @Success 200 {object} APIResponse{data=[]models.ModelBoxDeployment}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/{id}/deployments [get]
func (c *ConvertedModelController) GetModelDeployments(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的模型ID", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	deployments, err := c.modelBoxDeploymentRepo.GetByConvertedModelID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取模型部署信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型部署信息成功", deployments))
}

// GetModelDeploymentsByKey 根据模型Key获取部署信息
// @Summary 根据模型Key获取部署信息
// @Description 根据模型Key获取其在哪些盒子上已部署
// @Tags 转换后模型
// @Accept json
// @Produce json
// @Param modelKey path string true "模型Key"
// @Success 200 {object} APIResponse{data=[]models.ModelBoxDeployment}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/converted-models/key/{modelKey}/deployments [get]
func (c *ConvertedModelController) GetModelDeploymentsByKey(w http.ResponseWriter, r *http.Request) {
	modelKey := chi.URLParam(r, "modelKey")
	if modelKey == "" {
		render.Render(w, r, BadRequestResponse("模型Key不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	deployments, err := c.modelBoxDeploymentRepo.GetByModelKey(ctx, modelKey)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取模型部署信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型部署信息成功", deployments))
}

// getCurrentUserID 获取当前用户ID
func (c *ConvertedModelController) getCurrentUserID(r *http.Request) uint {
	// 这里应该从JWT token或session中获取用户ID
	// 为了简化，返回固定值1
	return 1
}
