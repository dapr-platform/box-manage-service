/*
 * @module service/executors/loop_executor
 * @description 循环节点执行器（成对节点）
 * @architecture 策略模式 - 循环执行器实现
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 循环开始 -> 执行循环体 -> 判断条件 -> 循环结束
 * @rules 支持for循环、while循环、foreach循环，支持break和continue
 * @dependencies models
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"time"
)

// LoopStartExecutor 循环开始节点执行器
type LoopStartExecutor struct {
	*BaseExecutor
}

// NewLoopStartExecutor 创建循环开始节点执行器
func NewLoopStartExecutor() *LoopStartExecutor {
	return &LoopStartExecutor{
		BaseExecutor: NewBaseExecutor("loop_start"),
	}
}

// Execute 执行循环开始节点
func (e *LoopStartExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行循环开始节点"}

	// 解析配置
	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("循环类型: %s", config.LoopType))

	// 初始化循环上下文
	loopContext := &LoopContext{
		LoopType:      config.LoopType,
		CurrentIndex:  0,
		MaxIterations: config.MaxIterations,
		StartTime:     time.Now(),
		IterationData: make([]interface{}, 0),
	}

	// 根据循环类型初始化
	switch config.LoopType {
	case "for":
		loopContext.TotalCount = config.Count
		logs = append(logs, fmt.Sprintf("For循环: 总次数=%d", config.Count))

	case "while":
		loopContext.Condition = config.Condition
		logs = append(logs, fmt.Sprintf("While循环: 条件=%s", config.Condition))

	case "foreach":
		if items, ok := config.Items.([]interface{}); ok {
			loopContext.Items = items
			loopContext.TotalCount = len(items)
			logs = append(logs, fmt.Sprintf("Foreach循环: 元素数=%d", len(items)))
		} else {
			err := fmt.Errorf("foreach循环需要提供items数组")
			logs = append(logs, err.Error())
			return CreateFailureResult(err, logs), err
		}

	default:
		err := fmt.Errorf("不支持的循环类型: %s", config.LoopType)
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), err
	}

	// 输出循环上下文
	outputs := map[string]interface{}{
		"loop_context":   loopContext,
		"loop_type":      config.LoopType,
		"max_iterations": config.MaxIterations,
		"started_at":     loopContext.StartTime.Format(time.RFC3339),
	}

	logs = append(logs, "循环开始节点执行完成")
	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *LoopStartExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// LoopConfig 循环配置
type LoopConfig struct {
	LoopType      string      `json:"loop_type"`      // for/while/foreach
	Count         int         `json:"count"`          // for循环次数
	Condition     string      `json:"condition"`      // while循环条件
	Items         interface{} `json:"items"`          // foreach循环的数据集
	MaxIterations int         `json:"max_iterations"` // 最大迭代次数（防止死循环）
	Timeout       int         `json:"timeout"`        // 超时时间（秒）
}

// LoopContext 循环上下文
type LoopContext struct {
	LoopType       string        `json:"loop_type"`
	CurrentIndex   int           `json:"current_index"`
	TotalCount     int           `json:"total_count"`
	MaxIterations  int           `json:"max_iterations"`
	Condition      string        `json:"condition"`
	Items          []interface{} `json:"items"`
	CurrentItem    interface{}   `json:"current_item"`
	StartTime      time.Time     `json:"start_time"`
	IterationData  []interface{} `json:"iteration_data"`
	ShouldBreak    bool          `json:"should_break"`
	ShouldContinue bool          `json:"should_continue"`
}

// parseConfig 解析配置
func (e *LoopStartExecutor) parseConfig(inputs map[string]interface{}) (*LoopConfig, error) {
	config := &LoopConfig{
		LoopType:      "for",
		Count:         1,
		MaxIterations: 1000, // 默认最大1000次迭代
		Timeout:       300,  // 默认5分钟超时
	}

	if loopType, ok := inputs["loop_type"].(string); ok {
		config.LoopType = loopType
	}

	if count, ok := inputs["count"].(float64); ok {
		config.Count = int(count)
	}

	if condition, ok := inputs["condition"].(string); ok {
		config.Condition = condition
	}

	if items, ok := inputs["items"]; ok {
		config.Items = items
	}

	if maxIterations, ok := inputs["max_iterations"].(float64); ok {
		config.MaxIterations = int(maxIterations)
	}

	if timeout, ok := inputs["timeout"].(float64); ok {
		config.Timeout = int(timeout)
	}

	return config, nil
}

// LoopEndExecutor 循环结束节点执行器
type LoopEndExecutor struct {
	*BaseExecutor
}

// NewLoopEndExecutor 创建循环结束节点执行器
func NewLoopEndExecutor() *LoopEndExecutor {
	return &LoopEndExecutor{
		BaseExecutor: NewBaseExecutor("loop_end"),
	}
}

// Execute 执行循环结束节点
func (e *LoopEndExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行循环结束节点"}

	// 获取循环上下文
	loopContext, err := e.getLoopContext(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("获取循环上下文失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("循环类型: %s, 当前迭代: %d", loopContext.LoopType, loopContext.CurrentIndex))

	// 检查是否应该继续循环
	shouldContinue, reason := e.shouldContinueLoop(loopContext)

	if shouldContinue {
		logs = append(logs, fmt.Sprintf("继续循环: %s", reason))

		// 更新循环上下文
		loopContext.CurrentIndex++
		if loopContext.LoopType == "foreach" && loopContext.CurrentIndex < len(loopContext.Items) {
			loopContext.CurrentItem = loopContext.Items[loopContext.CurrentIndex]
		}

		// 返回继续循环的信号
		outputs := map[string]interface{}{
			"loop_context":    loopContext,
			"should_continue": true,
			"current_index":   loopContext.CurrentIndex,
			"iteration_count": loopContext.CurrentIndex,
		}

		return CreateSuccessResult(outputs, logs), nil
	}

	// 循环结束
	logs = append(logs, fmt.Sprintf("循环结束: %s", reason))
	duration := time.Since(loopContext.StartTime)

	// 收集所有迭代的结果
	outputs := map[string]interface{}{
		"loop_context":     loopContext,
		"should_continue":  false,
		"total_iterations": loopContext.CurrentIndex,
		"iteration_data":   loopContext.IterationData,
		"duration_ms":      duration.Milliseconds(),
		"completed_at":     time.Now().Format(time.RFC3339),
		"end_reason":       reason,
	}

	logs = append(logs, fmt.Sprintf("循环执行完成，总迭代次数: %d, 耗时: %v", loopContext.CurrentIndex, duration))
	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *LoopEndExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// getLoopContext 获取循环上下文
func (e *LoopEndExecutor) getLoopContext(inputs map[string]interface{}) (*LoopContext, error) {
	if contextData, ok := inputs["loop_context"].(map[string]interface{}); ok {
		context := &LoopContext{
			IterationData: make([]interface{}, 0),
		}

		if loopType, ok := contextData["loop_type"].(string); ok {
			context.LoopType = loopType
		}

		if currentIndex, ok := contextData["current_index"].(float64); ok {
			context.CurrentIndex = int(currentIndex)
		}

		if totalCount, ok := contextData["total_count"].(float64); ok {
			context.TotalCount = int(totalCount)
		}

		if maxIterations, ok := contextData["max_iterations"].(float64); ok {
			context.MaxIterations = int(maxIterations)
		}

		if condition, ok := contextData["condition"].(string); ok {
			context.Condition = condition
		}

		if items, ok := contextData["items"].([]interface{}); ok {
			context.Items = items
		}

		if currentItem, ok := contextData["current_item"]; ok {
			context.CurrentItem = currentItem
		}

		if shouldBreak, ok := contextData["should_break"].(bool); ok {
			context.ShouldBreak = shouldBreak
		}

		if shouldContinue, ok := contextData["should_continue"].(bool); ok {
			context.ShouldContinue = shouldContinue
		}

		// 解析开始时间
		if startTimeStr, ok := contextData["start_time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				context.StartTime = t
			}
		}

		return context, nil
	}

	return nil, fmt.Errorf("未找到循环上下文")
}

// shouldContinueLoop 判断是否应该继续循环
func (e *LoopEndExecutor) shouldContinueLoop(context *LoopContext) (bool, string) {
	// 检查break标志
	if context.ShouldBreak {
		return false, "循环被break中断"
	}

	// 检查最大迭代次数
	if context.CurrentIndex >= context.MaxIterations {
		return false, "达到最大迭代次数"
	}

	// 检查超时
	if time.Since(context.StartTime) > 5*time.Minute {
		return false, "循环超时"
	}

	// 根据循环类型判断
	switch context.LoopType {
	case "for":
		if context.CurrentIndex < context.TotalCount {
			return true, fmt.Sprintf("for循环继续 (%d/%d)", context.CurrentIndex+1, context.TotalCount)
		}
		return false, "for循环完成"

	case "foreach":
		if context.CurrentIndex < len(context.Items) {
			return true, fmt.Sprintf("foreach循环继续 (%d/%d)", context.CurrentIndex+1, len(context.Items))
		}
		return false, "foreach循环完成"

	case "while":
		// while循环需要评估条件
		// 实际实现中应该调用条件评估器
		// 这里简化处理，假设条件在外部已评估
		if conditionResult, ok := context.Items[0].(bool); ok && conditionResult {
			return true, "while条件为真"
		}
		return false, "while条件为假"

	default:
		return false, "未知的循环类型"
	}
}
