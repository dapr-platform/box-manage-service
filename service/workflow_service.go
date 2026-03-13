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

	// 创建工作流
	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		return fmt.Errorf("创建工作流失败: %w", err)
	}

	// 同步结构到定义表
	if err := s.SyncStructureToDefinitions(ctx, workflow.ID); err != nil {
		return fmt.Errorf("同步结构到定义表失败: %w", err)
	}

	return nil
}

// GetByID 根据ID获取工作流
func (s *workflowService) GetByID(ctx context.Context, id uint) (*models.Workflow, error) {
	return s.workflowRepo.GetByID(ctx, id)
}

// Update 更新工作流
func (s *workflowService) Update(ctx context.Context, workflow *models.Workflow) error {
	// 验证结构
	if err := s.ValidateStructure(ctx, workflow); err != nil {
		return fmt.Errorf("工作流结构验证失败: %w", err)
	}

	// 更新工作流
	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		return fmt.Errorf("更新工作流失败: %w", err)
	}

	// 同步结构到定义表
	if err := s.SyncStructureToDefinitions(ctx, workflow.ID); err != nil {
		return fmt.Errorf("同步结构到定义表失败: %w", err)
	}

	return nil
}

// Delete 删除工作流
func (s *workflowService) Delete(ctx context.Context, id uint) error {
	return s.workflowRepo.SoftDelete(ctx, id)
}

