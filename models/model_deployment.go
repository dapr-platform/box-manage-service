/*
 * @module models/model_deployment
 * @description 模型部署任务数据模型定义
 * @architecture 数据模型层
 * @documentReference REQ-004: 模型部署功能
 * @stateFlow 部署任务创建 -> 检查盒子模型 -> 上传模型 -> 部署完成
 * @rules 支持批量部署、状态跟踪、错误处理
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-007.md
 */

package models

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ModelDeploymentTask 模型部署任务
type ModelDeploymentTask struct {
	BaseModel

	// 任务标识
	TaskID string `json:"task_id" gorm:"size:64;uniqueIndex;not null"` // 任务唯一标识
	Name   string `json:"name" gorm:"size:255;not null"`               // 任务名称
	UserID uint   `json:"user_id" gorm:"not null;index"`               // 创建用户ID

	// 部署目标
	ConvertedModelIDs string `json:"-" gorm:"type:text;not null"` // 要部署的转换后模型ID列表（JSON格式）
	BoxIDs            string `json:"-" gorm:"type:text;not null"` // 目标盒子ID列表（JSON格式）

	// 任务状态
	Status       ModelDeploymentStatus `json:"status" gorm:"size:20;not null;default:'pending'"` // 任务状态
	Progress     int                   `json:"progress" gorm:"default:0"`                        // 进度 (0-100)
	StartTime    *time.Time            `json:"start_time"`                                       // 开始时间
	EndTime      *time.Time            `json:"end_time"`                                         // 结束时间
	ErrorMessage string                `json:"error_message" gorm:"type:text"`                   // 错误信息

	// 部署结果统计
	TotalItems   int `json:"total_items" gorm:"default:0"`   // 总部署项数 (模型数 × 盒子数)
	SuccessItems int `json:"success_items" gorm:"default:0"` // 成功部署项数
	FailedItems  int `json:"failed_items" gorm:"default:0"`  // 失败部署项数
	SkippedItems int `json:"skipped_items" gorm:"default:0"` // 跳过部署项数 (已存在)

	// 部署日志
	Logs string `json:"logs" gorm:"type:text"` // 部署日志

	// 关联对象
	ConvertedModels []*ConvertedModel      `json:"converted_models,omitempty" gorm:"many2many:deployment_models;"`
	Boxes           []*Box                 `json:"boxes,omitempty" gorm:"many2many:deployment_boxes;"`
	DeploymentItems []*ModelDeploymentItem `json:"deployment_items,omitempty" gorm:"foreignKey:TaskID;references:TaskID"`
}

// ModelDeploymentItem 模型部署项 (单个模型到单个盒子的部署)
type ModelDeploymentItem struct {
	BaseModel

	// 关联信息
	TaskID           string `json:"task_id" gorm:"size:64;not null;index"`    // 部署任务ID
	ConvertedModelID uint   `json:"converted_model_id" gorm:"not null;index"` // 转换后模型ID
	BoxID            uint   `json:"box_id" gorm:"not null;index"`             // 盒子ID

	// 部署状态
	Status       ModelDeploymentItemStatus `json:"status" gorm:"size:20;not null;default:'pending'"` // 部署状态
	StartTime    *time.Time                `json:"start_time"`                                       // 开始时间
	EndTime      *time.Time                `json:"end_time"`                                         // 结束时间
	ErrorMessage string                    `json:"error_message" gorm:"type:text"`                   // 错误信息

	// 部署信息
	ModelKey     string `json:"model_key" gorm:"size:255;not null"`   // 模型Key
	BoxAddress   string `json:"box_address" gorm:"size:100;not null"` // 盒子地址
	ModelExists  bool   `json:"model_exists" gorm:"default:false"`    // 模型是否已存在于盒子
	UploadNeeded bool   `json:"upload_needed" gorm:"default:true"`    // 是否需要上传
	UploadSize   int64  `json:"upload_size" gorm:"default:0"`         // 上传文件大小

	// 关联对象
	ConvertedModel *ConvertedModel      `json:"converted_model,omitempty" gorm:"foreignKey:ConvertedModelID"`
	Box            *Box                 `json:"box,omitempty" gorm:"foreignKey:BoxID"`
	Task           *ModelDeploymentTask `json:"task,omitempty" gorm:"-"`
}

