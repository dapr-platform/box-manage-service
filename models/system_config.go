// Package models 系统配置数据模型
// 提供系统级配置的持久化存储，支持前端动态配置
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// SystemConfig 系统配置表
// 用于存储可通过前端配置的系统参数
type SystemConfig struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigKey   string         `gorm:"size:100;uniqueIndex;not null" json:"config_key"` // 配置键，唯一标识
	ConfigValue string         `gorm:"type:text" json:"config_value"`                   // 配置值，JSON格式存储
	ConfigType  string         `gorm:"size:50;not null" json:"config_type"`             // 配置类型：box/task/model/video/system
	Description string         `gorm:"size:500" json:"description"`                     // 配置描述
	IsSystem    bool           `gorm:"default:false" json:"is_system"`                  // 是否为系统配置（系统配置不可删除）
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (SystemConfig) TableName() string {
	return "system_configs"
}

// ==================== 配置类型常量 ====================

const (
	ConfigTypeBox        = "box"        // 盒子管理配置
	ConfigTypeTask       = "task"       // 任务调度配置
	ConfigTypeModel      = "model"      // 模型管理配置
	ConfigTypeConversion = "conversion" // 模型转换配置
	ConfigTypeVideo      = "video"      // 视频处理配置
	ConfigTypeSystem     = "system"     // 系统通用配置
)

// ==================== 配置键常量 ====================

// 盒子管理配置键
const (
	ConfigKeyBoxHeartbeatTimeout    = "box.heartbeat_timeout_seconds"    // 心跳超时时间（秒）
	ConfigKeyBoxDiscoveryScanRange  = "box.discovery_scan_range"         // 发现扫描IP范围
	ConfigKeyBoxMonitoringInterval  = "box.monitoring_interval_seconds"  // 监控刷新间隔（秒）
	ConfigKeyBoxAutoOfflineMinutes  = "box.auto_offline_minutes"         // 自动离线判定时间（分钟）
	ConfigKeyBoxMaxConcurrentUpload = "box.max_concurrent_model_upload"  // 最大并发模型上传数
)

// 任务调度配置键
const (
	ConfigKeyTaskAutoScheduleEnabled   = "task.auto_schedule_enabled"       // 自动调度开关
	ConfigKeyTaskScheduleIntervalSec   = "task.schedule_interval_seconds"   // 调度间隔（秒）
	ConfigKeyTaskMaxConcurrentPerBox   = "task.max_concurrent_per_box"      // 每盒子最大并发任务数
	ConfigKeyTaskDefaultPriority       = "task.default_priority"            // 默认任务优先级
	ConfigKeyTaskRetryMaxAttempts      = "task.retry_max_attempts"          // 任务重试最大次数
	ConfigKeyTaskDeploymentTimeoutSec  = "task.deployment_timeout_seconds"  // 部署超时时间（秒）
)

// 模型管理配置键
const (
	ConfigKeyModelAllowedTypes    = "model.allowed_file_types"    // 允许的文件类型列表
	ConfigKeyModelMaxFileSizeMB   = "model.max_file_size_mb"      // 最大文件大小（MB）
	ConfigKeyModelChunkSizeMB     = "model.chunk_size_mb"         // 分片大小（MB）
	ConfigKeyModelSessionTimeout  = "model.session_timeout_hours" // 会话超时时间（小时）
	ConfigKeyModelCleanupDays     = "model.cleanup_days"          // 清理天数
)

// 模型转换配置键
const (
	ConfigKeyConversionTargetChips     = "conversion.target_chips"          // 支持的目标芯片列表
	ConfigKeyConversionYoloVersions    = "conversion.yolo_versions"         // 支持的YOLO版本列表
	ConfigKeyConversionQuantizations   = "conversion.quantization_types"    // 支持的量化类型列表
	ConfigKeyConversionMaxConcurrent   = "conversion.max_concurrent_tasks"  // 最大并发转换任务数
	ConfigKeyConversionDefaultTimeout  = "conversion.default_timeout_hours" // 默认转换超时（小时）
)

