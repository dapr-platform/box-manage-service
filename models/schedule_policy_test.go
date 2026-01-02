/*
 * @module models/schedule_policy_test
 * @description 调度策略模型单元测试
 * @architecture 测试层
 */

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSchedulePolicy_Validate 测试调度策略验证
func TestSchedulePolicy_Validate(t *testing.T) {
	tests := []struct {
		name      string
		policy    *SchedulePolicy
		expectErr bool
	}{
		{
			name: "有效的优先级策略",
			policy: &SchedulePolicy{
				Name:       "测试策略",
				PolicyType: SchedulePolicyTypePriority,
			},
			expectErr: false,
		},
		{
			name: "有效的负载均衡策略",
			policy: &SchedulePolicy{
				Name:       "负载均衡策略",
				PolicyType: SchedulePolicyTypeLoadBalance,
			},
			expectErr: false,
		},
		{
			name: "有效的资源匹配策略",
			policy: &SchedulePolicy{
				Name:       "资源匹配策略",
				PolicyType: SchedulePolicyTypeResourceMatch,
			},
			expectErr: false,
		},
		{
			name: "无策略名称",
			policy: &SchedulePolicy{
				Name:       "",
				PolicyType: SchedulePolicyTypePriority,
			},
			expectErr: true,
		},
		{
			name: "无策略类型",
			policy: &SchedulePolicy{
				Name:       "测试策略",
				PolicyType: "",
			},
			expectErr: true,
		},
		{
			name: "无效的策略类型",
			policy: &SchedulePolicy{
				Name:       "测试策略",
				PolicyType: "invalid_type",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSchedulePolicy_Validate_AutoCorrect 测试验证时的自动修正
func TestSchedulePolicy_Validate_AutoCorrect(t *testing.T) {
	// 测试负载均衡策略的阈值自动修正
	policy := &SchedulePolicy{
		Name:       "测试负载均衡",
		PolicyType: SchedulePolicyTypeLoadBalance,
		ScheduleRules: ScheduleRules{
			CPUThreshold:    0,    // 无效值
			MemoryThreshold: 150,  // 超出范围
			MaxTasksPerBox:  -1,   // 无效值
		},
	}

	err := policy.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 80.0, policy.ScheduleRules.CPUThreshold)
	assert.Equal(t, 80.0, policy.ScheduleRules.MemoryThreshold)
	assert.Equal(t, 5, policy.ScheduleRules.MaxTasksPerBox)

	// 测试资源匹配策略的阈值自动修正
	policy2 := &SchedulePolicy{
		Name:       "测试资源匹配",
		PolicyType: SchedulePolicyTypeResourceMatch,
		ScheduleRules: ScheduleRules{
			MinMatchScore: -10, // 无效值
		},
	}

	err = policy2.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 60.0, policy2.ScheduleRules.MinMatchScore)

	// 测试执行配置的自动修正
	policy3 := &SchedulePolicy{
		Name:       "测试执行配置",
		PolicyType: SchedulePolicyTypePriority,
		ExecutionConfig: ExecutionConfig{
			MaxConcurrent:  0,  // 无效值
			TimeoutSeconds: -1, // 无效值
		},
	}

	err = policy3.Validate()
	assert.NoError(t, err)
	assert.Equal(t, 1, policy3.ExecutionConfig.MaxConcurrent)
	assert.Equal(t, 300, policy3.ExecutionConfig.TimeoutSeconds)
}

// TestSchedulePolicy_DefaultSchedulePolicy 测试创建默认策略
func TestSchedulePolicy_DefaultSchedulePolicy(t *testing.T) {
	tests := []struct {
		name       string
		policyType SchedulePolicyType
		expectName string
	}{
		{
			name:       "默认优先级策略",
			policyType: SchedulePolicyTypePriority,
			expectName: "默认优先级策略",
		},
		{
			name:       "默认负载均衡策略",
			policyType: SchedulePolicyTypeLoadBalance,
			expectName: "默认负载均衡策略",
		},
		{
			name:       "默认资源匹配策略",
			policyType: SchedulePolicyTypeResourceMatch,
			expectName: "默认资源匹配策略",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := DefaultSchedulePolicy(tt.policyType)
			assert.NotNil(t, policy)
			assert.Equal(t, tt.expectName, policy.Name)
			assert.Equal(t, tt.policyType, policy.PolicyType)
			assert.True(t, policy.IsEnabled)
		})
	}
}

// TestSchedulePolicy_IsTriggered 测试触发条件检查
func TestSchedulePolicy_IsTriggered(t *testing.T) {
	// 测试定时触发
	policy1 := &SchedulePolicy{
		TriggerConditions: TriggerConditions{
			IntervalSeconds: 60,
		},
	}
	assert.True(t, policy1.IsTriggeredByInterval())
	assert.False(t, policy1.IsTriggeredByCron())

	// 测试 Cron 触发
	policy2 := &SchedulePolicy{
		TriggerConditions: TriggerConditions{
			CronExpression: "0 * * * *",
		},
	}
	assert.False(t, policy2.IsTriggeredByInterval())
	assert.True(t, policy2.IsTriggeredByCron())

	// 测试事件触发
	policy3 := &SchedulePolicy{
		TriggerConditions: TriggerConditions{
			OnNewTask:   true,
			OnBoxOnline: true,
		},
	}
	assert.True(t, policy3.IsTriggeredByEvent())

	// 测试无触发条件
	policy4 := &SchedulePolicy{
		TriggerConditions: TriggerConditions{},
	}
	assert.False(t, policy4.IsTriggeredByInterval())
	assert.False(t, policy4.IsTriggeredByCron())
	assert.False(t, policy4.IsTriggeredByEvent())
}

// TestSchedulePolicy_CanScheduleToBox 测试盒子调度条件检查
func TestSchedulePolicy_CanScheduleToBox(t *testing.T) {
	policy := &SchedulePolicy{
		ScheduleRules: ScheduleRules{
			MaxTasksPerBox:  5,
			CPUThreshold:    80.0,
			MemoryThreshold: 80.0,
			TPUThreshold:    90.0,
		},
	}

	tests := []struct {
		name         string
		cpuUsage     float64
		memoryUsage  float64
		tpuUsage     float64
		currentTasks int
		canSchedule  bool
	}{
		{
			name:         "所有条件满足",
			cpuUsage:     50.0,
			memoryUsage:  50.0,
			tpuUsage:     50.0,
			currentTasks: 2,
			canSchedule:  true,
		},
		{
			name:         "任务数超限",
			cpuUsage:     50.0,
			memoryUsage:  50.0,
			tpuUsage:     50.0,
			currentTasks: 5,
			canSchedule:  false,
		},
		{
			name:         "CPU超限",
			cpuUsage:     85.0,
			memoryUsage:  50.0,
			tpuUsage:     50.0,
			currentTasks: 2,
			canSchedule:  false,
		},
		{
			name:         "内存超限",
			cpuUsage:     50.0,
			memoryUsage:  85.0,
			tpuUsage:     50.0,
			currentTasks: 2,
			canSchedule:  false,
		},
		{
			name:         "TPU超限",
			cpuUsage:     50.0,
			memoryUsage:  50.0,
			tpuUsage:     95.0,
			currentTasks: 2,
			canSchedule:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.CanScheduleToBox(tt.cpuUsage, tt.memoryUsage, tt.tpuUsage, tt.currentTasks)
			assert.Equal(t, tt.canSchedule, result)
		})
	}
}