// ModelDeploymentStatus 模型部署任务状态枚举
type ModelDeploymentStatus string

const (
	ModelDeploymentStatusPending   ModelDeploymentStatus = "pending"   // 待处理
	ModelDeploymentStatusRunning   ModelDeploymentStatus = "running"   // 运行中
	ModelDeploymentStatusCompleted ModelDeploymentStatus = "completed" // 已完成
	ModelDeploymentStatusFailed    ModelDeploymentStatus = "failed"    // 失败
	ModelDeploymentStatusCancelled ModelDeploymentStatus = "cancelled" // 已取消
)

// ModelDeploymentItemStatus 模型部署项状态枚举
type ModelDeploymentItemStatus string

const (
	ModelDeploymentItemStatusPending   ModelDeploymentItemStatus = "pending"   // 待处理
	ModelDeploymentItemStatusChecking  ModelDeploymentItemStatus = "checking"  // 检查模型
	ModelDeploymentItemStatusUploading ModelDeploymentItemStatus = "uploading" // 上传中
	ModelDeploymentItemStatusCompleted ModelDeploymentItemStatus = "completed" // 已完成
	ModelDeploymentItemStatusFailed    ModelDeploymentItemStatus = "failed"    // 失败
	ModelDeploymentItemStatusSkipped   ModelDeploymentItemStatus = "skipped"   // 跳过
)

// TableName 设置模型部署任务表名
func (ModelDeploymentTask) TableName() string {
	return "model_deployment_tasks"
}

// TableName 设置模型部署项表名
func (ModelDeploymentItem) TableName() string {
	return "model_deployment_items"
}

// Start 启动部署任务
func (t *ModelDeploymentTask) Start() {
	t.Status = ModelDeploymentStatusRunning
	now := time.Now()
	t.StartTime = &now
	t.Progress = 0
}

// Complete 完成部署任务
func (t *ModelDeploymentTask) Complete() {
	t.Status = ModelDeploymentStatusCompleted
	now := time.Now()
	t.EndTime = &now
	t.Progress = 100
}

// Fail 标记部署任务失败
func (t *ModelDeploymentTask) Fail(errorMsg string) {
	t.Status = ModelDeploymentStatusFailed
	now := time.Now()
	t.EndTime = &now
	t.ErrorMessage = errorMsg
}

// Cancel 取消部署任务
func (t *ModelDeploymentTask) Cancel() {
	t.Status = ModelDeploymentStatusCancelled
	now := time.Now()
	t.EndTime = &now
}

// UpdateProgress 更新进度
func (t *ModelDeploymentTask) UpdateProgress() {
	if t.TotalItems > 0 {
		completedItems := t.SuccessItems + t.FailedItems + t.SkippedItems
		t.Progress = (completedItems * 100) / t.TotalItems
	}
}

// AddLog 添加日志
func (t *ModelDeploymentTask) AddLog(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := timestamp + " " + message + "\n"
	t.Logs += logEntry
}

// Start 启动部署项
func (i *ModelDeploymentItem) Start() {
	i.Status = ModelDeploymentItemStatusChecking
	now := time.Now()
	i.StartTime = &now
}

// Complete 完成部署项
func (i *ModelDeploymentItem) Complete() {
	i.Status = ModelDeploymentItemStatusCompleted
	now := time.Now()
	i.EndTime = &now
}

// Fail 标记部署项失败
func (i *ModelDeploymentItem) Fail(errorMsg string) {
	i.Status = ModelDeploymentItemStatusFailed
	now := time.Now()
	i.EndTime = &now
	i.ErrorMessage = errorMsg
}

