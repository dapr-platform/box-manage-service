package models

import (
	"fmt"
	"time"
)

// VideoSource 视频源
type VideoSource struct {
	BaseModel

	// 基本信息
	Name        string          `json:"name" gorm:"size:255;not null;index"`
	Description string          `json:"description" gorm:"type:text"`
	Type        VideoSourceType `json:"type" gorm:"size:20;not null;index"`

	// 连接信息
	URL      string `json:"url" gorm:"size:500;not null"`
	Username string `json:"username" gorm:"size:100"`
	Password string `json:"password" gorm:"size:100"`

	// 状态信息
	Status     VideoSourceStatus `json:"status" gorm:"size:20;default:'inactive';index"`
	LastActive *time.Time        `json:"last_active"`
	ErrorMsg   string            `json:"error_msg" gorm:"type:text"`

	// ZLMediaKit 流信息
	StreamID string `json:"stream_id" gorm:"size:100"` // 流ID
	PlayURL  string `json:"play_url" gorm:"size:500"`  // 播放地址

	// 关联信息
	UserID      uint  `json:"user_id" gorm:"not null;index"`
	VideoFileID *uint `json:"video_file_id" gorm:"index"` // 关联的视频文件ID（仅文件类型使用）
}

// VideoSourceType 视频源类型
type VideoSourceType string

const (
	VideoSourceTypeStream VideoSourceType = "stream" // 实时流
	VideoSourceTypeFile   VideoSourceType = "file"   // 视频文件
)

// VideoSourceStatus 视频源状态
type VideoSourceStatus string

const (
	VideoSourceStatusInactive VideoSourceStatus = "inactive" // 未激活
	VideoSourceStatusActive   VideoSourceStatus = "active"   // 活跃
	VideoSourceStatusError    VideoSourceStatus = "error"    // 错误
)

// IsOnline 检查视频源是否在线
func (vs *VideoSource) IsOnline() bool {
	return vs.Status == VideoSourceStatusActive
}

// UpdateStatus 更新状态
func (vs *VideoSource) UpdateStatus(status VideoSourceStatus, errorMsg string) {
	vs.Status = status
	if status == VideoSourceStatusError {
		vs.ErrorMsg = errorMsg
	} else if status == VideoSourceStatusActive {
		vs.ErrorMsg = ""
		now := time.Now()
		vs.LastActive = &now
	}
}

// GenerateStreamInfo 生成流信息
func (vs *VideoSource) GenerateStreamInfo() {
	if vs.StreamID == "" {
		if vs.Type == VideoSourceTypeStream {
			vs.StreamID = fmt.Sprintf("stream_%d", vs.ID)
		} else {
			vs.StreamID = fmt.Sprintf("file_%d", vs.ID)
		}
	}
}

// GetPlayURL 获取播放地址 (相对路径)
func (vs *VideoSource) GetPlayURL(streamPrefix, filePrefix string) string {
	if vs.Type == VideoSourceTypeStream {
		return fmt.Sprintf("%s/%s.live.flv", streamPrefix, vs.StreamID)
	} else {
		return fmt.Sprintf("%s/%s.mp4", filePrefix, vs.StreamID)
	}
}