// TestSchedulePolicy_GetPriorityWeight 测试优先级权重获取
func TestSchedulePolicy_GetPriorityWeight(t *testing.T) {
	// 测试有自定义权重的情况
	policy1 := &SchedulePolicy{
		ScheduleRules: ScheduleRules{
			PriorityWeights: map[int]float64{
				1: 1.0,
				2: 2.0,
				3: 3.0,
				4: 4.0,
				5: 5.0,
			},
		},
	}

	assert.Equal(t, 1.0, policy1.GetPriorityWeight(1))
	assert.Equal(t, 3.0, policy1.GetPriorityWeight(3))
	assert.Equal(t, 5.0, policy1.GetPriorityWeight(5))
	assert.Equal(t, 1.0, policy1.GetPriorityWeight(6)) // 不存在的优先级返回默认值

	// 测试无自定义权重的情况（使用默认权重）
	policy2 := &SchedulePolicy{
		ScheduleRules: ScheduleRules{},
	}

	assert.Equal(t, 1.0, policy2.GetPriorityWeight(1))
	assert.Equal(t, 2.0, policy2.GetPriorityWeight(3))
	assert.Equal(t, 5.0, policy2.GetPriorityWeight(5))
}

// TestTriggerConditions_JSONMarshaling 测试触发条件的 JSON 序列化
func TestTriggerConditions_JSONMarshaling(t *testing.T) {
	tc := TriggerConditions{
		IntervalSeconds:  60,
		CronExpression:   "0 * * * *",
		OnNewTask:        true,
		OnBoxOnline:      false,
		OnResourceChange: true,
		OnTaskFailed:     false,
	}

	// 测试 Value 方法
	value, err := tc.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)

	// 测试 Scan 方法
	var tc2 TriggerConditions
	err = tc2.Scan(value)
	assert.NoError(t, err)
	assert.Equal(t, tc.IntervalSeconds, tc2.IntervalSeconds)
	assert.Equal(t, tc.CronExpression, tc2.CronExpression)
	assert.Equal(t, tc.OnNewTask, tc2.OnNewTask)
}

