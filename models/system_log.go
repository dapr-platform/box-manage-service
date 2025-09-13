/*
 * @module models/system_log
 * @description 系统日志数据模型
 * @architecture 数据模型层
 * @documentReference DESIGN-000.md
 * @stateFlow 日志创建 -> 存储 -> 查询 -> 清理
 * @rules 记录系统运行日志，支持按级别、来源、时间查询和自动清理
 * @dependencies gorm.io/gorm
 */

package models

import (
	"time"

	"gorm.io/gorm"
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// SystemLog 系统日志模型
type SystemLog struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Level     LogLevel       `gorm:"type:varchar(20);not null;index" json:"level"`        // 日志级别
	Source    string         `gorm:"type:varchar(100);not null;index" json:"source"`      // 日志来源（服务名称）
	SourceID  string         `gorm:"type:varchar(100);index" json:"source_id,omitempty"`  // 来源ID（如任务ID、盒子ID等）
	Title     string         `gorm:"type:varchar(200);not null" json:"title"`             // 日志标题
	Message   string         `gorm:"type:text;not null" json:"message"`                   // 日志消息
	Details   string         `gorm:"type:text" json:"details,omitempty"`                  // 详细信息（如错误堆栈）
	UserID    *uint          `gorm:"index" json:"user_id,omitempty"`                      // 关联用户ID
	RequestID string         `gorm:"type:varchar(100);index" json:"request_id,omitempty"` // 请求ID（用于追踪）
	Metadata  *string        `gorm:"type:jsonb" json:"metadata,omitempty"`                // 元数据（JSON格式）
	CreatedAt time.Time      `gorm:"index" json:"created_at"`                             // 创建时间
	UpdatedAt time.Time      `json:"updated_at"`                                          // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`                   // 软删除时间
}

// TableName 指定表名
func (SystemLog) TableName() string {
	return "system_logs"
}

// IsError 判断是否为错误级别日志
func (l *SystemLog) IsError() bool {
	return l.Level == LogLevelError || l.Level == LogLevelFatal
}

// IsWarning 判断是否为警告级别日志
func (l *SystemLog) IsWarning() bool {
	return l.Level == LogLevelWarn
}

// GetLevelPriority 获取日志级别优先级（数字越大优先级越高）
func (l *SystemLog) GetLevelPriority() int {
	switch l.Level {
	case LogLevelDebug:
		return 1
	case LogLevelInfo:
		return 2
	case LogLevelWarn:
		return 3
	case LogLevelError:
		return 4
	case LogLevelFatal:
		return 5
	default:
		return 0
	}
}
