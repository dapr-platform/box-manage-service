package models

import (
	"fmt"
)

// VideoFile 视频文件
type VideoFile struct {
	BaseModel

	// 基本信息
	Name             string `json:"name" gorm:"size:255;not null;index"`
	DisplayName      string `json:"display_name" gorm:"size:255"`
	OriginalFileName string `json:"original_file_name" gorm:"size:255"` // 存储用户上传时的原始文件名
	Description      string `json:"description" gorm:"type:text"`

	// 文件信息
	OriginalPath  string  `json:"original_path" gorm:"size:500;not null"`
	ConvertedPath string  `json:"converted_path" gorm:"size:500"`
	FileSize      int64   `json:"file_size"`
	Duration      float64 `json:"duration"`

	// 视频属性
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	FrameRate float64 `json:"frame_rate"`
	Bitrate   int64   `json:"bitrate"`
	Codec     string  `json:"codec" gorm:"size:50"`
	Format    string  `json:"format" gorm:"size:20"`

	// 处理状态
	Status          VideoFileStatus `json:"status" gorm:"size:20;default:'uploaded';index"`
	ProcessProgress int             `json:"process_progress" gorm:"default:0"`
	ErrorMsg        string          `json:"error_msg" gorm:"type:text"`

	// 关联信息
	VideoSourceID *uint `json:"video_source_id" gorm:"index"`
	UserID        uint  `json:"user_id" gorm:"not null;index"`

	// ZLMediaKit 相关
	StreamApp string `json:"stream_app" gorm:"size:50"`
	StreamID  string `json:"stream_id" gorm:"size:100"`
	PlayURL   string `json:"play_url" gorm:"size:500"`
}

// VideoFileStatus 视频文件状态
type VideoFileStatus string

const (
	VideoFileStatusUploaded   VideoFileStatus = "uploaded"   // 已上传
	VideoFileStatusConverting VideoFileStatus = "converting" // 转换中
	VideoFileStatusCompleted  VideoFileStatus = "completed"  // 转换完成
	VideoFileStatusReady      VideoFileStatus = "ready"      // 就绪
	VideoFileStatusFailed     VideoFileStatus = "failed"     // 失败
	VideoFileStatusError      VideoFileStatus = "error"      // 错误
)

// IsReady 检查文件是否就绪
func (vf *VideoFile) IsReady() bool {
	return vf.Status == VideoFileStatusReady
}

// NeedsConversion 检查是否需要转换
func (vf *VideoFile) NeedsConversion() bool {
	return !(vf.Codec == "h264" && vf.Format == "mp4")
}

// GetActivePath 获取活跃的文件路径
func (vf *VideoFile) GetActivePath() string {
	if vf.ConvertedPath != "" {
		return vf.ConvertedPath
	}
	return vf.OriginalPath
}

// MarkAsConverting 标记为转换中
func (vf *VideoFile) MarkAsConverting() {
	vf.Status = VideoFileStatusConverting
	vf.ProcessProgress = 0
	vf.ErrorMsg = ""
}

// MarkAsReady 标记为就绪
func (vf *VideoFile) MarkAsReady(convertedPath string) {
	vf.Status = VideoFileStatusReady
	vf.ProcessProgress = 100
	vf.ErrorMsg = ""
	if convertedPath != "" {
		vf.ConvertedPath = convertedPath
	}
}

// MarkAsError 标记为错误
func (vf *VideoFile) MarkAsError(errorMsg string) {
	vf.Status = VideoFileStatusError
	vf.ErrorMsg = errorMsg
}

// GenerateStreamInfo 生成流信息
func (vf *VideoFile) GenerateStreamInfo() {
	if vf.StreamApp == "" {
		vf.StreamApp = "vod"
	}
	if vf.StreamID == "" {
		vf.StreamID = fmt.Sprintf("video_%d", vf.ID)
	}
}

// GetResolution 获取分辨率字符串
func (vf *VideoFile) GetResolution() string {
	if vf.Width > 0 && vf.Height > 0 {
		return fmt.Sprintf("%dx%d", vf.Width, vf.Height)
	}
	return ""
}

// GetFormattedDuration 获取格式化的时长
func (vf *VideoFile) GetFormattedDuration() string {
	if vf.Duration <= 0 {
		return "00:00"
	}

	hours := int(vf.Duration) / 3600
	minutes := (int(vf.Duration) % 3600) / 60
	seconds := int(vf.Duration) % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