// TestExecutionConfig_JSONMarshaling 测试执行配置的 JSON 序列化
func TestExecutionConfig_JSONMarshaling(t *testing.T) {
	ec := ExecutionConfig{
		MaxConcurrent:     10,
		RetryAttempts:     3,
		RetryIntervalSecs: 30,
		TimeoutSeconds:    300,
		SkipOfflineBoxes:  true,
		AutoStartTask:     true,
		ValidateBeforeRun: true,
	}

	// 测试 Value 方法
	value, err := ec.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)

	// 测试 Scan 方法
	var ec2 ExecutionConfig
	err = ec2.Scan(value)
	assert.NoError(t, err)
	assert.Equal(t, ec.MaxConcurrent, ec2.MaxConcurrent)
	assert.Equal(t, ec.TimeoutSeconds, ec2.TimeoutSeconds)
}

// TestScheduleRules_JSONMarshaling 测试调度规则的 JSON 序列化
func TestScheduleRules_JSONMarshaling(t *testing.T) {
	sr := ScheduleRules{
		PriorityWeights:     map[int]float64{1: 1.0, 2: 2.0},
		MaxTasksPerBox:      5,
		CPUThreshold:        80.0,
		MemoryThreshold:     80.0,
		TPUThreshold:        90.0,
		RequireTagMatch:     true,
		MinMatchScore:       60.0,
		PreferOnlineBoxes:   true,
		PreferLessLoadedBox: true,
		EnableFallback:      true,
	}

	// 测试 Value 方法
	value, err := sr.Value()
	assert.NoError(t, err)
	assert.NotNil(t, value)

	// 测试 Scan 方法
	var sr2 ScheduleRules
	err = sr2.Scan(value)
	assert.NoError(t, err)
	assert.Equal(t, sr.MaxTasksPerBox, sr2.MaxTasksPerBox)
	assert.Equal(t, sr.CPUThreshold, sr2.CPUThreshold)
	assert.Equal(t, sr.RequireTagMatch, sr2.RequireTagMatch)
}

// TestTriggerConditions_ScanNil 测试空值扫描
func TestTriggerConditions_ScanNil(t *testing.T) {
	var tc TriggerConditions
	err := tc.Scan(nil)
	assert.NoError(t, err)
	assert.Equal(t, TriggerConditions{}, tc)
}

// TestExecutionConfig_ScanNil 测试空值扫描
func TestExecutionConfig_ScanNil(t *testing.T) {
	var ec ExecutionConfig
	err := ec.Scan(nil)
	assert.NoError(t, err)
	assert.Equal(t, ExecutionConfig{}, ec)
}

// TestScheduleRules_ScanNil 测试空值扫描
func TestScheduleRules_ScanNil(t *testing.T) {
	var sr ScheduleRules
	err := sr.Scan(nil)
	assert.NoError(t, err)
	assert.Equal(t, ScheduleRules{}, sr)
}

