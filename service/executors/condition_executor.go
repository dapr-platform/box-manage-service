/*
 * @module service/executors/condition_executor
 * @description 条件判断节点执行器
 * @architecture 策略模式 - 实现条件判断节点的执行逻辑
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 条件判断：评估条件表达式 -> 返回分支结果
 * @rules 支持 ==, !=, >, <, >=, <= 运算符
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ConditionExecutor 条件判断节点执行器
type ConditionExecutor struct {
	*BaseExecutor
}

// NewConditionExecutor 创建条件判断执行器
func NewConditionExecutor() *ConditionExecutor {
	return &ConditionExecutor{
		BaseExecutor: NewBaseExecutor("condition"),
	}
}

// Execute 执行条件判断节点
func (e *ConditionExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"条件判断节点执行"}

	// 获取条件表达式
	condition, ok := execCtx.Inputs["condition"].(string)
	if !ok || condition == "" {
		return CreateFailureResult(fmt.Errorf("条件表达式不能为空"), logs), nil
	}

	logs = append(logs, fmt.Sprintf("评估条件: %s", condition))

	// 评估条件
	result, err := e.evaluateCondition(condition, execCtx.Variables)
	if err != nil {
		logs = append(logs, fmt.Sprintf("条件评估失败: %v", err))
		return CreateFailureResult(err, logs), nil
	}

	// 构建标准输出格式：output 为条件结果，branch 为额外输出
	branch := "false"
	if result {
		branch = "true"
	}
	extras := map[string]interface{}{
		"branch": branch,
	}
	outputs := CreateOutputs(result, extras)

	logs = append(logs, fmt.Sprintf("条件评估结果: %v, 分支: %s", result, branch))

	return CreateSuccessResult(outputs, logs), nil
}

// evaluateCondition 评估条件表达式
func (e *ConditionExecutor) evaluateCondition(condition string, variables map[string]interface{}) (bool, error) {
	// 支持的运算符
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}

	var operator string
	var parts []string

	// 查找运算符
	for _, op := range operators {
		if strings.Contains(condition, op) {
			operator = op
			parts = strings.SplitN(condition, op, 2)
			break
		}
	}

	if operator == "" || len(parts) != 2 {
		return false, fmt.Errorf("无效的条件表达式: %s", condition)
	}

	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])

	// 解析左值
	leftValue, err := e.resolveValue(left, variables)
	if err != nil {
		return false, fmt.Errorf("解析左值失败: %v", err)
	}

	// 解析右值
	rightValue, err := e.resolveValue(right, variables)
	if err != nil {
		return false, fmt.Errorf("解析右值失败: %v", err)
	}

	// 执行比较
	return e.compare(leftValue, rightValue, operator)
}

// resolveValue 解析值（支持变量引用）
func (e *ConditionExecutor) resolveValue(value string, variables map[string]interface{}) (interface{}, error) {
	// 如果是变量引用 ${variable_name}
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		varName := value[2 : len(value)-1]
		if val, ok := variables[varName]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("变量 %s 不存在", varName)
	}

	// 尝试解析为数字
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num, nil
	}

	// 尝试解析为布尔值
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}

	// 去除引号的字符串
	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		return value[1 : len(value)-1], nil
	}

	// 默认作为字符串
	return value, nil
}

// compare 比较两个值
func (e *ConditionExecutor) compare(left, right interface{}, operator string) (bool, error) {
	// 尝试数值比较
	leftNum, leftIsNum := toFloat64(left)
	rightNum, rightIsNum := toFloat64(right)

	if leftIsNum && rightIsNum {
		switch operator {
		case "==":
			return leftNum == rightNum, nil
		case "!=":
			return leftNum != rightNum, nil
		case ">":
			return leftNum > rightNum, nil
		case "<":
			return leftNum < rightNum, nil
		case ">=":
			return leftNum >= rightNum, nil
		case "<=":
			return leftNum <= rightNum, nil
		}
	}

	// 字符串比较
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)

	switch operator {
	case "==":
		return leftStr == rightStr, nil
	case "!=":
		return leftStr != rightStr, nil
	default:
		return false, fmt.Errorf("不支持对非数值类型使用运算符: %s", operator)
	}
}

// toFloat64 尝试转换为float64
func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// Validate 验证条件节点配置
func (e *ConditionExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}

	config := nodeInstance.Config
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if _, ok := config["condition"]; !ok {
		return fmt.Errorf("缺少condition配置")
	}

	return nil
}
