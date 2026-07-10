package models

import (
	"time"

	"gorm.io/gorm"
)

// TaskScheduleHistory 任务调度历史记录
type TaskScheduleHistory struct {
	BaseModel
	TaskID             uint    `gorm:"not null;index" json:"task_id"`
	TaskName           string  `gorm:"size:255;not null" json:"task_name"`
	BoxID              uint    `gorm:"not null;index" json:"box_id"`
	BoxName            string  `gorm:"size:255" json:"box_name"`
	Reason             string  `gorm:"type:text" json:"reason"`
	Trigger            string  `gorm:"size:20;not null;default:'manual';index" json:"trigger"` // manual/auto
	Score              float64 `gorm:"default:0" json:"score"`
	SchedulePolicyName string  `gorm:"size:100" json:"schedule_policy_name"` // 使用的调度策略名称
}

func (TaskScheduleHistory) TableName() string {
	return "task_schedule_histories"
}

// BeforeCreate GORM hook
func (h *TaskScheduleHistory) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	h.CreatedAt = CustomTime{Time: now}
	h.UpdatedAt = CustomTime{Time: now}
	return nil
}
