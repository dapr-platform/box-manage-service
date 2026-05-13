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
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	// 导入导出
	ExportSQL(ctx context.Context, category string) (string, error)
	ImportSQL(ctx context.Context, sqlContent string) error
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
	return s.repo.Delete(ctx, id)
}

// List 列出所有节点模板（包含 Variables）
func (s *nodeTemplateService) List(ctx context.Context) ([]*models.NodeTemplate, error) {
	// 1. 查询所有节点模板基本信息
	templates, err := s.repo.Find(ctx, nil)
	if err != nil {
		return nil, err
	}

	// 2. 为每个模板加载 Variables
	for _, template := range templates {
		variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return nil, fmt.Errorf("查询模板 %d 的变量定义失败：%w", template.ID, err)
		}

		// 设置 Variables
		template.Variables = make([]models.VariableDefinition, len(variables))
		for i, v := range variables {
			template.Variables[i] = *v
		}
	}

	return templates, nil
}

// FindByType 根据类型查找节点模板（包含 Variables）
func (s *nodeTemplateService) FindByType(ctx context.Context, nodeType string) ([]*models.NodeTemplate, error) {
	// 1. 查询节点模板基本信息
	templates, err := s.repo.FindByType(ctx, nodeType)
	if err != nil {
		return nil, err
	}

	// 2. 为每个模板加载 Variables
	for _, template := range templates {
		variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return nil, fmt.Errorf("查询模板 %d 的变量定义失败：%w", template.ID, err)
		}

		// 设置 Variables
		template.Variables = make([]models.VariableDefinition, len(variables))
		for i, v := range variables {
			template.Variables[i] = *v
		}
	}

	return templates, nil
}

// FindByCategory 根据分类查找节点模板（包含 Variables）
func (s *nodeTemplateService) FindByCategory(ctx context.Context, category string) ([]*models.NodeTemplate, error) {
	// 1. 查询节点模板基本信息
	templates, err := s.repo.FindByCategory(ctx, category)
	if err != nil {
		return nil, err
	}

	// 2. 为每个模板加载 Variables
	for _, template := range templates {
		variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return nil, fmt.Errorf("查询模板 %d 的变量定义失败：%w", template.ID, err)
		}

		// 设置 Variables
		template.Variables = make([]models.VariableDefinition, len(variables))
		for i, v := range variables {
			template.Variables[i] = *v
		}
	}

	return templates, nil
}

// FindEnabled 查找启用的节点模板（包含 Variables）
func (s *nodeTemplateService) FindEnabled(ctx context.Context) ([]*models.NodeTemplate, error) {
	// 1. 查询节点模板基本信息
	templates, err := s.repo.FindEnabled(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 为每个模板加载 Variables
	for _, template := range templates {
		variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return nil, fmt.Errorf("查询模板 %d 的变量定义失败：%w", template.ID, err)
		}

		// 设置 Variables
		template.Variables = make([]models.VariableDefinition, len(variables))
		for i, v := range variables {
			template.Variables[i] = *v
		}
	}

	return templates, nil
}

// Search 搜索节点模板（包含 Variables）
func (s *nodeTemplateService) Search(ctx context.Context, keyword string) ([]*models.NodeTemplate, error) {
	// 1. 查询节点模板基本信息
	templates, err := s.repo.SearchTemplates(ctx, keyword)
	if err != nil {
		return nil, err
	}

	// 2. 为每个模板加载 Variables
	for _, template := range templates {
		variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return nil, fmt.Errorf("查询模板 %d 的变量定义失败：%w", template.ID, err)
		}

		// 设置 Variables
		template.Variables = make([]models.VariableDefinition, len(variables))
		for i, v := range variables {
			template.Variables[i] = *v
		}
	}

	return templates, nil
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

