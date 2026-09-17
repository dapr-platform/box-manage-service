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
	"log"
	"strings"
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

// WorkflowSchedule 工作流调度配置模型
// @Description 工作流的调度配置，定义如何触发工作流执行，一个调度对应一个部署
type WorkflowSchedule struct {
	BaseModel
	WorkflowID     uint               `gorm:"not null;index" json:"workflow_id" example:"1"`
	DeploymentID   uint               `gorm:"not null;index" json:"deployment_id" example:"1"` // 部署ID，一个调度对应一个部署
	Name           string             `gorm:"type:varchar(100);not null" json:"name" example:"每日视频分析"`
	ScheduleType   ScheduleType       `gorm:"type:varchar(20);not null;index" json:"schedule_type" example:"cron"`
	CronExpression string             `gorm:"type:varchar(100)" json:"cron_expression,omitempty" example:"0 0 * * *"`
	InputVariables InputVariablesJSON `gorm:"type:jsonb" json:"input_variables,omitempty"` // workflow start节点输入参数
	ParamOverrides string             `gorm:"type:jsonb" json:"param_overrides,omitempty"` // 参数覆盖，优先级高于Deployment
	EventType      string             `gorm:"type:varchar(50)" json:"event_type,omitempty"`
	EventFilter    JSONMap            `gorm:"type:jsonb" json:"event_filter,omitempty"`
	IsEnabled      bool               `gorm:"not null;default:true;index" json:"is_enabled" example:"true"`
	Priority       int                `gorm:"not null;default:0" json:"priority" example:"0"`
	MaxConcurrent  int                `gorm:"not null;default:1" json:"max_concurrent" example:"1"`
	Timeout        int                `gorm:"not null;default:3600" json:"timeout" example:"3600"`
	RetryPolicy    JSONMap            `gorm:"type:jsonb" json:"retry_policy,omitempty"`
	NextRunTime    *time.Time         `gorm:"index" json:"next_run_time,omitempty" example:"2025-01-27T00:00:00Z"`
	LastRunTime    *time.Time         `json:"last_run_time,omitempty" example:"2025-01-26T12:00:00Z"`
	RunCount       int                `gorm:"not null;default:0" json:"run_count" example:"10"`
	CreatedBy      uint               `gorm:"index" json:"created_by" example:"1"`
	DeploymentIDs  JSONArray          `gorm:"type:jsonb;not null;default:'[]'" json:"deployment_ids,omitempty"` // 部署ID列表（盒端新增字段）
}

// JSONArray jsonb 数组字段，元素为任意 JSON 值。
//
// 引入原因：DeploymentIDs 原先声明为 JSONMap（map[string]interface{}），
// 而该列默认值与线上数据均为 JSON 数组（如 [] / [1,2]），
// 反序列化时必然报 "json: cannot unmarshal array into Go value of type models.JSONMap"。
// 该错误会导致盒子上线恢复（StartupRecovery）在“查询部署 N 的调度失败”处持续失败并无限重试。
//
// 为兼容历史脏数据（可能存过 {} / null），Scan 对空值与非数组值做宽松处理，不返回错误。
type JSONArray []interface{}

// Scan 实现 sql.Scanner 接口
func (a *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*a = JSONArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		// 驱动可能直接给出字符串形式
		if s, isStr := value.(string); isStr {
			bytes = []byte(s)
		} else {
			*a = JSONArray{}
			return nil
		}
	}
	trimmed := strings.TrimSpace(string(bytes))
	if trimmed == "" || trimmed == "null" {
		*a = JSONArray{}
		return nil
	}
	var arr JSONArray
	if err := json.Unmarshal(bytes, &arr); err != nil {
		// 历史数据可能是对象（如 {}），此处不阻断扫描，退化为空数组并记录告警，
		// 避免单行脏数据导致整个恢复流程失败。
		log.Printf("[WorkflowSchedule] deployment_ids 反序列化失败(已退化为空数组): %v, raw=%s", err, trimmed)
		*a = JSONArray{}
		return nil
	}
	*a = arr
	return nil
}

// Value 实现 driver.Valuer 接口
func (a JSONArray) Value() (driver.Value, error) {
	if a == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(a)
}

// InputVariablesJSON workflow start节点输入参数，直接存储 key-value 键值对
type InputVariablesJSON map[string]interface{}

// Scan 实现 sql.Scanner 接口
func (i *InputVariablesJSON) Scan(value interface{}) error {
	if value == nil {
		*i = make(InputVariablesJSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	result := make(InputVariablesJSON)
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	*i = result
	return nil
}

// Value 实现 driver.Valuer 接口
func (i InputVariablesJSON) Value() (driver.Value, error) {
	if len(i) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(i)
}

// TableName 指定表名
func (WorkflowSchedule) TableName() string {
	return "workflow_schedules"
}

// BeforeCreate GORM钩子
func (w *WorkflowSchedule) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	w.CreatedAt = CustomTime{Time: now}
	w.UpdatedAt = CustomTime{Time: now}
	if w.ParamOverrides == "" {
		w.ParamOverrides = "null"
	}
	if w.DeploymentIDs == nil {
		// 该列语义是"部署ID列表"，DB 默认值为 '[]'，此处必须初始化为空数组而非对象，
		// 否则会写入 {} 与列语义/默认值不一致（历史上即因此导致扫描失败）。
		w.DeploymentIDs = JSONArray{}
	}
	return nil
}

// BeforeUpdate GORM钩子
func (w *WorkflowSchedule) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = CustomTime{Time: time.Now()}
	return nil
}

// MarshalJSON 自定义JSON序列化，使 param_overrides 输出为JSON数组而非字符串
func (w WorkflowSchedule) MarshalJSON() ([]byte, error) {
	type Alias WorkflowSchedule
	aux := struct {
		*Alias
		ParamOverrides interface{} `json:"param_overrides,omitempty"`
	}{
		Alias: (*Alias)(&w),
	}

	if w.ParamOverrides != "" {
		var v interface{}
		if err := json.Unmarshal([]byte(w.ParamOverrides), &v); err == nil {
			aux.ParamOverrides = v
		} else {
			aux.ParamOverrides = w.ParamOverrides
		}
	}

	return json.Marshal(aux)
}
