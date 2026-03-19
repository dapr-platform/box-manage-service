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
	"database/sql/driver"
	"fmt"
	"time"
)

// TimeFormat 统一的时间格式：yyyy-mm-dd hh:mm:ss
const TimeFormat = "2006-01-02 15:04:05"

// CustomTime 自定义时间类型，用于统一JSON序列化格式
// @Description 时间类型，格式为 yyyy-mm-dd hh:mm:ss
// @Example 2025-01-26 12:00:00
type CustomTime struct {
	time.Time
}

// SwaggerType 告诉 Swagger 这是一个字符串类型
func (CustomTime) SwaggerType() string {
	return "string"
}

// SwaggerFormat 告诉 Swagger 时间格式
func (CustomTime) SwaggerFormat() string {
	return "date-time"
}

// SwaggerExample 提供 Swagger 示例值
func (CustomTime) SwaggerExample() interface{} {
	return "2025-01-26 12:00:00"
}

// MarshalJSON 实现JSON序列化
func (ct CustomTime) MarshalJSON() ([]byte, error) {
	if ct.Time.IsZero() {
		return []byte("null"), nil
	}
	formatted := fmt.Sprintf("\"%s\"", ct.Time.Format(TimeFormat))
	return []byte(formatted), nil
}

// UnmarshalJSON 实现JSON反序列化
func (ct *CustomTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		ct.Time = time.Time{}
		return nil
	}

	// 移除引号
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	// 尝试多种时间格式
	formats := []string{
		TimeFormat,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	var err error
	for _, format := range formats {
		ct.Time, err = time.Parse(format, str)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("无法解析时间: %s", str)
}

// Value 实现 driver.Valuer 接口，用于数据库写入
func (ct CustomTime) Value() (driver.Value, error) {
	if ct.Time.IsZero() {
		return nil, nil
	}
	return ct.Time, nil
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (ct *CustomTime) Scan(value interface{}) error {
	if value == nil {
		ct.Time = time.Time{}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		ct.Time = v
		return nil
	case []byte:
		return ct.UnmarshalJSON(v)
	case string:
		return ct.UnmarshalJSON([]byte(v))
	default:
		return fmt.Errorf("无法将 %T 转换为 CustomTime", value)
	}
}

// BaseModel 基础模型，包含通用字段
// @Description 基础模型，包含通用字段
type BaseModel struct {
	ID        uint        `json:"id" gorm:"primarykey" example:"1"`                                            // 主键ID
	CreatedAt CustomTime  `json:"created_at" swaggertype:"string" example:"2025-01-26 12:00:00"`               // 创建时间
	UpdatedAt CustomTime  `json:"updated_at" swaggertype:"string" example:"2025-01-26 12:00:00"`               // 更新时间
	DeletedAt *CustomTime `json:"deleted_at,omitempty" gorm:"index" swaggertype:"string" swaggerignore:"true"` // 软删除时间，swagger忽略
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

// NodeGroupType 节点组类型枚举
type NodeGroupType string

const (
	NodeGroupTypeSingle    NodeGroupType = "single"    // 单节点
	NodeGroupTypePaired    NodeGroupType = "paired"    // 成对节点（如并发开始/结束、循环开始/结束）
	NodeGroupTypeContainer NodeGroupType = "container" // 容器节点（如子流程）
)
