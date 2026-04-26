/*
 * @module service/workflow_instance_service
 * @description 工作流实例服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Controller -> WorkflowInstanceService -> Repository -> Database
 * @rules 实现工作流实例的创建、执行、状态管理等业务逻辑
 * @dependencies repository, models
 * @refs 业务编排引擎需求文档.md 5.3节
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// WorkflowInstanceService 工作流实例服务接口
type WorkflowInstanceService interface {
	// 基础CRUD
	Create(ctx context.Context, instance *models.WorkflowInstance) error
	GetByID(ctx context.Context, id uint) (*models.WorkflowInstance, error)
	Update(ctx context.Context, instance *models.WorkflowInstance) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, page, pageSize int) ([]*models.WorkflowInstance, int64, error)
	ListWithDetails(ctx context.Context, filter *WorkflowInstanceFilter) ([]*WorkflowInstanceDetail, int64, error)

	// 实例创建
	CreateFromWorkflow(ctx context.Context, workflowID uint, boxID *uint, inputVariables map[string]interface{}, triggeredBy string) (*models.WorkflowInstance, error)

	// 状态管理
	Start(ctx context.Context, id uint) error
	Complete(ctx context.Context, id uint) error
	Fail(ctx context.Context, id uint, errorMsg string) error
	Cancel(ctx context.Context, id uint) error
	UpdateProgress(ctx context.Context, id uint, progress float64) error

	// 查询
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowInstance, error)
	FindByBoxID(ctx context.Context, boxID uint) ([]*models.WorkflowInstance, error)
	FindByStatus(ctx context.Context, status models.WorkflowInstanceStatus) ([]*models.WorkflowInstance, error)
	FindRunning(ctx context.Context) ([]*models.WorkflowInstance, error)

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
	GetExecutionReport(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
}

// WorkflowInstanceFilter 列表过滤条件
type WorkflowInstanceFilter struct {
	WorkflowID   uint
	BoxID        uint
	DeploymentID uint
	ScheduleID   uint
	Status       string
	Page         int
	PageSize     int
}

// WorkflowInstanceDetail 带关联名称的工作流实例详情
type WorkflowInstanceDetail struct {
	*models.WorkflowInstance
	WorkflowName   string `json:"workflow_name"`
	BoxName        string `json:"box_name"`
	DeploymentName string `json:"deployment_name"`
	ScheduleName   string `json:"schedule_name"`
}

// workflowInstanceService 工作流实例服务实现
type workflowInstanceService struct {
	instanceRepo     repository.WorkflowInstanceRepository
	workflowRepo     repository.WorkflowRepository
	nodeDefRepo      repository.NodeDefinitionRepository
	nodeInstRepo     repository.NodeInstanceRepository
	variableDefRepo  repository.VariableDefinitionRepository
	variableInstRepo repository.VariableInstanceRepository
	lineDefRepo      repository.LineDefinitionRepository
	lineInstRepo     repository.LineInstanceRepository
	repoManager      repository.RepositoryManager
}

// NewWorkflowInstanceService 创建工作流实例服务实例
func NewWorkflowInstanceService(repoManager repository.RepositoryManager) WorkflowInstanceService {
	return &workflowInstanceService{
		instanceRepo:     repository.NewWorkflowInstanceRepository(repoManager.DB()),
		workflowRepo:     repository.NewWorkflowRepository(repoManager.DB()),
		nodeDefRepo:      repository.NewNodeDefinitionRepository(repoManager.DB()),
		nodeInstRepo:     repository.NewNodeInstanceRepository(repoManager.DB()),
		variableDefRepo:  repository.NewVariableDefinitionRepository(repoManager.DB()),
		variableInstRepo: repository.NewVariableInstanceRepository(repoManager.DB()),
		lineDefRepo:      repository.NewLineDefinitionRepository(repoManager.DB()),
		lineInstRepo:     repository.NewLineInstanceRepository(repoManager.DB()),
		repoManager:      repoManager,
	}
}

// Create 创建工作流实例
func (s *workflowInstanceService) Create(ctx context.Context, instance *models.WorkflowInstance) error {
	return s.instanceRepo.Create(ctx, instance)
}

// GetByID 根据ID获取工作流实例
func (s *workflowInstanceService) GetByID(ctx context.Context, id uint) (*models.WorkflowInstance, error) {
	return s.instanceRepo.GetByID(ctx, id)
}

// Update 更新工作流实例
func (s *workflowInstanceService) Update(ctx context.Context, instance *models.WorkflowInstance) error {
	return s.instanceRepo.Update(ctx, instance)
}

// Delete 删除工作流实例
func (s *workflowInstanceService) Delete(ctx context.Context, id uint) error {
	return s.instanceRepo.Delete(ctx, id)
}

// List 列出工作流实例
func (s *workflowInstanceService) List(ctx context.Context, page, pageSize int) ([]*models.WorkflowInstance, int64, error) {
	return s.instanceRepo.FindWithPagination(ctx, nil, page, pageSize)
}

// ListWithDetails 列出工作流实例（带关联名称）
func (s *workflowInstanceService) ListWithDetails(ctx context.Context, filter *WorkflowInstanceFilter) ([]*WorkflowInstanceDetail, int64, error) {
	// 构建过滤条件
	conditions := map[string]interface{}{}
	if filter.WorkflowID > 0 {
		conditions["workflow_id"] = filter.WorkflowID
	}
	if filter.BoxID > 0 {
		conditions["box_id"] = filter.BoxID
	}
	if filter.DeploymentID > 0 {
		conditions["deployment_id"] = filter.DeploymentID
	}
	if filter.ScheduleID > 0 {
		conditions["schedule_id"] = filter.ScheduleID
	}
	if filter.Status != "" {
		conditions["status"] = filter.Status
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	instances, total, err := s.instanceRepo.FindWithPagination(ctx, conditions, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// 批量预加载关联名称，避免 N+1
	workflowNames := map[uint]string{}
	boxNames := map[uint]string{}
	deploymentNames := map[uint]string{}
	scheduleNames := map[uint]string{}

	for _, inst := range instances {
		if inst.WorkflowID > 0 {
			if _, ok := workflowNames[inst.WorkflowID]; !ok {
				if wf, err := s.workflowRepo.GetByID(ctx, inst.WorkflowID); err == nil {
					workflowNames[inst.WorkflowID] = wf.Name
				}
			}
		}
		if inst.BoxID > 0 {
			if _, ok := boxNames[inst.BoxID]; !ok {
				if box, err := s.repoManager.Box().GetByID(ctx, inst.BoxID); err == nil {
					boxNames[inst.BoxID] = box.Name
				}
			}
		}
		if inst.DeploymentID > 0 {
			if _, ok := deploymentNames[inst.DeploymentID]; !ok {
				if dep, err := s.repoManager.WorkflowDeployment().GetByID(ctx, inst.DeploymentID); err == nil {
					deploymentNames[inst.DeploymentID] = dep.Name
				}
			}
		}
		if inst.ScheduleID > 0 {
			if _, ok := scheduleNames[inst.ScheduleID]; !ok {
				if sch, err := s.repoManager.WorkflowSchedule().GetByID(ctx, inst.ScheduleID); err == nil {
					scheduleNames[inst.ScheduleID] = sch.Name
				}
			}
		}
	}

	details := make([]*WorkflowInstanceDetail, 0, len(instances))
	for _, inst := range instances {
		d := &WorkflowInstanceDetail{
			WorkflowInstance: inst,
			WorkflowName:     workflowNames[inst.WorkflowID],
			BoxName:          boxNames[inst.BoxID],
			DeploymentName:   deploymentNames[inst.DeploymentID],
			ScheduleName:     scheduleNames[inst.ScheduleID],
		}
		details = append(details, d)
	}

	return details, total, nil
}

// CreateFromWorkflow 从工作流创建实例
func (s *workflowInstanceService) CreateFromWorkflow(ctx context.Context, workflowID uint, boxID *uint, inputVariables map[string]interface{}, triggeredBy string) (*models.WorkflowInstance, error) {
	// 获取工作流
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("获取工作流失败: %w", err)
	}

	// 检查工作流状态
	if workflow.Status != models.WorkflowStatusPublished {
		return nil, fmt.Errorf("工作流未发布，无法创建实例")
	}

	if !workflow.IsEnabled {
		return nil, fmt.Errorf("工作流已禁用，无法创建实例")
	}

	var instance *models.WorkflowInstance
	err = s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 创建工作流实例
		instance = &models.WorkflowInstance{
			WorkflowID:  workflowID,
			Status:      models.WorkflowInstanceStatusPending,
			TriggerType: models.TriggerTypeManual,
			CreatedBy:   0, // TODO: Get from context
		}
		if boxID != nil {
			instance.BoxID = *boxID
		}

		if err := s.instanceRepo.Create(ctx, instance); err != nil {
			return fmt.Errorf("创建工作流实例失败: %w", err)
		}

		// 创建节点实例
		nodeDefs, err := s.nodeDefRepo.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return fmt.Errorf("获取节点定义失败: %w", err)
		}

		nodeDefIDMap := make(map[uint]uint) // node_def_id -> node_inst_id
		nodeIDMap := make(map[string]uint)  // node_id -> node_inst_id
		for _, nodeDef := range nodeDefs {
			nodeInst := &models.NodeInstance{
				WorkflowInstanceID: instance.ID,
				NodeDefID:          nodeDef.ID,
				NodeKeyName:        nodeDef.NodeKeyName,
				NodeName:           nodeDef.NodeName,
				NodeType:           nodeDef.TypeKey,
				Status:             models.NodeInstanceStatusPending,
				RetryCount:         0,
			}
			if err := s.nodeInstRepo.Create(ctx, nodeInst); err != nil {
				return fmt.Errorf("创建节点实例失败: %w", err)
			}
			nodeDefIDMap[nodeDef.ID] = nodeInst.ID
			nodeIDMap[nodeDef.NodeID] = nodeInst.ID
		}

		// 创建变量实例
		variableDefs, err := s.variableDefRepo.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return fmt.Errorf("获取变量定义失败: %w", err)
		}

		for _, variableDef := range variableDefs {
			value := variableDef.DefaultValue.Data
			// 如果提供了输入变量，使用输入变量的值
			if inputVariables != nil {
				if inputValue, ok := inputVariables[variableDef.KeyName]; ok {
					value = inputValue
				}
			}

			variableInst := &models.VariableInstance{
				WorkflowInstanceID: instance.ID,
				VariableDefID:      variableDef.ID,
				KeyName:            variableDef.KeyName,
				Name:               variableDef.Name,
				Type:               variableDef.Type,
				Value:              models.ValueJSON{Data: value},
				Scope:              "global", // 默认全局作用域
			}
			if err := s.variableInstRepo.Create(ctx, variableInst); err != nil {
				return fmt.Errorf("创建变量实例失败: %w", err)
			}
		}

		// 创建连接线实例
		lineDefs, err := s.lineDefRepo.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return fmt.Errorf("获取连接线定义失败: %w", err)
		}

		for _, lineDef := range lineDefs {
			lineInst := &models.LineInstance{
				WorkflowInstanceID:  instance.ID,
				LineID:              lineDef.LineID,
				SourceNodeID:        lineDef.SourceNodeID,
				TargetNodeID:        lineDef.TargetNodeID,
				ConditionType:       lineDef.ConditionType,
				LogicType:           lineDef.LogicType,
				ConditionExpression: lineDef.ConditionExpression,
				Executed:            false,
			}
			if err := s.lineInstRepo.Create(ctx, lineInst); err != nil {
				return fmt.Errorf("创建连接线实例失败: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return instance, nil
}

// Start 启动工作流实例
func (s *workflowInstanceService) Start(ctx context.Context, id uint) error {
	return s.instanceRepo.Start(ctx, id)
}

// Complete 完成工作流实例
func (s *workflowInstanceService) Complete(ctx context.Context, id uint) error {
	return s.instanceRepo.Complete(ctx, id)
}

// Fail 失败工作流实例
func (s *workflowInstanceService) Fail(ctx context.Context, id uint, errorMsg string) error {
	return s.instanceRepo.Fail(ctx, id, errorMsg)
}

// Cancel 取消工作流实例
func (s *workflowInstanceService) Cancel(ctx context.Context, id uint) error {
	return s.instanceRepo.Cancel(ctx, id)
}

// UpdateProgress 更新进度
func (s *workflowInstanceService) UpdateProgress(ctx context.Context, id uint, progress float64) error {
	return s.instanceRepo.UpdateProgress(ctx, id, progress)
}

// FindByWorkflowID 根据工作流ID查找实例
func (s *workflowInstanceService) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowInstance, error) {
	return s.instanceRepo.FindByWorkflowID(ctx, workflowID)
}

// FindByBoxID 根据盒子ID查找实例
func (s *workflowInstanceService) FindByBoxID(ctx context.Context, boxID uint) ([]*models.WorkflowInstance, error) {
	return s.instanceRepo.FindByBoxID(ctx, boxID)
}

// FindByStatus 根据状态查找实例
func (s *workflowInstanceService) FindByStatus(ctx context.Context, status models.WorkflowInstanceStatus) ([]*models.WorkflowInstance, error) {
	return s.instanceRepo.FindByStatus(ctx, status)
}

// FindRunning 查找运行中的实例
func (s *workflowInstanceService) FindRunning(ctx context.Context) ([]*models.WorkflowInstance, error) {
	return s.instanceRepo.FindRunning(ctx)
}

// GetStatistics 获取统计信息
func (s *workflowInstanceService) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	return s.instanceRepo.GetStatistics(ctx)
}

// GetExecutionReport 获取执行报告
func (s *workflowInstanceService) GetExecutionReport(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	return s.instanceRepo.GetExecutionReport(ctx, startDate, endDate)
}
