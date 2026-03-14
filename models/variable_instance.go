/*
 * @module models/variable_instance
 * @description 变量实例数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 工作流实例创建 -> 变量实例初始化 -> 变量值更新
 * @rules 变量实例记录工作流实例运行时的变量值
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.7节
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// VariableInstance 变量实例模型
// @Description 工作流实例运行时的变量实例，记录变量的当前值
type VariableInstance struct {
	BaseModel
	WorkflowInstanceID uint           `gorm:"not null;index:idx_workflow_instance" json:"workflow_instance_id" example:"1"`
	DeploymentID       *uint          `gorm:"index" json:"deployment_id,omitempty" example:"1"`
	VariableDefID      uint           `gorm:"not null;index" json:"variable_def_id" example:"1"`
	KeyName            string         `gorm:"type:varchar(100);not null;index" json:"key_name" example:"image_url"`
	Name               string         `gorm:"type:varchar(100);not null" json:"name" example:"图片URL"`
	Type               string         `gorm:"type:varchar(50);not null" json:"type" example:"string"`
	Value              ValueJSON      `gorm:"type:jsonb" json:"value"`
	RefKeyName         string         `gorm:"type:varchar(200)" json:"ref_key_name,omitempty" example:"start_node.image_url"` // 引用参数标识（格式：节点key_name.参数key_name）
	Scope              string         `gorm:"type:varchar(50);not null;default:'global'" json:"scope" example:"global"`       // 变量作用域（global/node）
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggerignore:"true"`
}

// ValueJSON 变量值JSON
// @Description 变量值的JSON表示，支持任意类型
type ValueJSON struct {
	Data interface{} `json:"value"`
}

// Scan 实现 sql.Scanner 接口
func (v *ValueJSON) Scan(value interface{}) error {
	if value == nil {
		v.Data = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	var result interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	v.Data = result
	return nil
}

// Value 实现 driver.Valuer 接口
func (v ValueJSON) Value() (driver.Value, error) {
	if v.Data == nil {
		return nil, nil
	}
	return json.Marshal(v.Data)
}

// TableName 指定表名
func (VariableInstance) TableName() string {
	return "variable_instances"
}

// BeforeCreate GORM钩子
func (v *VariableInstance) BeforeCreate(tx *gorm.DB) error {
	v.CreatedAt = time.Now()
	v.UpdatedAt = time.Now()
	return nil
}

// BeforeUpdate GORM钩子
func (v *VariableInstance) BeforeUpdate(tx *gorm.DB) error {
	v.UpdatedAt = time.Now()
	return nil
}