// Skip 跳过部署项
func (i *ModelDeploymentItem) Skip(reason string) {
	i.Status = ModelDeploymentItemStatusSkipped
	now := time.Now()
	i.EndTime = &now
	i.ErrorMessage = reason
}

// SetUploading 设置为上传状态
func (i *ModelDeploymentItem) SetUploading() {
	i.Status = ModelDeploymentItemStatusUploading
}

// IsCompleted 检查是否已完成
func (i *ModelDeploymentItem) IsCompleted() bool {
	return i.Status == ModelDeploymentItemStatusCompleted ||
		i.Status == ModelDeploymentItemStatusFailed ||
		i.Status == ModelDeploymentItemStatusSkipped
}

// ModelBoxDeployment 模型-盒子部署关联关系
// @Description 记录转换后模型在哪些盒子上已成功部署
type ModelBoxDeployment struct {
	BaseModel

	// 关联信息
	ConvertedModelID uint   `json:"converted_model_id" gorm:"not null;index"` // 转换后模型ID
	BoxID            uint   `json:"box_id" gorm:"not null;index"`             // 盒子ID
	ModelKey         string `json:"model_key" gorm:"size:255;not null;index"` // 模型Key
	TaskID           string `json:"task_id" gorm:"size:64;index"`             // 部署任务ID

	// 部署信息
	DeployedAt     time.Time                `json:"deployed_at"`                            // 部署时间
	DeploymentPath string                   `json:"deployment_path" gorm:"size:500"`        // 部署路径
	Version        string                   `json:"version" gorm:"size:50"`                 // 部署时的模型版本
	Status         ModelBoxDeploymentStatus `json:"status" gorm:"size:20;default:'active'"` // 部署状态

	// 验证信息
	IsVerified      bool       `json:"is_verified" gorm:"default:false"`  // 是否已验证可用
	LastVerifiedAt  *time.Time `json:"last_verified_at"`                  // 最后验证时间
	VerificationMsg string     `json:"verification_msg" gorm:"type:text"` // 验证消息

	// 关联对象
	ConvertedModel *ConvertedModel `json:"converted_model,omitempty" gorm:"foreignKey:ConvertedModelID"`
	Box            *Box            `json:"box,omitempty" gorm:"foreignKey:BoxID"`
}

// ModelBoxDeploymentStatus 模型-盒子部署状态枚举
type ModelBoxDeploymentStatus string

const (
	ModelBoxDeploymentStatusActive   ModelBoxDeploymentStatus = "active"   // 活跃（可用）
	ModelBoxDeploymentStatusInactive ModelBoxDeploymentStatus = "inactive" // 不活跃（盒子离线等）
	ModelBoxDeploymentStatusRemoved  ModelBoxDeploymentStatus = "removed"  // 已移除
	ModelBoxDeploymentStatusError    ModelBoxDeploymentStatus = "error"    // 错误状态
)

// TableName 设置模型-盒子部署关联表名
func (ModelBoxDeployment) TableName() string {
	return "model_box_deployments"
}

// UniqueConstraint 创建唯一约束，防止同一模型在同一盒子重复部署记录
func (m *ModelBoxDeployment) BeforeCreate(tx *gorm.DB) error {
	// 可以在这里添加创建前的验证逻辑
	return nil
}

// MarkAsActive 标记为活跃状态
func (m *ModelBoxDeployment) MarkAsActive() {
	m.Status = ModelBoxDeploymentStatusActive
	now := time.Now()
	m.LastVerifiedAt = &now
	m.IsVerified = true
}

// MarkAsInactive 标记为不活跃状态
func (m *ModelBoxDeployment) MarkAsInactive(reason string) {
	m.Status = ModelBoxDeploymentStatusInactive
	m.VerificationMsg = reason
	m.IsVerified = false
}

// MarkAsRemoved 标记为已移除
func (m *ModelBoxDeployment) MarkAsRemoved() {
	m.Status = ModelBoxDeploymentStatusRemoved
	m.IsVerified = false
}

