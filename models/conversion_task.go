/*
 * @module models/conversion_task
 * @description 模型转换任务数据结构定义，支持pt/onnx到bmodel的转换管理
 * @architecture 数据模型层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow 转换任务创建 -> 队列调度 -> Docker执行 -> 结果生成
 * @rules 支持转换参数配置、进度跟踪、错误处理
 * @dependencies gorm.io/gorm, encoding/json
 * @refs REQ-003.md, DESIGN-005.md, DESIGN-006.md
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConversionTask 转换任务数据结构
// @Description 模型转换任务，用于管理pt/onnx到bmodel的转换过程
type ConversionTask struct {
	BaseModel

	// 任务标识
	TaskID          string `json:"task_id" gorm:"size:64;uniqueIndex;not null" example:"conv_task_abc123"` // 任务唯一标识
	OriginalModelID uint   `json:"original_model_id" gorm:"not null;index" example:"1"`                    // 原始模型ID
	TargetFormat    string `json:"target_format" gorm:"size:20;not null" example:"bmodel"`                 // 目标格式

	// 转换参数
	Parameters ConversionParameters `json:"parameters" gorm:"type:jsonb"` // 转换参数

	// 任务状态
	Status       ConversionTaskStatus `json:"status" gorm:"size:20;not null;default:'pending'" example:"pending"` // 任务状态
	Progress     int                  `json:"progress" gorm:"default:0" example:"50"`                             // 进度 (0-100)
	StartTime    *time.Time           `json:"start_time" example:"2025-01-28T12:00:00Z"`                          // 开始时间
	EndTime      *time.Time           `json:"end_time" example:"2025-01-28T12:30:00Z"`                            // 结束时间
	ErrorMessage string               `json:"error_message" gorm:"type:text" example:""`                          // 错误信息

	// 文件路径和日志
	Logs       string `json:"logs" gorm:"type:text" example:"转换开始...\n正在处理模型...\n转换完成"`                            // 转换过程日志
	OutputPath string `json:"output_path" gorm:"size:500" example:"/data/models/converted/yolo-v5-bm1684x.bmodel"` // 输出文件路径

	// 创建信息
	CreatedBy uint `json:"created_by" gorm:"not null" example:"1"` // 创建用户ID

	// 关联模型
	OriginalModel *OriginalModel `json:"original_model,omitempty" gorm:"foreignKey:OriginalModelID"` // 关联的原始模型
}

// ConversionTaskStatus 转换任务状态
type ConversionTaskStatus string

const (
	ConversionTaskStatusPending   ConversionTaskStatus = "pending"   // 待处理
	ConversionTaskStatusRunning   ConversionTaskStatus = "running"   // 运行中
	ConversionTaskStatusCompleted ConversionTaskStatus = "completed" // 已完成
	ConversionTaskStatusFailed    ConversionTaskStatus = "failed"    // 失败
)

// ConversionParameters 转换参数
type ConversionParameters struct {
	TargetChip        string `json:"target_chip" example:"BM1684X"`                                 // 目标芯片
	TargetYoloVersion string `json:"target_yolo_version" example:"yolov8"`                          // 目标YOLO版本，默认yolov8
	InputShape        []int  `json:"input_shape" swaggertype:"array,integer" example:"1,3,640,640"` // 输入形状
	CustomParams      string `json:"custom_params,omitempty" example:""`                            // 自定义参数
	EnableDebug       bool   `json:"enable_debug" example:"false"`                                  // 是否启用调试
	ModelFormat       string `json:"model_format" example:"bmodel"`                                 // 模型格式
	Quantization      string `json:"quantization,omitempty" example:"F16"`                          // 量化参数 (F16/F32)
	CacheDir          string `json:"cache_dir,omitempty" example:""`                                // 缓存目录
	WorkDir           string `json:"work_dir,omitempty" example:""`                                 // 工作目录
}

// Scan 实现 sql.Scanner 接口
func (cp *ConversionParameters) Scan(value interface{}) error {
	if value == nil {
		*cp = ConversionParameters{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("无法将数据库值转换为ConversionParameters")
	}

	return json.Unmarshal(bytes, cp)
}

// Value 实现 driver.Valuer 接口
func (cp ConversionParameters) Value() (driver.Value, error) {
	return json.Marshal(cp)
}

// GenerateTaskID 生成唯一任务ID
func GenerateTaskID() string {
	return "conv_task_" + uuid.New().String()[:8]
}

// Start 开始任务
func (ct *ConversionTask) Start() {
	ct.Status = ConversionTaskStatusRunning
	ct.Progress = 0
	now := time.Now()
	ct.StartTime = &now
	ct.ErrorMessage = ""
}

// Complete 完成任务
func (ct *ConversionTask) Complete(outputPath string) {
	ct.Status = ConversionTaskStatusCompleted
	ct.Progress = 100
	ct.OutputPath = outputPath
	now := time.Now()
	ct.EndTime = &now
}

// Fail 任务失败
func (ct *ConversionTask) Fail(errorMsg string) {
	ct.Status = ConversionTaskStatusFailed
	ct.ErrorMessage = errorMsg
	now := time.Now()
	ct.EndTime = &now
}

// AppendLog 添加日志信息
func (ct *ConversionTask) AppendLog(logMessage string) {
	if ct.Logs == "" {
		ct.Logs = logMessage
	} else {
		ct.Logs += "\n" + logMessage
	}
}

// GetLogLines 获取日志行数组
func (ct *ConversionTask) GetLogLines() []string {
	if ct.Logs == "" {
		return []string{}
	}
	return strings.Split(ct.Logs, "\n")
}

// UpdateProgress 更新进度
func (ct *ConversionTask) UpdateProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	ct.Progress = progress
}

// IsRunning 检查任务是否在运行
func (ct *ConversionTask) IsRunning() bool {
	return ct.Status == ConversionTaskStatusRunning
}

// IsCompleted 检查任务是否已完成
func (ct *ConversionTask) IsCompleted() bool {
	return ct.Status == ConversionTaskStatusCompleted
}

// IsFailed 检查任务是否失败
func (ct *ConversionTask) IsFailed() bool {
	return ct.Status == ConversionTaskStatusFailed
}

// GetDurationFormatted 获取格式化的耗时
func (ct *ConversionTask) GetDurationFormatted() string {
	if ct.EndTime != nil && ct.StartTime != nil {
		duration := ct.EndTime.Sub(*ct.StartTime)
		return duration.String()
	}
	return "未完成"
}

// GetProgressStatus 获取进度状态描述
func (ct *ConversionTask) GetProgressStatus() string {
	switch ct.Status {
	case ConversionTaskStatusPending:
		return "等待开始"
	case ConversionTaskStatusRunning:
		return fmt.Sprintf("转换中 (%d%%)", ct.Progress)
	case ConversionTaskStatusCompleted:
		return "转换完成"
	case ConversionTaskStatusFailed:
		return "转换失败"
	default:
		return "未知状态"
	}
}

// GetEstimatedTimeRemaining 获取估算剩余时间
func (ct *ConversionTask) GetEstimatedTimeRemaining() time.Duration {
	if ct.StartTime == nil || ct.Progress <= 0 {
		return 0
	}

	elapsed := time.Since(*ct.StartTime)
	if ct.Progress >= 100 {
		return 0
	}

	// 基于当前进度估算剩余时间
	estimated := time.Duration(float64(elapsed) * (100.0 - float64(ct.Progress)) / float64(ct.Progress))
	return estimated
}

// IsPending 检查任务是否为待处理状态
func (ct *ConversionTask) IsPending() bool {
	return ct.Status == ConversionTaskStatusPending
}

// GetStatusColor 获取状态对应的颜色（用于前端显示）
func (ct *ConversionTask) GetStatusColor() string {
	switch ct.Status {
	case ConversionTaskStatusPending:
		return "gray"
	case ConversionTaskStatusRunning:
		return "blue"
	case ConversionTaskStatusCompleted:
		return "green"
	case ConversionTaskStatusFailed:
		return "red"
	default:
		return "gray"
	}
}

// ValidateParameters 验证转换参数
func (ct *ConversionTask) ValidateParameters() error {
	if ct.Parameters.TargetChip == "" {
		return errors.New("目标芯片不能为空")
	}
	if len(ct.Parameters.InputShape) < 2 {
		return errors.New("输入形状至少需要2个维度")
	}
	return nil
}
