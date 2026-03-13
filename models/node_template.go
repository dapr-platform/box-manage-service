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
	TypeKey          string         `gorm:"type:varchar(50);not null;uniqueIndex" json:"type_key"`
	TypeName         string         `gorm:"type:varchar(100);not null" json:"type_name"`
	Category         string         `gorm:"type:varchar(20);not null;index" json:"category"` // logic/business
	Icon             string         `gorm:"type:varchar(255)" json:"icon"`
	Description      string         `gorm:"type:text" json:"description"`
	ConfigSchema     JSONSchema     `gorm:"type:jsonb" json:"config_schema"`
	InputSchema      JSONSchema     `gorm:"type:jsonb" json:"input_schema"`
	OutputSchema     JSONSchema     `gorm:"type:jsonb" json:"output_schema"`
	DefaultVariables JSONSchema     `gorm:"type:jsonb" json:"default_variables"` // 预定义的变量配置，拖入工作流时自动复制
	ScriptTemplate   string         `gorm:"type:text" json:"script_template"`
	StartNodeKey     string         `gorm:"type:varchar(50)" json:"start_node_key"`
	EndNodeKey       string         `gorm:"type:varchar(50)" json:"end_node_key"`
	IsSystem         bool           `gorm:"not null;default:false" json:"is_system"`
	IsEnabled        bool           `gorm:"not null;default:true;index" json:"is_enabled"`
	SortOrder        int            `gorm:"default:0;index" json:"sort_order"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
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

// BeforeCreate GORM钩子
func (nt *NodeTemplate) BeforeCreate(tx *gorm.DB) error {
	nt.CreatedAt = time.Now()
	nt.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (nt *NodeTemplate) BeforeUpdate(tx *gorm.DB) error {
	nt.UpdatedAt = time.Now()
	return nil
}