// List 列出工作流
func (s *workflowService) List(ctx context.Context, page, pageSize int) ([]*models.Workflow, int64, error) {
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

// GetLatestVersion 获取最新版本
func (s *workflowService) GetLatestVersion(ctx context.Context, keyName string) (*models.Workflow, error) {
	return s.workflowRepo.GetLatestVersion(ctx, keyName)
}

// GetAllVersions 获取所有版本
func (s *workflowService) GetAllVersions(ctx context.Context, keyName string) ([]*models.Workflow, error) {
	return s.workflowRepo.GetAllVersions(ctx, keyName)
}

// GetByKeyNameAndVersion 根据key_name和version获取工作流
func (s *workflowService) GetByKeyNameAndVersion(ctx context.Context, keyName string, version int) (*models.Workflow, error) {
	return s.workflowRepo.FindByKeyNameAndVersion(ctx, keyName, version)
}

// Publish 发布工作流
func (s *workflowService) Publish(ctx context.Context, id uint) error {
	// 获取工作流
	workflow, err := s.workflowRepo.GetByID(ctx, id)
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

// Search 搜索工作流
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

// FindByCategory 根据分类查找工作流
func (s *workflowService) FindByCategory(ctx context.Context, category string) ([]*models.Workflow, error) {
	return s.workflowRepo.FindByCategory(ctx, category)
}

// FindByStatus 根据状态查找工作流
func (s *workflowService) FindByStatus(ctx context.Context, status models.WorkflowStatus) ([]*models.Workflow, error) {
	return s.workflowRepo.FindByStatus(ctx, status)
}

// ValidateStructure 验证工作流结构
func (s *workflowService) ValidateStructure(ctx context.Context, workflow *models.Workflow) error {
	structure := workflow.StructureJSON

	// 验证必须有开始节点和结束节点
	hasStart := false
	hasEnd := false
	for _, node := range structure.Nodes {
		if node.Type == "start" {
			hasStart = true
		}
		if node.Type == "end" {
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
	for _, node := range structure.Nodes {
		if node.NodeTemplateID > 0 {
			template, err := s.nodeTemplateRepo.GetByID(ctx, node.NodeTemplateID)
			if err != nil || template == nil {
				return fmt.Errorf("节点模板 %d 不存在", node.NodeTemplateID)
			}
		}
	}

	// 验证连接线的源节点和目标节点是否存在
	nodeMap := make(map[string]bool)
	for _, node := range structure.Nodes {
		nodeMap[node.ID] = true
	}

	for _, line := range structure.Lines {
		if !nodeMap[line.Source] {
			return fmt.Errorf("连接线的源节点 %s 不存在", line.Source)
		}
		if !nodeMap[line.Target] {
			return fmt.Errorf("连接线的目标节点 %s 不存在", line.Target)
		}
	}

	return nil
}

// SyncStructureToDefinitions 同步结构到定义表
func (s *workflowService) SyncStructureToDefinitions(ctx context.Context, workflowID uint) error {
	// 获取工作流
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
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
		nodeDefMap := make(map[string]uint) // node_id -> node_def_id
		for _, node := range workflow.StructureJSON.Nodes {
			nodeDef := &models.NodeDefinition{
				WorkflowID:     workflowID,
				NodeTemplateID: node.NodeTemplateID,
				NodeID:         node.ID,
				NodeKeyName:    node.KeyName,
				NodeName:       node.Name,
				TypeKey:        node.Type,
				TypeName:       node.Name, // 使用节点名称作为类型名称
				Position:       convertPositionToJSONMap(node.Position),
				Config:         node.Config,
				PythonScript:   node.PythonScript,
				Inputs:         convertParamsToJSONArr(node.Inputs),
				Outputs:        convertParamsToJSONArr(node.Outputs),
			}
			if err := s.nodeDefRepo.Create(ctx, nodeDef); err != nil {
				return err
			}
			nodeDefMap[node.ID] = nodeDef.ID

			// 从节点模板复制预定义的变量到 variable_definitions
			if node.NodeTemplateID > 0 {
				nodeTemplate, err := s.repoManager.NodeTemplate().GetByID(ctx, node.NodeTemplateID)
				if err == nil && nodeTemplate != nil && nodeTemplate.DefaultVariables != nil {
					// 解析 default_variables JSON
					if variables, ok := nodeTemplate.DefaultVariables["variables"].([]interface{}); ok {
						for _, v := range variables {
							if varMap, ok := v.(map[string]interface{}); ok {
								variableDef := &models.VariableDefinition{
									WorkflowID:     workflowID,
									NodeID:         node.ID,
									NodeTemplateID: &node.NodeTemplateID,
									KeyName:        getStringFromMap(varMap, "key_name"),
									Name:           getStringFromMap(varMap, "name"),
									Type:           getStringFromMap(varMap, "type"),
									Direction:      models.VariableDirection(getStringFromMap(varMap, "direction")),
									Required:       getBoolFromMap(varMap, "required"),
									Description:    getStringFromMap(varMap, "description"),
								}
								// 设置默认值
								if defaultValue, exists := varMap["default_value"]; exists {
									variableDef.DefaultValue = models.JSONValue{Data: defaultValue}
								}
								if err := s.variableDefRepo.Create(ctx, variableDef); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		}

		// 创建连接线定义
		for _, line := range workflow.StructureJSON.Lines {
			lineDef := &models.LineDefinition{
				WorkflowID:   workflowID,
				LineID:       line.ID,
				SourceNodeID: line.Source,
				TargetNodeID: line.Target,
			}
			if line.Condition != nil {
				lineDef.ConditionType = models.ConditionType(line.Condition.Type)
				lineDef.ConditionExpression = line.Condition.Expression
				lineDef.ConditionExpressionView = line.Condition.ExpressionView
			}
			if err := s.lineDefRepo.Create(ctx, lineDef); err != nil {
				return err
			}
		}

		// 创建全局变量定义
		for _, variable := range workflow.StructureJSON.Variables {
			variableDef := &models.VariableDefinition{
				WorkflowID:   workflowID,
				KeyName:      variable.KeyName,
				Name:         variable.Name,
				Type:         variable.Type,
				Direction:    models.VariableDirectionInput, // 默认为输入
				DefaultValue: models.JSONValue{Data: variable.Default},
				Required:     variable.Required,
				Description:  variable.Description,
			}
			if err := s.variableDefRepo.Create(ctx, variableDef); err != nil {
				return err
			}
		}

		return nil
	})
}

// SyncDefinitionsToStructure 同步定义表到结构
func (s *workflowService) SyncDefinitionsToStructure(ctx context.Context, workflowID uint) error {
	// 获取工作流
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}

	// 获取所有定义
	nodeDefs, err := s.nodeDefRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return err
	}

	lineDefs, err := s.lineDefRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return err
	}

	variableDefs, err := s.variableDefRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return err
	}

	// 构建结构JSON
	structure := models.StructureJSON{
		Nodes:     make([]models.NodeStructure, 0),
		Lines:     make([]models.LineStructure, 0),
		Variables: make([]models.VariableStructure, 0),
	}

	// 构建节点ID映射
	nodeDefIDMap := make(map[uint]string) // node_def_id -> node_id
	for _, nodeDef := range nodeDefs {
		nodeDefIDMap[nodeDef.ID] = nodeDef.NodeID

		// 转换Position
		var posX, posY float64
		if nodeDef.Position != nil {
			if x, ok := nodeDef.Position["x"].(float64); ok {
				posX = x
			}
			if y, ok := nodeDef.Position["y"].(float64); ok {
				posY = y
			}
		}

		// 转换Inputs和Outputs
		inputs := []models.ParameterStructure{}
		if nodeDef.Inputs != nil {
			for _, input := range nodeDef.Inputs {
				if paramMap, ok := input.(map[string]interface{}); ok {
					param := models.ParameterStructure{}
					if name, ok := paramMap["name"].(string); ok {
						param.Name = name
					}
					if keyName, ok := paramMap["key_name"].(string); ok {
						param.KeyName = keyName
					}
					if paramType, ok := paramMap["type"].(string); ok {
						param.Type = paramType
					}
					if required, ok := paramMap["required"].(bool); ok {
						param.Required = required
					}
					inputs = append(inputs, param)
				}
			}
		}

		outputs := []models.ParameterStructure{}
		if nodeDef.Outputs != nil {
			for _, output := range nodeDef.Outputs {
				if paramMap, ok := output.(map[string]interface{}); ok {
					param := models.ParameterStructure{}
					if name, ok := paramMap["name"].(string); ok {
						param.Name = name
					}
					if keyName, ok := paramMap["key_name"].(string); ok {
						param.KeyName = keyName
					}
					if paramType, ok := paramMap["type"].(string); ok {
						param.Type = paramType
					}
					outputs = append(outputs, param)
				}
			}
		}

		structure.Nodes = append(structure.Nodes, models.NodeStructure{
			ID:             nodeDef.NodeID,
			Type:           nodeDef.TypeKey,
			Name:           nodeDef.NodeName,
			KeyName:        nodeDef.NodeKeyName,
			NodeTemplateID: nodeDef.NodeTemplateID,
			Position: models.Position{
				X: posX,
				Y: posY,
			},
			Config:       nodeDef.Config,
			PythonScript: nodeDef.PythonScript,
			Inputs:       inputs,
			Outputs:      outputs,
		})
	}

	// 构建连接线
	for _, lineDef := range lineDefs {
		lineStruct := models.LineStructure{
			ID:     lineDef.LineID,
			Source: lineDef.SourceNodeID,
			Target: lineDef.TargetNodeID,
		}
		if lineDef.ConditionType != "" {
			lineStruct.Condition = &models.ConditionStructure{
				Type:           string(lineDef.ConditionType),
				Expression:     lineDef.ConditionExpression,
				ExpressionView: lineDef.ConditionExpressionView,
			}
		}
		structure.Lines = append(structure.Lines, lineStruct)
	}

	// 构建变量
	for _, variableDef := range variableDefs {
		structure.Variables = append(structure.Variables, models.VariableStructure{
			Name:        variableDef.Name,
			KeyName:     variableDef.KeyName,
			Type:        variableDef.Type,
			Scope:       "global", // 默认全局作用域
			Default:     variableDef.DefaultValue.Data,
			Required:    variableDef.Required,
			Description: variableDef.Description,
		})
	}

	// 更新工作流
	workflow.StructureJSON = structure
	return s.workflowRepo.Update(ctx, workflow)
}

// GetStatistics 获取统计信息
func (s *workflowService) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	return s.workflowRepo.GetStatistics(ctx)
}

// 辅助函数：转换参数结构
func convertToParamDefs(params []models.ParameterStructure) []models.ParameterStructure {
	result := make([]models.ParameterStructure, len(params))
	for i, p := range params {
		result[i] = models.ParameterStructure{
			Name:        p.Name,
			KeyName:     p.KeyName,
			Type:        p.Type,
			Required:    p.Required,
			Default:     p.Default,
			RefKeyName:  p.RefKeyName,
			Description: p.Description,
		}
	}
	return result
}

func convertToParamStructs(params []models.ParameterStructure) []models.ParameterStructure {
	result := make([]models.ParameterStructure, len(params))
	for i, p := range params {
		result[i] = models.ParameterStructure{
			Name:        p.Name,
			KeyName:     p.KeyName,
			Type:        p.Type,
			Required:    p.Required,
			Default:     p.Default,
			RefKeyName:  p.RefKeyName,
			Description: p.Description,
		}
	}
	return result
}

// convertParamsToJSONArr 将参数结构转换为JSONArr
func convertParamsToJSONArr(params []models.ParameterStructure) models.JSONArr {
	result := make(models.JSONArr, len(params))
	for i, p := range params {
		result[i] = map[string]interface{}{
			"name":         p.Name,
			"key_name":     p.KeyName,
			"type":         p.Type,
			"required":     p.Required,
			"default":      p.Default,
			"ref_key_name": p.RefKeyName,
			"description":  p.Description,
		}
	}
	return result
}

// convertPositionToJSONMap 将Position转换为JSONMap
func convertPositionToJSONMap(pos models.Position) models.JSONMap {
	return models.JSONMap{
		"x": pos.X,
		"y": pos.Y,
	}
}
