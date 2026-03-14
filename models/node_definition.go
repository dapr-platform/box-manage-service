/*
 * @module models/node_definition
 * @description 节点定义相关数据模型
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 节点模板 -> 节点定义 -> 节点实例
 * @rules 节点定义存储工作流中实际使用的节点配置
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.12节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"

	"gorm.io/gorm"
)

// NodeDefinition 节点定义模型
// @Description 工作流中实际使用的节点定义
type NodeDefinition struct {
	BaseModel
	WorkflowID     uint           `gorm:"not null;index;uniqueIndex:idx_workflow_node" json:"workflow_id" example:"1"`
	NodeID         string         `gorm:"type:varchar(100);not null;uniqueIndex:idx_workflow_node" json:"node_id" example:"node_1"`
	NodeTemplateID uint           `gorm:"not null;index" json:"node_template_id" example:"1"`
	TypeKey        string         `gorm:"type:varchar(50);not null" json:"type_key" example:"start"`
	TypeName       string         `gorm:"type:varchar(100);not null" json:"type_name" example:"开始节点"`
	NodeName       string         `gorm:"type:varchar(100);not null" json:"node_name" example:"开始"`
	NodeKeyName    string         `gorm:"type:varchar(100);not null;uniqueIndex:idx_workflow_key_name" json:"node_key_name" example:"start_node"`
	GroupType      NodeGroupType  `gorm:"type:varchar(20);not null;default:'single'" json:"group_type"` // 节点分组类型（从模板继承）
	StartNodeKey   string         `gorm:"type:varchar(50)" json:"start_node_key"`                       // 成对节点的开始节点key（从模板继承）
	EndNodeKey     string         `gorm:"type:varchar(50)" json:"end_node_key"`                         // 成对节点的结束节点key（从模板继承）
	Config         JSONMap        `gorm:"type:jsonb" json:"config"`
	PythonScript   string         `gorm:"type:text" json:"python_script"`
	Inputs         JSONArr        `gorm:"type:jsonb" json:"inputs"`
	Outputs        JSONArr        `gorm:"type:jsonb" json:"outputs"`
	Position       JSONMap        `gorm:"type:jsonb" json:"position"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// JSONArr JSON数组类型
type JSONArr []interface{}

// Scan 实现 sql.Scanner 接口
func (j *JSONArr) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONArr, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value 实现 driver.Valuer 接口
func (j JSONArr) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// TableName 指定表名
func (NodeDefinition) TableName() string {
	return "node_definitions"
}
