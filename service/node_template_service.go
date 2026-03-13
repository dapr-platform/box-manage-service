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
	repo repository.NodeTemplateRepository
}

// NewNodeTemplateService 创建节点模板服务实例
func NewNodeTemplateService(repoManager repository.RepositoryManager) NodeTemplateService {
	return &nodeTemplateService{
		repo: repository.NewNodeTemplateRepository(repoManager.DB()),
	}
}

// Create 创建节点模板
func (s *nodeTemplateService) Create(ctx context.Context, template *models.NodeTemplate) error {
	return s.repo.Create(ctx, template)
}

// GetByID 根据ID获取节点模板
func (s *nodeTemplateService) GetByID(ctx context.Context, id uint) (*models.NodeTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByKeyName 根据key_name获取节点模板
func (s *nodeTemplateService) GetByKeyName(ctx context.Context, keyName string) (*models.NodeTemplate, error) {
	return s.repo.FindByKeyName(ctx, keyName)
}

// Update 更新节点模板
func (s *nodeTemplateService) Update(ctx context.Context, template *models.NodeTemplate) error {
	return s.repo.Update(ctx, template)
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
