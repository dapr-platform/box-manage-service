/*
 * @module service/variable_manager_service
 * @description 变量管理服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow WorkflowExecutor -> VariableManager -> Repository -> Database
 * @rules 实现工作流实例运行时的变量管理，支持变量读写和引用解析
 * @dependencies repository, models
 * @refs 业务编排引擎需求文档.md 5.4节
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"regexp"
	"strings"
)

// VariableManagerService 变量管理服务接口
type VariableManagerService interface {
	// 变量操作
	GetVariable(ctx context.Context, workflowInstanceID uint, keyName string) (interface{}, error)
	SetVariable(ctx context.Context, workflowInstanceID uint, keyName string, value interface{}) error
	GetAllVariables(ctx context.Context, workflowInstanceID uint) (map[string]interface{}, error)

	// 变量引用解析
	ResolveReference(ctx context.Context, workflowInstanceID uint, refKeyName string) (interface{}, error)
	ResolveExpression(ctx context.Context, workflowInstanceID uint, expression string) (string, error)

	// 节点输出管理
	GetNodeOutput(ctx context.Context, workflowInstanceID uint, nodeKeyName string, outputKeyName string) (interface{}, error)
	SetNodeOutput(ctx context.Context, nodeInstanceID uint, output map[string]interface{}) error
}

// variableManagerService 变量管理服务实现
type variableManagerService struct {
	variableInstRepo repository.VariableInstanceRepository
	nodeInstRepo     repository.NodeInstanceRepository
}

// NewVariableManagerService 创建变量管理服务实例
func NewVariableManagerService(repoManager repository.RepositoryManager) VariableManagerService {
	return &variableManagerService{
		variableInstRepo: repository.NewVariableInstanceRepository(repoManager.DB()),
		nodeInstRepo:     repository.NewNodeInstanceRepository(repoManager.DB()),
	}
}

// GetVariable 获取变量值
func (s *variableManagerService) GetVariable(ctx context.Context, workflowInstanceID uint, keyName string) (interface{}, error) {
	variableInst, err := s.variableInstRepo.FindByWorkflowInstanceIDAndKeyName(ctx, workflowInstanceID, keyName)
	if err != nil {
		return nil, fmt.Errorf("获取变量失败: %w", err)
	}
	if variableInst == nil {
		return nil, fmt.Errorf("变量 %s 不存在", keyName)
	}
	return variableInst.Value.Value, nil
}

// SetVariable 设置变量值
func (s *variableManagerService) SetVariable(ctx context.Context, workflowInstanceID uint, keyName string, value interface{}) error {
	return s.variableInstRepo.UpdateValueByKeyName(ctx, workflowInstanceID, keyName, value)
}

// GetAllVariables 获取所有变量
func (s *variableManagerService) GetAllVariables(ctx context.Context, workflowInstanceID uint) (map[string]interface{}, error) {
	variables, err := s.variableInstRepo.FindByWorkflowInstanceID(ctx, workflowInstanceID)
	if err != nil {
		return nil, fmt.Errorf("获取变量列表失败: %w", err)
	}

	result := make(map[string]interface{})
	for _, v := range variables {
		result[v.KeyName] = v.Value.Value
	}
	return result, nil
}

// ResolveReference 解析变量引用
// 支持格式：
// - variable_name: 全局变量
// - node_key_name.output_key_name: 节点输出
func (s *variableManagerService) ResolveReference(ctx context.Context, workflowInstanceID uint, refKeyName string) (interface{}, error) {
	if refKeyName == "" {
		return nil, nil
	}

	// 检查是否是节点输出引用（包含点号）
	if strings.Contains(refKeyName, ".") {
		parts := strings.SplitN(refKeyName, ".", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的引用格式: %s", refKeyName)
		}
		nodeKeyName := parts[0]
		outputKeyName := parts[1]
		return s.GetNodeOutput(ctx, workflowInstanceID, nodeKeyName, outputKeyName)
	}

	// 全局变量引用
	return s.GetVariable(ctx, workflowInstanceID, refKeyName)
}

// ResolveExpression 解析表达式中的变量引用
// 将表达式中的 ${variable_name} 或 ${node.output} 替换为实际值
func (s *variableManagerService) ResolveExpression(ctx context.Context, workflowInstanceID uint, expression string) (string, error) {
	// 匹配 ${...} 格式的变量引用
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	result := expression
	matches := re.FindAllStringSubmatch(expression, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		placeholder := match[0] // ${variable_name}
		refKeyName := match[1]  // variable_name

		// 解析引用
		value, err := s.ResolveReference(ctx, workflowInstanceID, refKeyName)
		if err != nil {
			return "", fmt.Errorf("解析引用 %s 失败: %w", refKeyName, err)
		}

		// 替换占位符
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	return result, nil
}

// GetNodeOutput 获取节点输出
func (s *variableManagerService) GetNodeOutput(ctx context.Context, workflowInstanceID uint, nodeKeyName string, outputKeyName string) (interface{}, error) {
	// 查找节点实例
	nodeInstances, err := s.nodeInstRepo.FindByWorkflowInstanceID(ctx, workflowInstanceID)
	if err != nil {
		return nil, fmt.Errorf("查找节点实例失败: %w", err)
	}

	var targetNode *models.NodeInstance
	for _, node := range nodeInstances {
		if node.NodeKeyName == nodeKeyName {
			targetNode = node
			break
		}
	}

	if targetNode == nil {
		return nil, fmt.Errorf("节点 %s 不存在", nodeKeyName)
	}

	// 检查节点是否已完成
	if targetNode.Status != models.NodeInstanceStatusCompleted {
		return nil, fmt.Errorf("节点 %s 尚未完成执行", nodeKeyName)
	}

	// 获取输出值
	if targetNode.OutputData == nil {
		return nil, fmt.Errorf("节点 %s 没有输出数据", nodeKeyName)
	}

	value, ok := targetNode.OutputData[outputKeyName]
	if !ok {
		return nil, fmt.Errorf("节点 %s 的输出中不存在 %s", nodeKeyName, outputKeyName)
	}

	return value, nil
}

// SetNodeOutput 设置节点输出
func (s *variableManagerService) SetNodeOutput(ctx context.Context, nodeInstanceID uint, output map[string]interface{}) error {
	return s.nodeInstRepo.UpdateOutput(ctx, nodeInstanceID, output)
}
