package controllers

import (
	"box-manage-service/service"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type ExtractTaskController struct {
	extractTaskService service.ExtractTaskService
}

func NewExtractTaskController(extractTaskService service.ExtractTaskService) *ExtractTaskController {
	return &ExtractTaskController{
		extractTaskService: extractTaskService,
	}
}

// CreateExtractTask 创建抽帧任务
// @Summary 创建抽帧任务
// @Description 创建视频抽帧任务，支持从视频源或视频文件抽帧
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param request body service.CreateExtractTaskRequest true "抽帧任务创建请求"
// @Success 200 {object} APIResponse{data=models.ExtractTask} "创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks [post]
func (c *ExtractTaskController) CreateExtractTask(w http.ResponseWriter, r *http.Request) {
	var req service.CreateExtractTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	task, err := c.extractTaskService.CreateExtractTask(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("创建抽帧任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("创建成功", task))
}

// GetExtractTask 获取抽帧任务详情
// @Summary 获取抽帧任务详情
// @Description 根据ID获取抽帧任务的详细信息
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse{data=models.ExtractTask} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks/{id} [get]
func (c *ExtractTaskController) GetExtractTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	task, err := c.extractTaskService.GetExtractTask(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取抽帧任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取成功", task))
}

// GetExtractTasks 获取抽帧任务列表
// @Summary 获取抽帧任务列表
// @Description 分页获取抽帧任务列表，支持状态、数据源等条件筛选
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "任务状态" Enums(pending,extracting,completed,stopped,failed)
// @Param video_source_id query int false "视频源ID"
// @Param video_file_id query int false "视频文件ID"
// @Param user_id query int false "用户ID"
// @Param name query string false "任务名称搜索关键词"
// @Param keyword query string false "任务ID或描述搜索关键词"
// @Success 200 {object} PaginatedResponse{data=[]models.ExtractTask} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks [get]
func (c *ExtractTaskController) GetExtractTasks(w http.ResponseWriter, r *http.Request) {
	var req service.GetExtractTasksRequest

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
	if videoFileIDStr := r.URL.Query().Get("video_file_id"); videoFileIDStr != "" {
		if videoFileID, err := strconv.ParseUint(videoFileIDStr, 10, 32); err == nil {
			id := uint(videoFileID)
			req.VideoFileID = &id
		}
	}
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			id := uint(userID)
			req.UserID = &id
		}
	}
	// TODO: 需要在service.GetExtractTasksRequest中添加Name和Keyword字段的支持
	// if name := r.URL.Query().Get("name"); name != "" {
	// 	req.Name = name
	// }
	// if keyword := r.URL.Query().Get("keyword"); keyword != "" {
	// 	req.Keyword = keyword
	// }

	tasks, total, err := c.extractTaskService.GetExtractTasks(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取抽帧任务列表失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse("获取成功", tasks, total, req.Page, req.PageSize))
}

// StartExtractTask 启动抽帧任务
// @Summary 启动抽帧任务
// @Description 启动指定的抽帧任务
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse "启动成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks/{id}/start [post]
func (c *ExtractTaskController) StartExtractTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	if err := c.extractTaskService.StartExtractTask(r.Context(), uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("启动抽帧任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("启动成功", nil))
}

// StopExtractTask 停止抽帧任务
// @Summary 停止抽帧任务
// @Description 停止指定的抽帧任务
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse "停止成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks/{id}/stop [post]
func (c *ExtractTaskController) StopExtractTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	if err := c.extractTaskService.StopExtractTask(r.Context(), uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("停止抽帧任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("停止成功", nil))
}

// DeleteExtractTask 删除抽帧任务
// @Summary 删除抽帧任务
// @Description 删除指定的抽帧任务及其相关数据
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks/{id} [delete]
func (c *ExtractTaskController) DeleteExtractTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	if err := c.extractTaskService.DeleteExtractTask(r.Context(), uint(id)); err != nil {
		render.Render(w, r, InternalErrorResponse("删除抽帧任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除成功", nil))
}

// GetTaskFrames 获取任务的抽帧结果
// @Summary 获取任务的抽帧结果
// @Description 获取指定任务的抽帧结果列表
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} PaginatedResponse{data=[]models.ExtractFrame} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks/{id}/frames [get]
func (c *ExtractTaskController) GetTaskFrames(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	// 先获取任务信息，验证任务是否存在
	task, err := c.extractTaskService.GetExtractTask(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取抽帧任务失败", err))
		return
	}

	// 解析分页参数
	page := 1
	pageSize := 20
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	frames, total, err := c.extractTaskService.GetTaskFrames(r.Context(), task.TaskID, page, pageSize)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取抽帧结果失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse("获取成功", frames, total, page, pageSize))
}

// GetFrameImageInfo 获取抽帧图片信息
// @Summary 获取抽帧图片信息
// @Description 获取指定抽帧的图片信息（不返回图片内容）
// @Tags 抽帧任务管理
// @Accept json
// @Produce json
// @Param frame_id path int true "抽帧ID"
// @Success 200 {object} APIResponse{data=service.FrameImageInfo} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "抽帧不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-frames/{frame_id}/info [get]
func (c *ExtractTaskController) GetFrameImageInfo(w http.ResponseWriter, r *http.Request) {
	frameIDStr := chi.URLParam(r, "frame_id")
	frameID, err := strconv.ParseUint(frameIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的抽帧ID", err))
		return
	}

	frameInfo, err := c.extractTaskService.GetFrameImage(r.Context(), uint(frameID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取抽帧图片信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取成功", frameInfo))
}

// GetFrameImage 获取抽帧图片文件
// @Summary 获取抽帧图片文件
// @Description 直接返回指定抽帧的图片文件内容
// @Tags 抽帧任务管理
// @Accept json
// @Produce image/*
// @Param frame_id path int true "抽帧ID"
// @Success 200 {file} binary "图片文件"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "抽帧不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-frames/{frame_id}/image [get]
func (c *ExtractTaskController) GetFrameImage(w http.ResponseWriter, r *http.Request) {
	frameIDStr := chi.URLParam(r, "frame_id")
	frameID, err := strconv.ParseUint(frameIDStr, 10, 32)
	if err != nil {
		log.Printf("[ERROR] Invalid frame ID parameter: %s, error: %v", frameIDStr, err)
		render.Render(w, r, BadRequestResponse("无效的抽帧ID", err))
		return
	}

	log.Printf("[DEBUG] GetFrameImage called for frame ID: %d", frameID)

	// 获取抽帧图片信息
	frameInfo, err := c.extractTaskService.GetFrameImage(r.Context(), uint(frameID))
	if err != nil {
		log.Printf("[ERROR] Failed to get frame image info for frame %d: %v", frameID, err)
		render.Render(w, r, InternalErrorResponse("获取抽帧图片失败", err))
		return
	}

	log.Printf("[DEBUG] Frame image info retrieved - FrameID: %d, FilePath: %s, ContentType: %s, FileSize: %d",
		frameID, frameInfo.FilePath, frameInfo.ContentType, frameInfo.FileSize)

	// 打开文件
	file, err := os.Open(frameInfo.FilePath)
	if err != nil {
		log.Printf("[ERROR] Failed to open frame file %s: %v", frameInfo.FilePath, err)
		render.Render(w, r, InternalErrorResponse("打开图片文件失败", err))
		return
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("[ERROR] Failed to get file info for %s: %v", frameInfo.FilePath, err)
		render.Render(w, r, InternalErrorResponse("获取文件信息失败", err))
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", frameInfo.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Content-Disposition", "inline; filename=\""+frameInfo.FileName+"\"")
	w.Header().Set("Cache-Control", "public, max-age=86400") // 缓存24小时

	log.Printf("[DEBUG] Starting to serve frame image - FrameID: %d, FileName: %s, Size: %d bytes",
		frameID, frameInfo.FileName, fileInfo.Size())

	// 返回文件内容
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("[ERROR] Failed to copy frame file content for frame %d: %v", frameID, err)
		// 此时已经开始写入响应，无法返回JSON错误
		return
	}

	log.Printf("[DEBUG] Frame image served successfully - FrameID: %d, BytesServed: %d", frameID, fileInfo.Size())
}

// DownloadTaskImages 下载任务的所有抽帧图片
// @Summary 下载任务的所有抽帧图片
// @Description 将指定抽帧任务的所有图片压缩为zip包下载
// @Tags 抽帧任务管理
// @Accept json
// @Produce application/zip
// @Param id path int true "任务ID"
// @Success 200 {file} binary "压缩包文件"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/extract-tasks/{id}/download [get]
func (c *ExtractTaskController) DownloadTaskImages(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		log.Printf("[ERROR] Invalid task ID parameter: %s, error: %v", idStr, err)
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	log.Printf("[DEBUG] DownloadTaskImages called for task ID: %d", taskID)

	// 获取任务图片下载信息
	downloadInfo, err := c.extractTaskService.DownloadTaskImages(r.Context(), uint(taskID))
	if err != nil {
		log.Printf("[ERROR] Failed to prepare task images download for task %d: %v", taskID, err)
		render.Render(w, r, InternalErrorResponse("准备任务图片下载失败", err))
		return
	}

	log.Printf("[DEBUG] Task images download info prepared - TaskID: %d, ZipFile: %s, ZipSize: %d bytes, ImageCount: %d",
		taskID, downloadInfo.ZipFileName, downloadInfo.ZipFileSize, downloadInfo.ImageCount)

	// 打开zip文件
	zipFile, err := os.Open(downloadInfo.ZipFilePath)
	if err != nil {
		log.Printf("[ERROR] Failed to open zip file %s: %v", downloadInfo.ZipFilePath, err)
		render.Render(w, r, InternalErrorResponse("打开压缩文件失败", err))
		return
	}
	defer func() {
		zipFile.Close()
		// 下载完成后删除临时zip文件
		go func() {
			if removeErr := os.Remove(downloadInfo.ZipFilePath); removeErr != nil {
				log.Printf("[WARNING] Failed to remove temporary zip file %s: %v", downloadInfo.ZipFilePath, removeErr)
			} else {
				log.Printf("[DEBUG] Temporary zip file removed: %s", downloadInfo.ZipFilePath)
			}
		}()
	}()

	// 获取文件信息
	fileInfo, err := zipFile.Stat()
	if err != nil {
		log.Printf("[ERROR] Failed to get zip file info for %s: %v", downloadInfo.ZipFilePath, err)
		render.Render(w, r, InternalErrorResponse("获取压缩文件信息失败", err))
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", downloadInfo.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadInfo.ZipFileName+"\"")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	log.Printf("[DEBUG] Starting to serve task images zip - TaskID: %d, ZipFileName: %s, Size: %d bytes, ImageCount: %d",
		taskID, downloadInfo.ZipFileName, fileInfo.Size(), downloadInfo.ImageCount)

	// 返回文件内容
	bytesWritten, err := io.Copy(w, zipFile)
	if err != nil {
		log.Printf("[ERROR] Failed to copy zip file content for task %d: %v", taskID, err)
		// 此时已经开始写入响应，无法返回JSON错误
		return
	}

	log.Printf("[DEBUG] Task images zip served successfully - TaskID: %d, BytesServed: %d, ImageCount: %d",
		taskID, bytesWritten, downloadInfo.ImageCount)
}
