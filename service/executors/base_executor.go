/*
 * @module service/executors/base_executor
 * @description 节点执行器基础接口和通用实现
 * @architecture 策略模式 - 定义节点执行器的统一接口
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 节点执行状态流转：pending -> running -> completed/failed
 * @rules 所有节点执行器必须实现NodeExecutor接口
 * @dependencies models, repository
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
)

// ExecutionContext 节点执行上下文
type ExecutionContext struct {
	WorkflowInstanceID uint                   // 工作流实例ID
	NodeInstanceID     uint                   // 节点实例ID
	Variables          map[string]interface{} // 变量上下文
	Inputs             map[string]interface{} // 节点输入参数
	BoxID              *uint                  // 目标盒子ID（可选）
}

// ExecutionResult 节点执行结果
type ExecutionResult struct {
	Success bool                   // 是否成功
	Outputs map[string]interface{} // 输出参数
	Error   string                 // 错误信息
	Logs    []string               // 执行日志
}

// NodeExecutor 节点执行器接口
type NodeExecutor interface {
	// Execute 执行节点
	Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error)

	// Validate 验证节点配置
	Validate(nodeInstance *models.NodeInstance) error

	// GetType 获取执行器类型
	GetType() string
}

// BaseExecutor 基础执行器（提供通用功能）
type BaseExecutor struct {
	executorType string
}

// NewBaseExecutor 创建基础执行器
func NewBaseExecutor(executorType string) *BaseExecutor {
	return &BaseExecutor{
		executorType: executorType,
	}
}

// GetType 获取执行器类型
func (e *BaseExecutor) GetType() string {
	return e.executorType
}

// Validate 默认验证实现
func (e *BaseExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if nodeInstance == nil {
		return fmt.Errorf("节点实例不能为空")
	}
	if nodeInstance.NodeKeyName == "" {
		return fmt.Errorf("节点key名称不能为空")
	}
	return nil
}

// CreateSuccessResult 创建成功结果
func CreateSuccessResult(outputs map[string]interface{}, logs []string) *ExecutionResult {
	return &ExecutionResult{
		Success: true,
		Outputs: outputs,
		Logs:    logs,
	}
}

// CreateFailureResult 创建失败结果
func CreateFailureResult(err error, logs []string) *ExecutionResult {
	return &ExecutionResult{
		Success: false,
		Error:   err.Error(),
		Logs:    logs,
	}
}
