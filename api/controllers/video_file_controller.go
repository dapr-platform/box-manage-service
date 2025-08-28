package controllers

import (
	"box-manage-service/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type VideoFileController struct {
	videoFileService service.VideoFileService
}

func NewVideoFileController(videoFileService service.VideoFileService) *VideoFileController {
	return &VideoFileController{
		videoFileService: videoFileService,
	}
}

// UploadVideoFile 上传视频文件
// @Summary 上传视频文件
// @Description 上传视频文件到系统
// @Tags 视频文件管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "视频文件"
// @Param name formData string true "文件名称"
// @Param description formData string false "文件描述"
// @Param user_id formData int true "用户ID"
// @Success 200 {object} APIResponse{data=models.VideoFile} "上传成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files/upload [post]
func (c *VideoFileController) UploadVideoFile(w http.ResponseWriter, r *http.Request) {
	// 解析multipart form
	err := r.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		render.Render(w, r, BadRequestResponse("文件过大", err))
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		render.Render(w, r, BadRequestResponse("获取文件失败", err))
		return
	}
	defer file.Close()

	// 构造上传请求
	req := &service.UploadVideoFileRequest{
		File:        file,
		FileHeader:  handler,
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}

	if userIDStr := r.FormValue("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			req.UserID = uint(userID)
		}
	}

	videoFile, err := c.videoFileService.UploadVideoFile(r.Context(), req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("上传失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("上传成功", videoFile))
}

// GetVideoFiles 获取视频文件列表
// @Summary 获取视频文件列表
// @Description 分页获取视频文件列表，支持多条件搜索
// @Tags 视频文件管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query string false "文件状态" Enums(uploading,processing,ready,failed)
// @Param user_id query int false "用户ID"
// @Param name query string false "文件名称搜索关键词"
// @Param format query string false "文件格式筛选" Enums(mp4,avi,mov,mkv)
// @Param keyword query string false "关键词搜索（搜索文件名、描述等）"
// @Success 200 {object} APIResponse{data=PaginatedResponse{list=[]models.VideoFile}} "获取成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files [get]
func (c *VideoFileController) GetVideoFiles(w http.ResponseWriter, r *http.Request) {
	var req service.GetVideoFilesRequest

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
	if status := r.URL.Query().Get("status"); status != "" {
		req.Status = &status
	}
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		if uid, err := strconv.ParseUint(userID, 10, 32); err == nil {
			uidUint := uint(uid)
			req.UserID = &uidUint
		}
	}
	// TODO: 需要在service.GetVideoFilesRequest中添加Name、Format和Keyword字段的支持
	// if name := r.URL.Query().Get("name"); name != "" {
	// 	req.Name = name
	// }
	// if format := r.URL.Query().Get("format"); format != "" {
	// 	req.Format = format
	// }
	// if keyword := r.URL.Query().Get("keyword"); keyword != "" {
	// 	req.Keyword = keyword
	// }

	videoFiles, total, err := c.videoFileService.GetVideoFiles(r.Context(), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取视频文件列表失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse("获取成功", videoFiles, total, req.Page, req.PageSize))
}

// GetVideoFile 获取视频文件详情
// @Summary 获取视频文件详情
// @Description 根据ID获取视频文件详细信息
// @Tags 视频文件管理
// @Accept json
// @Produce json
// @Param id path int true "视频文件ID"
// @Success 200 {object} APIResponse{data=models.VideoFile} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频文件不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files/{id} [get]
func (c *VideoFileController) GetVideoFile(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	videoFile, err := c.videoFileService.GetVideoFile(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("视频文件不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取成功", videoFile))
}

// UpdateVideoFile 更新视频文件
// @Summary 更新视频文件信息
// @Description 更新视频文件的基本信息
// @Tags 视频文件管理
// @Accept json
// @Produce json
// @Param id path int true "视频文件ID"
// @Param request body service.UpdateVideoFileRequest true "更新请求"
// @Success 200 {object} APIResponse{data=models.VideoFile} "更新成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频文件不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files/{id} [put]
func (c *VideoFileController) UpdateVideoFile(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	var req service.UpdateVideoFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	videoFile, err := c.videoFileService.UpdateVideoFile(r.Context(), uint(id), &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("更新视频文件失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("更新成功", videoFile))
}

// DeleteVideoFile 删除视频文件
// @Summary 删除视频文件
// @Description 删除指定的视频文件
// @Tags 视频文件管理
// @Accept json
// @Produce json
// @Param id path int true "视频文件ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频文件不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files/{id} [delete]
func (c *VideoFileController) DeleteVideoFile(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	err = c.videoFileService.DeleteVideoFile(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("删除视频文件失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除成功", nil))
}

// DownloadVideoFile 下载视频文件
// @Summary 下载视频文件
// @Description 下载指定的视频文件
// @Tags 视频文件管理
// @Produce application/octet-stream
// @Param id path int true "视频文件ID"
// @Success 200 {file} binary "视频文件"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频文件不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files/{id}/download [get]
func (c *VideoFileController) DownloadVideoFile(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	downloadInfo, err := c.videoFileService.DownloadVideoFile(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("文件不存在", err))
		return
	}

	w.Header().Set("Content-Type", downloadInfo.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadInfo.FileName+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(downloadInfo.FileSize, 10))

	http.ServeFile(w, r, downloadInfo.FilePath)
}

// ConvertVideoFile 转换视频文件
// @Summary 转换视频文件
// @Description 将视频文件转换为MP4格式
// @Tags 视频文件管理
// @Accept json
// @Produce json
// @Param id path int true "视频文件ID"
// @Success 200 {object} APIResponse "转换启动成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "视频文件不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/video/files/{id}/convert [post]
func (c *VideoFileController) ConvertVideoFile(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("参数错误", err))
		return
	}

	err = c.videoFileService.ConvertVideoFile(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("转换启动失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("转换启动成功", nil))
}
