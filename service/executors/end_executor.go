/*
 * @module service/executors/end_executor
 * @description 结束节点执行器
 * @architecture 策略模式 - 实现结束节点的执行逻辑
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 结束节点：收集最终结果 -> 完成
 * @rules 结束节点标记工作流完成，收集最终输出
 */

package executors

import (
	"box-manage-service/models"
	"context"
)

// EndExecutor 结束节点执行器
type EndExecutor struct {
	*BaseExecutor
}

// NewEndExecutor 创建结束节点执行器
func NewEndExecutor() *EndExecutor {
	return &EndExecutor{
		BaseExecutor: NewBaseExecutor("end"),
	}
}

// Execute 执行结束节点
func (e *EndExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"结束节点执行"}

	// 结束节点收集最终结果
	outputs := make(map[string]interface{})
	outputs["status"] = "completed"
	outputs["final_variables"] = execCtx.Variables

	logs = append(logs, "工作流已完成")

	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证结束节点配置
func (e *EndExecutor) Validate(nodeInstance *models.NodeInstance) error {
	return e.BaseExecutor.Validate(nodeInstance)
}
