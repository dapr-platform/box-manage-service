/*
 * @module models/base
 * @description 基础模型定义，包含通用字段和方法
 * @architecture 数据模型层
 * @documentReference DESIGN-000.md
 * @stateFlow 模型定义 -> 数据库映射 -> 业务逻辑
 * @rules 所有模型继承基础模型，包含创建时间、更新时间等通用字段
 * @dependencies gorm.io/gorm
 * @refs DESIGN-000.md
 */

package models

import (
	"time"
)

// BaseModel 基础模型，包含通用字段
// @Description 基础模型，包含通用字段
type BaseModel struct {
	ID        uint       `json:"id" gorm:"primarykey" example:"1"`                       // 主键ID
	CreatedAt time.Time  `json:"created_at" example:"2025-01-26T12:00:00Z"`              // 创建时间
	UpdatedAt time.Time  `json:"updated_at" example:"2025-01-26T12:00:00Z"`              // 更新时间
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index" swaggerignore:"true"` // 软删除时间，swagger忽略
}

// SoftDelete 软删除接口
type SoftDelete interface {
	IsDeleted() bool
}

// IsDeleted 检查是否已软删除
func (m *BaseModel) IsDeleted() bool {
	return m.DeletedAt != nil
}

// Status 通用状态枚举
type Status string

const (
	StatusActive   Status = "active"   // 活跃
	StatusInactive Status = "inactive" // 非活跃
	StatusPending  Status = "pending"  // 待处理
	StatusRunning  Status = "running"  // 运行中
	StatusStopped  Status = "stopped"  // 已停止
	StatusFailed   Status = "failed"   // 失败
	StatusSuccess  Status = "success"  // 成功
)

// BoxStatus 盒子状态枚举
type BoxStatus string

const (
	BoxStatusOnline      BoxStatus = "online"      // 在线
	BoxStatusOffline     BoxStatus = "offline"     // 离线
	BoxStatusMaintenance BoxStatus = "maintenance" // 维护中
	BoxStatusError       BoxStatus = "error"       // 错误
)

// TaskStatus 任务状态枚举（保留用于向后兼容）
// Deprecated: 建议使用 ScheduleStatus 和 RunStatus 组合
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 待执行
	TaskStatusScheduled TaskStatus = "scheduled" // 已调度
	TaskStatusRunning   TaskStatus = "running"   // 执行中
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 执行失败
	TaskStatusCancelled TaskStatus = "cancelled" // 已取消
	TaskStatusPaused    TaskStatus = "paused"    // 已暂停
	TaskStatusStopped   TaskStatus = "stopped"   // 已停止
	TaskStatusStopping  TaskStatus = "stopping"  // 停止中
)

// ScheduleStatus 调度状态枚举（任务是否分配到盒子）
type ScheduleStatus string

const (
	ScheduleStatusUnassigned ScheduleStatus = "unassigned" // 未分配
	ScheduleStatusAssigned   ScheduleStatus = "assigned"   // 已分配
)

// RunStatus 运行状态枚举（任务在盒子上的执行状态）
type RunStatus string

const (
	RunStatusStopped RunStatus = "stopped" // 已停止
	RunStatusRunning RunStatus = "running" // 运行中
)

// ModelStatus 模型状态枚举
type ModelStatus string

const (
	ModelStatusUploaded   ModelStatus = "uploaded"   // 已上传
	ModelStatusConverting ModelStatus = "converting" // 转换中
	ModelStatusConverted  ModelStatus = "converted"  // 已转换
	ModelStatusDeploying  ModelStatus = "deploying"  // 部署中
	ModelStatusDeployed   ModelStatus = "deployed"   // 已部署
	ModelStatusError      ModelStatus = "error"      // 错误
)

// VideoStatus 视频状态枚举
type VideoStatus string

const (
	VideoStatusUploaded   VideoStatus = "uploaded"   // 已上传
	VideoStatusProcessing VideoStatus = "processing" // 处理中
	VideoStatusProcessed  VideoStatus = "processed"  // 已处理
	VideoStatusAnalyzing  VideoStatus = "analyzing"  // 分析中
	VideoStatusAnalyzed   VideoStatus = "analyzed"   // 已分析
	VideoStatusError      VideoStatus = "error"      // 错误
)
