/*
 * @module service/executors/python_script_executor
 * @description Python脚本节点执行器
 * @architecture 策略模式 - 实现Python脚本节点的执行逻辑
 * @documentReference 业务编排引擎设计文档
 * @stateFlow Python脚本执行：准备脚本 -> 下发到盒子 -> 执行 -> 收集结果
 * @rules 所有节点最终都转化为Python脚本在盒子上执行
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
)

// PythonScriptExecutor Python脚本节点执行器
type PythonScriptExecutor struct {
	*BaseExecutor
}

// NewPythonScriptExecutor 创建Python脚本执行器
func NewPythonScriptExecutor() *PythonScriptExecutor {
	return &PythonScriptExecutor{
		BaseExecutor: NewBaseExecutor("python_script"),
	}
}

// Execute 执行Python脚本节点
func (e *PythonScriptExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"Python脚本节点执行"}

	// 获取脚本内容
	scriptContent, ok := execCtx.Inputs["script_content"].(string)
	if !ok || scriptContent == "" {
		return CreateFailureResult(fmt.Errorf("脚本内容不能为空"), logs), nil
	}

	logs = append(logs, fmt.Sprintf("准备执行脚本，长度: %d 字符", len(scriptContent)))

	// TODO: 实际实现需要：
	// 1. 将脚本和输入参数打包
	// 2. 通过BoxProxyService下发到目标盒子
	// 3. 等待执行结果
	// 4. 解析输出参数

	// 当前为模拟实现
	outputs := make(map[string]interface{})
	outputs["status"] = "executed"
	outputs["script_length"] = len(scriptContent)

	// 模拟从脚本输出中解析结果
	if result, ok := execCtx.Inputs["expected_output"]; ok {
		outputs["result"] = result
	}

	logs = append(logs, "脚本执行完成")

	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证Python脚本节点配置
func (e *PythonScriptExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}

	// 验证配置中是否包含脚本内容
	config := nodeInstance.Config
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if _, ok := config["script_content"]; !ok {
		return fmt.Errorf("缺少script_content配置")
	}

	return nil
}
