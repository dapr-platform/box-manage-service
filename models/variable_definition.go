/*
 * @module models/variable_definition
 * @description 变量定义相关数据模型
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 变量定义 -> 变量实例（运行时）
 * @rules 变量定义存储工作流中的变量配置
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.4节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
)

// VariableDirection 变量方向枚举
type VariableDirection string

const (
	VariableDirectionInput  VariableDirection = "input"  // 输入
	VariableDirectionOutput VariableDirection = "output" // 输出
)

// VariableDefinition 变量定义模型
// @Description 工作流中的变量定义
type VariableDefinition struct {
	BaseModel
	WorkflowID     uint              `gorm:"not null;index;uniqueIndex:idx_workflow_node_key" json:"workflow_id" example:"1"`
	NodeID         string            `gorm:"type:varchar(100);uniqueIndex:idx_workflow_node_key" json:"node_id" example:"node_1"` // 为空表示全局变量
	NodeTemplateID *uint             `gorm:"index" json:"node_template_id,omitempty" example:"1"`                                 // 来源节点模板ID，用于追溯参数定义来源
	KeyName        string            `gorm:"type:varchar(100);not null;uniqueIndex:idx_workflow_node_key" json:"key_name" example:"image_url"`
	Name           string            `gorm:"type:varchar(100);not null" json:"name" example:"图片URL"`
	Type           string            `gorm:"type:varchar(50);not null" json:"type" example:"string"` // string/number/boolean/object/array/reference
	Direction      VariableDirection `gorm:"type:varchar(10);not null" json:"direction" example:"input"`
	DefaultValue   JSONValue         `gorm:"type:jsonb" json:"default_value"`
	Required       bool              `gorm:"default:false" json:"required" example:"true"`
	RefKeyName     string            `gorm:"type:varchar(200)" json:"ref_key_name" example:"start_node.image_url"`
	Description    string            `gorm:"type:text" json:"description" example:"待分析的图片URL"`
}

// JSONValue JSON值类型
type JSONValue struct {
	Data interface{}
}

// Scan 实现 sql.Scanner 接口
func (j *JSONValue) Scan(value interface{}) error {
	if value == nil {
		j.Data = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, &j.Data)
}

// Value 实现 driver.Valuer 接口
func (j JSONValue) Value() (driver.Value, error) {
	if j.Data == nil {
		return nil, nil
	}
	return json.Marshal(j.Data)
}

// TableName 指定表名
func (VariableDefinition) TableName() string {
	return "variable_definitions"
}
