/*
 * @module models/line_instance
 * @description 连接线实例数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 工作流实例创建 -> 连接线实例初始化 -> 连接线执行
 * @rules 连接线实例记录工作流实例运行时的连接线执行情况
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.8节
 */

package models

import (
	"time"

	"gorm.io/gorm"
)

// LineInstanceStatus 连接线实例状态枚举
type LineInstanceStatus string

const (
	LineInstanceStatusPending   LineInstanceStatus = "pending"   // 待执行
	LineInstanceStatusEvaluated LineInstanceStatus = "evaluated" // 已评估
	LineInstanceStatusSkipped   LineInstanceStatus = "skipped"   // 已跳过
)

// LineInstance 连接线实例模型
// @Description 工作流实例运行时的连接线实例，记录连接线的执行状态
type LineInstance struct {
	BaseModel
	WorkflowInstanceID uint               `gorm:"not null;index:idx_workflow_instance" json:"workflow_instance_id" example:"1"`
	LineDefID          uint               `gorm:"not null;index" json:"line_def_id" example:"1"`
	SourceNodeInstID   uint               `gorm:"not null;index" json:"source_node_inst_id" example:"1"`
	TargetNodeInstID   uint               `gorm:"not null;index" json:"target_node_inst_id" example:"2"`
	Status             LineInstanceStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status" example:"pending"`
	ConditionResult    *bool              `gorm:"default:null" json:"condition_result,omitempty" example:"true"`
	EvaluatedAt        *time.Time         `json:"evaluated_at,omitempty" example:"2025-01-26T12:00:00Z"`
	ErrorMessage       string             `gorm:"type:text" json:"error_message,omitempty" example:""`
	DeletedAt          gorm.DeletedAt     `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// TableName 指定表名
func (LineInstance) TableName() string {
	return "line_instances"
}

// BeforeCreate GORM钩子
func (l *LineInstance) BeforeCreate(tx *gorm.DB) error {
	l.CreatedAt = time.Now()
	l.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (l *LineInstance) BeforeUpdate(tx *gorm.DB) error {
	l.UpdatedAt = time.Now()
	return nil
}