// 视频处理配置键
const (
	ConfigKeyVideoExtractDefaultCount  = "video.extract_default_frame_count" // 默认抽帧数量
	ConfigKeyVideoExtractMaxCount      = "video.extract_max_frame_count"     // 最大抽帧数量
	ConfigKeyVideoExtractDefaultQuality = "video.extract_default_quality"    // 默认抽帧质量
	ConfigKeyVideoRecordDefaultDuration = "video.record_default_duration"    // 默认录制时长（秒）
	ConfigKeyVideoRecordMaxDuration     = "video.record_max_duration"        // 最大录制时长（秒）
	ConfigKeyVideoRecordDefaultFormat   = "video.record_default_format"      // 默认录制格式
)

// 系统通用配置键
const (
	ConfigKeySystemLogRetentionDays    = "system.log_retention_days"        // 日志保留天数
	ConfigKeySystemNotificationEnabled = "system.notification_enabled"      // 通知开关
	ConfigKeySystemMaintenanceMode     = "system.maintenance_mode"          // 维护模式开关
	ConfigKeySystemDefaultPageSize     = "system.default_page_size"         // 默认分页大小
)

// ==================== 配置值结构体 ====================

// BoxConfigValues 盒子管理配置值
type BoxConfigValues struct {
	HeartbeatTimeoutSeconds    int      `json:"heartbeat_timeout_seconds"`    // 心跳超时时间（秒），默认60
	DiscoveryScanRange         []string `json:"discovery_scan_range"`         // 发现扫描IP范围，如 ["192.168.1.1-192.168.1.254"]
	MonitoringIntervalSeconds  int      `json:"monitoring_interval_seconds"`  // 监控刷新间隔（秒），默认30
	AutoOfflineMinutes         int      `json:"auto_offline_minutes"`         // 自动离线判定时间（分钟），默认5
	MaxConcurrentModelUpload   int      `json:"max_concurrent_model_upload"`  // 最大并发模型上传数，默认3
}

// TaskConfigValues 任务调度配置值
type TaskConfigValues struct {
	AutoScheduleEnabled      bool   `json:"auto_schedule_enabled"`       // 自动调度开关，默认true
	ScheduleIntervalSeconds  int    `json:"schedule_interval_seconds"`   // 调度间隔（秒），默认60
	MaxConcurrentPerBox      int    `json:"max_concurrent_per_box"`      // 每盒子最大并发任务数，默认10
	DefaultPriority          int    `json:"default_priority"`            // 默认任务优先级（1-5），默认3
	RetryMaxAttempts         int    `json:"retry_max_attempts"`          // 任务重试最大次数，默认3
	DeploymentTimeoutSeconds int    `json:"deployment_timeout_seconds"`  // 部署超时时间（秒），默认300
}

// ModelConfigValues 模型管理配置值
type ModelConfigValues struct {
	AllowedFileTypes     []string `json:"allowed_file_types"`      // 允许的文件类型，默认[".pt", ".onnx"]
	MaxFileSizeMB        int64    `json:"max_file_size_mb"`        // 最大文件大小（MB），默认10240
	ChunkSizeMB          int      `json:"chunk_size_mb"`           // 分片大小（MB），默认5
	SessionTimeoutHours  int      `json:"session_timeout_hours"`   // 会话超时时间（小时），默认24
	CleanupDays          int      `json:"cleanup_days"`            // 清理天数，默认7
}

// ConversionConfigValues 模型转换配置值
type ConversionConfigValues struct {
	TargetChips          []string `json:"target_chips"`            // 支持的目标芯片列表
	YoloVersions         []string `json:"yolo_versions"`           // 支持的YOLO版本列表
	QuantizationTypes    []string `json:"quantization_types"`      // 支持的量化类型列表
	MaxConcurrentTasks   int      `json:"max_concurrent_tasks"`    // 最大并发转换任务数，默认3
	DefaultTimeoutHours  int      `json:"default_timeout_hours"`   // 默认转换超时（小时），默认2
}

