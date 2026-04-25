/*
 * @module service/workflow_executor_service
 * @description 工作流执行引擎服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Controller -> WorkflowExecutor -> NodeExecutor -> VariableManager
 * @rules 实现工作流的执行引擎，协调节点执行、条件评估、流程控制
 * @dependencies repository, models, service
 * @refs 业务编排引擎需求文档.md 5.6节
 */

package service

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
)

// WorkflowExecutorService 工作流执行引擎服务接口
type WorkflowExecutorService interface {
	// 执行工作流实例
	Execute(ctx context.Context, workflowInstanceID uint) error

	// 执行单个节点
	ExecuteNode(ctx context.Context, nodeInstanceID uint) error

	// 停止工作流实例
	Stop(ctx context.Context, workflowInstanceID uint) error

	// 暂停工作流实例
	Pause(ctx context.Context, workflowInstanceID uint) error

	// 恢复工作流实例
	Resume(ctx context.Context, workflowInstanceID uint) error
}

// workflowExecutorService 工作流执行引擎服务实现
type workflowExecutorService struct {
	instanceRepo       repository.WorkflowInstanceRepository
	nodeInstRepo       repository.NodeInstanceRepository
	lineInstRepo       repository.LineInstanceRepository
	logRepo            repository.WorkflowLogRepository
	nodeExecutor       NodeExecutorService
	variableManager    VariableManagerService
	conditionEvaluator ConditionEvaluatorService
	repoManager        repository.RepositoryManager
}

// NewWorkflowExecutorService 创建工作流执行引擎服务实例
func NewWorkflowExecutorService(
	repoManager repository.RepositoryManager,
	nodeExecutor NodeExecutorService,
	variableManager VariableManagerService,
	conditionEvaluator ConditionEvaluatorService,
) WorkflowExecutorService {
	return &workflowExecutorService{
		instanceRepo:       repository.NewWorkflowInstanceRepository(repoManager.DB()),
		nodeInstRepo:       repository.NewNodeInstanceRepository(repoManager.DB()),
		lineInstRepo:       repository.NewLineInstanceRepository(repoManager.DB()),
		logRepo:            repository.NewWorkflowLogRepository(repoManager.DB()),
		nodeExecutor:       nodeExecutor,
		variableManager:    variableManager,
		conditionEvaluator: conditionEvaluator,
		repoManager:        repoManager,
	}
}

