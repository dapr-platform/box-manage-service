/*
 * @module models/schedule_policy
 * @description 调度策略模型，定义自动调度的策略配置
 * @architecture 模型层
 * @documentReference REQ-005: 任务管理功能
 * @rules 支持多种调度策略类型（优先级、负载均衡、资源匹配）
 * @dependencies gorm.io/gorm
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// SchedulePolicyType 调度策略类型
type SchedulePolicyType string

const (
	// SchedulePolicyTypePriority 优先级策略 - 根据任务优先级调度
	SchedulePolicyTypePriority SchedulePolicyType = "priority"
	// SchedulePolicyTypeLoadBalance 负载均衡策略 - 均衡分配任务到各盒子
	SchedulePolicyTypeLoadBalance SchedulePolicyType = "load_balance"
	// SchedulePolicyTypeResourceMatch 资源匹配策略 - 根据资源需求匹配盒子
	SchedulePolicyTypeResourceMatch SchedulePolicyType = "resource_match"
)

// SchedulePolicy 调度策略
// @Description 自动调度策略配置
type SchedulePolicy struct {
	BaseModel
	Name              string             `json:"name" gorm:"not null;size:255;uniqueIndex"` // 策略名称
	Description       string             `json:"description" gorm:"type:text"`             // 策略描述
	PolicyType        SchedulePolicyType `json:"policy_type" gorm:"not null;size:50"`      // 策略类型
	IsEnabled         bool               `json:"is_enabled" gorm:"default:false"`          // 是否启用
	Priority          int                `json:"priority" gorm:"default:1"`                // 策略优先级(越大越优先)
	TriggerConditions TriggerConditions  `json:"trigger_conditions" gorm:"type:jsonb"`     // 触发条件
	ExecutionConfig   ExecutionConfig    `json:"execution_config" gorm:"type:jsonb"`       // 执行配置
	ScheduleRules     ScheduleRules      `json:"schedule_rules" gorm:"type:jsonb"`         // 调度规则
	CreatedBy         uint               `json:"created_by" gorm:"index"`                  // 创建者ID
}

// TableName 设置表名
func (SchedulePolicy) TableName() string {
	return "schedule_policies"
}

// TriggerConditions 触发条件配置
// @Description 调度触发条件配置
type TriggerConditions struct {
	IntervalSeconds  int    `json:"interval_seconds"`   // 定时间隔（秒），0表示不使用定时触发
	CronExpression   string `json:"cron_expression"`    // Cron 表达式，空表示不使用
	OnNewTask        bool   `json:"on_new_task"`        // 新任务创建时触发
	OnBoxOnline      bool   `json:"on_box_online"`      // 盒子上线时触发
	OnResourceChange bool   `json:"on_resource_change"` // 资源变化时触发
	OnTaskFailed     bool   `json:"on_task_failed"`     // 任务失败时触发（重新调度）
}

// Value 实现 driver.Valuer 接口
func (tc TriggerConditions) Value() (driver.Value, error) {
	return json.Marshal(tc)
}

// Scan 实现 sql.Scanner 接口
func (tc *TriggerConditions) Scan(value interface{}) error {
	if value == nil {
		*tc = TriggerConditions{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into TriggerConditions", value)
	}
	return json.Unmarshal(bytes, tc)
}

// ExecutionConfig 执行配置
// @Description 调度执行配置
type ExecutionConfig struct {
	MaxConcurrent      int  `json:"max_concurrent"`       // 最大并发调度数
	RetryAttempts      int  `json:"retry_attempts"`       // 重试次数
	RetryIntervalSecs  int  `json:"retry_interval_secs"`  // 重试间隔（秒）
	TimeoutSeconds     int  `json:"timeout_seconds"`      // 超时时间（秒）
	SkipOfflineBoxes   bool `json:"skip_offline_boxes"`   // 跳过离线盒子
	AutoStartTask      bool `json:"auto_start_task"`      // 调度后自动启动任务
	ValidateBeforeRun  bool `json:"validate_before_run"`  // 执行前验证任务配置
}

// Value 实现 driver.Valuer 接口
func (ec ExecutionConfig) Value() (driver.Value, error) {
	return json.Marshal(ec)
}

// Scan 实现 sql.Scanner 接口
func (ec *ExecutionConfig) Scan(value interface{}) error {
	if value == nil {
		*ec = ExecutionConfig{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ExecutionConfig", value)
	}
	return json.Unmarshal(bytes, ec)
}

// ScheduleRules 调度规则配置
// @Description 调度规则配置
type ScheduleRules struct {
	// 优先级策略配置
	PriorityWeights map[int]float64 `json:"priority_weights"` // 优先级权重映射(1-5 -> 权重)

	// 负载均衡策略配置
	MaxTasksPerBox  int     `json:"max_tasks_per_box"`  // 每个盒子最大任务数
	CPUThreshold    float64 `json:"cpu_threshold"`      // CPU 使用率阈值(%)
	MemoryThreshold float64 `json:"memory_threshold"`   // 内存使用率阈值(%)
	TPUThreshold    float64 `json:"tpu_threshold"`      // TPU 使用率阈值(%)

	// 资源匹配策略配置
	RequireTagMatch bool    `json:"require_tag_match"` // 是否要求标签匹配
	MinMatchScore   float64 `json:"min_match_score"`   // 最小匹配分数(0-100)

	// 通用配置
	PreferOnlineBoxes   bool `json:"prefer_online_boxes"`   // 优先选择在线盒子
	PreferLessLoadedBox bool `json:"prefer_less_loaded"`    // 优先选择负载较低的盒子
	EnableFallback      bool `json:"enable_fallback"`       // 启用回退策略
}

// Value 实现 driver.Valuer 接口
func (sr ScheduleRules) Value() (driver.Value, error) {
	return json.Marshal(sr)
}

// Scan 实现 sql.Scanner 接口
func (sr *ScheduleRules) Scan(value interface{}) error {
	if value == nil {
		*sr = ScheduleRules{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into ScheduleRules", value)
	}
	return json.Unmarshal(bytes, sr)
}

// DefaultSchedulePolicy 创建默认调度策略
func DefaultSchedulePolicy(policyType SchedulePolicyType) *SchedulePolicy {
	policy := &SchedulePolicy{
		PolicyType: policyType,
		IsEnabled:  true,
		Priority:   1,
		TriggerConditions: TriggerConditions{
			IntervalSeconds: 60, // 默认60秒检查一次
			OnNewTask:       true,
			OnBoxOnline:     true,
		},
		ExecutionConfig: ExecutionConfig{
			MaxConcurrent:     10,
			RetryAttempts:     3,
			RetryIntervalSecs: 30,
			TimeoutSeconds:    300,
			SkipOfflineBoxes:  true,
			AutoStartTask:     true,
			ValidateBeforeRun: true,
		},
	}

	// 根据策略类型设置默认规则
	switch policyType {
	case SchedulePolicyTypePriority:
		policy.Name = "默认优先级策略"
		policy.Description = "根据任务优先级调度，高优先级任务优先分配"
		policy.ScheduleRules = ScheduleRules{
			PriorityWeights: map[int]float64{
				1: 1.0,  // 最低优先级
				2: 1.5,
				3: 2.0,
				4: 3.0,
				5: 5.0,  // 最高优先级
			},
			PreferOnlineBoxes:   true,
			PreferLessLoadedBox: true,
		}

	case SchedulePolicyTypeLoadBalance:
		policy.Name = "默认负载均衡策略"
		policy.Description = "均衡分配任务到各盒子，避免单点过载"
		policy.ScheduleRules = ScheduleRules{
			MaxTasksPerBox:      5,
			CPUThreshold:        80.0,
			MemoryThreshold:     80.0,
			TPUThreshold:        90.0,
			PreferOnlineBoxes:   true,
			PreferLessLoadedBox: true,
			EnableFallback:      true,
		}

	case SchedulePolicyTypeResourceMatch:
		policy.Name = "默认资源匹配策略"
		policy.Description = "根据任务资源需求匹配最合适的盒子"
		policy.ScheduleRules = ScheduleRules{
			RequireTagMatch:     false,
			MinMatchScore:       60.0,
			PreferOnlineBoxes:   true,
			PreferLessLoadedBox: true,
			EnableFallback:      true,
		}
	}

	return policy
}

// Validate 验证调度策略配置
func (sp *SchedulePolicy) Validate() error {
	if sp.Name == "" {
		return fmt.Errorf("策略名称不能为空")
	}

	if sp.PolicyType == "" {
		return fmt.Errorf("策略类型不能为空")
	}

	// 验证策略类型
	switch sp.PolicyType {
	case SchedulePolicyTypePriority, SchedulePolicyTypeLoadBalance, SchedulePolicyTypeResourceMatch:
		// 有效的策略类型
	default:
		return fmt.Errorf("无效的策略类型: %s", sp.PolicyType)
	}

	// 验证执行配置
	if sp.ExecutionConfig.MaxConcurrent < 1 {
		sp.ExecutionConfig.MaxConcurrent = 1
	}
	if sp.ExecutionConfig.TimeoutSeconds < 1 {
		sp.ExecutionConfig.TimeoutSeconds = 300
	}

	// 验证调度规则
	if sp.PolicyType == SchedulePolicyTypeLoadBalance {
		if sp.ScheduleRules.CPUThreshold <= 0 || sp.ScheduleRules.CPUThreshold > 100 {
			sp.ScheduleRules.CPUThreshold = 80.0
		}
		if sp.ScheduleRules.MemoryThreshold <= 0 || sp.ScheduleRules.MemoryThreshold > 100 {
			sp.ScheduleRules.MemoryThreshold = 80.0
		}
		if sp.ScheduleRules.MaxTasksPerBox < 1 {
			sp.ScheduleRules.MaxTasksPerBox = 5
		}
	}

	if sp.PolicyType == SchedulePolicyTypeResourceMatch {
		if sp.ScheduleRules.MinMatchScore < 0 || sp.ScheduleRules.MinMatchScore > 100 {
			sp.ScheduleRules.MinMatchScore = 60.0
		}
	}

	return nil
}

// IsTriggeredByInterval 检查是否通过定时间隔触发
func (sp *SchedulePolicy) IsTriggeredByInterval() bool {
	return sp.TriggerConditions.IntervalSeconds > 0
}

// IsTriggeredByCron 检查是否通过 Cron 表达式触发
func (sp *SchedulePolicy) IsTriggeredByCron() bool {
	return sp.TriggerConditions.CronExpression != ""
}

// IsTriggeredByEvent 检查是否通过事件触发
func (sp *SchedulePolicy) IsTriggeredByEvent() bool {
	return sp.TriggerConditions.OnNewTask ||
		sp.TriggerConditions.OnBoxOnline ||
		sp.TriggerConditions.OnResourceChange ||
		sp.TriggerConditions.OnTaskFailed
}

// CanScheduleToBox 检查是否可以调度到指定盒子（基于资源阈值）
func (sp *SchedulePolicy) CanScheduleToBox(cpuUsage, memoryUsage, tpuUsage float64, currentTasks int) bool {
	rules := sp.ScheduleRules

	// 检查任务数限制
	if rules.MaxTasksPerBox > 0 && currentTasks >= rules.MaxTasksPerBox {
		return false
	}

	// 检查 CPU 阈值
	if rules.CPUThreshold > 0 && cpuUsage >= rules.CPUThreshold {
		return false
	}

	// 检查内存阈值
	if rules.MemoryThreshold > 0 && memoryUsage >= rules.MemoryThreshold {
		return false
	}

	// 检查 TPU 阈值
	if rules.TPUThreshold > 0 && tpuUsage >= rules.TPUThreshold {
		return false
	}

	return true
}

// GetPriorityWeight 获取优先级权重
func (sp *SchedulePolicy) GetPriorityWeight(priority int) float64 {
	if sp.ScheduleRules.PriorityWeights == nil {
		// 默认权重
		defaultWeights := map[int]float64{1: 1.0, 2: 1.5, 3: 2.0, 4: 3.0, 5: 5.0}
		if weight, ok := defaultWeights[priority]; ok {
			return weight
		}
		return 1.0
	}

	if weight, ok := sp.ScheduleRules.PriorityWeights[priority]; ok {
		return weight
	}
	return 1.0
}

