package models

import (
	"time"

	"gorm.io/gorm"
)

// Menu 管理平台菜单项。
type Menu struct {
	BaseModel
	ResourceID string `gorm:"type:varchar(100);not null;uniqueIndex" json:"resource_id"`
	ParentID   string `gorm:"type:varchar(100);index" json:"parent_id"`
	Name       string `gorm:"type:varchar(100);not null" json:"name"`
	Title      string `gorm:"type:varchar(100);not null" json:"title"`
	Path       string `gorm:"type:varchar(255)" json:"path"`
	Icon       string `gorm:"type:varchar(100)" json:"icon"`
	Component  string `gorm:"type:varchar(255)" json:"component,omitempty"`
	SortOrder  int    `gorm:"not null;default:0;index" json:"sort_order"`
	IsEnabled  bool   `gorm:"not null;default:true;index" json:"is_enabled"`
	IsVisible  bool   `gorm:"not null;default:true;index" json:"is_visible"`
	IsSystem   bool   `gorm:"not null;default:false" json:"is_system"`
	Remark     string `gorm:"type:text" json:"remark,omitempty"`

	Children []*Menu `gorm:"-" json:"children,omitempty"`
}

// TableName 指定菜单表位于现有权限体系的 postgrest schema。
func (Menu) TableName() string {
	return "postgrest.menus"
}

func (m *Menu) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	m.CreatedAt = CustomTime{Time: now}
	m.UpdatedAt = CustomTime{Time: now}
	return nil
}

func (m *Menu) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = CustomTime{Time: time.Now()}
	return nil
}

// RoleMenu 角色和菜单的关联关系。
type RoleMenu struct {
	RoleName   string     `gorm:"type:text;primaryKey" json:"role_name"`
	ResourceID string     `gorm:"type:varchar(100);primaryKey" json:"resource_id"`
	GrantedAt  CustomTime `gorm:"not null" json:"granted_at"`
	GrantedBy  string     `gorm:"type:text" json:"granted_by,omitempty"`
}

// TableName 指定角色菜单关系表位于现有权限体系的 postgrest schema。
func (RoleMenu) TableName() string {
	return "postgrest.role_menus"
}

func (rm *RoleMenu) BeforeCreate(tx *gorm.DB) error {
	if rm.GrantedAt.Time.IsZero() {
		rm.GrantedAt = CustomTime{Time: time.Now()}
	}
	return nil
}
