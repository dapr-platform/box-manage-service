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
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
)

// WorkflowDeploymentDetail 带关联工作流对象的部署详情
type WorkflowDeploymentDetail struct {
	*models.WorkflowDeployment
	Workflow *models.Workflow `json:"workflow,omitempty"` // 关联的工作流对象（含节点和变量）
}

// WorkflowDeploymentService 工作流部署服务接口
type WorkflowDeploymentService interface {
	// 部署管理
	Deploy(ctx context.Context, workflowID uint, boxID uint, paramOverrides string) error
	Rollback(ctx context.Context, deploymentID uint) error
	GetDeployment(ctx context.Context, id uint) (*WorkflowDeploymentDetail, error)
	ListDeployments(ctx context.Context, workflowID uint) ([]*WorkflowDeploymentDetail, error)
	ListDeploymentsByBox(ctx context.Context, boxID uint) ([]*WorkflowDeploymentDetail, error)

	// 分页查询
	ListDeploymentsWithPagination(ctx context.Context, workflowID *uint, boxID *uint, page, pageSize int) ([]*WorkflowDeploymentDetail, int64, error)

	// 批量部署
	DeployToMultipleBoxes(ctx context.Context, workflowID uint, boxIDs []uint, paramOverrides string) error

	// 部署状态查询
	GetDeploymentStatus(ctx context.Context, workflowID uint, boxID uint) (*models.WorkflowDeployment, error)
}

// workflowDeploymentService 工作流部署服务实现
type workflowDeploymentService struct {
	deploymentRepo repository.WorkflowDeploymentRepository
	workflowRepo   repository.WorkflowRepository
	boxRepo        repository.BoxRepository
	nodeRepo       repository.NodeDefinitionRepository
	variableRepo   repository.VariableDefinitionRepository
	lineRepo       repository.LineDefinitionRepository
	repoManager    repository.RepositoryManager
}

// NewWorkflowDeploymentService 创建工作流部署服务实例
func NewWorkflowDeploymentService(repoManager repository.RepositoryManager) WorkflowDeploymentService {
	return &workflowDeploymentService{
		deploymentRepo: repository.NewWorkflowDeploymentRepository(repoManager.DB()),
		workflowRepo:   repository.NewWorkflowRepository(repoManager.DB()),
		boxRepo:        repoManager.Box(),
		nodeRepo:       repository.NewNodeDefinitionRepository(repoManager.DB()),
		variableRepo:   repository.NewVariableDefinitionRepository(repoManager.DB()),
		lineRepo:       repository.NewLineDefinitionRepository(repoManager.DB()),
		repoManager:    repoManager,
	}
}

// Deploy 部署工作流到盒子
func (s *workflowDeploymentService) Deploy(ctx context.Context, workflowID uint, boxID uint, paramOverrides string) error {
	// 获取工作流
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("获取工作流失败: %w", err)
	}

	log.Printf("DistributeDeployment started - payload: %s", workflow.StructureJSONView)

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

	// 查找上一个已部署版本（用于记录 previous_version）
	existingDeployment, _ := s.deploymentRepo.GetLatestDeployment(ctx, workflowID, boxID)

	// 查找现有部署，重复部署时删除旧记录
	existingDeployments, _ := s.deploymentRepo.FindByWorkflowIDAndBoxID(ctx, workflowID, boxID)
	for _, old := range existingDeployments {
		if err := s.deploymentRepo.Delete(ctx, old.ID); err != nil {
			return fmt.Errorf("删除旧部署记录失败: %w", err)
		}
	}

	// 创建部署记录
	deployment := &models.WorkflowDeployment{
		Name:            fmt.Sprintf("%s - v%d", workflow.Name, workflow.Version),
		Key:             fmt.Sprintf("%s_box%d_v%d", workflow.KeyName, boxID, workflow.Version),
		Description:     workflow.Description,
		WorkflowID:      workflowID,
		BoxID:           boxID,
		WorkflowVersion: workflow.Version,
		Status:          models.DeploymentStatusPending,
		WorkflowJSON:    workflow.StructureJSON,
		ParamOverrides:  paramOverrides,
	}

	if existingDeployment != nil {
		deployment.PreviousVersion = &existingDeployment.WorkflowVersion
	}

	if err := s.deploymentRepo.Create(ctx, deployment); err != nil {
		return fmt.Errorf("创建部署记录失败: %w", err)
	}

	// 更新状态为部署中
	s.deploymentRepo.UpdateStatus(ctx, deployment.ID, models.DeploymentStatusDeploying)

	// 获取节点、变量、连接线定义
	nodes, err := s.nodeRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		s.deploymentRepo.UpdateStatus(ctx, deployment.ID, models.DeploymentStatusFailed)
		return fmt.Errorf("获取节点定义失败: %w", err)
	}

	variables, err := s.variableRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		s.deploymentRepo.UpdateStatus(ctx, deployment.ID, models.DeploymentStatusFailed)
		return fmt.Errorf("获取变量定义失败: %w", err)
	}

	lines, err := s.lineRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		s.deploymentRepo.UpdateStatus(ctx, deployment.ID, models.DeploymentStatusFailed)
		return fmt.Errorf("获取连接线定义失败: %w", err)
	}

	// 创建盒子客户端
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	// 构造部署下发数据（直接使用模型定义）
	deploymentData := &client.DeploymentDistributionRequest{
		Deployment: deployment,
		Workflow:   workflow,
		Nodes:      convertToInterfaceSlice(nodes),
		Variables:  convertToInterfaceSlice(variables),
		Lines:      convertToInterfaceSlice(lines),
	}

	// 下发部署配置到盒子
	if err := boxClient.DistributeDeployment(ctx, deploymentData); err != nil {
		s.deploymentRepo.UpdateStatus(ctx, deployment.ID, models.DeploymentStatusFailed)
		return fmt.Errorf("下发部署配置失败: %w", err)
	}

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

	// TODO: 实现回滚逻辑
	// 1. 获取上一个版本的工作流
	// 2. 重新部署上一个版本

	// 标记为已回滚
	if err := s.deploymentRepo.MarkAsRolledBack(ctx, deploymentID); err != nil {
		return fmt.Errorf("更新部署状态失败: %w", err)
	}

	return nil
}

