/*
 * @module models/workflow_deployment
 * @description 工作流部署数据模型定义
 * @architecture 数据模型层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow 工作流发布 -> 部署到盒子 -> 盒子执行
 * @rules 工作流部署到盒子后，盒子可以独立执行工作流
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.11节
 */

package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// DeploymentStatus 部署状态枚举
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"     // 待部署
	DeploymentStatusDeploying  DeploymentStatus = "deploying"   // 部署中
	DeploymentStatusDeployed   DeploymentStatus = "deployed"    // 已部署
	DeploymentStatusFailed     DeploymentStatus = "failed"      // 部署失败
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back" // 已回滚
)

// WorkflowDeployment 工作流部署模型
// @Description 工作流部署到盒子的记录，包含部署状态和版本信息
type WorkflowDeployment struct {
	BaseModel
	Name            string           `gorm:"type:varchar(100);not null" json:"name" example:"视频分析部署-摄像头1"`
	Key             string           `gorm:"type:varchar(100);not null;uniqueIndex:idx_deployment_key" json:"key" example:"video_analysis_camera1"`
	Description     string           `gorm:"type:text" json:"description" example:"用于摄像头1的视频分析工作流部署"`
	WorkflowID      uint             `gorm:"not null;index:idx_workflow_id" json:"workflow_id" example:"1"`
	BoxID           uint             `gorm:"not null;index:idx_box_id" json:"box_id" example:"1"`
	WorkflowVersion int              `gorm:"not null" json:"workflow_version" example:"1"`
	Status          DeploymentStatus `gorm:"type:varchar(20);not null;default:'pending';column:deployment_status" json:"deployment_status" example:"pending"`
	WorkflowJSON    string           `gorm:"type:jsonb;not null" json:"workflow_json"`
	ParamOverrides  string           `gorm:"type:jsonb" json:"param_overrides,omitempty"`
	DeployedAt      *time.Time       `json:"deployed_at,omitempty" example:"2025-01-26T12:00:00Z"`
	RolledBackAt    *time.Time       `json:"rolled_back_at,omitempty" example:"2025-01-26T13:00:00Z"`
	PreviousVersion *int             `json:"previous_version,omitempty" example:"0"`
	ErrorMessage    string           `gorm:"type:text" json:"error_message,omitempty" example:""`
	DeployedBy      uint             `gorm:"index" json:"deployed_by" example:"1"`
}

// TableName 指定表名
func (WorkflowDeployment) TableName() string {
	return "workflow_deployments"
}

// BeforeCreate GORM钩子
func (w *WorkflowDeployment) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	w.CreatedAt = CustomTime{Time: now}
	w.UpdatedAt = CustomTime{Time: now}
	w.normalizeJSONFields()
	return nil
}

// BeforeUpdate GORM钩子
func (w *WorkflowDeployment) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = CustomTime{Time: time.Now()}
	w.normalizeJSONFields()
	return nil
}

// normalizeJSONFields 兜底处理 JSONB 字段
// PostgreSQL 的 jsonb 类型不接受空字符串，必须是合法 JSON 字面量
// 当模型字段为空字符串时，写入数据库会触发 SQLSTATE 22P02
// 此方法在 BeforeCreate/BeforeUpdate 钩子中自动调用，保证写入安全
func (w *WorkflowDeployment) normalizeJSONFields() {
	// WorkflowJSON: NOT NULL，空值兜底为空对象
	if w.WorkflowJSON == "" {
		w.WorkflowJSON = "{}"
	}
	// ParamOverrides: 可空，但 jsonb 仍不接受空字符串，空值兜底为 null
	if w.ParamOverrides == "" {
		w.ParamOverrides = "null"
	}
}

// MarshalJSON 自定义JSON序列化，使 workflow_json 输出为JSON对象而非字符串，param_overrides 输出为JSON数组
func (w WorkflowDeployment) MarshalJSON() ([]byte, error) {
	type Alias WorkflowDeployment
	aux := struct {
		*Alias
		WorkflowJSON   interface{} `json:"workflow_json"`
		ParamOverrides interface{} `json:"param_overrides,omitempty"`
	}{
		Alias: (*Alias)(&w),
	}

	if w.WorkflowJSON != "" {
		var v interface{}
		if err := json.Unmarshal([]byte(w.WorkflowJSON), &v); err == nil {
			aux.WorkflowJSON = v
		} else {
			aux.WorkflowJSON = w.WorkflowJSON
		}
	} else {
		aux.WorkflowJSON = map[string]interface{}{}
	}

	if w.ParamOverrides != "" {
		var v interface{}
		if err := json.Unmarshal([]byte(w.ParamOverrides), &v); err == nil {
			aux.ParamOverrides = v
		} else {
			aux.ParamOverrides = w.ParamOverrides
		}
	}

	return json.Marshal(aux)
}
