/*
 * @module service/executors/start_executor
 * @description 开始节点执行器
 * @architecture 策略模式 - 实现开始节点的执行逻辑
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 开始节点：初始化工作流变量 -> 完成
 * @rules 开始节点不执行实际操作，仅初始化变量
 */

package executors

import (
	"box-manage-service/models"
	"context"
)

// StartExecutor 开始节点执行器
type StartExecutor struct {
	*BaseExecutor
}

// NewStartExecutor 创建开始节点执行器
func NewStartExecutor() *StartExecutor {
	return &StartExecutor{
		BaseExecutor: NewBaseExecutor("start"),
	}
}

// Execute 执行开始节点
func (e *StartExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始节点执行"}

	// 开始节点不执行实际操作，仅标记工作流开始
	extras := map[string]interface{}{
		"status": "started",
	}
	outputs := CreateOutputs(nil, extras)

	logs = append(logs, "工作流已启动")

	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证开始节点配置
func (e *StartExecutor) Validate(nodeInstance *models.NodeInstance) error {
	return e.BaseExecutor.Validate(nodeInstance)
}
