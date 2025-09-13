package controllers

import (
	"box-manage-service/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type VideoSourceController struct {
	videoSourceService service.VideoSourceService
}

func NewVideoSourceController(videoSourceService service.VideoSourceService) *VideoSourceController {
	return &VideoSourceController{
		videoSourceService: videoSourceService,
	}
}

// CreateVideoSource 创建视频源
// @Summary 创建视频源
// @Description 创建新的视频源
// @Tags 视频源管理
// @Accept json
// @Produce json
// @Param request body service.CreateVideoSourceRequest true "创建请求"
// @Success 200 {object} APIResponse{data=models.VideoSource} "创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/sources [post]
func (c *VideoSourceController) CreateVideoSource(w http.ResponseWriter, r *http.Request) {
	var req service.CreateVideoSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	videoSource, err := c.videoSourceService.CreateVideoSource(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("创建视频源失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("创建成功", videoSource))
}

// GetVideoSource 获取视频源详情
// @Summary 获取视频源详情
// @Description 根据ID获取视频源详细信息
// @Tags 视频源管理
// @Accept json
// @Produce json
// @Param id path int true "视频源ID"
// @Success 200 {object} APIResponse{data=models.VideoSource} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频源不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/sources/{id} [get]
func (c *VideoSourceController) GetVideoSource(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	videoSource, err := c.videoSourceService.GetVideoSource(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("视频源不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取成功", videoSource))
}

// GetVideoSources 获取视频源列表
// @Summary 获取视频源列表
// @Description 分页获取视频源列表，支持按名称和类型搜索
// @Tags 视频源管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param user_id query int false "用户ID"
// @Param name query string false "视频源名称搜索关键词"
// @Param type query string false "视频源类型" Enums(file,stream,camera)
// @Param status query string false "视频源状态" Enums(active,inactive,error)
// @Success 200 {object} APIResponse{data=PaginatedResponse{list=[]models.VideoSource}} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/sources [get]
func (c *VideoSourceController) GetVideoSources(w http.ResponseWriter, r *http.Request) {
	var req service.GetVideoSourcesRequest

	if page := r.URL.Query().Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			req.Page = p
		}
	}
	if pageSize := r.URL.Query().Get("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			req.PageSize = ps
		}
	}
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		if uid, err := strconv.ParseUint(userID, 10, 32); err == nil {
			uidUint := uint(uid)
			req.UserID = &uidUint
		}
	}
	// TODO: 需要在service.GetVideoSourcesRequest中添加以下字段的支持
	// if name := r.URL.Query().Get("name"); name != "" {
	// 	req.Name = name
	// }
	// if sourceType := r.URL.Query().Get("type"); sourceType != "" {
	// 	req.Type = sourceType
	// }
	// if status := r.URL.Query().Get("status"); status != "" {
	// 	req.Status = status
	// }

	videoSources, total, err := c.videoSourceService.GetVideoSources(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取视频源列表失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse("获取成功", videoSources, total, req.Page, req.PageSize))
}

// UpdateVideoSource 更新视频源
// @Summary 更新视频源
// @Description 更新视频源信息
// @Tags 视频源管理
// @Accept json
// @Produce json
// @Param id path int true "视频源ID"
// @Param request body service.UpdateVideoSourceRequest true "更新请求"
// @Success 200 {object} APIResponse{data=models.VideoSource} "更新成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频源不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/sources/{id} [put]
func (c *VideoSourceController) UpdateVideoSource(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	var req service.UpdateVideoSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	videoSource, err := c.videoSourceService.UpdateVideoSource(r.Context(), uint(id), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("更新视频源失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("更新成功", videoSource))
}

// DeleteVideoSource 删除视频源
// @Summary 删除视频源
// @Description 删除指定的视频源
// @Tags 视频源管理
// @Accept json
// @Produce json
// @Param id path int true "视频源ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频源不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/sources/{id} [delete]
func (c *VideoSourceController) DeleteVideoSource(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	err = c.videoSourceService.DeleteVideoSource(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("删除视频源失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除成功", nil))
}

// TakeScreenshot 视频源截图
// @Summary 视频源截图
// @Description 对指定视频源进行截图，返回base64编码的jpg图片
// @Tags 视频源管理
// @Accept json
// @Produce json
// @Param id path int true "视频源ID"
// @Success 200 {object} APIResponse{data=service.ScreenshotResponse} "截图成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频源不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/sources/{id}/screenshot [post]
func (c *VideoSourceController) TakeScreenshot(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	// 创建简化的截图请求
	req := &service.ScreenshotRequest{
		VideoSourceID: uint(id),
	}

	screenshot, err := c.videoSourceService.TakeScreenshot(r.Context(), req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("截图失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("截图成功", screenshot))
}

// GetMonitoringStatus 获取视频源监控状态
// @Summary 获取视频源监控状态
// @Description 获取视频源监控服务的当前状态
// @Tags 视频源管理
// @Produce json
// @Success 200 {object} APIResponse{data=object}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/video/sources/monitoring/status [get]
func (c *VideoSourceController) GetMonitoringStatus(w http.ResponseWriter, r *http.Request) {
	status := c.videoSourceService.GetMonitoringStatus()
	render.Render(w, r, SuccessResponse("获取监控状态成功", status))
}

// StartMonitoring 启动视频源监控
// @Summary 启动视频源监控
// @Description 手动启动视频源监控服务
// @Tags 视频源管理
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/video/sources/monitoring/start [post]
func (c *VideoSourceController) StartMonitoring(w http.ResponseWriter, r *http.Request) {
	c.videoSourceService.StartMonitoring()
	render.Render(w, r, SuccessResponse("视频源监控启动成功", nil))
}

// StopMonitoring 停止视频源监控
// @Summary 停止视频源监控
// @Description 手动停止视频源监控服务
// @Tags 视频源管理
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/video/sources/monitoring/stop [post]
func (c *VideoSourceController) StopMonitoring(w http.ResponseWriter, r *http.Request) {
	c.videoSourceService.StopMonitoring()
	render.Render(w, r, SuccessResponse("视频源监控停止成功", nil))
}