// VideoConfigValues 视频处理配置值
type VideoConfigValues struct {
	ExtractDefaultFrameCount  int    `json:"extract_default_frame_count"`  // 默认抽帧数量，默认10
	ExtractMaxFrameCount      int    `json:"extract_max_frame_count"`      // 最大抽帧数量，默认1000
	ExtractDefaultQuality     int    `json:"extract_default_quality"`      // 默认抽帧质量（1-5），默认2
	RecordDefaultDuration     int    `json:"record_default_duration"`      // 默认录制时长（秒），默认60
	RecordMaxDuration         int    `json:"record_max_duration"`          // 最大录制时长（秒），默认3600
	RecordDefaultFormat       string `json:"record_default_format"`        // 默认录制格式，默认"mp4"
}

// SystemConfigValues 系统通用配置值
type SystemConfigValues struct {
	LogRetentionDays      int  `json:"log_retention_days"`       // 日志保留天数，默认30
	NotificationEnabled   bool `json:"notification_enabled"`     // 通知开关，默认true
	MaintenanceMode       bool `json:"maintenance_mode"`         // 维护模式开关，默认false
	DefaultPageSize       int  `json:"default_page_size"`        // 默认分页大小，默认20
}

// ==================== 聚合配置结构体 ====================

// AllSystemConfigs 所有系统配置的聚合结构
type AllSystemConfigs struct {
	Box        BoxConfigValues        `json:"box"`        // 盒子管理配置
	Task       TaskConfigValues       `json:"task"`       // 任务调度配置
	Model      ModelConfigValues      `json:"model"`      // 模型管理配置
	Conversion ConversionConfigValues `json:"conversion"` // 模型转换配置
	Video      VideoConfigValues      `json:"video"`      // 视频处理配置
	System     SystemConfigValues     `json:"system"`     // 系统通用配置
}

// ==================== 默认配置值 ====================

// GetDefaultBoxConfig 获取默认盒子配置
func GetDefaultBoxConfig() BoxConfigValues {
	return BoxConfigValues{
		HeartbeatTimeoutSeconds:   60,
		DiscoveryScanRange:        []string{"192.168.1.1-192.168.1.254"},
		MonitoringIntervalSeconds: 30,
		AutoOfflineMinutes:        5,
		MaxConcurrentModelUpload:  3,
	}
}

// GetDefaultTaskConfig 获取默认任务配置
func GetDefaultTaskConfig() TaskConfigValues {
	return TaskConfigValues{
		AutoScheduleEnabled:      true,
		ScheduleIntervalSeconds:  60,
		MaxConcurrentPerBox:      10,
		DefaultPriority:          3,
		RetryMaxAttempts:         3,
		DeploymentTimeoutSeconds: 300,
	}
}

// GetDefaultModelConfig 获取默认模型配置
func GetDefaultModelConfig() ModelConfigValues {
	return ModelConfigValues{
		AllowedFileTypes:    []string{".pt", ".onnx"},
		MaxFileSizeMB:       10240, // 10GB
		ChunkSizeMB:         5,
		SessionTimeoutHours: 24,
		CleanupDays:         7,
	}
}

// GetDefaultConversionConfig 获取默认转换配置
func GetDefaultConversionConfig() ConversionConfigValues {
	return ConversionConfigValues{
		TargetChips:         []string{"BM1684", "BM1684X"},
		YoloVersions:        []string{"yolov5", "yolov8", "yolov9", "yolov10"},
		QuantizationTypes:   []string{"F16", "F32", "INT8"},
		MaxConcurrentTasks:  3,
		DefaultTimeoutHours: 2,
	}
}

// GetDefaultVideoConfig 获取默认视频配置
func GetDefaultVideoConfig() VideoConfigValues {
	return VideoConfigValues{
		ExtractDefaultFrameCount: 10,
		ExtractMaxFrameCount:     1000,
		ExtractDefaultQuality:    2,
		RecordDefaultDuration:    60,
		RecordMaxDuration:        3600,
		RecordDefaultFormat:      "mp4",
	}
}

// GetDefaultSystemConfig 获取默认系统配置
func GetDefaultSystemConfig() SystemConfigValues {
	return SystemConfigValues{
		LogRetentionDays:    30,
		NotificationEnabled: true,
		MaintenanceMode:     false,
		DefaultPageSize:     20,
	}
}

// GetDefaultAllConfigs 获取所有默认配置
func GetDefaultAllConfigs() AllSystemConfigs {
	return AllSystemConfigs{
		Box:        GetDefaultBoxConfig(),
		Task:       GetDefaultTaskConfig(),
		Model:      GetDefaultModelConfig(),
		Conversion: GetDefaultConversionConfig(),
		Video:      GetDefaultVideoConfig(),
		System:     GetDefaultSystemConfig(),
	}
}

