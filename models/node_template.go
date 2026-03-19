package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// NodeTemplate 节点模板模型
type NodeTemplate struct {
	BaseModel
	TypeKey        string        `gorm:"type:varchar(50);not null;uniqueIndex" json:"type_key"`
	TypeName       string        `gorm:"type:varchar(100);not null" json:"type_name"`
	Category       string        `gorm:"type:varchar(20);not null;index" json:"category"`                    // logic/business
	GroupType      NodeGroupType `gorm:"type:varchar(20);not null;default:'single';index" json:"group_type"` // 节点分组类型：single/paired/container
	Icon           string        `gorm:"type:varchar(255)" json:"icon"`
	Description    string        `gorm:"type:text" json:"description"`
	ConfigSchema   JSONSchema    `gorm:"type:jsonb" json:"config_schema"`
	StructureJSON  string        `gorm:"type:text;not null" json:"structure_json" swaggerignore:"true"` // JSON字符串，用于下发
	ScriptTemplate string        `gorm:"type:text" json:"script_template"`
	StartNodeKey   string        `gorm:"type:varchar(50)" json:"start_node_key"` // 成对节点的开始节点key（仅paired类型使用）
	EndNodeKey     string        `gorm:"type:varchar(50)" json:"end_node_key"`   // 成对节点的结束节点key（仅paired类型使用）
	IsSystem       bool          `gorm:"not null;default:false" json:"is_system"`
	IsEnabled      bool          `gorm:"not null;default:true;index" json:"is_enabled"`
	SortOrder      int           `gorm:"default:0;index" json:"sort_order"`

	Variables []VariableDefinition `gorm:"-" json:"variables,omitempty"` // 变量列表
}

// JSONSchema JSON Schema类型
type JSONSchema map[string]interface{}

// Scan 实现 sql.Scanner 接口
func (j *JSONSchema) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value 实现 driver.Valuer 接口
func (j JSONSchema) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// TableName 指定表名
func (NodeTemplate) TableName() string {
	return "node_templates"
}

// BuildStructureJSON 从 Variables 构建 StructureJSON 字符串
func (nt *NodeTemplate) BuildStructureJSON() error {
	structure := map[string]interface{}{
		"variables": nt.Variables,
	}

	bytes, err := json.Marshal(structure)
	if err != nil {
		return err
	}

	nt.StructureJSON = string(bytes)
	return nil
}

// ParseStructureJSON 解析 StructureJSON 字符串到 Variables
func (nt *NodeTemplate) ParseStructureJSON() error {
	if nt.StructureJSON == "" {
		return nil
	}

	var structure map[string]interface{}
	if err := json.Unmarshal([]byte(nt.StructureJSON), &structure); err != nil {
		return err
	}

	// 解析 variables
	if variablesData, ok := structure["variables"]; ok {
		variablesBytes, _ := json.Marshal(variablesData)
		json.Unmarshal(variablesBytes, &nt.Variables)
	}

	return nil
}

// BeforeCreate GORM钩子
func (nt *NodeTemplate) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	nt.CreatedAt = CustomTime{Time: now}
	nt.UpdatedAt = CustomTime{Time: now}
	return nil
}

// BeforeUpdate GORM钩子
func (nt *NodeTemplate) BeforeUpdate(tx *gorm.DB) error {
	nt.UpdatedAt = CustomTime{Time: time.Now()}
	return nil
}
