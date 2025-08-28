package models

import (
	"fmt"
)

// ExtractFrame 提取的帧
type ExtractFrame struct {
	BaseModel

	// 基本信息
	TaskID string `json:"task_id" gorm:"size:64;index"`
	Name   string `json:"name" gorm:"size:255"`

	// 帧信息
	FramePath  string  `json:"frame_path" gorm:"size:500;not null"`
	FrameIndex int64   `json:"frame_index"`
	Timestamp  float64 `json:"timestamp"`
	FileSize   int64   `json:"file_size"`

	// 图像属性
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format" gorm:"size:20;default:'jpg'"`

	// 关联信息
	VideoSourceID uint `json:"video_source_id" gorm:"not null;index"`
	UserID        uint `json:"user_id" gorm:"not null;index"`

	// 状态信息
	Status   ExtractFrameStatus `json:"status" gorm:"size:20;default:'extracted';index"`
	ErrorMsg string             `json:"error_msg" gorm:"type:text"`
}

// ExtractFrameStatus 提取帧状态
type ExtractFrameStatus string

const (
	ExtractFrameStatusExtracted ExtractFrameStatus = "extracted" // 已提取
	ExtractFrameStatusError     ExtractFrameStatus = "error"     // 错误
	ExtractFrameStatusDeleted   ExtractFrameStatus = "deleted"   // 已删除
)

// GetResolution 获取分辨率字符串
func (ef *ExtractFrame) GetResolution() string {
	if ef.Width > 0 && ef.Height > 0 {
		return fmt.Sprintf("%dx%d", ef.Width, ef.Height)
	}
	return ""
}

// GetFormattedTimestamp 获取格式化的时间戳
func (ef *ExtractFrame) GetFormattedTimestamp() string {
	if ef.Timestamp <= 0 {
		return "00:00"
	}

	minutes := int(ef.Timestamp) / 60
	seconds := int(ef.Timestamp) % 60
	milliseconds := int((ef.Timestamp - float64(int(ef.Timestamp))) * 1000)

	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, milliseconds)
}

// MarkAsError 标记为错误
func (ef *ExtractFrame) MarkAsError(errorMsg string) {
	ef.Status = ExtractFrameStatusError
	ef.ErrorMsg = errorMsg
}

