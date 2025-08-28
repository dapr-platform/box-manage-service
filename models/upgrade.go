/*
 * @module models/upgrade
 * @description 盒子升级相关数据模型定义
 * @architecture 数据模型层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow 升级任务创建 -> 执行中 -> 完成/失败 -> 可回滚
 * @rules 包含升级任务、进度跟踪、版本管理等功能
 * @dependencies gorm.io/gorm
 * @refs DESIGN-001.md
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// UpgradeTask 升级任务模型
type UpgradeTask struct {
	BaseModel
	BoxID            uint          `json:"box_id" gorm:"not null;index"`
	Name             string        `json:"name" gorm:"not null"`
	VersionFrom      string        `json:"version_from" gorm:"not null"`
	VersionTo        string        `json:"version_to" gorm:"not null"`
	Status           UpgradeStatus `json:"status" gorm:"not null;default:'pending'"`
	Progress         int           `json:"progress" gorm:"default:0"`                // 0-100
	UpgradePackageID uint          `json:"upgrade_package_id" gorm:"not null;index"` // 关联升级包
	ProgramFile      string        `json:"program_file"`                             // 兼容字段，逐步废弃
	Force            bool          `json:"force" gorm:"default:false"`

	// 时间相关
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 错误信息
	ErrorMessage string      `json:"error_message" gorm:"type:text"`
	ErrorDetails ErrorDetail `json:"error_details" gorm:"type:jsonb"`

	// 回滚信息
	CanRollback     bool   `json:"can_rollback" gorm:"default:false"`
	RollbackVersion string `json:"rollback_version"`
	RollbackTaskID  *uint  `json:"rollback_task_id"` // 关联的回滚任务ID

	// 创建者
	CreatedBy uint `json:"created_by" gorm:"index"`

	// 批量任务关联
	BatchUpgradeTaskID *uint `json:"batch_upgrade_task_id"`

	// 关联关系
	Box              Box               `json:"box,omitempty" gorm:"foreignKey:BoxID"`
	UpgradePackage   UpgradePackage    `json:"upgrade_package,omitempty" gorm:"foreignKey:UpgradePackageID"`
	BatchUpgradeTask *BatchUpgradeTask `json:"batch_upgrade_task,omitempty" gorm:"foreignKey:BatchUpgradeTaskID"`
}

// UpgradeStatus 升级状态枚举
type UpgradeStatus string

const (
	UpgradeStatusPending    UpgradeStatus = "pending"    // 待执行
	UpgradeStatusRunning    UpgradeStatus = "running"    // 执行中
	UpgradeStatusCompleted  UpgradeStatus = "completed"  // 已完成
	UpgradeStatusFailed     UpgradeStatus = "failed"     // 执行失败
	UpgradeStatusCancelled  UpgradeStatus = "cancelled"  // 已取消
	UpgradeStatusRolledback UpgradeStatus = "rolledback" // 已回滚
)

// ErrorDetail 错误详情结构
type ErrorDetail struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Details    string            `json:"details"`
	Timestamp  time.Time         `json:"timestamp"`
	StackTrace string            `json:"stack_trace,omitempty"`
	Context    map[string]string `json:"context,omitempty"`
}

// Value 实现 driver.Valuer 接口
func (e ErrorDetail) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan 实现 sql.Scanner 接口
func (e *ErrorDetail) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, e)
}

// TableName 设置表名
func (UpgradeTask) TableName() string {
	return "upgrade_tasks"
}

// Start 开始升级任务
func (u *UpgradeTask) Start() {
	u.Status = UpgradeStatusRunning
	now := time.Now()
	u.StartedAt = &now
}

// Complete 完成升级任务
func (u *UpgradeTask) Complete() {
	u.Status = UpgradeStatusCompleted
	u.Progress = 100
	now := time.Now()
	u.CompletedAt = &now
}

// Fail 标记任务失败
func (u *UpgradeTask) Fail(errorMsg string, errorDetail ErrorDetail) {
	u.Status = UpgradeStatusFailed
	u.ErrorMessage = errorMsg
	u.ErrorDetails = errorDetail
	now := time.Now()
	u.CompletedAt = &now
}

// Cancel 取消任务
func (u *UpgradeTask) Cancel() {
	u.Status = UpgradeStatusCancelled
	now := time.Now()
	u.CompletedAt = &now
}

// UpdateProgress 更新进度
func (u *UpgradeTask) UpdateProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	u.Progress = progress
}

// CanCancel 检查是否可以取消
func (u *UpgradeTask) CanCancel() bool {
	return u.Status == UpgradeStatusPending || u.Status == UpgradeStatusRunning
}

// CanRetry 检查是否可以重试
func (u *UpgradeTask) CanRetry() bool {
	return u.Status == UpgradeStatusFailed
}

// IsCompleted 检查是否已完成
func (u *UpgradeTask) IsCompleted() bool {
	return u.Status == UpgradeStatusCompleted ||
		u.Status == UpgradeStatusFailed ||
		u.Status == UpgradeStatusCancelled ||
		u.Status == UpgradeStatusRolledback
}

// BatchUpgradeTask 批量升级任务模型
type BatchUpgradeTask struct {
	BaseModel
	Name             string        `json:"name" gorm:"not null"`
	BoxIDs           BoxIDList     `json:"box_ids" gorm:"type:jsonb;not null"`
	VersionFrom      string        `json:"version_from"`
	VersionTo        string        `json:"version_to" gorm:"not null"`
	Status           UpgradeStatus `json:"status" gorm:"not null;default:'pending'"`
	TotalBoxes       int           `json:"total_boxes" gorm:"not null"`
	CompletedBoxes   int           `json:"completed_boxes" gorm:"default:0"`
	FailedBoxes      int           `json:"failed_boxes" gorm:"default:0"`
	UpgradePackageID uint          `json:"upgrade_package_id" gorm:"not null;index"` // 关联升级包
	ProgramFile      string        `json:"program_file"`                             // 兼容字段，逐步废弃
	Force            bool          `json:"force" gorm:"default:false"`

	// 时间相关
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// 创建者
	CreatedBy uint `json:"created_by" gorm:"index"`

	// 关联关系
	UpgradePackage UpgradePackage `json:"upgrade_package,omitempty" gorm:"foreignKey:UpgradePackageID"`
	UpgradeTasks   []UpgradeTask  `json:"upgrade_tasks,omitempty"`
}

// BoxIDList 盒子ID列表类型
type BoxIDList []uint

// Value 实现 driver.Valuer 接口
func (b BoxIDList) Value() (driver.Value, error) {
	return json.Marshal(b)
}

// Scan 实现 sql.Scanner 接口
func (b *BoxIDList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, b)
}

// TableName 设置表名
func (BatchUpgradeTask) TableName() string {
	return "batch_upgrade_tasks"
}

// UpdateProgress 更新批量任务进度
func (b *BatchUpgradeTask) UpdateProgress(completed, failed int) {
	b.CompletedBoxes = completed
	b.FailedBoxes = failed

	// 如果所有盒子都处理完了，更新状态
	if completed+failed >= b.TotalBoxes {
		if failed == 0 {
			b.Status = UpgradeStatusCompleted
		} else if completed == 0 {
			b.Status = UpgradeStatusFailed
		} else {
			// 部分成功，部分失败
			b.Status = UpgradeStatusCompleted
		}
		now := time.Now()
		b.CompletedAt = &now
	}
}

// UpgradeVersion 版本记录模型
type UpgradeVersion struct {
	BaseModel
	BoxID       uint      `json:"box_id" gorm:"not null;index"`
	Version     string    `json:"version" gorm:"not null"`
	IsCurrent   bool      `json:"is_current" gorm:"default:false"`
	InstallDate time.Time `json:"install_date" gorm:"not null"`
	FileSize    int64     `json:"file_size"`
	Checksum    string    `json:"checksum"`

	// 升级任务关联
	UpgradeTaskID *uint `json:"upgrade_task_id"`

	// 关联关系
	Box         Box          `json:"box,omitempty" gorm:"foreignKey:BoxID"`
	UpgradeTask *UpgradeTask `json:"upgrade_task,omitempty" gorm:"foreignKey:UpgradeTaskID"`
}

// TableName 设置表名
func (UpgradeVersion) TableName() string {
	return "upgrade_versions"
}

// SetAsCurrent 设置为当前版本
func (v *UpgradeVersion) SetAsCurrent() {
	v.IsCurrent = true
}
