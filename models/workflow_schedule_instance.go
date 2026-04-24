/*
 * @module models/workflow_schedule_instance
 * @description 调度实例相关数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 调度配置 -> 调度实例 -> 工作流实例
 * @rules 调度实例记录每次调度触发的实际数据
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.9.1节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// WorkflowScheduleInstanceStatus 调度实例状态枚举
type WorkflowScheduleInstanceStatus string

const (
	WorkflowScheduleInstanceStatusPending   WorkflowScheduleInstanceStatus = "pending"   // 等待执行
	WorkflowScheduleInstanceStatusRunning   WorkflowScheduleInstanceStatus = "running"   // 执行中
	WorkflowScheduleInstanceStatusCompleted WorkflowScheduleInstanceStatus = "completed" // 已完成
	WorkflowScheduleInstanceStatusFailed    WorkflowScheduleInstanceStatus = "failed"    // 执行失败
	WorkflowScheduleInstanceStatusCancelled WorkflowScheduleInstanceStatus = "cancelled" // 已取消
)

// WorkflowScheduleInstance 调度实例模型
// @Description 调度的一次具体触发，记录触发的实际数据
type WorkflowScheduleInstance struct {
	BaseModel
	ScheduleID          uint                           `gorm:"not null;index" json:"schedule_id" example:"1"`
	InstanceID          string                         `gorm:"type:varchar(100);not null;uniqueIndex" json:"instance_id" example:"schedule_inst_123456"`
	TriggerType         TriggerType                    `gorm:"type:varchar(20);not null" json:"trigger_type" example:"cron"`
	TriggerTime         time.Time                      `gorm:"not null;index" json:"trigger_time" example:"2025-01-26T12:00:00Z"`
	TriggerData         JSONMap                        `gorm:"type:jsonb" json:"trigger_data"`
	Status              WorkflowScheduleInstanceStatus `gorm:"type:varchar(20);not null;index" json:"status" example:"running"`
	DeploymentID        uint                           `gorm:"not null;index" json:"deployment_id" example:"1"`
	WorkflowInstanceIDs WorkflowInstanceIDList         `gorm:"type:jsonb" json:"workflow_instance_ids"`
	StartTime           *time.Time                     `json:"start_time" example:"2025-01-26T12:00:00Z"`
	EndTime             *time.Time                     `json:"end_time" example:"2025-01-26T12:05:00Z"`
	Duration            int                            `json:"duration" example:"300"` // 执行耗时（秒）
	SuccessCount        int                            `gorm:"default:0" json:"success_count" example:"2"`
	FailedCount         int                            `gorm:"default:0" json:"failed_count" example:"1"`
	ErrorMessage        string                         `gorm:"type:text" json:"error_message"`
}

// WorkflowInstanceIDList 工作流实例ID列表类型
type WorkflowInstanceIDList []WorkflowInstanceIDMapping

// WorkflowInstanceIDMapping 工作流实例ID映射
type WorkflowInstanceIDMapping struct {
	DeploymentID uint   `json:"deployment_id"`
	InstanceID   string `json:"instance_id"`
}

// Scan 实现 sql.Scanner 接口
func (w *WorkflowInstanceIDList) Scan(value interface{}) error {
	if value == nil {
		*w = make(WorkflowInstanceIDList, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, w)
}

// Value 实现 driver.Valuer 接口
func (w WorkflowInstanceIDList) Value() (driver.Value, error) {
	if w == nil {
		return nil, nil
	}
	return json.Marshal(w)
}

// TableName 指定表名
func (WorkflowScheduleInstance) TableName() string {
	return "workflow_schedule_instances"
}
