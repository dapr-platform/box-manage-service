package controllers

import (
	"box-manage-service/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type RecordTaskController struct {
	recordTaskService service.RecordTaskService
}

func NewRecordTaskController(recordTaskService service.RecordTaskService) *RecordTaskController {
	return &RecordTaskController{
		recordTaskService: recordTaskService,
	}
}

// CreateRecordTask 创建录制任务
// @Summary 创建录制任务
// @Description 创建视频录制任务，仅支持实时视频流录制
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param request body service.CreateRecordTaskRequest true "录制任务创建请求"
// @Success 200 {object} APIResponse{data=models.RecordTask} "创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks [post]
func (c *RecordTaskController) CreateRecordTask(w http.ResponseWriter, r *http.Request) {
	var req service.CreateRecordTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	task, err := c.recordTaskService.CreateRecordTask(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("创建录制任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("创建成功", task))
}

// GetRecordTask 获取录制任务详情
// @Summary 获取录制任务详情
// @Description 根据ID获取录制任务的详细信息
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse{data=models.RecordTask} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks/{id} [get]
func (c *RecordTaskController) GetRecordTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	task, err := c.recordTaskService.GetRecordTask(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取录制任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取成功", task))
}

// GetRecordTasks 获取录制任务列表
// @Summary 获取录制任务列表
// @Description 分页获取录制任务列表，支持状态、视频源等条件筛选
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "任务状态" Enums(pending,recording,completed,stopped,failed)
// @Param video_source_id query int false "视频源ID"
// @Param user_id query int false "用户ID"
// @Param name query string false "任务名称搜索关键词"
// @Param keyword query string false "任务ID或描述搜索关键词"
// @Success 200 {object} PaginatedResponse{data=[]models.RecordTask} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks [get]
func (c *RecordTaskController) GetRecordTasks(w http.ResponseWriter, r *http.Request) {
	var req service.GetRecordTasksRequest

	// 解析分页参数
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			req.Page = page
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			req.PageSize = pageSize
		}
	}

	// 解析筛选条件
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		req.Status = &statusStr
	}
	if videoSourceIDStr := r.URL.Query().Get("video_source_id"); videoSourceIDStr != "" {
		if videoSourceID, err := strconv.ParseUint(videoSourceIDStr, 10, 32); err == nil {
			id := uint(videoSourceID)
			req.VideoSourceID = &id
		}
	}
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			id := uint(userID)
			req.UserID = &id
		}
	}
	// TODO: 需要在service.GetRecordTasksRequest中添加Name和Keyword字段的支持
	// if name := r.URL.Query().Get("name"); name != "" {
	// 	req.Name = name
	// }
	// if keyword := r.URL.Query().Get("keyword"); keyword != "" {
	// 	req.Keyword = keyword
	// }

	tasks, total, err := c.recordTaskService.GetRecordTasks(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取录制任务列表失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse("获取成功", tasks, total, req.Page, req.PageSize))
}

// StartRecordTask 启动录制任务
// @Summary 启动录制任务
// @Description 启动指定的录制任务
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse "启动成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks/{id}/start [post]
func (c *RecordTaskController) StartRecordTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	if err := c.recordTaskService.StartRecordTask(r.Context(), uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("启动录制任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("启动成功", nil))
}

// StopRecordTask 停止录制任务
// @Summary 停止录制任务
// @Description 停止指定的录制任务
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse "停止成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks/{id}/stop [post]
func (c *RecordTaskController) StopRecordTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	if err := c.recordTaskService.StopRecordTask(r.Context(), uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("停止录制任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("停止成功", nil))
}

// DeleteRecordTask 删除录制任务
// @Summary 删除录制任务
// @Description 删除指定的录制任务及其相关数据
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks/{id} [delete]
func (c *RecordTaskController) DeleteRecordTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	if err := c.recordTaskService.DeleteRecordTask(r.Context(), uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("删除录制任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除成功", nil))
}

// DownloadRecord 下载录制文件
// @Summary 下载录制文件
// @Description 下载指定录制任务的录制文件
// @Tags 录制任务管理
// @Accept json
// @Produce application/octet-stream
// @Param id path int true "任务ID"
// @Success 200 {file} binary "录制文件"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在或文件不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks/{id}/download [get]
func (c *RecordTaskController) DownloadRecord(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	downloadInfo, err := c.recordTaskService.DownloadRecord(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取下载信息失败", err))
		return
	}

	// 设置响应头
	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadInfo.FileName+"\"")
	w.Header().Set("Content-Type", downloadInfo.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(downloadInfo.FileSize, 10))

	// 发送文件
	http.ServeFile(w, r, downloadInfo.FilePath)
}

// GetTaskStatistics 获取录制任务统计信息
// @Summary 获取录制任务统计信息
// @Description 获取录制任务的统计信息，包括各状态的任务数量
// @Tags 录制任务管理
// @Accept json
// @Produce json
// @Param user_id query int false "用户ID，不指定则统计所有用户"
// @Success 200 {object} APIResponse{data=service.RecordTaskStatistics} "获取成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/record-tasks/statistics [get]
func (c *RecordTaskController) GetTaskStatistics(w http.ResponseWriter, r *http.Request) {
	var userID *uint
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			id := uint(uid)
			userID = &id
		}
	}

	stats, err := c.recordTaskService.GetTaskStatistics(r.Context(), userID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取统计信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取成功", stats))
}
