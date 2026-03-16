/*
 * @module models/workflow_instance
 * @description 工作流实例相关数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 工作流定义 -> 工作流实例 -> 节点实例
 * @rules 工作流实例记录一次具体的执行，包含运行时数据
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.2节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// WorkflowInstanceStatus 工作流实例状态枚举
type WorkflowInstanceStatus string

const (
	WorkflowInstanceStatusPending   WorkflowInstanceStatus = "pending"   // 等待执行
	WorkflowInstanceStatusRunning   WorkflowInstanceStatus = "running"   // 运行中
	WorkflowInstanceStatusPaused    WorkflowInstanceStatus = "paused"    // 已暂停
	WorkflowInstanceStatusCompleted WorkflowInstanceStatus = "completed" // 已完成
	WorkflowInstanceStatusFailed    WorkflowInstanceStatus = "failed"    // 执行失败
	WorkflowInstanceStatusCancelled WorkflowInstanceStatus = "cancelled" // 已取消
)

// TriggerType 触发类型枚举
type TriggerType string

const (
	TriggerTypeManual   TriggerType = "manual"   // 手动触发
	TriggerTypeEvent    TriggerType = "event"    // 事件触发
	TriggerTypeSchedule TriggerType = "schedule" // 定时触发
	TriggerTypeAPI      TriggerType = "api"      // API触发
)

// WorkflowInstance 工作流实例模型
// @Description 工作流的一次具体执行，记录运行时数据
type WorkflowInstance struct {
	BaseModel
	WorkflowID    uint                   `gorm:"not null;index" json:"workflow_id" example:"1"`
	ScheduleID    uint                   `gorm:"index" json:"schedule_id" example:"1"`   // 调度ID（关联 workflow_schedules 表）
	DeploymentID  uint                   `gorm:"index" json:"deployment_id" example:"1"` // 部署ID（关联 workflow_deployments 表）
	InstanceID    string                 `gorm:"type:varchar(100);not null;uniqueIndex" json:"instance_id" example:"wf_inst_123456"`
	BoxID         uint                   `gorm:"index" json:"box_id" example:"1"`
	Status        WorkflowInstanceStatus `gorm:"type:varchar(20);not null;index" json:"status" example:"running"`
	TriggerType   TriggerType            `gorm:"type:varchar(20);not null" json:"trigger_type" example:"manual"`
	TriggerData   JSONMap                `gorm:"type:jsonb" json:"trigger_data"`
	ContextData   JSONMap                `gorm:"type:jsonb" json:"context_data"`
	Variables     JSONMap                `gorm:"type:jsonb" json:"variables"`
	CurrentNodeID string                 `gorm:"type:varchar(100)" json:"current_node_id" example:"node_2"`
	StartTime     *time.Time             `json:"start_time" example:"2025-01-26T12:00:00Z"`
	EndTime       *time.Time             `json:"end_time" example:"2025-01-26T12:05:00Z"`
	Duration      int                    `json:"duration" example:"300"` // 执行耗时（秒）
	ErrorMessage  string                 `gorm:"type:text" json:"error_message"`
	RetryCount    int                    `gorm:"default:0" json:"retry_count" example:"0"`
	CreatedBy     uint                   `json:"created_by" example:"1"`
}

// JSONMap JSON映射类型
type JSONMap map[string]interface{}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// TableName 指定表名
func (WorkflowInstance) TableName() string {
	return "workflow_instances"
}
