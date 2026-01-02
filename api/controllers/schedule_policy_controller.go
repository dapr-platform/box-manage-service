/*
 * @module api/controllers/schedule_policy_controller
 * @description 调度策略管理控制器
 * @architecture 控制层
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow HTTP请求 -> 参数验证 -> 业务处理 -> 响应返回
 * @rules 调度策略的 CRUD 操作和启用/禁用控制
 * @dependencies chi, repository, models
 */

package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"box-manage-service/models"
	"box-manage-service/repository"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// SchedulePolicyController 调度策略控制器
type SchedulePolicyController struct {
	repoManager repository.RepositoryManager
}

// NewSchedulePolicyController 创建调度策略控制器实例
func NewSchedulePolicyController(repoManager repository.RepositoryManager) *SchedulePolicyController {
	return &SchedulePolicyController{
		repoManager: repoManager,
	}
}

// CreateSchedulePolicyRequest 创建调度策略请求
// @Description 创建调度策略请求参数
type CreateSchedulePolicyRequest struct {
	Name              string                       `json:"name" validate:"required"`
	Description       string                       `json:"description"`
	PolicyType        models.SchedulePolicyType    `json:"policy_type" validate:"required"`
	IsEnabled         bool                         `json:"is_enabled"`
	Priority          int                          `json:"priority"`
	TriggerConditions models.TriggerConditions     `json:"trigger_conditions"`
	ExecutionConfig   models.ExecutionConfig       `json:"execution_config"`
	ScheduleRules     models.ScheduleRules         `json:"schedule_rules"`
}

// UpdateSchedulePolicyRequest 更新调度策略请求
// @Description 更新调度策略请求参数
type UpdateSchedulePolicyRequest struct {
	Name              *string                       `json:"name,omitempty"`
	Description       *string                       `json:"description,omitempty"`
	PolicyType        *models.SchedulePolicyType    `json:"policy_type,omitempty"`
	IsEnabled         *bool                         `json:"is_enabled,omitempty"`
	Priority          *int                          `json:"priority,omitempty"`
	TriggerConditions *models.TriggerConditions     `json:"trigger_conditions,omitempty"`
	ExecutionConfig   *models.ExecutionConfig       `json:"execution_config,omitempty"`
	ScheduleRules     *models.ScheduleRules         `json:"schedule_rules,omitempty"`
}

// SchedulePolicyResponse 调度策略响应
// @Description 调度策略数据
type SchedulePolicyResponse struct {
	ID                uint                         `json:"id"`
	Name              string                       `json:"name"`
	Description       string                       `json:"description"`
	PolicyType        models.SchedulePolicyType    `json:"policy_type"`
	IsEnabled         bool                         `json:"is_enabled"`
	Priority          int                          `json:"priority"`
	TriggerConditions models.TriggerConditions     `json:"trigger_conditions"`
	ExecutionConfig   models.ExecutionConfig       `json:"execution_config"`
	ScheduleRules     models.ScheduleRules         `json:"schedule_rules"`
	CreatedAt         string                       `json:"created_at"`
	UpdatedAt         string                       `json:"updated_at"`
}

// SchedulePolicyListResponse 调度策略列表响应
// @Description 调度策略列表响应
type SchedulePolicyListResponse struct {
	Items      []*SchedulePolicyResponse `json:"items"`
	Total      int64                     `json:"total"`
	Page       int                       `json:"page"`
	PageSize   int                       `json:"page_size"`
}

