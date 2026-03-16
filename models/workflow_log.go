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
	LogTypeNode LogType = "node" // 节点日志
	LogTypeLine LogType = "line" // 连接线日志
)

// WorkflowLog 工作流日志模型
// @Description 工作流和节点执行的详细日志记录
type WorkflowLog struct {
	BaseModel
	WorkflowInstanceID      uint          `gorm:"not null;index:idx_workflow_instance" json:"workflow_instance_id" example:"1"`
	ScheduleID              uint          `gorm:"index" json:"schedule_id" example:"1"`   // 调度ID（关联 workflow_schedules 表）
	DeploymentID            uint          `gorm:"index" json:"deployment_id" example:"1"` // 部署ID（关联 workflow_deployments 表）
	LogType                 LogType       `gorm:"type:varchar(20);not null;index:idx_log_type" json:"log_type" example:"node"`
	OperationInstanceID     string        `gorm:"type:varchar(100);not null;index:idx_operation_instance" json:"operation_instance_id" example:"node_inst_123"`
	OperationInstanceName   string        `gorm:"type:varchar(200)" json:"operation_instance_name,omitempty" example:"AI推理"`
	OperationInstanceInput  OperationJSON `gorm:"type:jsonb" json:"operation_instance_input,omitempty"`
	OperationInstanceOutput OperationJSON `gorm:"type:jsonb" json:"operation_instance_output,omitempty"`
	OperationInstanceStatus string        `gorm:"type:varchar(50)" json:"operation_instance_status,omitempty" example:"completed"`
	Message                 string        `gorm:"type:text;not null" json:"message" example:"节点执行完成"`
	Details                 DetailsJSON   `gorm:"type:jsonb" json:"details,omitempty"`
}

// OperationJSON 操作实例数据JSON
// @Description 操作实例的输入输出数据
type OperationJSON struct {
	Data map[string]interface{} `json:"data"`
}

// Scan 实现 sql.Scanner 接口
func (o *OperationJSON) Scan(value interface{}) error {
	if value == nil {
		o.Data = make(map[string]interface{})
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
	o.Data = result
	return nil
}

// Value 实现 driver.Valuer 接口
func (o OperationJSON) Value() (driver.Value, error) {
	if o.Data == nil {
		return nil, nil
	}
	return json.Marshal(o.Data)
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
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (w *WorkflowLog) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = time.Now()
	return nil
}
