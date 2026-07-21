package models

import (
	"time"

	"gorm.io/gorm"
)

// DictionaryFieldDefinition 字典字段定义。
type DictionaryFieldDefinition struct {
	BaseModel
	FieldKey    string `gorm:"type:varchar(100);not null;uniqueIndex" json:"field_key"`
	FieldName   string `gorm:"type:varchar(100);not null" json:"field_name"`
	FieldType   string `gorm:"type:varchar(50);not null;default:'string'" json:"field_type"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
	IsEnabled   bool   `gorm:"not null;default:true;index" json:"is_enabled"`
	IsSystem    bool   `gorm:"not null;default:false" json:"is_system"`
	Remark      string `gorm:"type:text" json:"remark,omitempty"`
}

func (DictionaryFieldDefinition) TableName() string {
	return "dictionary_field_definitions"
}

func (d *DictionaryFieldDefinition) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	d.CreatedAt = CustomTime{Time: now}
	d.UpdatedAt = CustomTime{Time: now}
	if d.FieldType == "" {
		d.FieldType = "string"
	}
	return nil
}

func (d *DictionaryFieldDefinition) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = CustomTime{Time: time.Now()}
	return nil
}

// DictionaryInstance 字典实例项。
type DictionaryInstance struct {
	BaseModel
	FieldKey    string `gorm:"type:varchar(100);not null;index;uniqueIndex:idx_dictionary_instances_field_key_instance_key" json:"field_key"`
	InstanceKey string `gorm:"type:varchar(100);not null;uniqueIndex:idx_dictionary_instances_field_key_instance_key" json:"instance_key"`
	Label       string `gorm:"type:varchar(255);not null" json:"label"`
	Value       string `gorm:"type:text" json:"value"`
	Extra       string `gorm:"type:text" json:"extra,omitempty"`
	SortOrder   int    `gorm:"not null;default:0;index" json:"sort_order"`
	IsEnabled   bool   `gorm:"not null;default:true;index" json:"is_enabled"`
	IsDefault   bool   `gorm:"not null;default:false" json:"is_default"`
	Remark      string `gorm:"type:text" json:"remark,omitempty"`
}

func (DictionaryInstance) TableName() string {
	return "dictionary_instances"
}

func (d *DictionaryInstance) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	d.CreatedAt = CustomTime{Time: now}
	d.UpdatedAt = CustomTime{Time: now}
	return nil
}

func (d *DictionaryInstance) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = CustomTime{Time: time.Now()}
	return nil
}
