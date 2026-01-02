/*
 * @module models/box
 * @description AI盒子数据模型定义
 * @architecture 数据模型层
 * @documentReference REQ-002: 盒子管理系统
 * @stateFlow 模型定义 -> 数据库映射 -> 业务逻辑
 * @rules 包含盒子基本信息、硬件配置、状态监控等字段
 * @dependencies gorm.io/gorm
 * @refs DESIGN-000.md
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Box AI盒子模型
type Box struct {
	BaseModel
	Name        string      `json:"name" gorm:"not null;uniqueIndex" validate:"required"`
	IPAddress   string      `json:"ip_address" gorm:"not null;uniqueIndex" validate:"required,ip"`
	Port        int         `json:"port" gorm:"default:8080" validate:"min=1,max=65535"`
	Status      BoxStatus   `json:"status" gorm:"default:'offline'" validate:"required"`
	Location    string      `json:"location" gorm:"size:255"`
	Description string      `json:"description" gorm:"type:text"`
	Hardware    Hardware    `json:"hardware" gorm:"type:jsonb"`
	Tags        StringArray `json:"tags" gorm:"type:jsonb;index"` // 盒子标签，用于任务调度匹配

	// 监控相关字段
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	Resources     Resources  `json:"resources" gorm:"type:jsonb"`

	// 设备能力信息（从/api/v1/meta获取）
	Meta      BoxMeta `json:"meta" gorm:"type:jsonb"`
	Version   string  `json:"version" gorm:"size:50"`    // 盒子软件版本
	BuildTime string  `json:"build_time" gorm:"size:50"` // 盒子软件构建时间

	// 设备认证信息（从心跳数据获取）
	ApiKey            string `json:"api_key" gorm:"size:255"`            // 盒子API密钥，用于调用盒子API时的认证
	DeviceFingerprint string `json:"device_fingerprint" gorm:"size:255"` // 设备指纹
	LicenseID         string `json:"license_id" gorm:"size:100"`         // 许可证ID
	Edition           string `json:"edition" gorm:"size:50"`             // 版本类型（如commercial）
	IsLicenseValid    bool   `json:"is_license_valid"`                   // 许可证是否有效
	// 关联字段
	CreatedBy uint `json:"created_by" gorm:"index"` // 创建者ID，对应用户表

	// 关联关系
	Tasks []Task `json:"tasks,omitempty" gorm:"foreignKey:BoxID"`
	// 注意：Models 关联通过 BoxModel 表在业务层处理，不在这里定义 GORM 关联
	Upgrades   []UpgradeTask  `json:"upgrades,omitempty" gorm:"foreignKey:BoxID"`
	Heartbeats []BoxHeartbeat `json:"heartbeats,omitempty" gorm:"foreignKey:BoxID"`
}

// Hardware 硬件配置结构（基础静态信息）
type Hardware struct {
	CPUCores          int    `json:"cpu_cores"`
	CPULogicalCores   int    `json:"cpu_logical_cores"`
	CPUModelName      string `json:"cpu_model_name"`
	Hostname          string `json:"hostname"`
	KernelArch        string `json:"kernel_arch"`
	KernelVersion     string `json:"kernel_version"`
	OS                string `json:"os"`
	Platform          string `json:"platform"`
	PlatformFamily    string `json:"platform_family"`
	PlatformVersion   string `json:"platform_version"`
	SDKVersion        string `json:"sdk_version"`
	SoftwareBuildTime string `json:"software_build_time"`
	SoftwareVersion   string `json:"software_version"`
}

// Resources 资源使用情况结构（当前动态状态）
type Resources struct {
	BoardTemperature      float64     `json:"board_temperature"`
	CoreTemperature       float64     `json:"core_temperature"`
	CPUPercent            []float64   `json:"cpu_percent"`
	CPUTotal              int         `json:"cpu_total"`
	CPUUsed               float64     `json:"cpu_used"`
	CPUUsedPercent        float64     `json:"cpu_used_percent"`
	DiskData              []DiskUsage `json:"disk_data"`
	IOCount               int64       `json:"io_count"`
	IOReadBytes           int64       `json:"io_read_bytes"`
	IOReadTime            int64       `json:"io_read_time"`
	IOWriteBytes          int64       `json:"io_write_bytes"`
	IOWriteTime           int64       `json:"io_write_time"`
	Load1                 float64     `json:"load1"`
	Load5                 float64     `json:"load5"`
	Load15                float64     `json:"load15"`
	LoadUsagePercent      float64     `json:"load_usage_percent"`
	MemoryAvailable       int64       `json:"memory_available"`
	MemoryTotal           int64       `json:"memory_total"`
	MemoryUsed            int64       `json:"memory_used"`
	MemoryUsedPercent     float64     `json:"memory_used_percent"`
	NetBytesRecv          int64       `json:"net_bytes_recv"`
	NetBytesSent          int64       `json:"net_bytes_sent"`
	NPUMemoryTotal        int         `json:"npu_memory_total"`
	NPUMemoryUsed         int         `json:"npu_memory_used"`
	Procs                 int64       `json:"procs"`
	ShotTime              string      `json:"shot_time"`
	SwapMemoryAvailable   int64       `json:"swap_memory_available"`
	SwapMemoryTotal       int64       `json:"swap_memory_total"`
	SwapMemoryUsed        int64       `json:"swap_memory_used"`
	SwapMemoryUsedPercent float64     `json:"swap_memory_used_percent"`
	TimeSinceUptime       string      `json:"time_since_uptime"`
	TPUUsed               int         `json:"tpu_used"`
	Uptime                int64       `json:"uptime"`
	VPPMemoryTotal        int         `json:"vpp_memory_total"`
	VPPMemoryUsed         int         `json:"vpp_memory_used"`
	VPUMemoryTotal        int         `json:"vpu_memory_total"`
	VPUMemoryUsed         int         `json:"vpu_memory_used"`
}

// DiskUsage 磁盘使用情况
type DiskUsage struct {
	Device            string  `json:"device"`
	Free              int64   `json:"free"`
	INodesFree        int64   `json:"inodes_free"`
	INodesTotal       int64   `json:"inodes_total"`
	INodesUsed        int64   `json:"inodes_used"`
	INodesUsedPercent float64 `json:"inodes_used_percent"`
	Path              string  `json:"path"`
	Total             int64   `json:"total"`
	Type              string  `json:"type"`
	Used              int64   `json:"used"`
	UsedPercent       float64 `json:"used_percent"`
}

// ResourceUsage 资源使用统计（兼容旧版本）
type ResourceUsage struct {
	Used       float64 `json:"used"`
	Total      float64 `json:"total"`
	Percentage float64 `json:"percentage"`
	Unit       string  `json:"unit"`
}

// BoxMeta 盒子设备能力元信息
type BoxMeta struct {
	SupportedTypes    []string          `json:"supported_types"`        // 支持的模型类型，如 ["detection", "segmentation"]
	SupportedVersions []string          `json:"supported_versions"`     // 支持的模型版本，如 ["yolov8"]
	SupportedHardware []string          `json:"supported_hardware"`     // 支持的硬件平台，如 ["BM1684", "BM1684X"]
	InferenceMapping  map[string]string `json:"inference_type_mapping"` // 推理类型映射
	FileLimits        FileLimits        `json:"file_limits"`            // 文件限制
	Defaults          MetaDefaults      `json:"defaults"`               // 默认配置
}

// FileLimits 文件限制信息
type FileLimits struct {
	ModelFileMaxSize      string   `json:"model_file_max_size"`     // 模型文件最大大小
	ImageFileMaxSize      string   `json:"image_file_max_size"`     // 图片文件最大大小
	SupportedModelFormats []string `json:"supported_model_formats"` // 支持的模型格式
	SupportedImageFormats []string `json:"supported_image_formats"` // 支持的图片格式
}

// MetaDefaults 默认配置
type MetaDefaults struct {
	ConfThreshold float64 `json:"conf_threshold"` // 默认置信度阈值
	NmsThreshold  float64 `json:"nms_threshold"`  // 默认NMS阈值
	Type          string  `json:"type"`           // 默认类型
	Version       string  `json:"version"`        // 默认版本
	Hardware      string  `json:"hardware"`       // 默认硬件
}

// Value 实现 driver.Valuer 接口，用于GORM存储JSON
func (h Hardware) Value() (driver.Value, error) {
	return json.Marshal(h)
}

// Scan 实现 sql.Scanner 接口，用于GORM从数据库读取JSON
func (h *Hardware) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, h)
}

// Value 实现 driver.Valuer 接口，用于GORM存储JSON
func (r Resources) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// GetTags 获取盒子标签
func (b *Box) GetTags() []string {
	return []string(b.Tags)
}

// Scan 实现 sql.Scanner 接口，用于GORM从数据库读取JSON
func (r *Resources) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, r)
}

// TableName 设置表名
func (Box) TableName() string {
	return "boxes"
}

// IsOnline 检查盒子是否在线
func (b *Box) IsOnline() bool {
	return b.Status == BoxStatusOnline &&
		b.LastHeartbeat != nil &&
		time.Since(*b.LastHeartbeat) < 5*time.Minute
}

// UpdateHeartbeat 更新心跳时间
func (b *Box) UpdateHeartbeat() {
	now := time.Now()
	b.LastHeartbeat = &now
}

// UpdateResources 更新资源使用情况
func (b *Box) UpdateResources(resources Resources) {
	b.Resources = resources
}

// SetTags 设置盒子标签
func (b *Box) SetTags(tags []string) {
	b.Tags = StringArray(tags)
}

// HasTag 检查盒子是否有指定标签
func (b *Box) HasTag(tag string) bool {
	for _, t := range b.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// AddTag 添加标签
func (b *Box) AddTag(tag string) {
	if !b.HasTag(tag) {
		b.Tags = append(b.Tags, tag)
	}
}

// RemoveTag 移除标签
func (b *Box) RemoveTag(tag string) {
	for i, t := range b.Tags {
		if t == tag {
			b.Tags = append(b.Tags[:i], b.Tags[i+1:]...)
			break
		}
	}
}

// CalculateTagMatchScore 计算与任务标签的匹配分数
func (b *Box) CalculateTagMatchScore(taskTags []string) int {
	if len(taskTags) == 0 {
		return 0
	}

	matchCount := 0
	for _, taskTag := range taskTags {
		if b.HasTag(taskTag) {
			matchCount++
		}
	}

	// 返回匹配百分比 * 100（避免浮点数）
	return (matchCount * 100) / len(taskTags)
}

// CanAcceptTask 检查盒子是否可以接受任务
func (b *Box) CanAcceptTask() bool {
	return b.IsOnline() &&
		b.Status == BoxStatusOnline &&
		b.Resources.CPUUsedPercent < 80 && // CPU使用率小于80%
		b.Resources.MemoryUsedPercent < 80 // 内存使用率小于80%
}

// BoxHeartbeat 盒子心跳记录模型
type BoxHeartbeat struct {
	BaseModel
	BoxID     uint      `json:"box_id" gorm:"not null;index"`
	Status    BoxStatus `json:"status" gorm:"not null"`
	Resources Resources `json:"resources" gorm:"type:jsonb"`
	Timestamp time.Time `json:"timestamp" gorm:"not null"`

	// 关联关系
	Box Box `json:"box,omitempty" gorm:"foreignKey:BoxID"`
}

// TableName 设置表名
func (BoxHeartbeat) TableName() string {
	return "box_heartbeats"
}

// BoxMeta JSON序列化方法
// Value 实现 driver.Valuer 接口，用于GORM存储JSON
func (m BoxMeta) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan 实现 sql.Scanner 接口，用于GORM读取JSON
func (m *BoxMeta) Scan(value interface{}) error {
	if value == nil {
		*m = BoxMeta{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, m)
}

// FileLimits JSON序列化方法
// Value 实现 driver.Valuer 接口，用于GORM存储JSON
func (f FileLimits) Value() (driver.Value, error) {
	return json.Marshal(f)
}

// Scan 实现 sql.Scanner 接口，用于GORM读取JSON
func (f *FileLimits) Scan(value interface{}) error {
	if value == nil {
		*f = FileLimits{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, f)
}

// MetaDefaults JSON序列化方法
// Value 实现 driver.Valuer 接口，用于GORM存储JSON
func (d MetaDefaults) Value() (driver.Value, error) {
	return json.Marshal(d)
}

// Scan 实现 sql.Scanner 接口，用于GORM读取JSON
func (d *MetaDefaults) Scan(value interface{}) error {
	if value == nil {
		*d = MetaDefaults{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, d)
}
