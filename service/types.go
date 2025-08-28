package service

import (
	"box-manage-service/models"
	"mime/multipart"
)

// DownloadInfo 下载信息
type DownloadInfo struct {
	FilePath    string
	FileName    string
	ContentType string
	FileSize    int64
}

// FrameImageInfo 抽帧图片信息
type FrameImageInfo struct {
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	Timestamp   string `json:"timestamp"`
}

// TaskImagesDownloadInfo 任务图片下载信息
type TaskImagesDownloadInfo struct {
	ZipFilePath string `json:"zip_file_path"`
	ZipFileName string `json:"zip_file_name"`
	ZipFileSize int64  `json:"zip_file_size"`
	ImageCount  int    `json:"image_count"`
	TaskID      string `json:"task_id"`
	ContentType string `json:"content_type"`
}

// AddStreamProxyRequest 添加流代理请求
type AddStreamProxyRequest struct {
	App    string `json:"app"`
	Stream string `json:"stream"`
	URL    string `json:"url"`
}

// ZLMStartRecordRequest ZLM开始录制请求
type ZLMStartRecordRequest struct {
	App        string `json:"app"`
	Stream     string `json:"stream"`
	OutputPath string `json:"output_path"`
	MaxTime    int    `json:"max_time"`
}

// ZLMStopRecordRequest ZLM停止录制请求
type ZLMStopRecordRequest struct {
	TaskID string `json:"task_id"`
}

// StreamInfo 流信息
type StreamInfo struct {
	App    string `json:"app"`
	Stream string `json:"stream"`
	Online bool   `json:"online"`
}

// FFmpegExtractRequest FFmpeg抽帧请求
type FFmpegExtractRequest struct {
	InputURL  string `json:"input_url"`
	OutputDir string `json:"output_dir"`
	Count     int    `json:"count"`
	Duration  int    `json:"duration"`
	Format    string `json:"format"`
	IsStream  bool   `json:"is_stream"`
}

// FFmpegExtractResult FFmpeg抽帧结果
type FFmpegExtractResult struct {
	FramePaths []string `json:"frame_paths"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
}

// ConvertVideoRequest 视频转换请求
type ConvertVideoRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
	Quality    string `json:"quality"`
}

// VideoSource相关请求
type CreateVideoSourceRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Type        models.VideoSourceType `json:"type" binding:"required"`
	URL         string                 `json:"url"` // 流类型必需，文件类型可选
	Username    string                 `json:"username"`
	Password    string                 `json:"password"`
	StreamID    string                 `json:"stream_id"`
	VideoFileID *uint                  `json:"video_file_id"` // 文件类型视频源关联的文件ID
	UserID      uint                   `json:"user_id" binding:"required"`
}

type UpdateVideoSourceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	URL         *string `json:"url"`
	Username    *string `json:"username"`
	Password    *string `json:"password"`
	StreamID    *string `json:"stream_id"`
	VideoFileID **uint  `json:"video_file_id"` // 指向指针的指针，支持设置为null
}

type GetVideoSourcesRequest struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	UserID   *uint `json:"user_id"`
}

// VideoFile相关请求
type UploadVideoFileRequest struct {
	File        multipart.File        `json:"-"`
	FileHeader  *multipart.FileHeader `json:"-"`
	Name        string                `json:"name" binding:"required"`
	Description string                `json:"description"`
	UserID      uint                  `json:"user_id" binding:"required"`
}

type GetVideoFilesRequest struct {
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Status   *string `json:"status"`
	UserID   *uint   `json:"user_id"`
}

type UpdateVideoFileRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	DisplayName *string `json:"display_name"`
}

// ExtractTask相关请求
type CreateExtractTaskRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	VideoSourceID *uint  `json:"video_source_id"`
	VideoFileID   *uint  `json:"video_file_id"`
	UserID        uint   `json:"user_id" binding:"required"`
	FrameCount    *int   `json:"frame_count"`
	Duration      *int   `json:"duration"`
	Interval      *int   `json:"interval"`
	StartTime     *int   `json:"start_time"`
	Quality       int    `json:"quality" default:"2"`
	Format        string `json:"format" default:"jpg"`
}

type GetExtractTasksRequest struct {
	Page          int     `json:"page"`
	PageSize      int     `json:"page_size"`
	Status        *string `json:"status"`
	VideoSourceID *uint   `json:"video_source_id"`
	VideoFileID   *uint   `json:"video_file_id"`
	UserID        *uint   `json:"user_id"`
}

// RecordTask相关请求
type CreateRecordTaskRequest struct {
	Name          string `json:"name" binding:"required"`
	VideoSourceID uint   `json:"video_source_id" binding:"required"`
	Duration      int    `json:"duration" binding:"required"`
	UserID        uint   `json:"user_id" binding:"required"`
}

type GetRecordTasksRequest struct {
	Page          int     `json:"page"`
	PageSize      int     `json:"page_size"`
	Status        *string `json:"status"`
	VideoSourceID *uint   `json:"video_source_id"`
	UserID        *uint   `json:"user_id"`
}

// RecordTaskStatistics 录制任务统计信息
type RecordTaskStatistics struct {
	TotalCount     int `json:"total_count"`
	PendingCount   int `json:"pending_count"`
	RecordingCount int `json:"recording_count"`
	CompletedCount int `json:"completed_count"`
	StoppedCount   int `json:"stopped_count"`
	FailedCount    int `json:"failed_count"`
}

// ScreenshotRequest 视频源截图请求
type ScreenshotRequest struct {
	VideoSourceID uint `json:"video_source_id" binding:"required"`
}

// ScreenshotResponse 视频源截图响应
type ScreenshotResponse struct {
	Base64Data  string `json:"base64_data"`  // 图片的base64编码
	ContentType string `json:"content_type"` // 内容类型，如image/jpeg
	Width       int    `json:"width"`        // 图片宽度
	Height      int    `json:"height"`       // 图片高度
	FileSize    int64  `json:"file_size"`    // 文件大小（字节）
	Timestamp   string `json:"timestamp"`    // 截图时间戳
}
