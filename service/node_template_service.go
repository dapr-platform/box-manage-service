/*
 * @module service/node_template_service
 * @description 节点模板服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Controller -> NodeTemplateService -> Repository -> Database
 * @rules 实现节点模板的管理业务逻辑
 * @dependencies repository, models
 * @refs 业务编排引擎需求文档.md 5.2节
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"

	"gorm.io/gorm"
)

// NodeTemplateService 节点模板服务接口
type NodeTemplateService interface {
	// 基础CRUD
	Create(ctx context.Context, template *models.NodeTemplate) error
	GetByID(ctx context.Context, id uint) (*models.NodeTemplate, error)
	GetByKeyName(ctx context.Context, keyName string) (*models.NodeTemplate, error)
	Update(ctx context.Context, template *models.NodeTemplate) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context) ([]*models.NodeTemplate, error)

	// 查询
	FindByType(ctx context.Context, nodeType string) ([]*models.NodeTemplate, error)
	FindByCategory(ctx context.Context, category string) ([]*models.NodeTemplate, error)
	FindEnabled(ctx context.Context) ([]*models.NodeTemplate, error)
	Search(ctx context.Context, keyword string) ([]*models.NodeTemplate, error)

	// 状态管理
	Enable(ctx context.Context, id uint) error
	Disable(ctx context.Context, id uint) error

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// nodeTemplateService 节点模板服务实现
type nodeTemplateService struct {
	repo            repository.NodeTemplateRepository
	variableDefRepo repository.VariableDefinitionRepository
	repoManager     repository.RepositoryManager
}

// NewNodeTemplateService 创建节点模板服务实例
func NewNodeTemplateService(repoManager repository.RepositoryManager) NodeTemplateService {
	return &nodeTemplateService{
		repo:            repository.NewNodeTemplateRepository(repoManager.DB()),
		variableDefRepo: repository.NewVariableDefinitionRepository(repoManager.DB()),
		repoManager:     repoManager,
	}
}

// Create 创建节点模板
func (s *nodeTemplateService) Create(ctx context.Context, template *models.NodeTemplate) error {
	// 使用事务处理整个创建流程
	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 1. 先保存节点模板基本信息（不包含 Variables）
		templateToSave := &models.NodeTemplate{
			TypeKey:        template.TypeKey,
			TypeName:       template.TypeName,
			Category:       template.Category,
			GroupType:      template.GroupType,
			Icon:           template.Icon,
			Description:    template.Description,
			ConfigSchema:   template.ConfigSchema,
			StructureJSON:  template.StructureJSON,
			ScriptTemplate: template.ScriptTemplate,
			StartNodeKey:   template.StartNodeKey,
			EndNodeKey:     template.EndNodeKey,
			IsSystem:       template.IsSystem,
			IsEnabled:      template.IsEnabled,
			SortOrder:      template.SortOrder,
		}

		if err := s.repo.Create(ctx, templateToSave); err != nil {
			return fmt.Errorf("创建节点模板失败：%w", err)
		}

		// 将生成的 ID 赋值回原 template 对象
		template.ID = templateToSave.ID

		// 2. 保存 Variables 到 variable_definitions 表
		if len(template.Variables) > 0 {
			for i := range template.Variables {
				template.Variables[i].NodeTemplateID = &template.ID
			}
			if err := s.variableDefRepo.CreateBatchForNodeTemplate(ctx, template.ID, convertToVariableDefPointers(template.Variables)); err != nil {
				return fmt.Errorf("保存变量定义失败：%w", err)
			}
		}

		// 3. 从 Variables 构建 StructureJSON 字符串
		if err := template.BuildStructureJSON(); err != nil {
			return fmt.Errorf("构建 StructureJSON 失败：%w", err)
		}

		// 4. 更新节点模板的 StructureJSON 字段
		if err := s.repo.Update(ctx, template); err != nil {
			return fmt.Errorf("更新 StructureJSON 失败：%w", err)
		}

		return nil
	})
}

// GetByID 根据 ID 获取节点模板
func (s *nodeTemplateService) GetByID(ctx context.Context, id uint) (*models.NodeTemplate, error) {
	// 1. 查询节点模板基本信息
	template, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, nil
	}

	// 2. 加载 Variables
	variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查询变量定义失败：%w", err)
	}

	// 3. 设置 Variables
	template.Variables = make([]models.VariableDefinition, len(variables))
	for i, v := range variables {
		template.Variables[i] = *v
	}

	return template, nil
}

// GetByKeyName 根据 key_name 获取节点模板
func (s *nodeTemplateService) GetByKeyName(ctx context.Context, keyName string) (*models.NodeTemplate, error) {
	// 1. 查询节点模板基本信息
	template, err := s.repo.FindByKeyName(ctx, keyName)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, nil
	}

	// 2. 加载 Variables
	variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
	if err != nil {
		return nil, fmt.Errorf("查询变量定义失败：%w", err)
	}

	// 3. 设置 Variables
	template.Variables = make([]models.VariableDefinition, len(variables))
	for i, v := range variables {
		template.Variables[i] = *v
	}

	return template, nil
}

// Update 更新节点模板
func (s *nodeTemplateService) Update(ctx context.Context, template *models.NodeTemplate) error {
	// 使用事务处理整个更新流程
	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		// 1. 更新节点模板基本信息
		templateToUpdate := &models.NodeTemplate{
			TypeKey:        template.TypeKey,
			TypeName:       template.TypeName,
			Category:       template.Category,
			GroupType:      template.GroupType,
			Icon:           template.Icon,
			Description:    template.Description,
			ConfigSchema:   template.ConfigSchema,
			StructureJSON:  template.StructureJSON,
			ScriptTemplate: template.ScriptTemplate,
			StartNodeKey:   template.StartNodeKey,
			EndNodeKey:     template.EndNodeKey,
			IsSystem:       template.IsSystem,
			IsEnabled:      template.IsEnabled,
			SortOrder:      template.SortOrder,
		}
		templateToUpdate.ID = template.ID

		if err := s.repo.Update(ctx, templateToUpdate); err != nil {
			return fmt.Errorf("更新节点模板失败：%w", err)
		}

		// 2. 删除旧的 Variables
		if err := s.variableDefRepo.DeleteByNodeTemplateID(ctx, template.ID); err != nil {
			return fmt.Errorf("删除旧变量定义失败：%w", err)
		}

		// 3. 保存新的 Variables
		if len(template.Variables) > 0 {
			for i := range template.Variables {
				template.Variables[i].NodeTemplateID = &template.ID
			}
			if err := s.variableDefRepo.CreateBatchForNodeTemplate(ctx, template.ID, convertToVariableDefPointers(template.Variables)); err != nil {
				return fmt.Errorf("保存变量定义失败：%w", err)
			}
		}

		// 4. 从 Variables 构建 StructureJSON 字符串
		if err := template.BuildStructureJSON(); err != nil {
			return fmt.Errorf("构建 StructureJSON 失败：%w", err)
		}

		// 5. 更新节点模板的 StructureJSON 字段
		if err := s.repo.Update(ctx, template); err != nil {
			return fmt.Errorf("更新 StructureJSON 失败：%w", err)
		}

		return nil
	})
}

// Delete 删除节点模板
func (s *nodeTemplateService) Delete(ctx context.Context, id uint) error {
	return s.repo.SoftDelete(ctx, id)
}

// List 列出所有节点模板
func (s *nodeTemplateService) List(ctx context.Context) ([]*models.NodeTemplate, error) {
	return s.repo.Find(ctx, nil)
}

// FindByType 根据类型查找节点模板
func (s *nodeTemplateService) FindByType(ctx context.Context, nodeType string) ([]*models.NodeTemplate, error) {
	return s.repo.FindByType(ctx, nodeType)
}

// FindByCategory 根据分类查找节点模板
func (s *nodeTemplateService) FindByCategory(ctx context.Context, category string) ([]*models.NodeTemplate, error) {
	return s.repo.FindByCategory(ctx, category)
}

// FindEnabled 查找启用的节点模板
func (s *nodeTemplateService) FindEnabled(ctx context.Context) ([]*models.NodeTemplate, error) {
	return s.repo.FindEnabled(ctx)
}

// Search 搜索节点模板
func (s *nodeTemplateService) Search(ctx context.Context, keyword string) ([]*models.NodeTemplate, error) {
	return s.repo.SearchTemplates(ctx, keyword)
}

// Enable 启用节点模板
func (s *nodeTemplateService) Enable(ctx context.Context, id uint) error {
	return s.repo.Enable(ctx, id)
}

// Disable 禁用节点模板
func (s *nodeTemplateService) Disable(ctx context.Context, id uint) error {
	return s.repo.Disable(ctx, id)
}

// GetStatistics 获取统计信息
func (s *nodeTemplateService) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetStatistics(ctx)
}