// policyToResponse 转换为响应格式
func policyToResponse(p *models.SchedulePolicy) *SchedulePolicyResponse {
	return &SchedulePolicyResponse{
		ID:                p.ID,
		Name:              p.Name,
		Description:       p.Description,
		PolicyType:        p.PolicyType,
		IsEnabled:         p.IsEnabled,
		Priority:          p.Priority,
		TriggerConditions: p.TriggerConditions,
		ExecutionConfig:   p.ExecutionConfig,
		ScheduleRules:     p.ScheduleRules,
		CreatedAt:         p.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:         p.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// GetSchedulePolicies 获取调度策略列表
// @Summary 获取调度策略列表
// @Description 获取所有调度策略列表，支持分页
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param is_enabled query bool false "是否启用"
// @Param policy_type query string false "策略类型"
// @Success 200 {object} APIResponse{data=SchedulePolicyListResponse} "获取成功"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies [get]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) GetSchedulePolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析分页参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 构建查询条件
	conditions := make(map[string]interface{})
	if enabled := r.URL.Query().Get("is_enabled"); enabled != "" {
		conditions["is_enabled"] = enabled == "true"
	}
	if policyType := r.URL.Query().Get("policy_type"); policyType != "" {
		conditions["policy_type"] = policyType
	}

	// 查询策略
	policies, total, err := c.repoManager.SchedulePolicy().FindWithPagination(ctx, conditions, page, pageSize)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取调度策略列表失败", err))
		return
	}

	// 构建响应
	items := make([]*SchedulePolicyResponse, len(policies))
	for i, p := range policies {
		items[i] = policyToResponse(p)
	}

	response := &SchedulePolicyListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	render.Render(w, r, SuccessResponse("获取调度策略列表成功", response))
}

// GetSchedulePolicy 获取单个调度策略
// @Summary 获取单个调度策略
// @Description 根据ID获取调度策略详情
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param id path int true "策略ID"
// @Success 200 {object} APIResponse{data=SchedulePolicyResponse} "获取成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "策略不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/{id} [get]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) GetSchedulePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的策略ID", err))
		return
	}

	policy, err := c.repoManager.SchedulePolicy().GetByID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("调度策略不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取调度策略成功", policyToResponse(policy)))
}

// CreateSchedulePolicy 创建调度策略
// @Summary 创建调度策略
// @Description 创建新的调度策略
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param request body CreateSchedulePolicyRequest true "创建请求"
// @Success 200 {object} APIResponse{data=SchedulePolicyResponse} "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies [post]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) CreateSchedulePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateSchedulePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("无效的请求数据", err))
		return
	}

	// 创建策略
	policy := &models.SchedulePolicy{
		Name:              req.Name,
		Description:       req.Description,
		PolicyType:        req.PolicyType,
		IsEnabled:         req.IsEnabled,
		Priority:          req.Priority,
		TriggerConditions: req.TriggerConditions,
		ExecutionConfig:   req.ExecutionConfig,
		ScheduleRules:     req.ScheduleRules,
	}

	// 验证策略配置
	if err := policy.Validate(); err != nil {
		render.Render(w, r, BadRequestResponse("策略配置无效", err))
		return
	}

	if err := c.repoManager.SchedulePolicy().Create(ctx, policy); err != nil {
		render.Render(w, r, InternalErrorResponse("创建调度策略失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("创建调度策略成功", policyToResponse(policy)))
}

// UpdateSchedulePolicy 更新调度策略
// @Summary 更新调度策略
// @Description 更新调度策略配置
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param id path int true "策略ID"
// @Param request body UpdateSchedulePolicyRequest true "更新请求"
// @Success 200 {object} APIResponse{data=SchedulePolicyResponse} "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "策略不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/{id} [put]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) UpdateSchedulePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的策略ID", err))
		return
	}

	var req UpdateSchedulePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("无效的请求数据", err))
		return
	}

	policy, err := c.repoManager.SchedulePolicy().GetByID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("调度策略不存在", err))
		return
	}

	// 更新字段
	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Description != nil {
		policy.Description = *req.Description
	}
	if req.PolicyType != nil {
		policy.PolicyType = *req.PolicyType
	}
	if req.IsEnabled != nil {
		policy.IsEnabled = *req.IsEnabled
	}
	if req.Priority != nil {
		policy.Priority = *req.Priority
	}
	if req.TriggerConditions != nil {
		policy.TriggerConditions = *req.TriggerConditions
	}
	if req.ExecutionConfig != nil {
		policy.ExecutionConfig = *req.ExecutionConfig
	}
	if req.ScheduleRules != nil {
		policy.ScheduleRules = *req.ScheduleRules
	}

	// 验证策略配置
	if err := policy.Validate(); err != nil {
		render.Render(w, r, BadRequestResponse("策略配置无效", err))
		return
	}

	if err := c.repoManager.SchedulePolicy().Update(ctx, policy); err != nil {
		render.Render(w, r, InternalErrorResponse("更新调度策略失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("更新调度策略成功", policyToResponse(policy)))
}

// DeleteSchedulePolicy 删除调度策略
// @Summary 删除调度策略
// @Description 删除指定的调度策略
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param id path int true "策略ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/{id} [delete]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) DeleteSchedulePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的策略ID", err))
		return
	}

	if err := c.repoManager.SchedulePolicy().Delete(ctx, uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("删除调度策略失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除调度策略成功", nil))
}

