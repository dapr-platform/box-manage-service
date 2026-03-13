/*
 * @module models/workflow_log
 * @description 工作流日志数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 工作流执行 -> 日志记录 -> 日志查询
 * @rules 记录工作流和节点执行的详细日志，支持按级别和类型查询
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.9节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// 日志级别（使用系统日志的LogLevel）
// LogLevel, LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError 已在 system_log.go 中定义

// LogType 日志类型枚举
type LogType string

const (
	LogTypeWorkflow LogType = "workflow" // 工作流日志
	LogTypeNode     LogType = "node"     // 节点日志
	LogTypeSystem   LogType = "system"   // 系统日志
)

// WorkflowLog 工作流日志模型
// @Description 工作流和节点执行的详细日志记录
type WorkflowLog struct {
	BaseModel
	WorkflowInstanceID uint           `gorm:"not null;index:idx_workflow_instance" json:"workflow_instance_id" example:"1"`
	NodeInstanceID     *uint          `gorm:"index" json:"node_instance_id,omitempty" example:"1"`
	Level              LogLevel       `gorm:"type:varchar(20);not null;index" json:"level" example:"info"`
	Type               LogType        `gorm:"type:varchar(20);not null;index" json:"type" example:"workflow"`
	Message            string         `gorm:"type:text;not null" json:"message" example:"工作流开始执行"`
	Details            DetailsJSON    `gorm:"type:jsonb" json:"details,omitempty"`
	Timestamp          time.Time      `gorm:"not null;index" json:"timestamp" example:"2025-01-26T12:00:00Z"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// DetailsJSON 日志详情JSON
// @Description 日志的详细信息，支持任意结构
type DetailsJSON struct {
	Data map[string]interface{} `json:"data"`
}

// Scan 实现 sql.Scanner 接口
func (d *DetailsJSON) Scan(value interface{}) error {
	if value == nil {
		d.Data = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	result := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	d.Data = result
	return nil
}

// Value 实现 driver.Valuer 接口
func (d DetailsJSON) Value() (driver.Value, error) {
	if d.Data == nil {
		return nil, nil
	}
	return json.Marshal(d.Data)
}

// TableName 指定表名
func (WorkflowLog) TableName() string {
	return "workflow_logs"
}

// BeforeCreate GORM钩子
func (w *WorkflowLog) BeforeCreate(tx *gorm.DB) error {
	if w.Timestamp.IsZero() {
		w.Timestamp = time.Now()
	}
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (w *WorkflowLog) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = time.Now()
	return nil
}
