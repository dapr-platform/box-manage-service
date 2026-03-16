/*
 * @module service/workflow_service
 * @description 工作流服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Controller -> WorkflowService -> Repository -> Database
 * @rules 实现工作流的创建、更新、版本管理、发布等业务逻辑
 * @dependencies repository, models
 * @refs 业务编排引擎需求文档.md 5.1节
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"

	"gorm.io/gorm"
)

// WorkflowService 工作流服务接口
type WorkflowService interface {
	// 基础CRUD
	Create(ctx context.Context, workflow *models.Workflow) error
	GetByID(ctx context.Context, id uint) (*models.Workflow, error)
	Update(ctx context.Context, workflow *models.Workflow) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error)

	// 版本管理
	CreateNewVersion(ctx context.Context, keyName string, workflow *models.Workflow) error
	GetLatestVersion(ctx context.Context, keyName string) (*models.Workflow, error)
	GetAllVersions(ctx context.Context, keyName string) ([]*models.Workflow, error)
	GetByKeyNameAndVersion(ctx context.Context, keyName string, version int) (*models.Workflow, error)

	// 状态管理
	Publish(ctx context.Context, id uint) error
	Archive(ctx context.Context, id uint) error
	Enable(ctx context.Context, id uint) error
	Disable(ctx context.Context, id uint) error

	// 查询
	Search(ctx context.Context, keyword string, page, pageSize int) ([]*models.Workflow, int64, error)
	FindByCategory(ctx context.Context, category string) ([]*models.Workflow, error)
	FindByStatus(ctx context.Context, status models.WorkflowStatus) ([]*models.Workflow, error)

	// 结构管理
	ValidateStructure(ctx context.Context, workflow *models.Workflow) error
	SyncStructureToDefinitions(ctx context.Context, workflowID uint) error
	SyncDefinitionsToStructure(ctx context.Context, workflowID uint) error

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// workflowService 工作流服务实现
type workflowService struct {
	workflowRepo     repository.WorkflowRepository
	nodeDefRepo      repository.NodeDefinitionRepository
	lineDefRepo      repository.LineDefinitionRepository
	variableDefRepo  repository.VariableDefinitionRepository
	nodeTemplateRepo repository.NodeTemplateRepository
	repoManager      repository.RepositoryManager
}

// NewWorkflowService 创建工作流服务实例
func NewWorkflowService(repoManager repository.RepositoryManager) WorkflowService {
	return &workflowService{
		workflowRepo:     repository.NewWorkflowRepository(repoManager.DB()),
		nodeDefRepo:      repository.NewNodeDefinitionRepository(repoManager.DB()),
		lineDefRepo:      repository.NewLineDefinitionRepository(repoManager.DB()),
		variableDefRepo:  repository.NewVariableDefinitionRepository(repoManager.DB()),
		nodeTemplateRepo: repository.NewNodeTemplateRepository(repoManager.DB()),
		repoManager:      repoManager,
	}
}

// Create 创建工作流
func (s *workflowService) Create(ctx context.Context, workflow *models.Workflow) error {
	// 验证结构
	if err := s.ValidateStructure(ctx, workflow); err != nil {
		return fmt.Errorf("工作流结构验证失败: %w", err)
	}

	// 如果没有指定版本，获取下一个版本号
	if workflow.Version == 0 {
		nextVersion, err := s.workflowRepo.GetNextVersion(ctx, workflow.KeyName)
		if err != nil {
			return fmt.Errorf("获取版本号失败: %w", err)
		}
		workflow.Version = nextVersion
	}

	// 使用事务处理整个创建流程
	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 1. 先保存workflow基本信息（不包含关联对象）
		workflowToSave := &models.Workflow{
			KeyName:           workflow.KeyName,
			Name:              workflow.Name,
			Description:       workflow.Description,
			Category:          workflow.Category,
			Tags:              workflow.Tags,
			Version:           workflow.Version,
			Status:            workflow.Status,
			IsEnabled:         workflow.IsEnabled,
			CreatedBy:         workflow.CreatedBy,
			UpdatedBy:         workflow.UpdatedBy,
			StructureJSONView: workflow.StructureJSONView,
		}

		if err := s.workflowRepo.Create(ctx, workflowToSave); err != nil {
			return fmt.Errorf("创建工作流失败: %w", err)
		}

		// 将生成的ID赋值回原workflow对象
		workflow.ID = workflowToSave.ID

		// 2. 更新关联对象的WorkflowID并保存到各自的表
		if len(workflow.Nodes) > 0 {
			for i := range workflow.Nodes {
				workflow.Nodes[i].WorkflowID = workflow.ID
			}
			if err := s.nodeDefRepo.CreateBatchForWorkflow(ctx, workflow.ID, convertToNodeDefPointers(workflow.Nodes)); err != nil {
				return fmt.Errorf("保存节点定义失败: %w", err)
			}
		}

		if len(workflow.Lines) > 0 {
			for i := range workflow.Lines {
				workflow.Lines[i].WorkflowID = workflow.ID
			}
			if err := s.lineDefRepo.CreateBatchForWorkflow(ctx, workflow.ID, convertToLineDefPointers(workflow.Lines)); err != nil {
				return fmt.Errorf("保存连接线定义失败: %w", err)
			}
		}

		if len(workflow.Variables) > 0 {
			for i := range workflow.Variables {
				workflow.Variables[i].WorkflowID = workflow.ID
			}
			if err := s.variableDefRepo.CreateBatchForWorkflow(ctx, workflow.ID, convertToVariableDefPointers(workflow.Variables)); err != nil {
				return fmt.Errorf("保存变量定义失败: %w", err)
			}
		}

		// 3. 从关联对象构建StructureJSON字符串
		if err := workflow.BuildStructureJSON(); err != nil {
			return fmt.Errorf("构建StructureJSON失败: %w", err)
		}

		// 4. 更新workflow的StructureJSON字段
		if err := s.workflowRepo.Update(ctx, workflow); err != nil {
			return fmt.Errorf("更新StructureJSON失败: %w", err)
		}

		return nil
	})
}

// GetByID 根据ID获取工作流（包含关联对象）
func (s *workflowService) GetByID(ctx context.Context, id uint) (*models.Workflow, error) {
	// 1. 查询工作流基本信息
	workflow, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return nil, nil
	}

	// 2. 查询关联的定义对象
	nodeDefs, err := s.nodeDefRepo.FindByWorkflowID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询节点定义失败: %w", err)
	}

	lineDefs, err := s.lineDefRepo.FindByWorkflowID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询连接线定义失败: %w", err)
	}

	variableDefs, err := s.variableDefRepo.FindByWorkflowID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询变量定义失败: %w", err)
	}

	// 3. 转换为值切片并设置到workflow对象
	workflow.Nodes = make([]models.NodeDefinition, len(nodeDefs))
	for i, node := range nodeDefs {
		workflow.Nodes[i] = *node
	}

	workflow.Lines = make([]models.LineDefinition, len(lineDefs))
	for i, line := range lineDefs {
		workflow.Lines[i] = *line
	}

	workflow.Variables = make([]models.VariableDefinition, len(variableDefs))
	for i, variable := range variableDefs {
		workflow.Variables[i] = *variable
	}

	return workflow, nil
}

// Update 更新工作流
func (s *workflowService) Update(ctx context.Context, workflow *models.Workflow) error {
	// 验证结构
	if err := s.ValidateStructure(ctx, workflow); err != nil {
		return fmt.Errorf("工作流结构验证失败: %w", err)
	}

	// 使用事务处理整个更新流程
	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 1. 先更新workflow基本信息
		workflowToUpdate := &models.Workflow{
			BaseModel:         workflow.BaseModel,
			KeyName:           workflow.KeyName,
			Name:              workflow.Name,
			Description:       workflow.Description,
			Category:          workflow.Category,
			Tags:              workflow.Tags,
			Version:           workflow.Version,
			Status:            workflow.Status,
			IsEnabled:         workflow.IsEnabled,
			CreatedBy:         workflow.CreatedBy,
			UpdatedBy:         workflow.UpdatedBy,
			StructureJSONView: workflow.StructureJSONView,
		}

		if err := s.workflowRepo.Update(ctx, workflowToUpdate); err != nil {
			return fmt.Errorf("更新工作流失败: %w", err)
		}

		// 2. 删除旧的关联定义
		if err := s.nodeDefRepo.DeleteByWorkflowID(ctx, workflow.ID); err != nil {
			return fmt.Errorf("删除旧节点定义失败: %w", err)
		}
		if err := s.lineDefRepo.DeleteByWorkflowID(ctx, workflow.ID); err != nil {
			return fmt.Errorf("删除旧连接线定义失败: %w", err)
		}
		if err := s.variableDefRepo.DeleteByWorkflowID(ctx, workflow.ID); err != nil {
			return fmt.Errorf("删除旧变量定义失败: %w", err)
		}

		// 3. 更新关联对象的WorkflowID并保存到各自的表
		if len(workflow.Nodes) > 0 {
			for i := range workflow.Nodes {
				workflow.Nodes[i].WorkflowID = workflow.ID
				workflow.Nodes[i].ID = 0 // 清空ID，让数据库自动生成新ID
			}
			if err := s.nodeDefRepo.CreateBatchForWorkflow(ctx, workflow.ID, convertToNodeDefPointers(workflow.Nodes)); err != nil {
				return fmt.Errorf("保存节点定义失败: %w", err)
			}
		}

		if len(workflow.Lines) > 0 {
			for i := range workflow.Lines {
				workflow.Lines[i].WorkflowID = workflow.ID
				workflow.Lines[i].ID = 0 // 清空ID，让数据库自动生成新ID
			}
			if err := s.lineDefRepo.CreateBatchForWorkflow(ctx, workflow.ID, convertToLineDefPointers(workflow.Lines)); err != nil {
				return fmt.Errorf("保存连接线定义失败: %w", err)
			}
		}

		if len(workflow.Variables) > 0 {
			for i := range workflow.Variables {
				workflow.Variables[i].WorkflowID = workflow.ID
				workflow.Variables[i].ID = 0 // 清空ID，让数据库自动生成新ID
			}
			if err := s.variableDefRepo.CreateBatchForWorkflow(ctx, workflow.ID, convertToVariableDefPointers(workflow.Variables)); err != nil {
				return fmt.Errorf("保存变量定义失败: %w", err)
			}
		}

		// 4. 从关联对象构建StructureJSON字符串
		if err := workflow.BuildStructureJSON(); err != nil {
			return fmt.Errorf("构建StructureJSON失败: %w", err)
		}

		// 5. 更新workflow的StructureJSON字段
		workflow.StructureJSON = workflowToUpdate.StructureJSON
		if err := s.workflowRepo.Update(ctx, workflow); err != nil {
			return fmt.Errorf("更新StructureJSON失败: %w", err)
		}

		return nil
	})
}

// Delete 删除工作流
func (s *workflowService) Delete(ctx context.Context, id uint) error {
	return s.workflowRepo.SoftDelete(ctx, id)
}

// List 列出工作流（不包含关联对象，用于列表展示）
func (s *workflowService) List(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error) {
	// 列表查询只返回基本信息，不加载关联对象
	return s.workflowRepo.FindWithPagination(ctx, nil, page, pageSize)
}

// CreateNewVersion 创建新版本
func (s *workflowService) CreateNewVersion(ctx context.Context, keyName string, workflow *models.Workflow) error {
	// 获取下一个版本号
	nextVersion, err := s.workflowRepo.GetNextVersion(ctx, keyName)
	if err != nil {
		return fmt.Errorf("获取版本号失败: %w", err)
	}

	workflow.KeyName = keyName
	workflow.Version = nextVersion
	workflow.Status = models.WorkflowStatusDraft

	return s.Create(ctx, workflow)
}

// GetLatestVersion 获取最新版本（包含关联对象）
func (s *workflowService) GetLatestVersion(ctx context.Context, keyName string) (*models.Workflow, error) {
	workflow, err := s.workflowRepo.GetLatestVersion(ctx, keyName)
	if err != nil || workflow == nil {
		return workflow, err
	}

	// 加载关联对象
	return s.loadWorkflowDefinitions(ctx, workflow)
}

// GetAllVersions 获取所有版本（不包含关联对象，用于版本列表展示）
func (s *workflowService) GetAllVersions(ctx context.Context, keyName string) ([]*models.Workflow, error) {
	// 版本列表只返回基本信息
	return s.workflowRepo.GetAllVersions(ctx, keyName)
}

// GetByKeyNameAndVersion 根据key_name和version获取工作流（包含关联对象）
func (s *workflowService) GetByKeyNameAndVersion(ctx context.Context, keyName string, version int) (*models.Workflow, error) {
	workflow, err := s.workflowRepo.FindByKeyNameAndVersion(ctx, keyName, version)
	if err != nil || workflow == nil {
		return workflow, err
	}

	// 加载关联对象
	return s.loadWorkflowDefinitions(ctx, workflow)
}

// Publish 发布工作流
func (s *workflowService) Publish(ctx context.Context, id uint) error {
	// 获取工作流（包含关联对象）
	workflow, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取工作流失败: %w", err)
	}

	// 验证结构
	if err := s.ValidateStructure(ctx, workflow); err != nil {
		return fmt.Errorf("工作流结构验证失败: %w", err)
	}

	// 更新状态
	return s.workflowRepo.Publish(ctx, id)
}

// Archive 归档工作流
func (s *workflowService) Archive(ctx context.Context, id uint) error {
	return s.workflowRepo.Archive(ctx, id)
}

// Enable 启用工作流
func (s *workflowService) Enable(ctx context.Context, id uint) error {
	return s.workflowRepo.Enable(ctx, id)
}

// Disable 禁用工作流
func (s *workflowService) Disable(ctx context.Context, id uint) error {
	return s.workflowRepo.Disable(ctx, id)
}

// Search 搜索工作流（不包含关联对象，用于搜索结果列表）
func (s *workflowService) Search(ctx context.Context, keyword string, page, pageSize int) ([]*models.Workflow, int64, error) {
	workflows, err := s.workflowRepo.SearchWorkflows(ctx, keyword, &repository.QueryOptions{
		Pagination: &repository.PaginationOptions{
			Page:     page,
			PageSize: pageSize,
		},
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.workflowRepo.Count(ctx, nil)
	if err != nil {
		return nil, 0, err
	}

	return workflows, total, nil
}

// FindByCategory 根据分类查找工作流（不包含关联对象）
func (s *workflowService) FindByCategory(ctx context.Context, category string) ([]*models.Workflow, error) {
	return s.workflowRepo.FindByCategory(ctx, category)
}

// FindByStatus 根据状态查找工作流（不包含关联对象）
func (s *workflowService) FindByStatus(ctx context.Context, status models.WorkflowStatus) ([]*models.Workflow, error) {
	return s.workflowRepo.FindByStatus(ctx, status)
}

// ValidateStructure 验证工作流结构
func (s *workflowService) ValidateStructure(ctx context.Context, workflow *models.Workflow) error {
	// 使用 Nodes、Lines 进行验证
	if len(workflow.Nodes) == 0 {
		return fmt.Errorf("工作流必须包含节点")
	}

	// 验证必须有开始节点和结束节点
	hasStart := false
	hasEnd := false
	for _, node := range workflow.Nodes {
		if node.TypeKey == "start" {
			hasStart = true
		}
		if node.TypeKey == "end" {
			hasEnd = true
		}
	}

	if !hasStart {
		return fmt.Errorf("工作流必须包含开始节点")
	}
	if !hasEnd {
		return fmt.Errorf("工作流必须包含结束节点")
	}

	// 验证节点模板是否存在
	for _, node := range workflow.Nodes {
		if node.NodeTemplateID > 0 {
			template, err := s.nodeTemplateRepo.GetByID(ctx, node.NodeTemplateID)
			if err != nil || template == nil {
				return fmt.Errorf("节点模板 %d 不存在", node.NodeTemplateID)
			}
		}
	}

	// 验证连接线的源节点和目标节点是否存在
	nodeMap := make(map[string]bool)
	for _, node := range workflow.Nodes {
		nodeMap[node.NodeID] = true
	}

	for _, line := range workflow.Lines {
		if !nodeMap[line.SourceNodeID] {
			return fmt.Errorf("连接线的源节点 %s 不存在", line.SourceNodeID)
		}
		if !nodeMap[line.TargetNodeID] {
			return fmt.Errorf("连接线的目标节点 %s 不存在", line.TargetNodeID)
		}
	}

	return nil
}

// SyncStructureToDefinitions 同步结构到定义表（已废弃，保留用于兼容性）
// 新的Create和Update方法已经内置了同步逻辑
func (s *workflowService) SyncStructureToDefinitions(ctx context.Context, workflowID uint) error {
	// 获取工作流
	workflow, err := s.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}

	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 删除旧的定义
		if err := s.nodeDefRepo.DeleteByWorkflowID(ctx, workflowID); err != nil {
			return err
		}
		if err := s.lineDefRepo.DeleteByWorkflowID(ctx, workflowID); err != nil {
			return err
		}
		if err := s.variableDefRepo.DeleteByWorkflowID(ctx, workflowID); err != nil {
			return err
		}

		// 创建节点定义
		if len(workflow.Nodes) > 0 {
			for i := range workflow.Nodes {
				workflow.Nodes[i].WorkflowID = workflowID
			}
			if err := s.nodeDefRepo.CreateBatchForWorkflow(ctx, workflowID, convertToNodeDefPointers(workflow.Nodes)); err != nil {
				return err
			}
		}

		// 创建连接线定义
		if len(workflow.Lines) > 0 {
			for i := range workflow.Lines {
				workflow.Lines[i].WorkflowID = workflowID
			}
			if err := s.lineDefRepo.CreateBatchForWorkflow(ctx, workflowID, convertToLineDefPointers(workflow.Lines)); err != nil {
				return err
			}
		}

		// 创建全局变量定义
		if len(workflow.Variables) > 0 {
			for i := range workflow.Variables {
				workflow.Variables[i].WorkflowID = workflowID
			}
			if err := s.variableDefRepo.CreateBatchForWorkflow(ctx, workflowID, convertToVariableDefPointers(workflow.Variables)); err != nil {
				return err
			}
		}

		// 构建并保存 StructureJSON
		if err := workflow.BuildStructureJSON(); err != nil {
			return fmt.Errorf("构建StructureJSON失败: %w", err)
		}
		if err := s.workflowRepo.Update(ctx, workflow); err != nil {
			return fmt.Errorf("更新StructureJSON失败: %w", err)
		}

		return nil
	})
}

// SyncDefinitionsToStructure 同步定义表到结构（已废弃，保留用于兼容性）
// 新的GetByID方法已经自动加载关联对象
func (s *workflowService) SyncDefinitionsToStructure(ctx context.Context, workflowID uint) error {
	// 获取工作流（会自动加载关联对象）
	workflow, err := s.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}

	// 构建 StructureJSON 字符串
	if err := workflow.BuildStructureJSON(); err != nil {
		return fmt.Errorf("构建StructureJSON失败: %w", err)
	}

	// 更新工作流
	return s.workflowRepo.Update(ctx, workflow)
}

// GetStatistics 获取统计信息
func (s *workflowService) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	return s.workflowRepo.GetStatistics(ctx)
}

// 辅助函数：从map中获取字符串值（工作流服务专用）
func getStringFromMapWorkflow(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// 辅助函数：从map中获取布尔值（工作流服务专用）
func getBoolFromMapWorkflow(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

// 辅助函数：转换NodeDefinition值切片为指针切片
func convertToNodeDefPointers(nodes []models.NodeDefinition) []*models.NodeDefinition {
	result := make([]*models.NodeDefinition, len(nodes))
	for i := range nodes {
		result[i] = &nodes[i]
	}
	return result
}

// 辅助函数：转换LineDefinition值切片为指针切片
func convertToLineDefPointers(lines []models.LineDefinition) []*models.LineDefinition {
	result := make([]*models.LineDefinition, len(lines))
	for i := range lines {
		result[i] = &lines[i]
	}
	return result
}

// 辅助函数：转换VariableDefinition值切片为指针切片
func convertToVariableDefPointers(variables []models.VariableDefinition) []*models.VariableDefinition {
	result := make([]*models.VariableDefinition, len(variables))
	for i := range variables {
		result[i] = &variables[i]
	}
	return result
}

// 辅助函数：加载工作流的关联定义对象
func (s *workflowService) loadWorkflowDefinitions(ctx context.Context, workflow *models.Workflow) (*models.Workflow, error) {
	// 查询关联的定义对象
	nodeDefs, err := s.nodeDefRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("查询节点定义失败: %w", err)
	}

	lineDefs, err := s.lineDefRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("查询连接线定义失败: %w", err)
	}

	variableDefs, err := s.variableDefRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("查询变量定义失败: %w", err)
	}

	// 转换为值切片并设置到workflow对象
	workflow.Nodes = make([]models.NodeDefinition, len(nodeDefs))
	for i, node := range nodeDefs {
		workflow.Nodes[i] = *node
	}

	workflow.Lines = make([]models.LineDefinition, len(lineDefs))
	for i, line := range lineDefs {
		workflow.Lines[i] = *line
	}

	workflow.Variables = make([]models.VariableDefinition, len(variableDefs))
	for i, variable := range variableDefs {
		workflow.Variables[i] = *variable
	}

	return workflow, nil
}