// ExportSQL 导出节点模板及关联数据为SQL文件内容
func (s *nodeTemplateService) ExportSQL(ctx context.Context, category string) (string, error) {
	// 1. 查询节点模板
	var templates []*models.NodeTemplate
	var err error
	if category != "" {
		templates, err = s.repo.FindByCategory(ctx, category)
	} else {
		templates, err = s.repo.Find(ctx, nil)
	}
	if err != nil {
		return "", fmt.Errorf("查询节点模板失败: %w", err)
	}

	if len(templates) == 0 {
		return "", fmt.Errorf("没有找到可导出的节点模板")
	}

	var sb strings.Builder

	// 写入文件头
	sb.WriteString("-- ============================================================\n")
	sb.WriteString("-- Node Templates Export\n")
	sb.WriteString(fmt.Sprintf("-- Generated at: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("-- Total templates: %d\n", len(templates)))
	if category != "" {
		sb.WriteString(fmt.Sprintf("-- Category filter: %s\n", category))
	}
	sb.WriteString("-- ============================================================\n\n")

	// 写入事务开始
	sb.WriteString("BEGIN;\n\n")

	// 2. 为每个模板生成 SQL
	for _, template := range templates {
		// 加载关联的变量定义
		variables, err := s.variableDefRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return "", fmt.Errorf("查询模板 %s 的变量定义失败: %w", template.TypeKey, err)
		}

		sb.WriteString(fmt.Sprintf("-- Template: %s (%s)\n", template.TypeName, template.TypeKey))

		// 生成 node_templates INSERT 语句 (使用 ON CONFLICT 实现 upsert，包含 id 列)
		configSchemaJSON := "NULL"
		if template.ConfigSchema != nil {
			if jsonBytes, err := json.Marshal(template.ConfigSchema); err == nil {
				configSchemaJSON = fmt.Sprintf("'%s'", escapeSQL(string(jsonBytes)))
			}
		}

		sb.WriteString("INSERT INTO node_templates (id, type_key, type_name, category, group_type, icon, description, config_schema, structure_json, script_template, start_node_key, end_node_key, is_system, is_enabled, sort_order, created_at, updated_at) VALUES (\n")
		sb.WriteString(fmt.Sprintf("  %d, '%s', '%s', '%s', '%s', '%s', '%s', %s, '%s', '%s', '%s', '%s', %t, %t, %d, NOW(), NOW()\n",
			template.ID,
			escapeSQL(template.TypeKey),
			escapeSQL(template.TypeName),
			escapeSQL(template.Category),
			escapeSQL(string(template.GroupType)),
			escapeSQL(template.Icon),
			escapeSQL(template.Description),
			configSchemaJSON,
			escapeSQL(template.StructureJSON),
			escapeSQL(template.ScriptTemplate),
			escapeSQL(template.StartNodeKey),
			escapeSQL(template.EndNodeKey),
			template.IsSystem,
			template.IsEnabled,
			template.SortOrder,
		))
		sb.WriteString(") ON CONFLICT (type_key) DO UPDATE SET\n")
		sb.WriteString("  type_name = EXCLUDED.type_name,\n")
		sb.WriteString("  category = EXCLUDED.category,\n")
		sb.WriteString("  group_type = EXCLUDED.group_type,\n")
		sb.WriteString("  icon = EXCLUDED.icon,\n")
		sb.WriteString("  description = EXCLUDED.description,\n")
		sb.WriteString("  config_schema = EXCLUDED.config_schema,\n")
		sb.WriteString("  structure_json = EXCLUDED.structure_json,\n")
		sb.WriteString("  script_template = EXCLUDED.script_template,\n")
		sb.WriteString("  start_node_key = EXCLUDED.start_node_key,\n")
		sb.WriteString("  end_node_key = EXCLUDED.end_node_key,\n")
		sb.WriteString("  is_system = EXCLUDED.is_system,\n")
		sb.WriteString("  is_enabled = EXCLUDED.is_enabled,\n")
		sb.WriteString("  sort_order = EXCLUDED.sort_order,\n")
		sb.WriteString("  updated_at = NOW();\n\n")

		// 生成 variable_definitions 的 SQL
		if len(variables) > 0 {
			// 先删除该模板关联的旧变量定义
			sb.WriteString(fmt.Sprintf("DELETE FROM variable_definitions WHERE node_template_id = %d;\n", template.ID))

			for _, v := range variables {
				defaultValueJSON := "NULL"
				if v.DefaultValue.Data != nil {
					if jsonBytes, err := json.Marshal(v.DefaultValue.Data); err == nil {
						defaultValueJSON = fmt.Sprintf("'%s'", escapeSQL(string(jsonBytes)))
					}
				}

				sb.WriteString("INSERT INTO variable_definitions (id, workflow_id, node_id, node_template_id, key_name, name, type, direction, default_value, required, ref_key_name, description, created_at, updated_at) VALUES (\n")
				sb.WriteString(fmt.Sprintf("  %d, %d, '%s', %d, '%s', '%s', '%s', '%s', %s, %t, '%s', '%s', NOW(), NOW()\n",
					v.ID,
					v.WorkflowID,
					escapeSQL(v.NodeID),
					template.ID,
					escapeSQL(v.KeyName),
					escapeSQL(v.Name),
					escapeSQL(v.Type),
					escapeSQL(string(v.Direction)),
					defaultValueJSON,
					v.Required,
					escapeSQL(v.RefKeyName),
					escapeSQL(v.Description),
				))
				sb.WriteString(");\n")
			}
			sb.WriteString("\n")
		}
	}

	// 重置序列，确保后续自增ID不冲突
	sb.WriteString("-- 重置序列以避免后续插入主键冲突\n")
	sb.WriteString("SELECT setval('node_templates_id_seq', (SELECT COALESCE(MAX(id), 0) FROM node_templates));\n")
	sb.WriteString("SELECT setval('variable_definitions_id_seq', (SELECT COALESCE(MAX(id), 0) FROM variable_definitions));\n\n")

	// 写入事务结束
	sb.WriteString("COMMIT;\n")

	return sb.String(), nil
}

// ImportSQL 导入SQL文件内容（执行SQL语句）
func (s *nodeTemplateService) ImportSQL(ctx context.Context, sqlContent string) error {
	if strings.TrimSpace(sqlContent) == "" {
		return fmt.Errorf("SQL内容为空")
	}

	// 直接在事务中执行SQL
	return s.repoManager.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Exec(sqlContent).Error; err != nil {
			return fmt.Errorf("执行导入SQL失败: %w", err)
		}
		return nil
	})
}

// escapeSQL 转义SQL字符串中的特殊字符
func escapeSQL(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return s
}
