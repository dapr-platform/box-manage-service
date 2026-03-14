/*
 * @module models/node_instance
 * @description 节点实例相关数据模型
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 节点定义 -> 节点实例（运行时）
 * @rules 节点实例记录节点的一次具体执行
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.3节
 */

package models

import (
	"time"

	"gorm.io/gorm"
)

// NodeInstanceStatus 节点实例状态枚举
type NodeInstanceStatus string

const (
	NodeInstanceStatusPending   NodeInstanceStatus = "pending"   // 等待执行
	NodeInstanceStatusRunning   NodeInstanceStatus = "running"   // 运行中
	NodeInstanceStatusCompleted NodeInstanceStatus = "completed" // 已完成
	NodeInstanceStatusFailed    NodeInstanceStatus = "failed"    // 执行失败
	NodeInstanceStatusSkipped   NodeInstanceStatus = "skipped"   // 已跳过
)

// NodeInstance 节点实例模型
// @Description 节点的一次具体执行，记录运行时数据
type NodeInstance struct {
	BaseModel
	InstanceID         string             `gorm:"type:varchar(100);not null;uniqueIndex" json:"instance_id" example:"node_inst_123"`
	WorkflowInstanceID uint               `gorm:"not null;index" json:"workflow_instance_id" example:"1"`
	NodeDefID          uint               `gorm:"not null;index" json:"node_def_id" example:"1"` // 关联节点定义，用于追溯节点配置来源
	NodeID             string             `gorm:"type:varchar(100);not null;index" json:"node_id" example:"node_1"`
	NodeType           string             `gorm:"type:varchar(50);not null" json:"node_type" example:"start"`
	NodeName           string             `gorm:"type:varchar(100);not null" json:"node_name" example:"开始"`
	NodeKeyName        string             `gorm:"type:varchar(100);not null;index" json:"node_key_name" example:"start_node"`
	Config             JSONMap            `gorm:"type:jsonb" json:"config"` // 节点配置（从NodeDefinition复制，用于记录执行时的配置快照）
	Status             NodeInstanceStatus `gorm:"type:varchar(20);not null;index" json:"status" example:"completed"`
	InputData          JSONMap            `gorm:"type:jsonb" json:"input_data"`
	OutputData         JSONMap            `gorm:"type:jsonb" json:"output_data"`
	StartTime          *time.Time         `json:"start_time" example:"2025-01-26T12:00:00Z"`
	EndTime            *time.Time         `json:"end_time" example:"2025-01-26T12:00:01Z"`
	Duration           int                `json:"duration" example:"1000"` // 执行耗时（毫秒）
	ErrorMessage       string             `gorm:"type:text" json:"error_message"`
	RetryCount         int                `gorm:"default:0" json:"retry_count" example:"0"`
	DeletedAt          gorm.DeletedAt     `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// TableName 指定表名
func (NodeInstance) TableName() string {
	return "node_instances"
}