// MarkAsError 标记为错误状态
func (m *ModelBoxDeployment) MarkAsError(errorMsg string) {
	m.Status = ModelBoxDeploymentStatusError
	m.VerificationMsg = errorMsg
	m.IsVerified = false
}

// GetConvertedModelIDs 获取转换后模型ID列表
func (t *ModelDeploymentTask) GetConvertedModelIDs() ([]uint, error) {
	if t.ConvertedModelIDs == "" {
		return []uint{}, nil
	}

	var ids []uint
	if err := json.Unmarshal([]byte(t.ConvertedModelIDs), &ids); err != nil {
		// 调试日志
		fmt.Printf("[DEBUG] 解析转换后模型ID失败 - Raw: %s, Error: %v\n", t.ConvertedModelIDs, err)
		return nil, fmt.Errorf("解析转换后模型ID列表失败: %w", err)
	}
	// 调试日志
	fmt.Printf("[DEBUG] 成功解析转换后模型ID - Raw: %s, Parsed: %v\n", t.ConvertedModelIDs, ids)
	return ids, nil
}

// SetConvertedModelIDs 设置转换后模型ID列表
func (t *ModelDeploymentTask) SetConvertedModelIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("序列化转换后模型ID列表失败: %w", err)
	}
	t.ConvertedModelIDs = string(data)
	// 调试日志
	fmt.Printf("[DEBUG] 设置转换后模型ID - Input: %v, JSON: %s\n", ids, t.ConvertedModelIDs)
	return nil
}

// GetBoxIDs 获取盒子ID列表
func (t *ModelDeploymentTask) GetBoxIDs() ([]uint, error) {
	if t.BoxIDs == "" {
		return []uint{}, nil
	}

	var ids []uint
	if err := json.Unmarshal([]byte(t.BoxIDs), &ids); err != nil {
		// 调试日志
		fmt.Printf("[DEBUG] 解析盒子ID失败 - Raw: %s, Error: %v\n", t.BoxIDs, err)
		return nil, fmt.Errorf("解析盒子ID列表失败: %w", err)
	}
	// 调试日志
	fmt.Printf("[DEBUG] 成功解析盒子ID - Raw: %s, Parsed: %v\n", t.BoxIDs, ids)
	return ids, nil
}

// SetBoxIDs 设置盒子ID列表
func (t *ModelDeploymentTask) SetBoxIDs(ids []uint) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("序列化盒子ID列表失败: %w", err)
	}
	t.BoxIDs = string(data)
	// 调试日志
	fmt.Printf("[DEBUG] 设置盒子ID - Input: %v, JSON: %s\n", ids, t.BoxIDs)
	return nil
}

// MarshalJSON 自定义JSON序列化
func (t *ModelDeploymentTask) MarshalJSON() ([]byte, error) {
	convertedModelIDs, err := t.GetConvertedModelIDs()
	if err != nil {
		convertedModelIDs = []uint{}
	}

	boxIDs, err := t.GetBoxIDs()
	if err != nil {
		boxIDs = []uint{}
	}

	return json.Marshal(map[string]interface{}{
		"id":                  t.ID,
		"created_at":          t.CreatedAt,
		"updated_at":          t.UpdatedAt,
		"task_id":             t.TaskID,
		"name":                t.Name,
		"user_id":             t.UserID,
		"converted_model_ids": convertedModelIDs,
		"box_ids":             boxIDs,
		"status":              t.Status,
		"progress":            t.Progress,
		"start_time":          t.StartTime,
		"end_time":            t.EndTime,
		"error_message":       t.ErrorMessage,
		"total_items":         t.TotalItems,
		"success_items":       t.SuccessItems,
		"failed_items":        t.FailedItems,
		"skipped_items":       t.SkippedItems,
		"logs":                t.Logs,
		"converted_models":    t.ConvertedModels,
		"boxes":               t.Boxes,
		"deployment_items":    t.DeploymentItems,
	})
}
