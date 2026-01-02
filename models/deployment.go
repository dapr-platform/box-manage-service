/*
 * @module models/deployment
 * @description 模型部署和任务管理相关数据模型定义
 * @architecture 数据模型层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow 模型上传 -> 部署 -> 任务创建 -> 运行监控
 * @rules 包含模型管理、任务配置、部署状态等功能
 * @dependencies gorm.io/gorm
 * @refs DESIGN-001.md, apis.md
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Task 任务模型
type Task struct {
	BaseModel
	Name           string         `json:"name" gorm:"not null;size:255"`         // 任务名称
	Description    string         `json:"description" gorm:"type:text"`          // 任务描述
	BoxID          *uint          `json:"box_id" gorm:"index"`                   // 修改为指针类型，允许为空
	VideoSourceID  uint           `json:"video_source_id" gorm:"not null;index"` // 关联的视频源ID
	TaskID         string         `json:"task_id" gorm:"not null;uniqueIndex"`   // 在盒子上的任务ID
	ExternalID     string         `json:"external_id" gorm:"size:255;index"`     // 盒子上的原始任务ID（同步时使用）
	SkipFrame      int            `json:"skip_frame" gorm:"default:0"`
	AutoStart      bool           `json:"auto_start" gorm:"default:false"`
	Status         TaskStatus     `json:"status" gorm:"not null;default:'pending'"`                   // 保留用于向后兼容
	ScheduleStatus ScheduleStatus `json:"schedule_status" gorm:"not null;default:'unassigned';index"` // 调度状态
	RunStatus      RunStatus      `json:"run_status" gorm:"not null;default:'stopped';index"`         // 运行状态
	Source         string         `json:"source" gorm:"not null;default:'manual';size:50"`            // 任务来源：manual(手动创建), synced(从盒子同步)
	IsSynchronized bool           `json:"is_synchronized" gorm:"default:false"`                       // 是否已同步

	// 任务配置
	OutputSettings OutputSettings    `json:"output_settings" gorm:"type:jsonb"`
	ROIs           ROIConfigList     `json:"rois" gorm:"type:jsonb"`
	InferenceTasks InferenceTaskList `json:"inference_tasks" gorm:"type:jsonb"`

	// 运行统计
	StartTime      *time.Time `json:"start_time"`
	StopTime       *time.Time `json:"stop_time"`
	TotalFrames    int64      `json:"total_frames" gorm:"default:0"`
	InferenceCount int64      `json:"inference_count" gorm:"default:0"`
	ForwardSuccess int64      `json:"forward_success" gorm:"default:0"`
	ForwardFailed  int64      `json:"forward_failed" gorm:"default:0"`
	LastError      string     `json:"last_error" gorm:"type:text"`

	// REQ-005 新增字段
	Tags              StringArray `json:"tags" gorm:"type:jsonb"`                 // 任务标签
	Priority          int         `json:"priority" gorm:"default:2"`              // 优先级(1-5)
	ScheduledAt       *time.Time  `json:"scheduled_at"`                           // 计划执行时间
	DeploymentID      string      `json:"deployment_id" gorm:"size:64"`           // 部署ID
	SubStatus         string      `json:"sub_status" gorm:"size:50"`              // 子状态
	Progress          float64     `json:"progress" gorm:"default:0"`              // 进度(0-100)
	RetryCount        int         `json:"retry_count" gorm:"default:0"`           // 重试次数
	MaxRetries        int         `json:"max_retries" gorm:"default:3"`           // 最大重试次数
	LastHeartbeat     time.Time   `json:"last_heartbeat"`                         // 最后心跳时间
	UseROItoInference bool        `json:"useROItoInference" gorm:"default:false"` // 是否使用ROI进行推理

	// 任务调度和亲和性配置
	AutoSchedule bool        `json:"auto_schedule" gorm:"default:false"` // 是否启用自动调度
	AffinityTags StringArray `json:"affinity_tags" gorm:"type:jsonb"`    // 亲和性标签，用于匹配盒子标签

	// 任务级别转发配置
	TaskLevelForwardInfos ForwardInfoList `json:"task_level_forward_infos" gorm:"type:jsonb"` // 任务级别转发配置列表

	// 创建者
	CreatedBy uint `json:"created_by" gorm:"index"`

	// 关联关系
	Box         Box         `json:"box,omitempty" gorm:"foreignKey:BoxID"`
	VideoSource VideoSource `json:"video_source,omitempty" gorm:"foreignKey:VideoSourceID"`
}

// OutputSettings 输出设置
type OutputSettings struct {
	SendFullImage int `json:"sendFullImage"` // 0=否，1=是
}

// ROIConfig ROI区域配置 - 兼容task.json格式
type ROIConfig struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`   // ROI名称
	Width  int     `json:"width"`  // 区域宽度
	Height int     `json:"height"` // 区域高度
	X      int     `json:"x"`      // X坐标
	Y      int     `json:"y"`      // Y坐标
	Areas  []Point `json:"areas"`  // 多边形顶点（可选）
}

// Point 坐标点
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ROIConfigList ROI配置列表
type ROIConfigList []ROIConfig

// InferenceTask 推理任务配置 - 兼容task.json格式
type InferenceTask struct {
	Type            string        `json:"type"`      // detection, segmentation, classification, custom
	ModelName       string        `json:"modelName"` // 保持向后兼容
	Threshold       float64       `json:"threshold,omitempty"`
	SendSSEImage    bool          `json:"sendSSEImage,omitempty"`
	BusinessProcess string        `json:"businessProcess,omitempty"` // Lua脚本路径
	RtspPushUrl     string        `json:"rtspPushUrl,omitempty"`
	ROIIds          []int         `json:"roiIds,omitempty"` // 关联的ROI ID列表
	ForwardInfos    []ForwardInfo `json:"forwardInfos,omitempty"`
	OriginalModelID string        `json:"originalModelId,omitempty"`
	// 为了向后兼容，保留单个ForwardInfo
	ForwardInfo *ForwardInfo `json:"forwardInfo,omitempty"`
}

// InferenceTaskList 推理任务列表
type InferenceTaskList []InferenceTask

// ForwardInfo 转发配置
type ForwardInfo struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Type     string `json:"type"` // mqtt, http_post, websocket
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Topic    string `json:"topic"` // MQTT主题或HTTP路径
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ForwardInfoList 转发配置列表类型
type ForwardInfoList []ForwardInfo

// Value 实现 driver.Valuer 接口
func (f ForwardInfoList) Value() (driver.Value, error) {
	return json.Marshal(f)
}

// Scan 实现 sql.Scanner 接口
func (f *ForwardInfoList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, f)
}

// Value 实现 driver.Valuer 接口
func (o OutputSettings) Value() (driver.Value, error) {
	return json.Marshal(o)
}

// Scan 实现 sql.Scanner 接口
func (o *OutputSettings) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, o)
}

// Value 实现 driver.Valuer 接口
func (r ROIConfigList) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan 实现 sql.Scanner 接口
func (r *ROIConfigList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, r)
}

// Value 实现 driver.Valuer 接口
func (i InferenceTaskList) Value() (driver.Value, error) {
	return json.Marshal(i)
}

// Scan 实现 sql.Scanner 接口
func (i *InferenceTaskList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, i)
}

// TableName 设置表名
func (Task) TableName() string {
	return "tasks"
}

// Start 启动任务
func (t *Task) Start() {
	t.Status = TaskStatusRunning
	t.RunStatus = RunStatusRunning
	now := time.Now()
	t.StartTime = &now
}

// Stop 停止任务
func (t *Task) Stop() {
	t.Status = TaskStatusCompleted
	t.RunStatus = RunStatusStopped
	now := time.Now()
	t.StopTime = &now
}

// Fail 任务失败
func (t *Task) Fail(errorMsg string) {
	t.Status = TaskStatusFailed
	t.RunStatus = RunStatusStopped
	t.LastError = errorMsg
	now := time.Now()
	t.StopTime = &now
}

// UpdateStats 更新运行统计
func (t *Task) UpdateStats(totalFrames, inferenceCount, forwardSuccess, forwardFailed int64) {
	t.TotalFrames = totalFrames
	t.InferenceCount = inferenceCount
	t.ForwardSuccess = forwardSuccess
	t.ForwardFailed = forwardFailed
}

// IsRunning 检查任务是否在运行
func (t *Task) IsRunning() bool {
	return t.RunStatus == RunStatusRunning
}

// AssignToBox 分配任务到盒子
func (t *Task) AssignToBox(boxID uint) {
	t.BoxID = &boxID
	t.ScheduleStatus = ScheduleStatusAssigned
	t.Status = TaskStatusScheduled // 保持向后兼容
}

// UnassignFromBox 从盒子上移除任务
func (t *Task) UnassignFromBox() {
	t.BoxID = nil
	t.ScheduleStatus = ScheduleStatusUnassigned
	t.RunStatus = RunStatusStopped
	t.Status = TaskStatusPending // 保持向后兼容
}

// IsAssigned 检查任务是否已分配到盒子
func (t *Task) IsAssigned() bool {
	return t.ScheduleStatus == ScheduleStatusAssigned && t.BoxID != nil
}

// BoxModel 盒子模型关联表
type BoxModel struct {
	BoxID   uint `json:"box_id" gorm:"primaryKey"`
	ModelID uint `json:"model_id" gorm:"primaryKey"`

	// 部署信息
	DeployedAt   time.Time   `json:"deployed_at"`
	Status       ModelStatus `json:"status" gorm:"default:'deployed'"`
	IsLoaded     bool        `json:"is_loaded" gorm:"default:false"`
	ErrorMessage string      `json:"error_message" gorm:"type:text"`

	// 关联关系
	Box Box `json:"box,omitempty" gorm:"foreignKey:BoxID"`
	// 注意：ModelID 引用原始模型或转换后模型，具体关联在业务层处理
}

// TableName 设置表名
func (BoxModel) TableName() string {
	return "box_models"
}

// StringArray 字符串数组类型
type StringArray []string

// Value 实现 driver.Valuer 接口
func (sa StringArray) Value() (driver.Value, error) {
	return json.Marshal(sa)
}

// Scan 实现 sql.Scanner 接口
func (sa *StringArray) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, sa)
}

// TaskExecution 任务执行记录模型
type TaskExecution struct {
	BaseModel
	TaskID       uint       `json:"task_id" gorm:"not null;index"`
	ExecutionID  string     `json:"execution_id" gorm:"not null;uniqueIndex"`
	BoxID        uint       `json:"box_id" gorm:"not null;index"`
	Status       string     `json:"status" gorm:"not null"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	ErrorMessage string     `json:"error_message" gorm:"type:text"`
	Logs         string     `json:"logs" gorm:"type:text"`
	Metrics      string     `json:"metrics" gorm:"type:jsonb"`

	// 关联关系
	Task Task `json:"task,omitempty" gorm:"foreignKey:TaskID"`
	Box  Box  `json:"box,omitempty" gorm:"foreignKey:BoxID"`
}

// TableName 设置表名
func (TaskExecution) TableName() string {
	return "task_executions"
}

// GenerateExecutionID 生成执行ID
func (te *TaskExecution) GenerateExecutionID() {
	if te.ExecutionID == "" {
		te.ExecutionID = "exec_" + time.Now().Format("20060102150405") + "_" + fmt.Sprintf("%d", te.TaskID)
	}
}

// Task模型的扩展方法

// SetTags 设置任务标签
func (t *Task) SetTags(tags []string) {
	t.Tags = StringArray(tags)
}

// GetTags 获取任务标签
func (t *Task) GetTags() []string {
	return []string(t.Tags)
}

// HasTag 检查是否包含指定标签
func (t *Task) HasTag(tag string) bool {
	for _, t := range t.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// AddTag 添加标签
func (t *Task) AddTag(tag string) {
	if !t.HasTag(tag) {
		t.Tags = append(t.Tags, tag)
	}
}

// RemoveTag 移除标签
func (t *Task) RemoveTag(tag string) {
	var newTags StringArray
	for _, existingTag := range t.Tags {
		if existingTag != tag {
			newTags = append(newTags, existingTag)
		}
	}
	t.Tags = newTags
}

// SetAffinityTags 设置亲和性标签
func (t *Task) SetAffinityTags(tags []string) {
	t.AffinityTags = StringArray(tags)
}

// GetAffinityTags 获取亲和性标签
func (t *Task) GetAffinityTags() []string {
	return []string(t.AffinityTags)
}

// 已移除SetModelKey方法，ModelKey在转换为盒子任务时动态生成

// CanRetry 检查任务是否可以重试
func (t *Task) CanRetry() bool {
	return t.Status == TaskStatusFailed && t.RetryCount < t.MaxRetries
}

// Retry 重试任务
func (t *Task) Retry() bool {
	if !t.CanRetry() {
		return false
	}
	t.RetryCount++
	t.Status = TaskStatusPending
	t.RunStatus = RunStatusStopped
	t.LastError = ""
	t.Progress = 0
	return true
}

// Pause 暂停任务（暂停只改变旧状态，运行状态保持 running，因为新状态只有 running/stopped）
func (t *Task) Pause() {
	if t.Status == TaskStatusRunning {
		t.Status = TaskStatusPaused
		// 注意：RunStatus 只有 running 和 stopped，暂停时仍然算 running
	}
}

// Resume 恢复任务
func (t *Task) Resume() {
	if t.Status == TaskStatusPaused {
		t.Status = TaskStatusRunning
	}
}

// 已移除GetModelKeys方法，ModelKey在转换为盒子任务时动态生成

// GetCompatibleBoxes 根据亲和性标签获取兼容的盒子
func (t *Task) IsCompatibleWithBox(box *Box) bool {
	// 如果没有设置亲和性标签，与所有盒子兼容
	if len(t.AffinityTags) == 0 {
		return true
	}

	boxTags := box.GetTags()
	affinityTags := t.GetAffinityTags()

	// 检查是否有匹配的标签
	for _, affinityTag := range affinityTags {
		for _, boxTag := range boxTags {
			if affinityTag == boxTag {
				return true
			}
		}
	}

	return false
}

// NormalizeInferenceTasks 标准化推理任务配置，处理向后兼容
func (t *Task) NormalizeInferenceTasks() {
	for i := range t.InferenceTasks {
		task := &t.InferenceTasks[i]

		// 处理单个ForwardInfo到ForwardInfos的转换
		if task.ForwardInfo != nil && len(task.ForwardInfos) == 0 {
			task.ForwardInfo.Enabled = true // 默认启用
			task.ForwardInfos = []ForwardInfo{*task.ForwardInfo}
		}

		// 确保所有ForwardInfos都有enabled字段
		for j := range task.ForwardInfos {
			if task.ForwardInfos[j].Enabled == false && task.ForwardInfos[j].Type != "" {
				task.ForwardInfos[j].Enabled = true
			}
		}
	}
}

// UpdateProgress 更新任务进度
func (t *Task) UpdateProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	t.Progress = progress
}

// UpdateHeartbeat 更新心跳时间
func (t *Task) UpdateHeartbeat() {
	t.LastHeartbeat = time.Now()
}

// IsHeartbeatTimeout 检查心跳是否超时
func (t *Task) IsHeartbeatTimeout(timeout time.Duration) bool {
	return time.Since(t.LastHeartbeat) > timeout
}

// GetPriorityName 获取优先级名称
func (t *Task) GetPriorityName() string {
	switch t.Priority {
	case 1:
		return "低"
	case 2:
		return "普通"
	case 3:
		return "高"
	case 4:
		return "紧急"
	case 5:
		return "最高"
	default:
		return "未知"
	}
}

// 已移除SetConvertedModelKey, IsUsingConvertedModel, GetModelDisplayName方法
// ModelKey在转换为盒子任务时动态生成

// Cancel 取消任务
func (t *Task) Cancel(reason string) {
	t.Status = TaskStatusCancelled
	t.RunStatus = RunStatusStopped
	t.LastError = "任务已取消: " + reason
	now := time.Now()
	t.StopTime = &now
}

// DeploymentTask 部署任务模型 - 管理单个或多个任务的部署过程
type DeploymentTask struct {
	BaseModel
	Name         string               `json:"name" gorm:"not null;size:255"`            // 部署任务名称
	Description  string               `json:"description" gorm:"type:text"`             // 部署描述
	Status       DeploymentTaskStatus `json:"status" gorm:"not null;default:'pending'"` // 部署状态
	Priority     int                  `json:"priority" gorm:"default:3"`                // 优先级(1-5)
	Progress     float64              `json:"progress" gorm:"default:0"`                // 总体进度(0-100)
	Message      string               `json:"message" gorm:"type:text"`                 // 当前状态消息
	ErrorMessage string               `json:"error_message" gorm:"type:text"`           // 错误信息

	// 部署配置
	TargetBoxIDs     UintArray        `json:"target_box_ids" gorm:"type:jsonb" swaggertype:"array,integer"` // 目标盒子ID列表
	TaskIDs          UintArray        `json:"task_ids" gorm:"type:jsonb" swaggertype:"array,integer"`       // 要部署的任务ID列表
	DeploymentConfig DeploymentConfig `json:"deployment_config" gorm:"type:jsonb" swaggertype:"object"`     // 部署配置

	// 执行统计
	TotalTasks     int `json:"total_tasks" gorm:"default:0"`     // 总任务数
	CompletedTasks int `json:"completed_tasks" gorm:"default:0"` // 已完成任务数
	FailedTasks    int `json:"failed_tasks" gorm:"default:0"`    // 失败任务数
	SkippedTasks   int `json:"skipped_tasks" gorm:"default:0"`   // 跳过任务数

	// 时间记录
	StartedAt     *time.Time     `json:"started_at"`                                          // 开始时间
	CompletedAt   *time.Time     `json:"completed_at"`                                        // 完成时间
	EstimatedTime *time.Duration `json:"estimated_time" swaggertype:"integer" example:"1800"` // 预计耗时(秒)

	// 日志和结果
	ExecutionLogs DeploymentLogList    `json:"execution_logs" gorm:"type:jsonb" swaggertype:"array,object"` // 执行日志
	Results       DeploymentResultList `json:"results" gorm:"type:jsonb" swaggertype:"array,object"`        // 部署结果

	// 创建者信息
	CreatedBy uint `json:"created_by" gorm:"index"` // 创建者ID
}

// DeploymentTaskStatus 部署任务状态
type DeploymentTaskStatus string

const (
	DeploymentTaskStatusPending   DeploymentTaskStatus = "pending"   // 等待执行
	DeploymentTaskStatusRunning   DeploymentTaskStatus = "running"   // 执行中
	DeploymentTaskStatusCompleted DeploymentTaskStatus = "completed" // 已完成
	DeploymentTaskStatusFailed    DeploymentTaskStatus = "failed"    // 失败
	DeploymentTaskStatusCancelled DeploymentTaskStatus = "cancelled" // 已取消

)

// DeploymentConfig 部署配置
type DeploymentConfig struct {
	ConcurrentLimit      int  `json:"concurrent_limit"`       // 并发限制
	RetryAttempts        int  `json:"retry_attempts"`         // 重试次数
	TimeoutSeconds       int  `json:"timeout_seconds"`        // 超时时间(秒)
	StopOnFirstFailure   bool `json:"stop_on_first_failure"`  // 遇到第一个失败时停止
	ValidateBeforeDeploy bool `json:"validate_before_deploy"` // 部署前验证
	CleanupOnFailure     bool `json:"cleanup_on_failure"`     // 失败时清理
}

// DeploymentLog 部署日志条目
type DeploymentLog struct {
	ID         uint                   `json:"id"`                                                      // 日志序号
	Timestamp  time.Time              `json:"timestamp"`                                               // 日志时间
	Level      string                 `json:"level"`                                                   // 日志级别: INFO, WARN, ERROR, DEBUG
	Phase      string                 `json:"phase"`                                                   // 部署阶段: init, validate, prepare, deploy, verify, cleanup
	Action     string                 `json:"action"`                                                  // 具体操作: get_task, check_box, convert_config, upload_model, create_task, etc
	Message    string                 `json:"message"`                                                 // 日志消息
	TaskID     *uint                  `json:"task_id,omitempty"`                                       // 关联的任务ID
	BoxID      *uint                  `json:"box_id,omitempty"`                                        // 关联的盒子ID
	Success    *bool                  `json:"success,omitempty"`                                       // 操作是否成功
	Duration   *time.Duration         `json:"duration,omitempty" swaggertype:"integer" example:"1500"` // 操作耗时(毫秒)
	Error      string                 `json:"error,omitempty"`                                         // 错误信息
	Details    map[string]interface{} `json:"details,omitempty"`                                       // 详细信息
	StackTrace string                 `json:"stack_trace,omitempty"`                                   // 错误堆栈（ERROR级别时）
}

// DeploymentResult 单个任务的部署结果
type DeploymentResult struct {
	TaskID    uint          `json:"task_id"`
	BoxID     uint          `json:"box_id"`
	Success   bool          `json:"success"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	ErrorCode string        `json:"error_code,omitempty"`
	StartTime time.Time     `json:"start_time"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration" swaggertype:"integer" example:"300"`
}

// UintArray uint数组类型
type UintArray []uint

// Value 实现 driver.Valuer 接口
func (ua UintArray) Value() (driver.Value, error) {
	return json.Marshal(ua)
}

// Scan 实现 sql.Scanner 接口
func (ua *UintArray) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, ua)
}

// Value 实现 driver.Valuer 接口
func (dc DeploymentConfig) Value() (driver.Value, error) {
	return json.Marshal(dc)
}

// Scan 实现 sql.Scanner 接口
func (dc *DeploymentConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, dc)
}

// DeploymentLogList 部署日志列表类型
type DeploymentLogList []DeploymentLog

// Value 实现 driver.Valuer 接口
func (dl DeploymentLogList) Value() (driver.Value, error) {
	return json.Marshal(dl)
}

// Scan 实现 sql.Scanner 接口
func (dl *DeploymentLogList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, dl)
}

// DeploymentResultList 部署结果列表类型
type DeploymentResultList []DeploymentResult

// Value 实现 driver.Valuer 接口
func (dr DeploymentResultList) Value() (driver.Value, error) {
	return json.Marshal(dr)
}

// Scan 实现 sql.Scanner 接口
func (dr *DeploymentResultList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, dr)
}

// TableName 设置表名
func (DeploymentTask) TableName() string {
	return "deployment_tasks"
}

// DeploymentTask的方法

// Start 开始执行部署任务
func (dt *DeploymentTask) Start() {
	dt.Status = DeploymentTaskStatusRunning
	now := time.Now()
	dt.StartedAt = &now
	dt.Progress = 0
	dt.Message = "部署任务开始执行"
}

// Complete 完成部署任务
func (dt *DeploymentTask) Complete() {
	dt.Status = DeploymentTaskStatusCompleted
	now := time.Now()
	dt.CompletedAt = &now
	dt.Progress = 100
	dt.Message = "部署任务执行完成"
}

// Fail 失败部署任务
func (dt *DeploymentTask) Fail(errorMsg string) {
	dt.Status = DeploymentTaskStatusFailed
	now := time.Now()
	dt.CompletedAt = &now
	dt.ErrorMessage = errorMsg
	dt.Message = "部署任务执行失败"
}

// Cancel 取消部署任务
func (dt *DeploymentTask) Cancel(reason string) {
	dt.Status = DeploymentTaskStatusCancelled
	now := time.Now()
	dt.CompletedAt = &now
	dt.ErrorMessage = "任务已取消: " + reason
	dt.Message = "部署任务已取消"
}

// UpdateProgress 更新进度
func (dt *DeploymentTask) UpdateProgress(completed, failed, skipped int) {
	dt.CompletedTasks = completed
	dt.FailedTasks = failed
	dt.SkippedTasks = skipped

	if dt.TotalTasks > 0 {
		dt.Progress = float64(completed+failed+skipped) / float64(dt.TotalTasks) * 100
	}

	dt.Message = fmt.Sprintf("已完成 %d/%d 个任务，失败 %d 个，跳过 %d 个",
		completed, dt.TotalTasks, failed, skipped)
}

// AddLog 添加日志 - 简化版本
func (dt *DeploymentTask) AddLog(level, message string, taskID, boxID *uint, details map[string]interface{}) {
	logID := uint(len(dt.ExecutionLogs) + 1)
	log := DeploymentLog{
		ID:        logID,
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		TaskID:    taskID,
		BoxID:     boxID,
		Details:   details,
	}
	dt.ExecutionLogs = append(dt.ExecutionLogs, log)
}

// AddDetailedLog 添加详细日志
func (dt *DeploymentTask) AddDetailedLog(level, phase, action, message string, taskID, boxID *uint, success *bool, duration *time.Duration, err error, details map[string]interface{}) {
	logID := uint(len(dt.ExecutionLogs) + 1)
	log := DeploymentLog{
		ID:        logID,
		Timestamp: time.Now(),
		Level:     level,
		Phase:     phase,
		Action:    action,
		Message:   message,
		TaskID:    taskID,
		BoxID:     boxID,
		Success:   success,
		Duration:  duration,
		Details:   details,
	}

	if err != nil {
		log.Error = err.Error()
		if level == "ERROR" {
			// 可以在这里添加堆栈跟踪
			log.StackTrace = fmt.Sprintf("%+v", err)
		}
	}

	dt.ExecutionLogs = append(dt.ExecutionLogs, log)
}

// AddPhaseLog 添加阶段日志
func (dt *DeploymentTask) AddPhaseLog(phase, action, message string, taskID, boxID *uint, success bool, duration time.Duration) {
	level := "INFO"
	if !success {
		level = "ERROR"
	}
	dt.AddDetailedLog(level, phase, action, message, taskID, boxID, &success, &duration, nil, nil)
}

// AddResult 添加部署结果
func (dt *DeploymentTask) AddResult(result DeploymentResult) {
	dt.Results = append(dt.Results, result)

	// 更新统计
	if result.Success {
		dt.CompletedTasks++
	} else {
		dt.FailedTasks++
	}

	dt.UpdateProgress(dt.CompletedTasks, dt.FailedTasks, dt.SkippedTasks)
}

// GetSuccessRate 获取成功率
func (dt *DeploymentTask) GetSuccessRate() float64 {
	total := dt.CompletedTasks + dt.FailedTasks
	if total == 0 {
		return 0
	}
	return float64(dt.CompletedTasks) / float64(total) * 100
}

// GetDuration 获取执行时长
func (dt *DeploymentTask) GetDuration() time.Duration {
	if dt.StartedAt == nil {
		return 0
	}
	endTime := time.Now()
	if dt.CompletedAt != nil {
		endTime = *dt.CompletedAt
	}
	return endTime.Sub(*dt.StartedAt)
}

// IsRunning 检查是否正在运行
func (dt *DeploymentTask) IsRunning() bool {
	return dt.Status == DeploymentTaskStatusRunning
}

// CanCancel 检查是否可以取消
func (dt *DeploymentTask) CanCancel() bool {
	return dt.Status == DeploymentTaskStatusPending || dt.Status == DeploymentTaskStatusRunning
}
