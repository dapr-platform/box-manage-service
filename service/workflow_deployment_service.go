/*
 * @module service/workflow_deployment_service
 * @description 工作流部署服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Controller -> WorkflowDeploymentService -> BoxClient
 * @rules 实现工作流到盒子的部署和回滚
 * @dependencies repository, models, client
 * @refs 业务编排引擎需求文档.md 5.9节
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"time"
)

// WorkflowDeploymentService 工作流部署服务接口
type WorkflowDeploymentService interface {
	// 部署管理
	Deploy(ctx context.Context, workflowID uint, boxID uint) error
	Rollback(ctx context.Context, deploymentID uint) error
	GetDeployment(ctx context.Context, id uint) (*models.WorkflowDeployment, error)
	ListDeployments(ctx context.Context, workflowID uint) ([]*models.WorkflowDeployment, error)
	ListDeploymentsByBox(ctx context.Context, boxID uint) ([]*models.WorkflowDeployment, error)

	// 批量部署
	DeployToMultipleBoxes(ctx context.Context, workflowID uint, boxIDs []uint) error

	// 部署状态查询
	GetDeploymentStatus(ctx context.Context, workflowID uint, boxID uint) (*models.WorkflowDeployment, error)
}

// workflowDeploymentService 工作流部署服务实现
type workflowDeploymentService struct {
	deploymentRepo repository.WorkflowDeploymentRepository
	workflowRepo   repository.WorkflowRepository
	boxRepo        repository.BoxRepository
	repoManager    repository.RepositoryManager
}

// NewWorkflowDeploymentService 创建工作流部署服务实例
func NewWorkflowDeploymentService(repoManager repository.RepositoryManager) WorkflowDeploymentService {
	return &workflowDeploymentService{
		deploymentRepo: repository.NewWorkflowDeploymentRepository(repoManager.DB()),
		workflowRepo:   repository.NewWorkflowRepository(repoManager.DB()),
		boxRepo:        repoManager.Box(),
		repoManager:    repoManager,
	}
}

// Deploy 部署工作流到盒子
func (s *workflowDeploymentService) Deploy(ctx context.Context, workflowID uint, boxID uint) error {
	// 获取工作流
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("获取工作流失败: %w", err)
	}

	// 检查工作流状态
	if workflow.Status != models.WorkflowStatusPublished {
		return fmt.Errorf("工作流未发布，无法部署")
	}

	// 检查盒子是否在线
	box, err := s.boxRepo.GetByID(ctx, boxID)
	if err != nil {
		return fmt.Errorf("获取盒子失败: %w", err)
	}

	if box.Status != models.BoxStatusOnline {
		return fmt.Errorf("盒子不在线，无法部署")
	}

	// 查找现有部署
	existingDeployment, _ := s.deploymentRepo.GetLatestDeployment(ctx, workflowID, boxID)

	// 创建部署记录
	deployment := &models.WorkflowDeployment{
		WorkflowID:      workflowID,
		BoxID:           boxID,
		WorkflowVersion: workflow.Version,
		Status:          models.DeploymentStatusPending,
	}

	if existingDeployment != nil {
		deployment.PreviousVersion = &existingDeployment.WorkflowVersion
	}

	if err := s.deploymentRepo.Create(ctx, deployment); err != nil {
		return fmt.Errorf("创建部署记录失败: %w", err)
	}

	// 更新状态为部署中
	s.deploymentRepo.UpdateStatus(ctx, deployment.ID, models.DeploymentStatusDeploying)

	// TODO: 调用盒子客户端API进行实际部署
	// 1. 将工作流结构JSON发送到盒子
	// 2. 等待盒子确认部署成功
	// 3. 更新部署状态

	// 模拟部署过程
	time.Sleep(1 * time.Second)

	// 标记为已部署
	if err := s.deploymentRepo.MarkAsDeployed(ctx, deployment.ID); err != nil {
		return fmt.Errorf("更新部署状态失败: %w", err)
	}

	return nil
}

// Rollback 回滚部署
func (s *workflowDeploymentService) Rollback(ctx context.Context, deploymentID uint) error {
	// 获取部署记录
	deployment, err := s.deploymentRepo.GetByID(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("获取部署记录失败: %w", err)
	}

	if deployment.Status != models.DeploymentStatusDeployed {
		return fmt.Errorf("只能回滚已部署的工作流")
	}

	if deployment.PreviousVersion == nil {
		return fmt.Errorf("没有可回滚的版本")
	}

	// TODO: 调用盒子客户端API进行回滚
	// 1. 获取上一个版本的工作流
	// 2. 将上一个版本发送到盒子
	// 3. 等待盒子确认回滚成功

	// 标记为已回滚
	if err := s.deploymentRepo.MarkAsRolledBack(ctx, deploymentID); err != nil {
		return fmt.Errorf("更新部署状态失败: %w", err)
	}

	return nil
}

// GetDeployment 获取部署记录
func (s *workflowDeploymentService) GetDeployment(ctx context.Context, id uint) (*models.WorkflowDeployment, error) {
	return s.deploymentRepo.GetByID(ctx, id)
}

// ListDeployments 列出工作流的部署记录
func (s *workflowDeploymentService) ListDeployments(ctx context.Context, workflowID uint) ([]*models.WorkflowDeployment, error) {
	return s.deploymentRepo.FindByWorkflowID(ctx, workflowID)
}

// ListDeploymentsByBox 列出盒子的部署记录
func (s *workflowDeploymentService) ListDeploymentsByBox(ctx context.Context, boxID uint) ([]*models.WorkflowDeployment, error) {
	return s.deploymentRepo.FindByBoxID(ctx, boxID)
}

// DeployToMultipleBoxes 批量部署到多个盒子
func (s *workflowDeploymentService) DeployToMultipleBoxes(ctx context.Context, workflowID uint, boxIDs []uint) error {
	for _, boxID := range boxIDs {
		if err := s.Deploy(ctx, workflowID, boxID); err != nil {
			// 记录错误但继续部署其他盒子
			fmt.Printf("部署到盒子 %d 失败: %v\n", boxID, err)
		}
	}
	return nil
}

// GetDeploymentStatus 获取部署状态
func (s *workflowDeploymentService) GetDeploymentStatus(ctx context.Context, workflowID uint, boxID uint) (*models.WorkflowDeployment, error) {
	return s.deploymentRepo.GetLatestDeployment(ctx, workflowID, boxID)
}