// EnableSchedulePolicy 启用调度策略
// @Summary 启用调度策略
// @Description 启用指定的调度策略
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param id path int true "策略ID"
// @Success 200 {object} APIResponse "启用成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/{id}/enable [post]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) EnableSchedulePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的策略ID", err))
		return
	}

	if err := c.repoManager.SchedulePolicy().Enable(ctx, uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("启用调度策略失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("启用调度策略成功", nil))
}

// DisableSchedulePolicy 禁用调度策略
// @Summary 禁用调度策略
// @Description 禁用指定的调度策略
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param id path int true "策略ID"
// @Success 200 {object} APIResponse "禁用成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/{id}/disable [post]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) DisableSchedulePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的策略ID", err))
		return
	}

	if err := c.repoManager.SchedulePolicy().Disable(ctx, uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("禁用调度策略失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("禁用调度策略成功", nil))
}

// CreateDefaultPolicy 创建默认调度策略
// @Summary 创建默认调度策略
// @Description 创建指定类型的默认调度策略
// @Tags 调度策略
// @Accept json
// @Produce json
// @Param type query string true "策略类型 (priority|load_balance|resource_match)"
// @Success 200 {object} APIResponse{data=SchedulePolicyResponse} "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/defaults [post]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) CreateDefaultPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	policyType := r.URL.Query().Get("type")
	if policyType == "" {
		render.Render(w, r, BadRequestResponse("缺少策略类型参数", nil))
		return
	}

	var schedulePolicyType models.SchedulePolicyType
	switch policyType {
	case "priority":
		schedulePolicyType = models.SchedulePolicyTypePriority
	case "load_balance":
		schedulePolicyType = models.SchedulePolicyTypeLoadBalance
	case "resource_match":
		schedulePolicyType = models.SchedulePolicyTypeResourceMatch
	default:
		render.Render(w, r, BadRequestResponse("无效的策略类型", nil))
		return
	}

	policy := models.DefaultSchedulePolicy(schedulePolicyType)

	if err := c.repoManager.SchedulePolicy().Create(ctx, policy); err != nil {
		render.Render(w, r, InternalErrorResponse("创建默认调度策略失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("创建默认调度策略成功", policyToResponse(policy)))
}

// GetSchedulePolicyStatistics 获取调度策略统计
// @Summary 获取调度策略统计
// @Description 获取调度策略的统计信息
// @Tags 调度策略
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse "获取成功"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /api/v1/schedule-policies/statistics [get]
// @Security ApiKeyAuth
func (c *SchedulePolicyController) GetSchedulePolicyStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := c.repoManager.SchedulePolicy().GetStatistics(ctx)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取统计信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取统计信息成功", stats))
}

// RegisterRoutes 注册路由
func (c *SchedulePolicyController) RegisterRoutes(r chi.Router) {
	r.Route("/schedule-policies", func(r chi.Router) {
		r.Get("/", c.GetSchedulePolicies)
		r.Post("/", c.CreateSchedulePolicy)
		r.Post("/defaults", c.CreateDefaultPolicy)
		r.Get("/statistics", c.GetSchedulePolicyStatistics)
		r.Get("/{id}", c.GetSchedulePolicy)
		r.Put("/{id}", c.UpdateSchedulePolicy)
		r.Delete("/{id}", c.DeleteSchedulePolicy)
		r.Post("/{id}/enable", c.EnableSchedulePolicy)
		r.Post("/{id}/disable", c.DisableSchedulePolicy)
	})
}

// GetEnabledSchedulePolicies 获取启用的调度策略
func (c *SchedulePolicyController) GetEnabledSchedulePolicies(ctx context.Context) ([]*models.SchedulePolicy, error) {
	return c.repoManager.SchedulePolicy().FindEnabled(ctx)
}

