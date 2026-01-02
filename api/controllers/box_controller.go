/*
 * @module api/controllers/box_controller
 * @description AI盒子管理控制器，提供盒子注册、监控、控制等功能
 * @architecture MVC架构 - 控制器层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow HTTP请求处理 -> 业务逻辑处理 -> 数据库操作 -> 响应返回
 * @rules 遵循PostgREST RBAC权限验证，所有操作需要相应权限
 * @dependencies box-manage-service/service
 * @refs DESIGN-001.md
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"box-manage-service/service"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gorm.io/gorm"
)

// BoxController AI盒子管理控制器
type BoxController struct {
	discoveryService  *service.BoxDiscoveryService
	monitoringService *service.BoxMonitoringService
	proxyService      *service.BoxProxyService
	upgradeService    *service.UpgradeService
	taskSyncService   *service.TaskSyncService
	sseService        service.SSEService
}

// NewBoxController 创建盒子控制器实例
func NewBoxController(
	discoveryService *service.BoxDiscoveryService,
	monitoringService *service.BoxMonitoringService,
	proxyService *service.BoxProxyService,
	upgradeService *service.UpgradeService,
	taskSyncService *service.TaskSyncService,
	sseService service.SSEService,
) *BoxController {
	return &BoxController{
		discoveryService:  discoveryService,
		monitoringService: monitoringService,
		proxyService:      proxyService,
		upgradeService:    upgradeService,
		taskSyncService:   taskSyncService,
		sseService:        sseService,
	}
}

// 请求/响应结构体定义

// AddBoxRequest 手动添加盒子请求
type AddBoxRequest struct {
	Name        string   `json:"name" binding:"required" example:"AI-Box-001"`
	IPAddress   string   `json:"ip_address" binding:"required,ip" example:"192.168.1.100"`
	Port        int      `json:"port" binding:"required,min=1,max=65535" example:"9000"`
	Location    string   `json:"location" example:"机房A-机柜01"`
	Description string   `json:"description" example:"边缘AI推理盒子"`
	Tags        []string `json:"tags" example:"gpu,high-performance,edge"`
}

// DiscoverBoxesRequest 自动发现盒子请求
type DiscoverBoxesRequest struct {
	IPRange string `json:"ip_range" binding:"required" example:"192.168.1.1-192.168.1.254"`
	Port    int    `json:"port" binding:"required" example:"9000"`
}

// DiscoverBoxesResponse 发现盒子响应
type DiscoverBoxesResponse struct {
	ScanID    string    `json:"scan_id"`    // 扫描任务ID
	StartTime time.Time `json:"start_time"` // 开始时间
	IPRange   string    `json:"ip_range"`   // 扫描范围
	Port      int       `json:"port"`       // 扫描端口
	Status    string    `json:"status"`     // 扫描状态：started
}

// UpdateBoxRequest 更新盒子请求
type UpdateBoxRequest struct {
	Name        string   `json:"name" example:"AI-Box-001-Updated"`
	Location    string   `json:"location" example:"机房B-机柜02"`
	Description string   `json:"description" example:"更新的盒子描述"`
	Tags        []string `json:"tags" example:"gpu,high-performance,updated"`
}

// BoxResponse 盒子响应结构（直接使用模型，更完整的信息）
type BoxResponse struct {
	*models.Box
	IsOnline     bool          `json:"is_online"`       // 计算字段：是否在线
	CurrentTasks []BoxTaskInfo `json:"current_tasks"`   // 当前运行的任务
	TaskCount    int           `json:"task_count"`      // 任务总数
	CanAccept    bool          `json:"can_accept_task"` // 是否可以接受新任务
}

// BoxTaskInfo 盒子任务信息
type BoxTaskInfo struct {
	TaskID              string `json:"task_id"`
	Status              string `json:"status"`
	DevID               string `json:"dev_id"`
	RTSPUrl             string `json:"rtsp_url"`
	AutoStart           bool   `json:"auto_start"`
	InferenceTasksCount int    `json:"inference_tasks_count"`
}

// 盒子管理API

// AddBox 手动添加盒子
// @Summary 手动添加盒子
// @Description 手动添加AI盒子到系统
// @Tags 盒子管理
// @Accept json
// @Produce json
// @Param request body AddBoxRequest true "添加盒子请求"
// @Success 200 {object} APIResponse{data=BoxResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/boxes [post]
func (c *BoxController) AddBox(w http.ResponseWriter, r *http.Request) {
	log.Printf("[BoxController] AddBox request received from %s", r.RemoteAddr)

	var req AddBoxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[BoxController] Failed to decode AddBox request - Error: %v", err)
		render.Render(w, r, BadRequestResponse("请求参数错误", err))
		return
	}

	// 获取当前用户ID
	createdBy := c.getCurrentUserID(r)
	log.Printf("[BoxController] Processing AddBox request - Name: %s, IP: %s:%d, CreatedBy: %d",
		req.Name, req.IPAddress, req.Port, createdBy)

	// 添加盒子
	box, err := c.discoveryService.ManualAddBox(req.Name, req.IPAddress, req.Port, createdBy)
	if err != nil {
		log.Printf("[BoxController] Failed to add box - Name: %s, IP: %s:%d, Error: %v",
			req.Name, req.IPAddress, req.Port, err)
		render.Render(w, r, BadRequestResponse("添加盒子失败", err))
		return
	}

	// 设置tags（如果提供了）
	if len(req.Tags) > 0 {
		box.SetTags(req.Tags)
		repoManager := c.discoveryService.GetRepoManager()
		if updateErr := repoManager.Box().Update(r.Context(), box); updateErr != nil {
			log.Printf("[BoxController] Failed to update box tags - BoxID: %d, Tags: %v, Error: %v",
				box.ID, req.Tags, updateErr)
			// 这里不阻止响应，只记录错误
		}
	}

	// 转换为响应格式
	response := &BoxResponse{
		Box:          box,
		IsOnline:     box.IsOnline(),
		CurrentTasks: []BoxTaskInfo{}, // 新添加的盒子没有任务
		TaskCount:    0,               // 新添加的盒子任务数为0
		CanAccept:    box.CanAcceptTask(),
	}

	log.Printf("[BoxController] Box added successfully - ID: %d, Name: %s, IP: %s:%d",
		box.ID, box.Name, box.IPAddress, box.Port)
	render.Render(w, r, SuccessResponse("盒子添加成功", response))
}

// GetBoxes 获取盒子列表
// @Summary 获取盒子列表
// @Description 获取AI盒子列表，支持分页和筛选
// @Tags 盒子管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param status query string false "盒子状态" Enums(online,offline,error,upgrading)
// @Param name query string false "盒子名称搜索关键词"
// @Param location query string false "盒子位置搜索关键词"
// @Param tags query string false "标签搜索，多个标签用逗号分隔"
// @Success 200 {object} PaginatedResponse{data=[]BoxResponse}
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/boxes [get]
func (c *BoxController) GetBoxes(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = 10
	}

	// 解析搜索和筛选参数
	status := r.URL.Query().Get("status")
	name := r.URL.Query().Get("name")
	location := r.URL.Query().Get("location")
	tags := r.URL.Query().Get("tags")

	// 构建查询条件
	conditions := make(map[string]interface{})
	if status != "" {
		conditions["status"] = status
	}
	if name != "" {
		conditions["name_like"] = name
	}
	if location != "" {
		conditions["location_like"] = location
	}
	if tags != "" {
		conditions["tags"] = tags
	}

	// 从discovery service中获取repository manager
	repoManager := c.discoveryService.GetRepoManager()
	ctx := r.Context()

	// 查询盒子列表
	boxModels, total, err := repoManager.Box().FindWithPagination(ctx, conditions, page, size)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("查询盒子列表失败", err))
		return
	}

	// 转换为响应格式
	var boxes []BoxResponse
	for _, box := range boxModels {
		// 获取任务数量
		taskCount := 0
		if taskRepo := repoManager.Task(); taskRepo != nil {
			if tasks, err := taskRepo.FindByBoxID(ctx, box.ID); err == nil {
				taskCount = len(tasks)
			}
		}

		boxResponse := BoxResponse{
			Box:          box,
			IsOnline:     box.IsOnline(),
			CurrentTasks: []BoxTaskInfo{}, // 列表页面暂不加载详细任务信息，提升性能
			TaskCount:    taskCount,
			CanAccept:    box.CanAcceptTask(),
		}

		boxes = append(boxes, boxResponse)
	}

	render.Render(w, r, PaginatedSuccessResponse("获取盒子列表成功", boxes, total, page, size))
}

// GetBox 获取盒子详情
// @Summary 获取盒子详情
// @Description 获取指定AI盒子的详细信息
// @Tags 盒子管理
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse{data=BoxResponse}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id} [get]
func (c *BoxController) GetBox(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 从discovery service中获取repository manager
	repoManager := c.discoveryService.GetRepoManager()
	ctx := r.Context()

	// 查询盒子详情
	box, err := repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("盒子不存在", err))
		return
	}

	// 获取任务数量
	taskCount := 0
	repoTaskRepo := repoManager.Task()
	if tasks, err := repoTaskRepo.FindByBoxID(ctx, box.ID); err == nil {
		taskCount = len(tasks)
	}

	// 获取当前运行的任务信息
	var currentTasks []BoxTaskInfo
	if box.Status == models.BoxStatusOnline {
		tasks, err := c.getCurrentTasks(boxID)
		if err != nil {
			log.Printf("获取盒子 %d 当前任务失败: %v", boxID, err)
			currentTasks = []BoxTaskInfo{}
		} else {
			currentTasks = tasks
		}
	} else {
		currentTasks = []BoxTaskInfo{}
	}

	// 转换为响应格式
	response := &BoxResponse{
		Box:          box,
		IsOnline:     box.IsOnline(),
		CurrentTasks: currentTasks,
		TaskCount:    taskCount,
		CanAccept:    box.CanAcceptTask(),
	}

	render.Render(w, r, SuccessResponse("获取盒子详情成功", response))
}

// UpdateBox 更新盒子信息
// @Summary 更新盒子信息
// @Description 更新指定AI盒子的基本信息
// @Tags 盒子管理
// @Accept json
// @Produce json
// @Param id path int true "盒子ID"
// @Param request body UpdateBoxRequest true "更新盒子请求"
// @Success 200 {object} APIResponse{data=BoxResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id} [put]
func (c *BoxController) UpdateBox(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	var req UpdateBoxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数错误", err))
		return
	}

	boxID := uint(id)

	// 从discovery service中获取repository manager
	repoManager := c.discoveryService.GetRepoManager()
	ctx := r.Context()

	// 查询盒子是否存在
	box, err := repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("盒子不存在", err))
		return
	}

	// 更新盒子信息
	if req.Name != "" {
		box.Name = req.Name
	}
	if req.Location != "" {
		box.Location = req.Location
	}
	if req.Description != "" {
		box.Description = req.Description
	}
	if req.Tags != nil {
		box.SetTags(req.Tags)
	}

	// 保存更新
	if err := repoManager.Box().Update(ctx, box); err != nil {
		render.Render(w, r, InternalErrorResponse("更新盒子信息失败", err))
		return
	}

	// 获取任务数量
	taskCount := 0
	if tasks, err := repoManager.Task().FindByBoxID(ctx, box.ID); err == nil {
		taskCount = len(tasks)
	}

	// 转换为响应格式
	response := &BoxResponse{
		Box:          box,
		IsOnline:     box.IsOnline(),
		CurrentTasks: []BoxTaskInfo{}, // 更新操作不加载详细任务信息
		TaskCount:    taskCount,
		CanAccept:    box.CanAcceptTask(),
	}

	render.Render(w, r, SuccessResponse("盒子信息更新成功", response))
}

// DeleteBox 删除盒子
// @Summary 删除盒子
// @Description 删除指定的AI盒子
// @Tags 盒子管理
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id} [delete]
func (c *BoxController) DeleteBox(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 从discovery service中获取repository manager
	repoManager := c.discoveryService.GetRepoManager()
	ctx := r.Context()

	// 查询盒子是否存在
	box, err := repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("盒子不存在", err))
		return
	}

	// 检查盒子是否有正在运行的任务
	taskRepo := repoManager.Task()
	runningTasks, err := taskRepo.FindByBoxID(ctx, boxID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("检查盒子任务状态失败", err))
		return
	}

	// 如果有正在运行的任务，不允许删除
	for _, task := range runningTasks {
		if task.Status == "running" || task.Status == "scheduled" {
			render.Render(w, r, BadRequestResponse("盒子存在正在运行的任务，无法删除", nil))
			return
		}
	}

	// 删除盒子（级联删除相关数据）
	if err := c.cascadeDeleteBox(ctx, repoManager, boxID); err != nil {
		render.Render(w, r, InternalErrorResponse("删除盒子失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("盒子删除成功", map[string]interface{}{
		"deleted_box_id":   boxID,
		"deleted_box_name": box.Name,
	}))
}

// DiscoverBoxes 自动发现盒子
// @Summary 自动发现盒子
// @Description 在指定IP范围内异步自动发现AI盒子，立即返回扫描任务ID，可通过SSE监听进度
// @Tags 盒子管理
// @Accept json
// @Produce json
// @Param request body DiscoverBoxesRequest true "发现盒子请求"
// @Success 200 {object} APIResponse{data=DiscoverBoxesResponse}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/boxes/discover [post]
func (c *BoxController) DiscoverBoxes(w http.ResponseWriter, r *http.Request) {
	log.Printf("[BoxController] DiscoverBoxes request received from %s", r.RemoteAddr)

	var req DiscoverBoxesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[BoxController] Failed to decode DiscoverBoxes request - Error: %v", err)
		render.Render(w, r, BadRequestResponse("请求参数错误", err))
		return
	}

	// 获取当前用户ID
	createdBy := c.getCurrentUserID(r)
	log.Printf("[BoxController] Processing DiscoverBoxes request - IPRange: %s, Port: %d, CreatedBy: %d",
		req.IPRange, req.Port, createdBy)

	// 启动异步扫描
	scanID, err := c.discoveryService.ScanNetworkAsync(req.IPRange, req.Port, createdBy, c.sseService)
	if err != nil {
		log.Printf("[BoxController] Failed to start network scan - IPRange: %s, Port: %d, Error: %v",
			req.IPRange, req.Port, err)
		render.Render(w, r, InternalErrorResponse("启动网络扫描失败", err))
		return
	}

	// 立即返回扫描任务信息
	response := &DiscoverBoxesResponse{
		ScanID:    scanID,
		StartTime: time.Now(),
		IPRange:   req.IPRange,
		Port:      req.Port,
		Status:    "started",
	}

	log.Printf("[BoxController] Network scan started successfully - ScanID: %s, IPRange: %s, Port: %d",
		scanID, req.IPRange, req.Port)
	render.Render(w, r, SuccessResponse("网络扫描已启动，请通过SSE监听进度", response))
}

// GetScanTask 获取扫描任务状态
// @Summary 获取扫描任务状态
// @Description 获取指定扫描任务的详细状态信息
// @Tags 盒子管理
// @Produce json
// @Param scan_id path string true "扫描任务ID"
// @Success 200 {object} APIResponse{data=service.ScanTask}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/discover/{scan_id} [get]
func (c *BoxController) GetScanTask(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scan_id")
	log.Printf("[BoxController] GetScanTask request - ScanID: %s", scanID)

	// 获取扫描任务状态
	scanTask, err := c.discoveryService.GetScanTask(scanID)
	if err != nil {
		log.Printf("[BoxController] Failed to get scan task - ScanID: %s, Error: %v", scanID, err)
		render.Render(w, r, NotFoundResponse("扫描任务不存在", err))
		return
	}

	log.Printf("[BoxController] Scan task retrieved successfully - ScanID: %s, Status: %s", scanID, scanTask.Status)
	render.Render(w, r, SuccessResponse("获取扫描任务状态成功", scanTask))
}

// CancelScanTask 取消扫描任务
// @Summary 取消扫描任务
// @Description 取消正在进行的扫描任务
// @Tags 盒子管理
// @Produce json
// @Param scan_id path string true "扫描任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/discover/{scan_id}/cancel [post]
func (c *BoxController) CancelScanTask(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scan_id")
	log.Printf("[BoxController] CancelScanTask request - ScanID: %s", scanID)

	// 取消扫描任务
	err := c.discoveryService.CancelScanTask(scanID)
	if err != nil {
		log.Printf("[BoxController] Failed to cancel scan task - ScanID: %s, Error: %v", scanID, err)
		render.Render(w, r, BadRequestResponse("取消扫描任务失败", err))
		return
	}

	log.Printf("[BoxController] Scan task cancelled successfully - ScanID: %s", scanID)
	render.Render(w, r, SuccessResponse("扫描任务已取消", map[string]interface{}{
		"scan_id":      scanID,
		"cancelled_at": time.Now(),
	}))
}

// 盒子状态监控API

// GetBoxStatus 获取盒子实时状态
// @Summary 获取盒子实时状态
// @Description 获取指定盒子的实时状态信息
// @Tags 盒子监控
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse{data=interface{}}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/status [get]
func (c *BoxController) GetBoxStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 获取实时状态
	status, err := c.proxyService.GetSystemStatus(boxID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取盒子状态失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取盒子状态成功", status.Data))
}

// RefreshBoxStatus 手动刷新盒子状态
// @Summary 手动刷新盒子状态
// @Description 手动刷新指定盒子的状态信息
// @Tags 盒子监控
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/refresh [post]
func (c *BoxController) RefreshBoxStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 刷新状态
	err = c.monitoringService.RefreshBoxStatus(boxID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("刷新盒子状态失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("盒子状态刷新成功", nil))
}

// GetBoxStatusHistory 获取盒子状态历史
// @Summary 获取盒子状态历史
// @Description 获取指定盒子的历史状态数据
// @Tags 盒子监控
// @Produce json
// @Param id path int true "盒子ID"
// @Param start_time query string false "开始时间" format(date-time)
// @Param end_time query string false "结束时间" format(date-time)
// @Param limit query int false "记录数量限制" default(100)
// @Success 200 {object} APIResponse{data=[]models.BoxHeartbeat}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/status/history [get]
func (c *BoxController) GetBoxStatusHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 解析时间参数
	var startTime, endTime time.Time
	if startStr := r.URL.Query().Get("start_time"); startStr != "" {
		startTime, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr := r.URL.Query().Get("end_time"); endStr != "" {
		endTime, _ = time.Parse(time.RFC3339, endStr)
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	// 获取状态历史
	history, err := c.monitoringService.GetBoxStatusHistory(boxID, startTime, endTime, limit)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取状态历史失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取状态历史成功", history))
}

// GetBoxesOverview 获取盒子概览统计
// @Summary 获取盒子概览统计
// @Description 获取所有盒子的概览统计信息
// @Tags 盒子监控
// @Produce json
// @Success 200 {object} APIResponse{data=interface{}}
// @Router /api/v1/boxes/overview [get]
func (c *BoxController) GetBoxesOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := c.monitoringService.GetSystemOverview()
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取系统概览失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取系统概览成功", overview))
}

// 盒子代理API（转发到盒子的API调用）

// GetBoxModels 获取盒子模型列表
// @Summary 获取盒子模型列表
// @Description 获取指定盒子上的AI模型列表
// @Tags 盒子代理
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse{data=interface{}}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/models [get]
func (c *BoxController) GetBoxModels(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 调用盒子API
	resp, err := c.proxyService.GetModels(boxID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取模型列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取模型列表成功", resp.Models))
}

// GetBoxTasks 获取盒子任务列表
// @Summary 获取盒子任务列表
// @Description 获取指定盒子上的推理任务列表
// @Tags 盒子代理
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse{data=interface{}}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/tasks [get]
func (c *BoxController) GetBoxTasks(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	boxID := uint(id)

	// 调用盒子API
	resp, err := c.proxyService.GetTasks(boxID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取任务列表失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取任务列表成功", resp.Tasks))
}

// 辅助方法

// getCurrentUserID 从请求上下文获取当前用户ID
func (c *BoxController) getCurrentUserID(r *http.Request) uint {
	return getUserIDFromRequest(r)
}

// getCurrentTasks 获取盒子当前任务信息
func (c *BoxController) getCurrentTasks(boxID uint) ([]BoxTaskInfo, error) {
	// 通过proxyService获取盒子的任务列表
	resp, err := c.proxyService.GetTasks(boxID)
	if err != nil {
		return nil, fmt.Errorf("调用盒子API获取任务失败: %w", err)
	}

	var tasks []BoxTaskInfo

	// 直接使用类型化的响应
	if resp != nil && resp.Tasks != nil {
		for _, task := range resp.Tasks {
			taskInfo := BoxTaskInfo{
				TaskID:              task.TaskID,
				Status:              task.Status,
				DevID:               "", // apis.md中的tasks列表响应没有dev_id字段
				RTSPUrl:             task.RTSPUrl,
				AutoStart:           task.AutoStart,
				InferenceTasksCount: task.InferenceTasksCount,
			}
			tasks = append(tasks, taskInfo)
		}
	}

	return tasks, nil
}

// 辅助函数：从map中安全获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// 辅助函数：从map中安全获取布尔值
func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// 辅助函数：从map中安全获取整数值
func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return int(f)
		}
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

// cascadeDeleteBox 级联删除盒子及其相关数据
func (c *BoxController) cascadeDeleteBox(ctx context.Context, repoManager repository.RepositoryManager, boxID uint) error {
	// 在事务中执行所有删除操作
	return repoManager.Transaction(ctx, func(tx *gorm.DB) error {

		// 1. 删除盒子心跳记录
		if err := tx.Where("box_id = ?", boxID).Delete(&models.BoxHeartbeat{}).Error; err != nil {
			return fmt.Errorf("删除盒子心跳记录失败: %w", err)
		}

		// 2. 删除盒子任务（将任务的box_id设为null，不删除任务本身）
		if err := tx.Model(&models.Task{}).Where("box_id = ?", boxID).Update("box_id", nil).Error; err != nil {
			return fmt.Errorf("清理盒子任务关联失败: %w", err)
		}

		// 3. 删除任务执行记录
		if err := tx.Where("box_id = ?", boxID).Delete(&models.TaskExecution{}).Error; err != nil {
			return fmt.Errorf("删除任务执行记录失败: %w", err)
		}

		// 4. 删除升级任务
		if err := tx.Where("box_id = ?", boxID).Delete(&models.UpgradeTask{}).Error; err != nil {
			return fmt.Errorf("删除升级任务失败: %w", err)
		}

		// 5. 删除模型部署记录（将box_id设为null）
		if err := tx.Model(&models.BoxModel{}).Where("box_id = ?", boxID).Update("box_id", nil).Error; err != nil {
			return fmt.Errorf("清理模型部署记录失败: %w", err)
		}

		// 6. 最后删除盒子本身
		if err := tx.Unscoped().Delete(&models.Box{}, boxID).Error; err != nil {
			return fmt.Errorf("删除盒子失败: %w", err)
		}

		return nil
	})
}

// SyncBoxTasks 同步盒子任务到管理端
// @Summary 同步盒子任务
// @Description 一键同步指定盒子的所有任务到管理端，操作是幂等的
// @Tags 盒子管理
// @Accept json
// @Produce json
// @Param id path int true "盒子ID"
// @Success 200 {object} APIResponse{data=service.TaskSyncResult} "同步成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "盒子不存在"
// @Failure 500 {object} ErrorResponse "内部服务器错误"
// @Router /api/v1/boxes/{id}/sync-tasks [post]
func (c *BoxController) SyncBoxTasks(w http.ResponseWriter, r *http.Request) {
	boxIDStr := chi.URLParam(r, "id")
	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	ctx := r.Context()

	// 执行任务同步，内部会检查盒子是否存在
	result, err := c.taskSyncService.SyncBoxTasks(ctx, uint(boxID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("同步任务失败", err))
		return
	}

	// 设置同步时间
	result.SyncTime = time.Now()

	render.Render(w, r, SuccessResponse(
		fmt.Sprintf("盒子 %s 任务同步完成：共 %d 个任务，新建 %d 个，更新 %d 个，跳过 %d 个，失败 %d 个",
			result.BoxName, result.TotalTasks, result.SyncedTasks, result.UpdatedTasks, result.SkippedTasks, result.ErrorTasks),
		result,
	))
}
