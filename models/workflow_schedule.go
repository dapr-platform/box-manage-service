/*
 * @module models/workflow_schedule
 * @description 工作流调度配置数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 调度配置创建 -> 调度器执行 -> 工作流实例创建
 * @rules 支持manual和cron两种调度类型，event类型暂不实现
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.10节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ScheduleType 调度类型枚举
type ScheduleType string

const (
	ScheduleTypeManual ScheduleType = "manual" // 手动触发
	ScheduleTypeCron   ScheduleType = "cron"   // 定时触发
	// ScheduleTypeEvent  ScheduleType = "event"  // 事件触发（暂不实现）
)

// WorkflowScheduleStatus 工作流调度状态枚举
type WorkflowScheduleStatus string

const (
	WorkflowScheduleStatusEnabled  WorkflowScheduleStatus = "enabled"  // 启用
	WorkflowScheduleStatusDisabled WorkflowScheduleStatus = "disabled" // 禁用
)

// WorkflowSchedule 工作流调度配置模型
// @Description 工作流的调度配置，定义如何触发工作流执行
type WorkflowSchedule struct {
	BaseModel
	WorkflowID      uint                   `gorm:"not null;index" json:"workflow_id" example:"1"`
	Name            string                 `gorm:"type:varchar(100);not null" json:"name" example:"每日视频分析"`
	Type            ScheduleType           `gorm:"type:varchar(20);not null" json:"type" example:"cron"`
	CronExpression  string                 `gorm:"type:varchar(100)" json:"cron_expression,omitempty" example:"0 0 * * *"`
	Timezone        string                 `gorm:"type:varchar(50);default:'Asia/Shanghai'" json:"timezone" example:"Asia/Shanghai"`
	InputVariables  InputVariablesJSON     `gorm:"type:jsonb" json:"input_variables,omitempty"`
	Status          WorkflowScheduleStatus `gorm:"type:varchar(20);not null;default:'enabled'" json:"status" example:"enabled"`
	LastExecutedAt  *time.Time             `json:"last_executed_at,omitempty" example:"2025-01-26T12:00:00Z"`
	NextExecutionAt *time.Time             `json:"next_execution_at,omitempty" example:"2025-01-27T00:00:00Z"`
	ExecutionCount  int                    `gorm:"not null;default:0" json:"execution_count" example:"10"`
	CreatedBy       uint                   `gorm:"index" json:"created_by" example:"1"`
	UpdatedBy       uint                   `json:"updated_by" example:"1"`
	DeletedAt       gorm.DeletedAt         `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// InputVariablesJSON 输入变量JSON
// @Description 调度执行时的输入变量值
type InputVariablesJSON struct {
	Variables map[string]interface{} `json:"variables"`
}

// Scan 实现 sql.Scanner 接口
func (i *InputVariablesJSON) Scan(value interface{}) error {
	if value == nil {
		i.Variables = make(map[string]interface{})
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
	i.Variables = result
	return nil
}

// Value 实现 driver.Valuer 接口
func (i InputVariablesJSON) Value() (driver.Value, error) {
	if i.Variables == nil {
		return nil, nil
	}
	return json.Marshal(i.Variables)
}

// TableName 指定表名
func (WorkflowSchedule) TableName() string {
	return "workflow_schedules"
}

// BeforeCreate GORM钩子
func (w *WorkflowSchedule) BeforeCreate(tx *gorm.DB) error {
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (w *WorkflowSchedule) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = time.Now()
	return nil
}
