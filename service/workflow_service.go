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
	ListAll(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error)
	ListDrafts(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error)

	// 版本管理
	CreateNewVersion(ctx context.Context, keyName string, workflow *models.Workflow) error
	GetLatestVersion(ctx context.Context, keyName string) (*models.Workflow, error)
	GetAllVersions(ctx context.Context, keyName string) ([]*models.Workflow, error)
	GetAllVersionsByKeyName(ctx context.Context, keyName string, status *models.WorkflowStatus) ([]*models.Workflow, error)
	GetRelatedVersions(ctx context.Context, draftWorkflowID uint) ([]*models.Workflow, error)
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

// Create 创建工作流（只能创建草稿状态）
func (s *workflowService) Create(ctx context.Context, workflow *models.Workflow) error {
	// 强制设置为草稿状态
	workflow.Status = models.WorkflowStatusDraft

	// 验证结构
	if err := s.ValidateStructure(ctx, workflow); err != nil {
		return fmt.Errorf("工作流结构验证失败: %w", err)
	}

	// 检查是否已存在草稿版本
	draftWorkflows, err := s.workflowRepo.FindByKeyNameAndStatus(ctx, workflow.KeyName, models.WorkflowStatusDraft)
	if err != nil {
		return fmt.Errorf("查询草稿版本失败: %w", err)
	}
	if len(draftWorkflows) > 0 {
		return fmt.Errorf("已存在草稿版本，请先发布或删除现有草稿")
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

// Update 更新工作流（只能更新草稿状态）
func (s *workflowService) Update(ctx context.Context, workflow *models.Workflow) error {
	// 1. 检查工作流是否存在
	existingWorkflow, err := s.workflowRepo.GetByID(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("获取工作流失败: %w", err)
	}
	if existingWorkflow == nil {
		return fmt.Errorf("工作流不存在")
	}

	// 2. 只能更新草稿状态的工作流
	if existingWorkflow.Status != models.WorkflowStatusDraft {
		return fmt.Errorf("只能更新草稿状态的工作流，当前状态: %s", existingWorkflow.Status)
	}

	// 3. 强制保持草稿状态
	workflow.Status = models.WorkflowStatusDraft

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

// List 列出工作流（只返回草稿状态，不包含关联对象，用于列表展示）
func (s *workflowService) List(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error) {
	// 只查询草稿状态
	workflows, err := s.workflowRepo.FindByStatus(ctx, models.WorkflowStatusDraft)
	if err != nil {
		return nil, 0, err
	}

	// 手动分页
	total := int64(len(workflows))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(workflows) {
		return []*models.Workflow{}, total, nil
	}
	if end > len(workflows) {
		end = len(workflows)
	}

	return workflows[start:end], total, nil
}

// ListAll 列出所有工作流（不包含关联对象，用于列表展示）
func (s *workflowService) ListAll(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error) {
	// 列表查询返回所有状态的基本信息，不加载关联对象
	return s.workflowRepo.FindWithPagination(ctx, nil, page, pageSize)
}

// ListDrafts 列出草稿工作流（不包含关联对象，用于列表展示）
func (s *workflowService) ListDrafts(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error) {
	// 只查询草稿状态
	return s.List(ctx, page, pageSize)
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

// GetAllVersionsByKeyName 根据 key_name 查询所有 workflow，可选按状态过滤
func (s *workflowService) GetAllVersionsByKeyName(ctx context.Context, keyName string, status *models.WorkflowStatus) ([]*models.Workflow, error) {
	if status != nil {
		// 按状态过滤
		return s.workflowRepo.FindByKeyNameAndStatus(ctx, keyName, *status)
	}
	// 返回所有版本
	return s.workflowRepo.FindByKeyName(ctx, keyName)
}

// GetRelatedVersions 根据草稿 workflow_id 查询所有相关的归档和发布版本
func (s *workflowService) GetRelatedVersions(ctx context.Context, draftWorkflowID uint) ([]*models.Workflow, error) {
	// 1. 获取草稿 workflow
	draftWorkflow, err := s.workflowRepo.GetByID(ctx, draftWorkflowID)
	if err != nil {
		return nil, fmt.Errorf("获取草稿工作流失败: %w", err)
	}
	if draftWorkflow == nil {
		return nil, fmt.Errorf("工作流不存在")
	}

	// 2. 验证是否为草稿状态
	if draftWorkflow.Status != models.WorkflowStatusDraft {
		return nil, fmt.Errorf("指定的工作流不是草稿状态")
	}

	// 3. 查询同一 key_name 下的所有版本
	allVersions, err := s.workflowRepo.FindByKeyName(ctx, draftWorkflow.KeyName)
	if err != nil {
		return nil, fmt.Errorf("查询相关版本失败: %w", err)
	}

	// 4. 过滤出发布和归档版本
	var relatedVersions []*models.Workflow
	for _, version := range allVersions {
		if version.Status == models.WorkflowStatusPublished || version.Status == models.WorkflowStatusArchived {
			relatedVersions = append(relatedVersions, version)
		}
	}

	return relatedVersions, nil
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
// 发布逻辑：
// 1. 只能发布草稿状态的工作流
// 2. 发布时 copy 一份完整数据（包括 nodes、lines、variables）作为新版本
// 3. 如果当前 workflow 存在发布版本，将其置为归档状态
// 4. 一个 workflow 只有一个发布版本、一个草稿版本，其他版本为归档状态
func (s *workflowService) Publish(ctx context.Context, id uint) error {
	// 1. 获取要发布的工作流（草稿版本，包含完整数据）
	draftWorkflow, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取工作流失败: %w", err)
	}
	if draftWorkflow == nil {
		return fmt.Errorf("工作流不存在")
	}

	// 2. 验证状态：只能发布草稿状态的工作流
	if draftWorkflow.Status != models.WorkflowStatusDraft {
		return fmt.Errorf("只能发布草稿状态的工作流，当前状态: %s", draftWorkflow.Status)
	}

	// 3. 验证结构
	if err := s.ValidateStructure(ctx, draftWorkflow); err != nil {
		return fmt.Errorf("工作流结构验证失败: %w", err)
	}

	// 使用事务处理整个发布流程
	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 4. 查找当前 key_name 下是否存在已发布版本
		publishedWorkflows, err := s.workflowRepo.FindByKeyNameAndStatus(ctx, draftWorkflow.KeyName, models.WorkflowStatusPublished)
		if err != nil {
			return fmt.Errorf("查询已发布版本失败: %w", err)
		}

		// 5. 如果存在已发布版本，将其归档
		for _, published := range publishedWorkflows {
			if err := s.workflowRepo.Archive(ctx, published.ID); err != nil {
				return fmt.Errorf("归档旧版本失败: %w", err)
			}
		}

		// 6. 获取下一个版本号
		nextVersion, err := s.workflowRepo.GetNextVersion(ctx, draftWorkflow.KeyName)
		if err != nil {
			return fmt.Errorf("获取版本号失败: %w", err)
		}

		// 7. 创建新的发布版本（copy 草稿的完整数据）
		publishedWorkflow := &models.Workflow{
			KeyName:           draftWorkflow.KeyName,
			Name:              draftWorkflow.Name,
			Description:       draftWorkflow.Description,
			Category:          draftWorkflow.Category,
			Tags:              draftWorkflow.Tags,
			Version:           nextVersion,
			Status:            models.WorkflowStatusPublished,
			IsEnabled:         true, // 发布版本默认启用
			CreatedBy:         draftWorkflow.CreatedBy,
			UpdatedBy:         draftWorkflow.UpdatedBy,
			StructureJSONView: draftWorkflow.StructureJSONView,
		}

		// 8. 保存发布版本的基本信息
		if err := s.workflowRepo.Create(ctx, publishedWorkflow); err != nil {
			return fmt.Errorf("创建发布版本失败: %w", err)
		}

		// 9. Copy nodes 到新版本
		if len(draftWorkflow.Nodes) > 0 {
			newNodes := make([]*models.NodeDefinition, len(draftWorkflow.Nodes))
			for i := range draftWorkflow.Nodes {
				newNodes[i] = &models.NodeDefinition{
					WorkflowID:     publishedWorkflow.ID,
					NodeID:         draftWorkflow.Nodes[i].NodeID,
					NodeTemplateID: draftWorkflow.Nodes[i].NodeTemplateID,
					TypeKey:        draftWorkflow.Nodes[i].TypeKey,
					TypeName:       draftWorkflow.Nodes[i].TypeName,
					NodeName:       draftWorkflow.Nodes[i].NodeName,
					NodeKeyName:    draftWorkflow.Nodes[i].NodeKeyName,
					GroupType:      draftWorkflow.Nodes[i].GroupType,
					StartNodeKey:   draftWorkflow.Nodes[i].StartNodeKey,
					EndNodeKey:     draftWorkflow.Nodes[i].EndNodeKey,
					Config:         draftWorkflow.Nodes[i].Config,
					PythonScript:   draftWorkflow.Nodes[i].PythonScript,
					Inputs:         draftWorkflow.Nodes[i].Inputs,
					Outputs:        draftWorkflow.Nodes[i].Outputs,
					Position:       draftWorkflow.Nodes[i].Position,
				}
			}
			if err := s.nodeDefRepo.CreateBatchForWorkflow(ctx, publishedWorkflow.ID, newNodes); err != nil {
				return fmt.Errorf("保存发布版本节点定义失败: %w", err)
			}
		}

		// 10. Copy lines 到新版本
		if len(draftWorkflow.Lines) > 0 {
			newLines := make([]*models.LineDefinition, len(draftWorkflow.Lines))
			for i := range draftWorkflow.Lines {
				newLines[i] = &models.LineDefinition{
					WorkflowID:              publishedWorkflow.ID,
					LineID:                  draftWorkflow.Lines[i].LineID,
					SourceNodeID:            draftWorkflow.Lines[i].SourceNodeID,
					TargetNodeID:            draftWorkflow.Lines[i].TargetNodeID,
					ConditionType:           draftWorkflow.Lines[i].ConditionType,
					LogicType:               draftWorkflow.Lines[i].LogicType,
					ConditionExpression:     draftWorkflow.Lines[i].ConditionExpression,
					ConditionExpressionView: draftWorkflow.Lines[i].ConditionExpressionView,
					Description:             draftWorkflow.Lines[i].Description,
				}
			}
			if err := s.lineDefRepo.CreateBatchForWorkflow(ctx, publishedWorkflow.ID, newLines); err != nil {
				return fmt.Errorf("保存发布版本连接线定义失败: %w", err)
			}
		}

		// 11. Copy variables 到新版本
		if len(draftWorkflow.Variables) > 0 {
			newVariables := make([]*models.VariableDefinition, len(draftWorkflow.Variables))
			for i := range draftWorkflow.Variables {
				newVariables[i] = &models.VariableDefinition{
					WorkflowID:     publishedWorkflow.ID,
					NodeID:         draftWorkflow.Variables[i].NodeID,
					NodeTemplateID: draftWorkflow.Variables[i].NodeTemplateID,
					KeyName:        draftWorkflow.Variables[i].KeyName,
					Name:           draftWorkflow.Variables[i].Name,
					Type:           draftWorkflow.Variables[i].Type,
					Direction:      draftWorkflow.Variables[i].Direction,
					DefaultValue:   draftWorkflow.Variables[i].DefaultValue,
					Required:       draftWorkflow.Variables[i].Required,
					RefKeyName:     draftWorkflow.Variables[i].RefKeyName,
					Description:    draftWorkflow.Variables[i].Description,
				}
			}
			if err := s.variableDefRepo.CreateBatchForWorkflow(ctx, publishedWorkflow.ID, newVariables); err != nil {
				return fmt.Errorf("保存发布版本变量定义失败: %w", err)
			}
		}

		// 12. 构建并保存发布版本的 StructureJSON
		publishedWorkflow.Nodes = draftWorkflow.Nodes
		publishedWorkflow.Lines = draftWorkflow.Lines
		publishedWorkflow.Variables = draftWorkflow.Variables
		if err := publishedWorkflow.BuildStructureJSON(); err != nil {
			return fmt.Errorf("构建发布版本StructureJSON失败: %w", err)
		}
		if err := s.workflowRepo.Update(ctx, publishedWorkflow); err != nil {
			return fmt.Errorf("更新发布版本StructureJSON失败: %w", err)
		}

		return nil
	})
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