// ==================== JSON转换辅助方法 ====================

// ConfigValueJSON 配置值JSON类型（用于数据库存储）
type ConfigValueJSON map[string]interface{}

// Value 实现driver.Valuer接口
func (c ConfigValueJSON) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// Scan 实现sql.Scanner接口
func (c *ConfigValueJSON) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal ConfigValueJSON value: %v", value)
	}
	return json.Unmarshal(bytes, c)
}

// ==================== 配置项元数据 ====================

// ConfigMetadata 配置项元数据，用于前端展示
type ConfigMetadata struct {
	Key          string      `json:"key"`           // 配置键
	Name         string      `json:"name"`          // 显示名称
	Description  string      `json:"description"`   // 描述
	Type         string      `json:"type"`          // 数据类型：string/int/bool/array
	Category     string      `json:"category"`      // 分类
	DefaultValue interface{} `json:"default_value"` // 默认值
	Validation   string      `json:"validation"`    // 验证规则描述
	Required     bool        `json:"required"`      // 是否必填
}

// GetConfigMetadataList 获取所有配置项的元数据列表
func GetConfigMetadataList() []ConfigMetadata {
	return []ConfigMetadata{
		// 盒子管理配置
		{Key: ConfigKeyBoxHeartbeatTimeout, Name: "心跳超时时间", Description: "盒子心跳超时判定时间（秒）", Type: "int", Category: ConfigTypeBox, DefaultValue: 60, Validation: "10-300", Required: true},
		{Key: ConfigKeyBoxMonitoringInterval, Name: "监控刷新间隔", Description: "盒子状态监控刷新间隔（秒）", Type: "int", Category: ConfigTypeBox, DefaultValue: 30, Validation: "5-120", Required: true},
		{Key: ConfigKeyBoxAutoOfflineMinutes, Name: "自动离线时间", Description: "盒子无心跳自动标记离线的时间（分钟）", Type: "int", Category: ConfigTypeBox, DefaultValue: 5, Validation: "1-60", Required: true},
		{Key: ConfigKeyBoxMaxConcurrentUpload, Name: "最大并发上传", Description: "盒子最大并发模型上传数", Type: "int", Category: ConfigTypeBox, DefaultValue: 3, Validation: "1-10", Required: true},
		
		// 任务调度配置
		{Key: ConfigKeyTaskAutoScheduleEnabled, Name: "自动调度开关", Description: "是否启用任务自动调度", Type: "bool", Category: ConfigTypeTask, DefaultValue: true, Validation: "", Required: true},
		{Key: ConfigKeyTaskScheduleIntervalSec, Name: "调度间隔", Description: "自动调度执行间隔（秒）", Type: "int", Category: ConfigTypeTask, DefaultValue: 60, Validation: "10-600", Required: true},
		{Key: ConfigKeyTaskMaxConcurrentPerBox, Name: "每盒子最大任务数", Description: "单个盒子允许的最大并发任务数", Type: "int", Category: ConfigTypeTask, DefaultValue: 10, Validation: "1-50", Required: true},
		{Key: ConfigKeyTaskDefaultPriority, Name: "默认任务优先级", Description: "新建任务的默认优先级（1-5）", Type: "int", Category: ConfigTypeTask, DefaultValue: 3, Validation: "1-5", Required: true},
		{Key: ConfigKeyTaskRetryMaxAttempts, Name: "任务重试次数", Description: "任务失败后最大重试次数", Type: "int", Category: ConfigTypeTask, DefaultValue: 3, Validation: "0-10", Required: true},
		{Key: ConfigKeyTaskDeploymentTimeoutSec, Name: "部署超时时间", Description: "任务部署超时时间（秒）", Type: "int", Category: ConfigTypeTask, DefaultValue: 300, Validation: "60-1800", Required: true},
		
		// 模型管理配置
		{Key: ConfigKeyModelAllowedTypes, Name: "允许的文件类型", Description: "允许上传的模型文件扩展名", Type: "array", Category: ConfigTypeModel, DefaultValue: []string{".pt", ".onnx"}, Validation: "", Required: true},
		{Key: ConfigKeyModelMaxFileSizeMB, Name: "最大文件大小", Description: "单个模型文件最大大小（MB）", Type: "int", Category: ConfigTypeModel, DefaultValue: 10240, Validation: "100-102400", Required: true},
		{Key: ConfigKeyModelChunkSizeMB, Name: "分片大小", Description: "分片上传的分片大小（MB）", Type: "int", Category: ConfigTypeModel, DefaultValue: 5, Validation: "1-100", Required: true},
		{Key: ConfigKeyModelSessionTimeout, Name: "会话超时时间", Description: "上传会话超时时间（小时）", Type: "int", Category: ConfigTypeModel, DefaultValue: 24, Validation: "1-72", Required: true},
		
		// 模型转换配置
		{Key: ConfigKeyConversionTargetChips, Name: "支持的目标芯片", Description: "支持转换的目标芯片列表", Type: "array", Category: ConfigTypeConversion, DefaultValue: []string{"BM1684", "BM1684X"}, Validation: "", Required: true},
		{Key: ConfigKeyConversionYoloVersions, Name: "支持的YOLO版本", Description: "支持转换的YOLO版本列表", Type: "array", Category: ConfigTypeConversion, DefaultValue: []string{"yolov5", "yolov8", "yolov9", "yolov10"}, Validation: "", Required: true},
		{Key: ConfigKeyConversionQuantizations, Name: "量化类型", Description: "支持的模型量化类型", Type: "array", Category: ConfigTypeConversion, DefaultValue: []string{"F16", "F32", "INT8"}, Validation: "", Required: true},
		{Key: ConfigKeyConversionMaxConcurrent, Name: "最大并发转换任务", Description: "同时进行的最大转换任务数", Type: "int", Category: ConfigTypeConversion, DefaultValue: 3, Validation: "1-10", Required: true},
		
		// 视频处理配置
		{Key: ConfigKeyVideoExtractDefaultCount, Name: "默认抽帧数量", Description: "视频抽帧任务默认提取帧数", Type: "int", Category: ConfigTypeVideo, DefaultValue: 10, Validation: "1-100", Required: true},
		{Key: ConfigKeyVideoExtractMaxCount, Name: "最大抽帧数量", Description: "单次抽帧任务最大帧数", Type: "int", Category: ConfigTypeVideo, DefaultValue: 1000, Validation: "10-10000", Required: true},
		{Key: ConfigKeyVideoRecordDefaultDuration, Name: "默认录制时长", Description: "录制任务默认时长（秒）", Type: "int", Category: ConfigTypeVideo, DefaultValue: 60, Validation: "10-3600", Required: true},
		{Key: ConfigKeyVideoRecordMaxDuration, Name: "最大录制时长", Description: "单次录制最大时长（秒）", Type: "int", Category: ConfigTypeVideo, DefaultValue: 3600, Validation: "60-86400", Required: true},
		{Key: ConfigKeyVideoRecordDefaultFormat, Name: "默认录制格式", Description: "录制任务默认输出格式", Type: "string", Category: ConfigTypeVideo, DefaultValue: "mp4", Validation: "mp4/mkv/flv", Required: true},
		
		// 系统通用配置
		{Key: ConfigKeySystemLogRetentionDays, Name: "日志保留天数", Description: "系统日志保留天数", Type: "int", Category: ConfigTypeSystem, DefaultValue: 30, Validation: "7-365", Required: true},
		{Key: ConfigKeySystemNotificationEnabled, Name: "通知开关", Description: "是否启用系统通知", Type: "bool", Category: ConfigTypeSystem, DefaultValue: true, Validation: "", Required: true},
		{Key: ConfigKeySystemMaintenanceMode, Name: "维护模式", Description: "是否开启维护模式", Type: "bool", Category: ConfigTypeSystem, DefaultValue: false, Validation: "", Required: true},
		{Key: ConfigKeySystemDefaultPageSize, Name: "默认分页大小", Description: "列表查询默认分页大小", Type: "int", Category: ConfigTypeSystem, DefaultValue: 20, Validation: "10-100", Required: true},
	}
}

