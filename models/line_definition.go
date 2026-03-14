/*
 * @module models/line_definition
 * @description 连接线定义相关数据模型
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 连接线定义 -> 连接线实例（运行时）
 * @rules 连接线定义存储节点之间的连接关系和条件
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.6节
 */

package models

import (
	"gorm.io/gorm"
)

// ConditionType 条件类型枚举
type ConditionType string

const (
	ConditionTypeNone       ConditionType = "none"       // 无条件
	ConditionTypeSimple     ConditionType = "simple"     // 简单条件
	ConditionTypeComplex    ConditionType = "complex"    // 复合条件
	ConditionTypeExpression ConditionType = "expression" // 表达式条件
)

// LogicType 逻辑类型枚举（用于复合条件）
type LogicType string

const (
	LogicTypeAnd LogicType = "and" // 与逻辑
	LogicTypeOr  LogicType = "or"  // 或逻辑
)

// LineDefinition 连接线定义模型
// @Description 工作流中节点之间的连接线定义
type LineDefinition struct {
	BaseModel
	WorkflowID              uint           `gorm:"not null;index;uniqueIndex:idx_workflow_line" json:"workflow_id" example:"1"`
	LineID                  string         `gorm:"type:varchar(100);not null;uniqueIndex:idx_workflow_line" json:"line_id" example:"line_1"`
	SourceNodeID            string         `gorm:"type:varchar(100);not null;index" json:"source_node_id" example:"node_1"`
	TargetNodeID            string         `gorm:"type:varchar(100);not null;index" json:"target_node_id" example:"node_2"`
	ConditionType           ConditionType  `gorm:"type:varchar(20);default:'none'" json:"condition_type" example:"expression"`
	LogicType               LogicType      `gorm:"type:varchar(10);default:'and'" json:"logic_type" example:"and"`
	ConditionExpression     string         `gorm:"type:text" json:"condition_expression" example:"result.confidence > 0.8"`
	ConditionExpressionView string         `gorm:"type:text" json:"condition_expression_view" example:"置信度 > 0.8"`
	Description             string         `gorm:"type:text" json:"description"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// TableName 指定表名
func (LineDefinition) TableName() string {
	return "line_definitions"
}