// Execute 执行工作流实例
func (s *workflowExecutorService) Execute(ctx context.Context, workflowInstanceID uint) error {
	// 获取工作流实例
	instance, err := s.instanceRepo.GetByID(ctx, workflowInstanceID)
	if err != nil {
		return fmt.Errorf("获取工作流实例失败: %w", err)
	}

	// 检查状态
	if instance.Status != models.WorkflowInstanceStatusPending && instance.Status != models.WorkflowInstanceStatusPaused {
		return fmt.Errorf("工作流实例状态不正确，无法执行: %s", instance.Status)
	}

	// 记录日志
	s.logInfo(ctx, workflowInstanceID, nil, "工作流开始执行")

	// 更新状态为运行中
	if err := s.instanceRepo.Start(ctx, workflowInstanceID); err != nil {
		return fmt.Errorf("更新工作流实例状态失败: %w", err)
	}

	// 查找开始节点
	nodeInstances, err := s.nodeInstRepo.FindByWorkflowInstanceID(ctx, workflowInstanceID)
	if err != nil {
		return fmt.Errorf("获取节点实例失败: %w", err)
	}

	var startNode *models.NodeInstance
	for _, node := range nodeInstances {
		if node.NodeType == "start" {
			startNode = node
			break
		}
	}

	if startNode == nil {
		s.logError(ctx, workflowInstanceID, nil, "未找到开始节点")
		s.instanceRepo.Fail(ctx, workflowInstanceID, "未找到开始节点")
		return fmt.Errorf("未找到开始节点")
	}

	// 从开始节点执行
	if err := s.executeFromNode(ctx, workflowInstanceID, startNode.ID); err != nil {
		s.logError(ctx, workflowInstanceID, nil, fmt.Sprintf("工作流执行失败: %v", err))
		s.instanceRepo.Fail(ctx, workflowInstanceID, err.Error())
		return err
	}

	// 检查是否所有节点都已完成
	allCompleted := true
	for _, node := range nodeInstances {
		if node.NodeType != "end" && node.Status != models.NodeInstanceStatusCompleted && node.Status != models.NodeInstanceStatusSkipped {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		s.logInfo(ctx, workflowInstanceID, nil, "工作流执行完成")
		s.instanceRepo.Complete(ctx, workflowInstanceID)
	}

	return nil
}

// executeFromNode 从指定节点开始执行
func (s *workflowExecutorService) executeFromNode(ctx context.Context, workflowInstanceID uint, nodeInstanceID uint) error {
	// 执行当前节点
	if err := s.ExecuteNode(ctx, nodeInstanceID); err != nil {
		return err
	}

	// 获取节点实例
	nodeInst, err := s.nodeInstRepo.GetByID(ctx, nodeInstanceID)
	if err != nil {
		return err
	}

	// 如果是结束节点，停止执行
	if nodeInst.NodeType == "end" {
		return nil
	}

	// 查找出边（根据源节点ID）
	outgoingLines, err := s.lineInstRepo.FindBySourceNodeID(ctx, workflowInstanceID, nodeInst.NodeID)
	if err != nil {
		return fmt.Errorf("查找出边失败: %w", err)
	}

	// 评估每条出边的条件
	for _, line := range outgoingLines {
		// 评估条件
		conditionResult := true
		var conditionContext string

		if line.ConditionType != "" && line.ConditionType != models.ConditionTypeNone {
			result, err := s.conditionEvaluator.Evaluate(ctx, workflowInstanceID, string(line.ConditionType), line.ConditionExpression, string(line.LogicType))
			if err != nil {
				s.logWarning(ctx, workflowInstanceID, &nodeInstanceID, fmt.Sprintf("条件评估失败: %v", err))
				// 记录错误但继续执行
				s.lineInstRepo.UpdateError(ctx, line.ID, err.Error())
				continue
			}
			conditionResult = result

			// 记录条件上下文（可以包含变量值等信息）
			conditionContext = fmt.Sprintf("expression: %s, result: %v", line.ConditionExpression, result)
		}

		// 更新连接线实例状态
		s.lineInstRepo.Evaluate(ctx, line.ID, conditionResult, conditionContext)

		// 如果条件为真，执行目标节点
		if conditionResult {
			// 根据目标节点ID查找节点实例
			targetNodeInst, err := s.nodeInstRepo.FindByNodeID(ctx, workflowInstanceID, line.TargetNodeID)
			if err != nil {
				s.logWarning(ctx, workflowInstanceID, &nodeInstanceID, fmt.Sprintf("查找目标节点失败: %v", err))
				continue
			}

			if err := s.executeFromNode(ctx, workflowInstanceID, targetNodeInst.ID); err != nil {
				return err
			}
		} else {
			// 条件为假，跳过目标节点
			targetNodeInst, err := s.nodeInstRepo.FindByNodeID(ctx, workflowInstanceID, line.TargetNodeID)
			if err == nil {
				s.nodeInstRepo.Skip(ctx, targetNodeInst.ID)
			}
		}
	}

	return nil
}

// ExecuteNode 执行单个节点
func (s *workflowExecutorService) ExecuteNode(ctx context.Context, nodeInstanceID uint) error {
	// 获取节点实例
	nodeInst, err := s.nodeInstRepo.GetByID(ctx, nodeInstanceID)
	if err != nil {
		return fmt.Errorf("获取节点实例失败: %w", err)
	}

	// 检查状态
	if nodeInst.Status != models.NodeInstanceStatusPending {
		return nil // 节点已执行或跳过
	}

	// 记录日志
	s.logInfo(ctx, nodeInst.WorkflowInstanceID, &nodeInstanceID, fmt.Sprintf("开始执行节点: %s", nodeInst.NodeName))

	// 更新状态为运行中
	if err := s.nodeInstRepo.Start(ctx, nodeInstanceID); err != nil {
		return fmt.Errorf("更新节点状态失败: %w", err)
	}

	// 执行节点
	output, err := s.nodeExecutor.Execute(ctx, nodeInstanceID)
	if err != nil {
		s.logError(ctx, nodeInst.WorkflowInstanceID, &nodeInstanceID, fmt.Sprintf("节点执行失败: %v", err))
		s.nodeInstRepo.Fail(ctx, nodeInstanceID, err.Error())
		return fmt.Errorf("节点执行失败: %w", err)
	}

	// 保存输出
	if err := s.variableManager.SetNodeOutput(ctx, nodeInstanceID, output); err != nil {
		s.logWarning(ctx, nodeInst.WorkflowInstanceID, &nodeInstanceID, fmt.Sprintf("保存节点输出失败: %v", err))
	}

	// 更新状态为完成
	if err := s.nodeInstRepo.Complete(ctx, nodeInstanceID, output); err != nil {
		return fmt.Errorf("更新节点状态失败: %w", err)
	}

	s.logInfo(ctx, nodeInst.WorkflowInstanceID, &nodeInstanceID, fmt.Sprintf("节点执行完成: %s", nodeInst.NodeName))

	return nil
}

// Stop 停止工作流实例
func (s *workflowExecutorService) Stop(ctx context.Context, workflowInstanceID uint) error {
	s.logInfo(ctx, workflowInstanceID, nil, "工作流被停止")
	return s.instanceRepo.Cancel(ctx, workflowInstanceID)
}

// Pause 暂停工作流实例
func (s *workflowExecutorService) Pause(ctx context.Context, workflowInstanceID uint) error {
	instance, err := s.instanceRepo.GetByID(ctx, workflowInstanceID)
	if err != nil {
		return fmt.Errorf("获取工作流实例失败: %w", err)
	}
	if instance.Status != models.WorkflowInstanceStatusRunning {
		return fmt.Errorf("实例状态不是 running，无法暂停: %s", instance.Status)
	}

	// 通知 box-app 暂停
	if instance.BoxID > 0 {
		box, err := s.repoManager.Box().GetByID(ctx, instance.BoxID)
		if err == nil && box != nil {
			boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
			if err := boxClient.PauseWorkflowInstance(ctx, instance.InstanceID); err != nil {
				log.Printf("[WorkflowExecutor] 通知 box-app 暂停失败: %v", err)
			}
		}
	}

	s.logInfo(ctx, workflowInstanceID, nil, "工作流被暂停")
	return s.instanceRepo.UpdateStatus(ctx, workflowInstanceID, models.WorkflowInstanceStatusPaused)
}

// Resume 恢复工作流实例
func (s *workflowExecutorService) Resume(ctx context.Context, workflowInstanceID uint) error {
	instance, err := s.instanceRepo.GetByID(ctx, workflowInstanceID)
	if err != nil {
		return fmt.Errorf("获取工作流实例失败: %w", err)
	}
	if instance.Status != models.WorkflowInstanceStatusPaused {
		return fmt.Errorf("实例状态不是 paused，无法恢复: %s", instance.Status)
	}

	// 通知 box-app 恢复
	if instance.BoxID > 0 {
		box, err := s.repoManager.Box().GetByID(ctx, instance.BoxID)
		if err == nil && box != nil {
			boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
			if err := boxClient.ResumeWorkflowInstance(ctx, instance.InstanceID); err != nil {
				log.Printf("[WorkflowExecutor] 通知 box-app 恢复失败: %v", err)
			}
		}
	}

	s.logInfo(ctx, workflowInstanceID, nil, "工作流恢复执行")
	return s.instanceRepo.UpdateStatus(ctx, workflowInstanceID, models.WorkflowInstanceStatusRunning)
}

// 日志记录辅助方法
func (s *workflowExecutorService) logInfo(ctx context.Context, workflowInstanceID uint, nodeInstanceID *uint, message string) {
	log := &models.WorkflowLog{
		WorkflowInstanceID: workflowInstanceID,
		LogType:            models.LogTypeNode,
		Message:            message,
	}
	if nodeInstanceID != nil {
		// 获取节点实例信息
		nodeInst, err := s.nodeInstRepo.GetByID(ctx, *nodeInstanceID)
		if err == nil {
			log.OperationInstanceID = nodeInst.InstanceID
			log.OperationInstanceName = nodeInst.NodeName
			log.OperationInstanceStatus = string(nodeInst.Status)
		}
	}
	s.logRepo.Create(ctx, log)
}

func (s *workflowExecutorService) logWarning(ctx context.Context, workflowInstanceID uint, nodeInstanceID *uint, message string) {
	log := &models.WorkflowLog{
		WorkflowInstanceID: workflowInstanceID,
		LogType:            models.LogTypeNode,
		Message:            message,
	}
	if nodeInstanceID != nil {
		// 获取节点实例信息
		nodeInst, err := s.nodeInstRepo.GetByID(ctx, *nodeInstanceID)
		if err == nil {
			log.OperationInstanceID = nodeInst.InstanceID
			log.OperationInstanceName = nodeInst.NodeName
			log.OperationInstanceStatus = string(nodeInst.Status)
		}
	}
	s.logRepo.Create(ctx, log)
}

func (s *workflowExecutorService) logError(ctx context.Context, workflowInstanceID uint, nodeInstanceID *uint, message string) {
	log := &models.WorkflowLog{
		WorkflowInstanceID: workflowInstanceID,
		LogType:            models.LogTypeNode,
		Message:            message,
	}
	if nodeInstanceID != nil {
		// 获取节点实例信息
		nodeInst, err := s.nodeInstRepo.GetByID(ctx, *nodeInstanceID)
		if err == nil {
			log.OperationInstanceID = nodeInst.InstanceID
			log.OperationInstanceName = nodeInst.NodeName
			log.OperationInstanceStatus = string(nodeInst.Status)
		}
	}
	s.logRepo.Create(ctx, log)
}
