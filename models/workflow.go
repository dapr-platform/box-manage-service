/*
 * @module models/workflow
 * @description 工作流相关数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 工作流定义 -> 工作流实例 -> 节点实例
 * @rules 工作流通过key_name+version唯一标识，structure_json与关联表保持一致
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1节
 */

package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// WorkflowStatus 工作流状态枚举
type WorkflowStatus string

const (
	WorkflowStatusDraft     WorkflowStatus = "draft"     // 草稿
	WorkflowStatusPublished WorkflowStatus = "published" // 已发布
	WorkflowStatusArchived  WorkflowStatus = "archived"  // 已归档
)

// Workflow 工作流定义模型
// @Description 工作流定义，包含流程结构和配置信息
type Workflow struct {
	BaseModel
	KeyName           string         `gorm:"type:varchar(100);not null;uniqueIndex:idx_key_version" json:"key_name" example:"video_analysis_workflow"`
	Name              string         `gorm:"type:varchar(100);not null" json:"name" example:"视频分析工作流"`
	Description       string         `gorm:"type:text" json:"description" example:"对视频进行AI分析并推送结果"`
	Category          string         `gorm:"type:varchar(50)" json:"category" example:"video_analysis"`
	Tags              string         `gorm:"type:varchar(255)" json:"tags" example:"ai,video,mqtt"`
	Version           int            `gorm:"not null;default:0;uniqueIndex:idx_key_version" json:"version" example:"1"`
	StructureJSON     string         `gorm:"type:text;not null" json:"structure_json" swaggerignore:"true"` // JSON字符串，用于下发
	StructureJSONView string         `gorm:"type:text" json:"structure_json_view"`                          // 前端结构JSON字符串（前端存前端消费）
	Status            WorkflowStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status" example:"draft"`
	IsEnabled         bool           `gorm:"not null;default:true" json:"is_enabled" example:"true"`
	CreatedBy         uint           `gorm:"index" json:"created_by" example:"1"`
	UpdatedBy         uint           `json:"updated_by" example:"1"`

	// 关联对象（不存储到数据库，用于编辑时的数据传递）
	Nodes     []NodeDefinition     `gorm:"-" json:"nodes,omitempty"`     // 节点列表
	Lines     []LineDefinition     `gorm:"-" json:"lines,omitempty"`     // 连接线列表
	Variables []VariableDefinition `gorm:"-" json:"variables,omitempty"` // 变量列表
}

// TableName 指定表名
func (Workflow) TableName() string {
	return "workflows"
}

// BuildStructureJSON 从 Nodes、Lines、Variables 构建 StructureJSON 字符串
func (w *Workflow) BuildStructureJSON() error {
	structure := map[string]interface{}{
		"nodes":     w.Nodes,
		"lines":     w.Lines,
		"variables": w.Variables,
	}

	bytes, err := json.Marshal(structure)
	if err != nil {
		return err
	}

	w.StructureJSON = string(bytes)
	return nil
}

// ParseStructureJSON 解析 StructureJSON 字符串到 Nodes、Lines、Variables
func (w *Workflow) ParseStructureJSON() error {
	if w.StructureJSON == "" {
		return nil
	}

	var structure map[string]interface{}
	if err := json.Unmarshal([]byte(w.StructureJSON), &structure); err != nil {
		return err
	}

	// 解析 nodes
	if nodesData, ok := structure["nodes"]; ok {
		nodesBytes, _ := json.Marshal(nodesData)
		json.Unmarshal(nodesBytes, &w.Nodes)
	}

	// 解析 lines
	if linesData, ok := structure["lines"]; ok {
		linesBytes, _ := json.Marshal(linesData)
		json.Unmarshal(linesBytes, &w.Lines)
	}

	// 解析 variables
	if variablesData, ok := structure["variables"]; ok {
		variablesBytes, _ := json.Marshal(variablesData)
		json.Unmarshal(variablesBytes, &w.Variables)
	}

	return nil
}

// GetStructureMap 获取结构对象的 map 表示
func (w *Workflow) GetStructureMap() (map[string]interface{}, error) {
	if w.StructureJSON == "" {
		return map[string]interface{}{
			"nodes":     w.Nodes,
			"lines":     w.Lines,
			"variables": w.Variables,
		}, nil
	}

	var structure map[string]interface{}
	if err := json.Unmarshal([]byte(w.StructureJSON), &structure); err != nil {
		return nil, err
	}

	return structure, nil
}

// BeforeCreate GORM钩子
func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	w.CreatedAt = CustomTime{Time: now}
	w.UpdatedAt = CustomTime{Time: now}
	return nil
}

// BeforeUpdate GORM钩子
func (w *Workflow) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = CustomTime{Time: time.Now()}
	return nil
}
