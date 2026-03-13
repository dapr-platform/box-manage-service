/*
 * @module service/condition_evaluator_service
 * @description 条件评估服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow WorkflowExecutor -> ConditionEvaluator -> VariableManager
 * @rules 实现连接线条件的评估，支持表达式计算
 * @dependencies repository, models
 * @refs 业务编排引擎需求文档.md 5.5节
 */

package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ConditionEvaluatorService 条件评估服务接口
type ConditionEvaluatorService interface {
	// 评估条件
	Evaluate(ctx context.Context, workflowInstanceID uint, conditionType string, expression string) (bool, error)

	// 评估表达式
	EvaluateExpression(ctx context.Context, workflowInstanceID uint, expression string) (bool, error)
}

// conditionEvaluatorService 条件评估服务实现
type conditionEvaluatorService struct {
	variableManager VariableManagerService
}

// NewConditionEvaluatorService 创建条件评估服务实例
func NewConditionEvaluatorService(variableManager VariableManagerService) ConditionEvaluatorService {
	return &conditionEvaluatorService{
		variableManager: variableManager,
	}
}

// Evaluate 评估条件
func (s *conditionEvaluatorService) Evaluate(ctx context.Context, workflowInstanceID uint, conditionType string, expression string) (bool, error) {
	if conditionType == "" || expression == "" {
		// 无条件，默认为true
		return true, nil
	}

	switch conditionType {
	case "expression":
		return s.EvaluateExpression(ctx, workflowInstanceID, expression)
	case "always":
		return true, nil
	case "never":
		return false, nil
	default:
		return false, fmt.Errorf("不支持的条件类型: %s", conditionType)
	}
}

// EvaluateExpression 评估表达式
// 支持的表达式格式：
// - variable > 10
// - node.output == "success"
// - variable1 >= variable2
// - node.confidence > 0.8
func (s *conditionEvaluatorService) EvaluateExpression(ctx context.Context, workflowInstanceID uint, expression string) (bool, error) {
	// 解析表达式中的变量引用
	resolvedExpr, err := s.variableManager.ResolveExpression(ctx, workflowInstanceID, expression)
	if err != nil {
		return false, fmt.Errorf("解析表达式失败: %w", err)
	}

	// 简单的表达式解析和计算
	// 支持的操作符: ==, !=, >, <, >=, <=
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}

	for _, op := range operators {
		if strings.Contains(resolvedExpr, op) {
			parts := strings.SplitN(resolvedExpr, op, 2)
			if len(parts) != 2 {
				continue
			}

			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			return s.compareValues(left, right, op)
		}
	}

	// 如果没有操作符，尝试将表达式作为布尔值
	return s.parseBoolean(resolvedExpr)
}

// compareValues 比较两个值
func (s *conditionEvaluatorService) compareValues(left, right, operator string) (bool, error) {
	// 去除引号
	left = strings.Trim(left, `"'`)
	right = strings.Trim(right, `"'`)

	// 尝试作为数字比较
	leftNum, leftErr := strconv.ParseFloat(left, 64)
	rightNum, rightErr := strconv.ParseFloat(right, 64)

	if leftErr == nil && rightErr == nil {
		// 数字比较
		return s.compareNumbers(leftNum, rightNum, operator), nil
	}

	// 字符串比较
	return s.compareStrings(left, right, operator), nil
}

// compareNumbers 比较数字
func (s *conditionEvaluatorService) compareNumbers(left, right float64, operator string) bool {
	switch operator {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

// compareStrings 比较字符串
func (s *conditionEvaluatorService) compareStrings(left, right, operator string) bool {
	switch operator {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

// parseBoolean 解析布尔值
func (s *conditionEvaluatorService) parseBoolean(value string) (bool, error) {
	value = strings.ToLower(strings.TrimSpace(value))

	switch value {
	case "true", "1", "yes", "y":
		return true, nil
	case "false", "0", "no", "n", "":
		return false, nil
	default:
		return false, fmt.Errorf("无法解析为布尔值: %s", value)
	}
}
