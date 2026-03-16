package controllers

import (
	"box-manage-service/repository"
	"box-manage-service/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowScheduleInstanceController 调度实例控制器
type WorkflowScheduleInstanceController struct {
	scheduleInstanceService service.WorkflowScheduleInstanceService
}

// NewWorkflowScheduleInstanceController 创建调度实例控制器
func NewWorkflowScheduleInstanceController(scheduleInstanceService service.WorkflowScheduleInstanceService) *WorkflowScheduleInstanceController {
	return &WorkflowScheduleInstanceController{
		scheduleInstanceService: scheduleInstanceService,
	}
}

// GetScheduleInstanceList 获取调度实例列表
// @Summary 获取调度实例列表
// @Description 获取调度实例列表，支持多种过滤条件
// @Tags 工作流api-调度实例
// @Accept json
// @Produce json
// @Param schedule_id query int false "调度ID"
// @Param status query string false "状态"
// @Param trigger_type query string false "触发类型"
// @Param start_date query string false "开始日期"
// @Param end_date query string false "结束日期"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} controllers.PaginatedResponse
// @Router /api/v1/workflow-schedule-instances [get]
func (c *WorkflowScheduleInstanceController) GetScheduleInstanceList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 解析查询参数
	filter := &repository.ScheduleInstanceFilter{
		Page:     1,
		PageSize: 20,
	}

	if scheduleIDStr := r.URL.Query().Get("schedule_id"); scheduleIDStr != "" {
		scheduleID, err := strconv.ParseUint(scheduleIDStr, 10, 32)
		if err != nil {
			render.Render(w, r, BadRequestResponse("invalid schedule_id", err))
			return
		}
		scheduleIDUint := uint(scheduleID)
		filter.ScheduleID = &scheduleIDUint
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = &status
	}

	if triggerType := r.URL.Query().Get("trigger_type"); triggerType != "" {
		filter.TriggerType = &triggerType
	}

	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			render.Render(w, r, BadRequestResponse("invalid start_date format", err))
			return
		}
		filter.StartDate = &startDate
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			render.Render(w, r, BadRequestResponse("invalid end_date format", err))
			return
		}
		filter.EndDate = &endDate
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			render.Render(w, r, BadRequestResponse("invalid page", err))
			return
		}
		filter.Page = page
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 || pageSize > 100 {
			render.Render(w, r, BadRequestResponse("invalid page_size", err))
			return
		}
		filter.PageSize = pageSize
	}

	// 获取列表
	instances, total, err := c.scheduleInstanceService.GetScheduleInstanceList(ctx, filter)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("failed to get schedule instance list", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse("success", instances, total, filter.Page, filter.PageSize))
}

// GetScheduleInstanceDetail 获取调度实例详情
// @Summary 获取调度实例详情
// @Description 获取调度实例详情，包含所有关联的工作流实例
// @Tags 工作流api-调度实例
// @Accept json
// @Produce json
// @Param instance_id path string true "实例ID"
// @Success 200 {object} controllers.APIResponse
// @Router /api/v1/workflow-schedule-instances/{instance_id} [get]
func (c *WorkflowScheduleInstanceController) GetScheduleInstanceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instanceID := chi.URLParam(r, "instance_id")

	if instanceID == "" {
		render.Render(w, r, BadRequestResponse("instance_id is required", nil))
		return
	}

	detail, err := c.scheduleInstanceService.GetScheduleInstanceDetail(ctx, instanceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			render.Render(w, r, NotFoundResponse("schedule instance not found", err))
			return
		}
		render.Render(w, r, InternalErrorResponse("failed to get schedule instance detail", err))
		return
	}

	render.Render(w, r, SuccessResponse("success", detail))
}

// GetScheduleInstanceStatistics 获取调度实例统计
// @Summary 获取调度实例统计
// @Description 获取调度实例统计数据
// @Tags 工作流api-调度实例
// @Accept json
// @Produce json
// @Param schedule_id query int true "调度ID"
// @Param start_date query string false "开始日期"
// @Param end_date query string false "结束日期"
// @Success 200 {object} controllers.APIResponse
// @Router /api/v1/workflow-schedule-instances/statistics [get]
func (c *WorkflowScheduleInstanceController) GetScheduleInstanceStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scheduleIDStr := r.URL.Query().Get("schedule_id")
	if scheduleIDStr == "" {
		render.Render(w, r, BadRequestResponse("schedule_id is required", nil))
		return
	}

	scheduleID, err := strconv.ParseUint(scheduleIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("invalid schedule_id", err))
		return
	}

	var startDate, endDate *time.Time

	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		sd, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			render.Render(w, r, BadRequestResponse("invalid start_date format", err))
			return
		}
		startDate = &sd
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		ed, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			render.Render(w, r, BadRequestResponse("invalid end_date format", err))
			return
		}
		endDate = &ed
	}

	stats, err := c.scheduleInstanceService.GetScheduleInstanceStatistics(ctx, uint(scheduleID), startDate, endDate)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("failed to get statistics", err))
		return
	}

	render.Render(w, r, SuccessResponse("success", stats))
}
