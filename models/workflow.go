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
	"database/sql/driver"
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
	StructureJSON     StructureJSON  `gorm:"type:jsonb;not null" json:"structure_json"`
	StructureJSONView StructureJSON  `gorm:"type:jsonb" json:"structure_json_view"` // 前端结构JSON（前端存前端消费）
	Status            WorkflowStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status" example:"draft"`
	IsEnabled         bool           `gorm:"not null;default:true" json:"is_enabled" example:"true"`
	CreatedBy         uint           `gorm:"index" json:"created_by" example:"1"`
	UpdatedBy         uint           `json:"updated_by" example:"1"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// StructureJSON 工作流结构JSON
// @Description 工作流的完整结构定义，包含节点、连接线和变量
type StructureJSON struct {
	Nodes     []NodeStructure     `json:"nodes"`     // 节点列表
	Lines     []LineStructure     `json:"lines"`     // 连接线列表
	Variables []VariableStructure `json:"variables"` // 全局变量列表
}

// NodeStructure 节点结构
// @Description 节点的完整定义，包含配置、脚本和输入输出
type NodeStructure struct {
	ID             string                 `json:"id" example:"node_1"`                                // 节点ID
	Type           string                 `json:"type" example:"start"`                               // 节点类型
	Name           string                 `json:"name" example:"开始"`                                  // 节点名称
	KeyName        string                 `json:"key_name" example:"start_node"`                      // 节点标识
	NodeTemplateID uint                   `json:"node_template_id" example:"1"`                       // 节点模板ID
	Position       Position               `json:"position"`                                           // 节点位置
	Config         map[string]interface{} `json:"config"`                                             // 节点配置
	PythonScript   string                 `json:"python_script" example:"def execute(context): pass"` // Python脚本
	Inputs         []ParameterStructure   `json:"inputs"`                                             // 输入参数
	Outputs        []ParameterStructure   `json:"outputs"`                                            // 输出参数
}

// LineStructure 连接线结构
// @Description 连接线定义，连接两个节点并可包含条件
type LineStructure struct {
	ID        string              `json:"id" example:"line_1"`     // 连接线ID
	Source    string              `json:"source" example:"node_1"` // 源节点ID
	Target    string              `json:"target" example:"node_2"` // 目标节点ID
	Condition *ConditionStructure `json:"condition,omitempty"`     // 条件（可选）
}

// ConditionStructure 条件结构
// @Description 连接线上的条件定义
type ConditionStructure struct {
	Type           string `json:"type" example:"expression"`                    // 条件类型
	Expression     string `json:"expression" example:"result.confidence > 0.8"` // 条件表达式
	ExpressionView string `json:"expression_view" example:"置信度 > 0.8"`          // 条件表达式显示
}

// VariableStructure 变量结构
// @Description 全局变量定义
type VariableStructure struct {
	Name        string      `json:"name" example:"图片URL"`            // 变量名称
	KeyName     string      `json:"key_name" example:"image_url"`    // 变量标识
	Type        string      `json:"type" example:"string"`           // 变量类型
	Scope       string      `json:"scope" example:"global"`          // 变量作用域
	Default     interface{} `json:"default" example:""`              // 默认值
	Required    bool        `json:"required" example:"true"`         // 是否必填
	Description string      `json:"description" example:"待分析的图片URL"` // 变量描述
}

// ParameterStructure 参数结构
// @Description 节点的输入输出参数定义
type ParameterStructure struct {
	Name        string      `json:"name" example:"图片"`                                     // 参数名称
	KeyName     string      `json:"key_name" example:"image"`                              // 参数标识
	Type        string      `json:"type" example:"string"`                                 // 参数类型
	Required    bool        `json:"required" example:"true"`                               // 是否必填
	Default     interface{} `json:"default,omitempty"`                                     // 默认值
	RefKeyName  string      `json:"ref_key_name,omitempty" example:"start_node.image_url"` // 引用参数标识
	Description string      `json:"description" example:"待推理的图片URL"`                       // 参数描述
}

// Position 位置
// @Description 节点在画布上的位置
type Position struct {
	X float64 `json:"x" example:"100"` // X坐标
	Y float64 `json:"y" example:"100"` // Y坐标
}

// Scan 实现 sql.Scanner 接口
func (s *StructureJSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// Value 实现 driver.Valuer 接口
func (s StructureJSON) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// TableName 指定表名
func (Workflow) TableName() string {
	return "workflows"
}

// BeforeCreate GORM钩子
func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (w *Workflow) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = time.Now()
	return nil
}
