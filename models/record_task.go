package models

import (
	"fmt"
	"time"
)

// RecordTask 录制任务
type RecordTask struct {
	BaseModel

	// 任务信息
	TaskID   string `json:"task_id" gorm:"size:64;uniqueIndex;not null"`
	Name     string `json:"name" gorm:"size:255;not null"`
	Duration int    `json:"duration"` // 录制时长(分钟)

	// 输出信息
	OutputPath     string  `json:"output_path" gorm:"size:500"`
	FileSize       int64   `json:"file_size"`
	ActualDuration float64 `json:"actual_duration"`

	// 状态信息
	Status   RecordTaskStatus `json:"status" gorm:"size:20;default:'pending';index"`
	Progress int              `json:"progress" gorm:"default:0"`
	ErrorMsg string           `json:"error_msg" gorm:"type:text"`

	// 执行信息
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 关联信息
	VideoSourceID uint `json:"video_source_id" gorm:"not null;index"`
	UserID        uint `json:"user_id" gorm:"not null;index"`
}

// RecordTaskStatus 录制任务状态
type RecordTaskStatus string

const (
	RecordTaskStatusPending   RecordTaskStatus = "pending"   // 待开始
	RecordTaskStatusRecording RecordTaskStatus = "recording" // 录制中
	RecordTaskStatusCompleted RecordTaskStatus = "completed" // 已完成
	RecordTaskStatusStopped   RecordTaskStatus = "stopped"   // 已停止
	RecordTaskStatusFailed    RecordTaskStatus = "failed"    // 失败
)

// IsActive 检查任务是否活跃
func (rt *RecordTask) IsActive() bool {
	return rt.Status == RecordTaskStatusRecording
}

// CanStart 检查是否可以开始录制
func (rt *RecordTask) CanStart() bool {
	return rt.Status == RecordTaskStatusPending || rt.Status == RecordTaskStatusFailed
}

// CanStop 检查是否可以停止录制
func (rt *RecordTask) CanStop() bool {
	return rt.Status == RecordTaskStatusRecording
}

// MarkAsRecording 标记为录制中
func (rt *RecordTask) MarkAsRecording() {
	rt.Status = RecordTaskStatusRecording
	rt.Progress = 0
	now := time.Now()
	rt.StartedAt = &now
	rt.ErrorMsg = ""
}

// MarkAsCompleted 标记为已完成
func (rt *RecordTask) MarkAsCompleted(outputPath string, fileSize int64, actualDuration float64) {
	rt.Status = RecordTaskStatusCompleted
	rt.Progress = 100
	rt.OutputPath = outputPath
	rt.FileSize = fileSize
	rt.ActualDuration = actualDuration
	now := time.Now()
	rt.CompletedAt = &now
	rt.ErrorMsg = ""
}

// MarkAsStopped 标记为已停止
func (rt *RecordTask) MarkAsStopped() {
	rt.Status = RecordTaskStatusStopped
	now := time.Now()
	rt.CompletedAt = &now
}

// MarkAsFailed 标记为失败
func (rt *RecordTask) MarkAsFailed(errorMsg string) {
	rt.Status = RecordTaskStatusFailed
	rt.ErrorMsg = errorMsg
	now := time.Now()
	rt.CompletedAt = &now
}

// UpdateProgress 更新进度
func (rt *RecordTask) UpdateProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	rt.Progress = progress
}

// GetFormattedDuration 获取格式化的录制时长
func (rt *RecordTask) GetFormattedDuration() string {
	if rt.ActualDuration <= 0 {
		return "00:00:00"
	}

	hours := int(rt.ActualDuration) / 3600
	minutes := (int(rt.ActualDuration) % 3600) / 60
	seconds := int(rt.ActualDuration) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