// enrichDeploymentsWithWorkflow 批量加载部署关联的工作流对象（含节点和变量）
func (s *workflowDeploymentService) enrichDeploymentsWithWorkflow(ctx context.Context, deployments []*models.WorkflowDeployment) []*WorkflowDeploymentDetail {
	if len(deployments) == 0 {
		return []*WorkflowDeploymentDetail{}
	}

	// 使用 map 缓存避免重复查询同一工作流
	workflowCache := map[uint]*models.Workflow{}

	for _, dep := range deployments {
		if dep.WorkflowID > 0 {
			if _, ok := workflowCache[dep.WorkflowID]; !ok {
				if wf, err := s.workflowRepo.GetByID(ctx, dep.WorkflowID); err == nil {
					// 解析结构JSON以填充 Nodes、Lines、Variables
					wf.ParseStructureJSON()
					// 清空大字段，避免重复冗余数据
					wf.StructureJSON = ""
					// 保留 StructureJSONView，盒子端需要用于视图绘制
					workflowCache[dep.WorkflowID] = wf
				}
			}
		}
	}

	details := make([]*WorkflowDeploymentDetail, 0, len(deployments))
	for _, dep := range deployments {
		d := &WorkflowDeploymentDetail{
			WorkflowDeployment: dep,
		}
		if wf, ok := workflowCache[dep.WorkflowID]; ok {
			d.Workflow = wf
		}
		details = append(details, d)
	}
	return details
}

// GetDeployment 获取部署记录
func (s *workflowDeploymentService) GetDeployment(ctx context.Context, id uint) (*WorkflowDeploymentDetail, error) {
	deployment, err := s.deploymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	details := s.enrichDeploymentsWithWorkflow(ctx, []*models.WorkflowDeployment{deployment})
	if len(details) == 0 {
		return nil, fmt.Errorf("部署不存在")
	}
	return details[0], nil
}

// ListDeployments 列出工作流的部署记录
func (s *workflowDeploymentService) ListDeployments(ctx context.Context, workflowID uint) ([]*WorkflowDeploymentDetail, error) {
	deployments, err := s.deploymentRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	return s.enrichDeploymentsWithWorkflow(ctx, deployments), nil
}

// ListDeploymentsByBox 列出盒子的部署记录
func (s *workflowDeploymentService) ListDeploymentsByBox(ctx context.Context, boxID uint) ([]*WorkflowDeploymentDetail, error) {
	deployments, err := s.deploymentRepo.FindByBoxID(ctx, boxID)
	if err != nil {
		return nil, err
	}
	return s.enrichDeploymentsWithWorkflow(ctx, deployments), nil
}

// ListDeploymentsWithPagination 分页查询部署记录（workflow_id 和 box_id 均为可选筛选条件）
func (s *workflowDeploymentService) ListDeploymentsWithPagination(ctx context.Context, workflowID *uint, boxID *uint, page, pageSize int) ([]*WorkflowDeploymentDetail, int64, error) {
	deployments, total, err := s.deploymentRepo.FindWithFilters(ctx, workflowID, boxID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return s.enrichDeploymentsWithWorkflow(ctx, deployments), total, nil
}

// DeployToMultipleBoxes 批量部署到多个盒子
func (s *workflowDeploymentService) DeployToMultipleBoxes(ctx context.Context, workflowID uint, boxIDs []uint, paramOverrides string) error {
	for _, boxID := range boxIDs {
		if err := s.Deploy(ctx, workflowID, boxID, paramOverrides); err != nil {
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

// convertToInterfaceSlice 将任意类型的切片转换为 interface{} 切片
func convertToInterfaceSlice(slice interface{}) []interface{} {
	switch v := slice.(type) {
	case []*models.NodeDefinition:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result
	case []*models.VariableDefinition:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result
	case []*models.LineDefinition:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result
	default:
		return []interface{}{}
	}
}
