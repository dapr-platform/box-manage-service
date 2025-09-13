package models

import (
	"fmt"
	"time"
)

// ExtractTask 抽帧任务
type ExtractTask struct {
	BaseModel

	// 任务信息
	TaskID      string `json:"task_id" gorm:"size:64;uniqueIndex;not null"`
	Name        string `json:"name" gorm:"size:255;not null"`
	Description string `json:"description" gorm:"type:text"`

	// 抽帧配置
	FrameCount *int   `json:"frame_count"`                         // 抽帧数量
	Duration   *int   `json:"duration"`                            // 抽帧时长(秒)
	Interval   *int   `json:"interval"`                            // 抽帧间隔(秒)
	StartTime  *int   `json:"start_time"`                          // 开始时间(秒)
	Quality    int    `json:"quality" gorm:"default:2"`            // 图片质量(1-31,越小质量越高)
	Format     string `json:"format" gorm:"size:10;default:'jpg'"` // 图片格式

	// 输出信息
	OutputDir      string `json:"output_dir" gorm:"size:500"` // 输出目录
	ExtractedCount int    `json:"extracted_count"`            // 实际提取的帧数
	TotalSize      int64  `json:"total_size"`                 // 总文件大小

	// 状态信息
	Status   ExtractTaskStatus `json:"status" gorm:"size:20;default:'pending';index"`
	Progress int               `json:"progress" gorm:"default:0"`
	ErrorMsg string            `json:"error_msg" gorm:"type:text"`

	// 执行信息
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 关联信息
	VideoSourceID *uint `json:"video_source_id" gorm:"index"`
	VideoFileID   *uint `json:"video_file_id" gorm:"index"`
	UserID        uint  `json:"user_id" gorm:"not null;index"`
}

// ExtractTaskStatus 抽帧任务状态
type ExtractTaskStatus string

const (
	ExtractTaskStatusPending    ExtractTaskStatus = "pending"    // 待开始
	ExtractTaskStatusExtracting ExtractTaskStatus = "extracting" // 抽帧中
	ExtractTaskStatusCompleted  ExtractTaskStatus = "completed"  // 已完成
	ExtractTaskStatusStopped    ExtractTaskStatus = "stopped"    // 已停止
	ExtractTaskStatusFailed     ExtractTaskStatus = "failed"     // 失败
)

// IsActive 检查任务是否活跃
func (et *ExtractTask) IsActive() bool {
	return et.Status == ExtractTaskStatusExtracting
}

// CanStart 检查是否可以开始抽帧
func (et *ExtractTask) CanStart() bool {
	return et.Status == ExtractTaskStatusPending || et.Status == ExtractTaskStatusFailed
}

// CanStop 检查是否可以停止抽帧
func (et *ExtractTask) CanStop() bool {
	return et.Status == ExtractTaskStatusExtracting
}

// MarkAsExtracting 标记为抽帧中
func (et *ExtractTask) MarkAsExtracting() {
	et.Status = ExtractTaskStatusExtracting
	et.Progress = 0
	now := time.Now()
	et.StartedAt = &now
	et.ErrorMsg = ""
}

// MarkAsCompleted 标记为已完成
func (et *ExtractTask) MarkAsCompleted(outputDir string, frameCount int, totalSize int64) {
	et.Status = ExtractTaskStatusCompleted
	et.Progress = 100
	et.OutputDir = outputDir
	et.ExtractedCount = frameCount
	et.TotalSize = totalSize
	now := time.Now()
	et.CompletedAt = &now
	et.ErrorMsg = ""
}

// MarkAsStopped 标记为已停止
func (et *ExtractTask) MarkAsStopped() {
	et.Status = ExtractTaskStatusStopped
	now := time.Now()
	et.CompletedAt = &now
}

// MarkAsFailed 标记为失败
func (et *ExtractTask) MarkAsFailed(errorMsg string) {
	et.Status = ExtractTaskStatusFailed
	et.ErrorMsg = errorMsg
	now := time.Now()
	et.CompletedAt = &now
}

// UpdateProgress 更新进度
func (et *ExtractTask) UpdateProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	et.Progress = progress
}

// GetSourceType 获取数据源类型
func (et *ExtractTask) GetSourceType() string {
	if et.VideoSourceID != nil {
		return "video_source"
	}
	if et.VideoFileID != nil {
		return "video_file"
	}
	return "unknown"
}

// GetFormattedSize 获取格式化的文件大小
func (et *ExtractTask) GetFormattedSize() string {
	if et.TotalSize == 0 {
		return "0 B"
	}

	const unit = 1024
	if et.TotalSize < unit {
		return fmt.Sprintf("%d B", et.TotalSize)
	}

	div, exp := int64(unit), 0
	for n := et.TotalSize / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(et.TotalSize)/float64(div), "KMGTPE"[exp])
}

// Validate 验证抽帧任务配置
func (et *ExtractTask) Validate() error {
	// 必须指定数据源
	if et.VideoSourceID == nil && et.VideoFileID == nil {
		return fmt.Errorf("必须指定视频源或视频文件")
	}

	// 不能同时指定两个数据源
	if et.VideoSourceID != nil && et.VideoFileID != nil {
		return fmt.Errorf("不能同时指定视频源和视频文件")
	}

	// 必须指定抽帧方式
	if et.FrameCount == nil && et.Duration == nil && et.Interval == nil {
		return fmt.Errorf("必须指定抽帧数量、时长或间隔中的一种")
	}

	// 验证参数范围
	if et.FrameCount != nil && *et.FrameCount <= 0 {
		return fmt.Errorf("抽帧数量必须大于0")
	}

	if et.Duration != nil && *et.Duration <= 0 {
		return fmt.Errorf("抽帧时长必须大于0")
	}

	if et.Interval != nil && *et.Interval <= 0 {
		return fmt.Errorf("抽帧间隔必须大于0")
	}

	if et.Quality < 1 || et.Quality > 31 {
		return fmt.Errorf("图片质量参数必须在1-31之间")
	}

	// 验证图片格式
	validFormats := []string{"jpg", "jpeg", "png", "webp"}
	isValidFormat := false
	for _, format := range validFormats {
		if et.Format == format {
			isValidFormat = true
			break
		}
	}
	if !isValidFormat {
		return fmt.Errorf("不支持的图片格式: %s", et.Format)
	}

	return nil
}
