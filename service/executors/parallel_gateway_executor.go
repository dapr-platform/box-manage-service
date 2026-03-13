/*
 * @module service/executors/parallel_gateway_executor
 * @description 并行网关节点执行器（成对节点）
 * @architecture 策略模式 - 并行网关执行器实现
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 并发开始 -> 并行执行多个分支 -> 并发结束（等待所有分支）
 * @rules 并发开始节点触发多个分支并行执行，并发结束节点等待所有分支完成
 * @dependencies models, sync
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"sync"
	"time"
)

// ParallelGatewayStartExecutor 并发开始节点执行器
type ParallelGatewayStartExecutor struct {
	*BaseExecutor
}

// NewParallelGatewayStartExecutor 创建并发开始节点执行器
func NewParallelGatewayStartExecutor() *ParallelGatewayStartExecutor {
	return &ParallelGatewayStartExecutor{
		BaseExecutor: NewBaseExecutor("concurrency_start"),
	}
}

// Execute 执行并发开始节点
func (e *ParallelGatewayStartExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行并发开始节点"}

	// 解析配置
	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("并发模式: %s, 最大并发数: %d", config.Mode, config.MaxConcurrency))

	// 并发开始节点主要是标记并发区域的开始
	// 实际的并发执行由工作流引擎控制
	outputs := map[string]interface{}{
		"concurrency_mode": config.Mode,
		"max_concurrency":  config.MaxConcurrency,
		"started_at":       time.Now().Format(time.RFC3339),
		"branch_count":     0, // 将由引擎填充实际分支数
	}

	logs = append(logs, "并发开始节点执行完成，等待分支执行")
	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *ParallelGatewayStartExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// ParallelGatewayConfig 并发网关配置
type ParallelGatewayConfig struct {
	Mode           string `json:"mode"`            // all: 等待所有分支, any: 任意分支完成即可
	MaxConcurrency int    `json:"max_concurrency"` // 最大并发数，0表示不限制
	Timeout        int    `json:"timeout"`         // 超时时间（秒），0表示不限制
}

// parseConfig 解析配置
func (e *ParallelGatewayStartExecutor) parseConfig(inputs map[string]interface{}) (*ParallelGatewayConfig, error) {
	config := &ParallelGatewayConfig{
		Mode:           "all",
		MaxConcurrency: 0,
		Timeout:        0,
	}

	if mode, ok := inputs["mode"].(string); ok {
		config.Mode = mode
	}

	if maxConcurrency, ok := inputs["max_concurrency"].(float64); ok {
		config.MaxConcurrency = int(maxConcurrency)
	}

	if timeout, ok := inputs["timeout"].(float64); ok {
		config.Timeout = int(timeout)
	}

	return config, nil
}

// ParallelGatewayEndExecutor 并发结束节点执行器
type ParallelGatewayEndExecutor struct {
	*BaseExecutor
	branchResults sync.Map // 存储各分支的执行结果
}

// NewParallelGatewayEndExecutor 创建并发结束节点执行器
func NewParallelGatewayEndExecutor() *ParallelGatewayEndExecutor {
	return &ParallelGatewayEndExecutor{
		BaseExecutor: NewBaseExecutor("concurrency_end"),
	}
}

// Execute 执行并发结束节点
func (e *ParallelGatewayEndExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行并发结束节点"}

	// 解析配置
	config, err := e.parseEndConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("等待模式: %s", config.WaitMode))

	// 获取分支执行结果
	branchResults := e.collectBranchResults(execCtx)
	logs = append(logs, fmt.Sprintf("收集到 %d 个分支结果", len(branchResults)))

	// 根据等待模式处理结果
	var mergedResult map[string]interface{}
	switch config.WaitMode {
	case "all":
		// 等待所有分支完成
		mergedResult = e.mergeAllResults(branchResults)
		logs = append(logs, "所有分支已完成")
	case "any":
		// 任意分支完成即可
		mergedResult = e.mergeAnyResult(branchResults)
		logs = append(logs, "至少一个分支已完成")
	case "first":
		// 第一个完成的分支
		mergedResult = e.getFirstResult(branchResults)
		logs = append(logs, "使用第一个完成的分支结果")
	default:
		mergedResult = e.mergeAllResults(branchResults)
	}

	// 构建输出
	outputs := map[string]interface{}{
		"branch_results": branchResults,
		"merged_result":  mergedResult,
		"branch_count":   len(branchResults),
		"completed_at":   time.Now().Format(time.RFC3339),
		"wait_mode":      config.WaitMode,
	}

	logs = append(logs, "并发结束节点执行完成")
	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *ParallelGatewayEndExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// ParallelGatewayEndConfig 并发结束配置
type ParallelGatewayEndConfig struct {
	WaitMode      string `json:"wait_mode"`      // all: 等待所有, any: 任意一个, first: 第一个
	MergeStrategy string `json:"merge_strategy"` // merge: 合并结果, array: 数组形式, first: 只取第一个
	FailOnError   bool   `json:"fail_on_error"`  // 任意分支失败时是否整体失败
}

// parseEndConfig 解析结束节点配置
func (e *ParallelGatewayEndExecutor) parseEndConfig(inputs map[string]interface{}) (*ParallelGatewayEndConfig, error) {
	config := &ParallelGatewayEndConfig{
		WaitMode:      "all",
		MergeStrategy: "merge",
		FailOnError:   false,
	}

	if waitMode, ok := inputs["wait_mode"].(string); ok {
		config.WaitMode = waitMode
	}

	if mergeStrategy, ok := inputs["merge_strategy"].(string); ok {
		config.MergeStrategy = mergeStrategy
	}

	if failOnError, ok := inputs["fail_on_error"].(bool); ok {
		config.FailOnError = failOnError
	}

	return config, nil
}

// collectBranchResults 收集分支执行结果
func (e *ParallelGatewayEndExecutor) collectBranchResults(execCtx *ExecutionContext) []map[string]interface{} {
	// 从输入中获取分支结果
	// 实际实现中，这些结果应该由工作流引擎传递
	results := []map[string]interface{}{}

	if branchData, ok := execCtx.Inputs["branch_results"].([]interface{}); ok {
		for _, branch := range branchData {
			if branchMap, ok := branch.(map[string]interface{}); ok {
				results = append(results, branchMap)
			}
		}
	}

	return results
}

// mergeAllResults 合并所有分支结果
func (e *ParallelGatewayEndExecutor) mergeAllResults(results []map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	for i, result := range results {
		branchKey := fmt.Sprintf("branch_%d", i)
		merged[branchKey] = result
	}

	return merged
}

// mergeAnyResult 合并任意分支结果
func (e *ParallelGatewayEndExecutor) mergeAnyResult(results []map[string]interface{}) map[string]interface{} {
	if len(results) > 0 {
		return results[0]
	}
	return make(map[string]interface{})
}

// getFirstResult 获取第一个结果
func (e *ParallelGatewayEndExecutor) getFirstResult(results []map[string]interface{}) map[string]interface{} {
	if len(results) > 0 {
		return results[0]
	}
	return make(map[string]interface{})
}
