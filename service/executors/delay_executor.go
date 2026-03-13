/*
 * @module service/executors/delay_executor
 * @description 延时节点执行器
 * @architecture 策略模式 - 延时执行器实现
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 开始延时 -> 等待指定时间 -> 完成
 * @rules 支持秒、分钟、小时等时间单位，支持动态延时时间
 * @dependencies time, models
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"time"
)

// DelayExecutor 延时执行器
type DelayExecutor struct {
	*BaseExecutor
}

// NewDelayExecutor 创建延时执行器
func NewDelayExecutor() *DelayExecutor {
	return &DelayExecutor{
		BaseExecutor: NewBaseExecutor("delay"),
	}
}

// Execute 执行延时
func (e *DelayExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行延时节点"}

	// 解析配置
	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 计算延时时长
	duration, err := e.calculateDuration(config)
	if err != nil {
		logs = append(logs, fmt.Sprintf("计算延时时长失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("延时时长: %v", duration))

	// 执行延时
	startTime := time.Now()
	select {
	case <-time.After(duration):
		actualDuration := time.Since(startTime)
		logs = append(logs, fmt.Sprintf("延时完成，实际耗时: %v", actualDuration))

		outputs := map[string]interface{}{
			"delay_duration_ms":  duration.Milliseconds(),
			"actual_duration_ms": actualDuration.Milliseconds(),
			"completed_at":       time.Now().Format(time.RFC3339),
		}

		return CreateSuccessResult(outputs, logs), nil

	case <-ctx.Done():
		logs = append(logs, "延时被取消")
		return CreateFailureResult(ctx.Err(), logs), ctx.Err()
	}
}

// Validate 验证节点配置
func (e *DelayExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// DelayConfig 延时配置
type DelayConfig struct {
	Duration int    `json:"duration"` // 延时时长
	Unit     string `json:"unit"`     // 时间单位: seconds/minutes/hours
}

// parseConfig 解析配置
func (e *DelayExecutor) parseConfig(inputs map[string]interface{}) (*DelayConfig, error) {
	config := &DelayConfig{
		Duration: 1,
		Unit:     "seconds",
	}

	if duration, ok := inputs["duration"].(float64); ok {
		config.Duration = int(duration)
	} else if duration, ok := inputs["duration"].(int); ok {
		config.Duration = duration
	}

	if unit, ok := inputs["unit"].(string); ok {
		config.Unit = unit
	}

	if config.Duration <= 0 {
		return nil, fmt.Errorf("延时时长必须大于0")
	}

	return config, nil
}

// calculateDuration 计算延时时长
func (e *DelayExecutor) calculateDuration(config *DelayConfig) (time.Duration, error) {
	switch config.Unit {
	case "seconds", "s":
		return time.Duration(config.Duration) * time.Second, nil
	case "minutes", "m":
		return time.Duration(config.Duration) * time.Minute, nil
	case "hours", "h":
		return time.Duration(config.Duration) * time.Hour, nil
	case "milliseconds", "ms":
		return time.Duration(config.Duration) * time.Millisecond, nil
	default:
		return 0, fmt.Errorf("不支持的时间单位: %s", config.Unit)
	}
}
