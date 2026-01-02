/*
 * @module models/model
 * @description 原始模型数据结构定义，支持 pt/onnx 类型模型管理
 * @architecture 数据模型层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow 模型定义 -> 数据库映射 -> 业务逻辑
 * @rules 原始模型文件管理，支持版本控制和元数据管理
 * @dependencies gorm.io/gorm
 * @refs REQ-002.md, DESIGN-003.md
 */

package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// OriginalModel 原始模型数据结构
// @Description 原始模型数据结构，用于管理用户上传的 pt/onnx 格式模型
type OriginalModel struct {
	BaseModel

	// 基本信息
	Name        string `json:"name" gorm:"size:255;not null" example:"yolo-v5" binding:"required"` // 模型名称
	Description string `json:"description" gorm:"type:text" example:"YOLO v5 目标检测模型"`              // 模型描述
	Version     string `json:"version" gorm:"size:50;not null" example:"1.0.0" binding:"required"` // 版本号

	// 文件信息
	FileName   string `json:"file_name" gorm:"size:255;not null" example:"yolo-v5.pt"`                                               // 原始文件名
	FilePath   string `json:"file_path" gorm:"size:500;not null" example:"/data/models/original/2025/01/28/yolo-v5-abc123.pt"`       // 文件存储路径
	FileSize   int64  `json:"file_size" gorm:"not null" example:"52428800"`                                                          // 文件大小（字节）
	FileMD5    string `json:"file_md5" gorm:"size:32;not null;unique" example:"d41d8cd98f00b204e9800998ecf8427e"`                    // 文件MD5校验和
	FileSHA256 string `json:"file_sha256" gorm:"size:64" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"` // 文件SHA256校验和

	// 模型属性
	ModelType   OriginalModelType `json:"model_type" gorm:"size:10;not null" example:"pt" binding:"required"` // 模型类型 (pt/onnx)
	Framework   string            `json:"framework" gorm:"size:50" example:"pytorch"`                         // 模型框架
	ModelFormat string            `json:"model_format" gorm:"size:20" example:"pytorch"`                      // 模型格式

	// AI任务属性
	TaskType      ModelTaskType `json:"task_type" gorm:"size:20;default:'detection'" example:"detection"` // AI任务类型 (detection/segmentation)
	InputWidth    int           `json:"input_width" gorm:"default:640" example:"640"`                     // 输入宽度
	InputHeight   int           `json:"input_height" gorm:"default:640" example:"640"`                    // 输入高度
	InputChannels int           `json:"input_channels" gorm:"default:3" example:"3"`                      // 输入通道数

	// 状态管理
	Status         OriginalModelStatus `json:"status" gorm:"size:20;default:'uploading'" example:"uploading"` // 模型状态
	UploadProgress int                 `json:"upload_progress" gorm:"default:0" example:"100"`                // 上传进度 (0-100)
	IsValidated    bool                `json:"is_validated" gorm:"default:false" example:"false"`             // 是否已验证
	ValidationMsg  string              `json:"validation_msg" gorm:"type:text" example:""`                    // 验证信息
	StorageClass   string              `json:"storage_class" gorm:"size:20;default:'hot'" example:"hot"`      // 存储级别 (hot/warm/cold)

	// 元数据
	Tags     string `json:"tags" gorm:"type:text" example:"object_detection,yolo,pytorch"`             // 标签，逗号分隔
	Author   string `json:"author" gorm:"size:100" example:"张三"`                                       // 作者
	ModelURL string `json:"model_url" gorm:"size:500" example:"https://github.com/ultralytics/yolov5"` // 模型源地址

	// 访问控制
	UserID        uint `json:"user_id" gorm:"not null" example:"1"`          // 上传用户ID
	DownloadCount int  `json:"download_count" gorm:"default:0" example:"10"` // 下载次数

	// 转换关联
	ConvertedModels []ConvertedModel `json:"converted_models,omitempty" gorm:"foreignKey:OriginalModelID"` // 关联的转换后模型

	LastAccessed time.Time `json:"last_accessed" example:"2025-01-28T12:00:00Z"` // 最后访问时间
}

// OriginalModelType 原始模型类型枚举
type OriginalModelType string

const (
	OriginalModelTypePT   OriginalModelType = "pt"   // PyTorch 模型
	OriginalModelTypeONNX OriginalModelType = "onnx" // ONNX 模型
)

// ModelTaskType AI任务类型枚举
type ModelTaskType string

const (
	ModelTaskTypeDetection    ModelTaskType = "detection"    // 目标检测
	ModelTaskTypeDetectionObb ModelTaskType = "detection_obb" // 目标检测OBB
	ModelTaskTypeSegmentation ModelTaskType = "segmentation" // 图像分割
)

// OriginalModelStatus 原始模型状态枚举
type OriginalModelStatus string

const (
	OriginalModelStatusUploading OriginalModelStatus = "uploading" // 上传中
	OriginalModelStatusFailed    OriginalModelStatus = "failed"    // 失败
	OriginalModelStatusReady     OriginalModelStatus = "ready"     // 就绪
)

// ConvertedModel 转换后模型（完整实体模型）
// @Description 转换后模型数据结构，支持bmodel等转换后格式的完整管理
type ConvertedModel struct {
	BaseModel

	// 基本信息
	Name        string `json:"name" gorm:"size:255;not null;index" example:"yolo-v5-bm1684x"`   // 转换后模型名称
	DisplayName string `json:"display_name" gorm:"size:255" example:"YOLO v5 目标检测模型 (BM1684X)"` // 显示名称
	Description string `json:"description" gorm:"type:text" example:"转换为BM1684X芯片优化的YOLO v5模型"` // 模型描述
	Version     string `json:"version" gorm:"size:50;not null" example:"1.0.0"`                 // 版本号

	// 关联信息
	OriginalModelID  uint   `json:"original_model_id" gorm:"not null;index" example:"1"`                // 原始模型ID
	ConversionTaskID string `json:"conversion_task_id" gorm:"size:64;index" example:"conv_task_abc123"` // 关联转换任务ID

	// 模型标识 - 格式: type-targetyoloVersion-targetChip-name
	ModelKey string `json:"model_key" gorm:"size:255;not null;uniqueIndex" example:"detection-yolov8-bm1684x-cow_head"` // 模型唯一标识

	// 文件信息
	FileName      string `json:"file_name" gorm:"size:255;not null" example:"yolo-v5-bm1684x.bmodel"`                                   // 转换后文件名
	FilePath      string `json:"file_path" gorm:"size:500;not null" example:"/data/models/converted/yolo-v5-bm1684x.bmodel"`            // 文件存储路径
	ConvertedPath string `json:"converted_path" gorm:"size:500;not null" example:"/data/models/converted/yolo-v5-bm1684x.bmodel"`       // 转换后文件路径（与FilePath相同，为了兼容数据库）
	FileSize      int64  `json:"file_size" gorm:"default:0" example:"52428800"`                                                         // 文件大小（字节）
	FileMD5       string `json:"file_md5" gorm:"size:32" example:"d41d8cd98f00b204e9800998ecf8427e"`                                    // 文件MD5校验和
	FileSHA256    string `json:"file_sha256" gorm:"size:64" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"` // 文件SHA256校验和

	// AI任务属性（继承自原始模型）
	TaskType      ModelTaskType `json:"task_type" gorm:"size:20" example:"detection"` // AI任务类型
	InputWidth    int           `json:"input_width" example:"640"`                    // 输入宽度
	InputHeight   int           `json:"input_height" example:"640"`                   // 输入高度
	InputChannels int           `json:"input_channels" example:"3"`                   // 输入通道数

	// 转换参数
	ConvertParams     string `json:"convert_params" gorm:"type:text" example:"{}"`                         // 转换参数 (JSON格式)
	Quantization      string `json:"quantization" gorm:"size:10" example:"F16"`                            // 量化类型 (F16/F32)
	TargetYoloVersion string `json:"target_yolo_version" gorm:"size:20;default:'yolov8'" example:"yolov8"` // 目标YOLO版本
	TargetChip        string `json:"target_chip" gorm:"size:20" example:"bm1684x"`                         // 目标芯片

	// 状态管理
	Status      ConvertedModelStatus `json:"status" gorm:"size:20;default:'completed'" example:"completed"` // 模型状态
	ConvertedAt time.Time            `json:"converted_at" example:"2025-01-28T12:00:00Z"`                   // 转换完成时间

	// 元数据和标签
	Tags   string `json:"tags" gorm:"type:text" example:"optimized,production,bm1684x"` // 标签，逗号分隔
	UserID uint   `json:"user_id" gorm:"not null" example:"1"`                          // 创建用户ID

	// 访问控制和统计
	AccessCount    int        `json:"access_count" gorm:"default:0" example:"50"`      // 访问次数
	DownloadCount  int        `json:"download_count" gorm:"default:0" example:"10"`    // 下载次数
	LastAccessedAt *time.Time `json:"last_accessed_at" example:"2025-01-28T12:00:00Z"` // 最后访问时间

	// 关联模型
	OriginalModel *OriginalModel `json:"original_model,omitempty" gorm:"foreignKey:OriginalModelID"` // 关联的原始模型
}

// ConvertedModelStatus 转换后模型状态
type ConvertedModelStatus string

const (
	ConvertedModelStatusPending    ConvertedModelStatus = "pending"    // 待转换
	ConvertedModelStatusConverting ConvertedModelStatus = "converting" // 转换中
	ConvertedModelStatusCompleted  ConvertedModelStatus = "completed"  // 已完成
	ConvertedModelStatusFailed     ConvertedModelStatus = "failed"     // 转换失败
)

// UploadSession 文件上传会话
// @Description 文件上传会话，支持分片上传和断点续传
type UploadSession struct {
	BaseModel

	// 会话信息
	SessionID string `json:"session_id" gorm:"size:64;unique;not null" example:"upload_session_123456"` // 会话ID
	UserID    uint   `json:"user_id" gorm:"not null" example:"1"`                                       // 用户ID

	// 文件信息
	FileName string `json:"file_name" gorm:"size:255;not null" example:"yolo-v5.pt"`            // 文件名
	FileSize int64  `json:"file_size" gorm:"not null" example:"52428800"`                       // 文件总大小
	FileMD5  string `json:"file_md5" gorm:"size:32" example:"d41d8cd98f00b204e9800998ecf8427e"` // 文件MD5

	// 分片信息
	ChunkSize      int64  `json:"chunk_size" gorm:"not null;default:5242880" example:"5242880"` // 分片大小 (5MB)
	TotalChunks    int    `json:"total_chunks" gorm:"not null" example:"10"`                    // 总分片数
	UploadedChunks string `json:"uploaded_chunks" gorm:"type:text" example:"1,2,3,5"`           // 已上传分片列表 (JSON数组)

	// 状态管理
	Status   UploadStatus `json:"status" gorm:"size:20;default:'initialized'" example:"uploading"` // 上传状态
	Progress int          `json:"progress" gorm:"default:0" example:"50"`                          // 上传进度 (0-100)
	ErrorMsg string       `json:"error_msg" gorm:"type:text" example:""`                           // 错误信息

	// 时间管理
	ExpiresAt   time.Time  `json:"expires_at" example:"2025-01-29T12:00:00Z"`   // 过期时间
	CompletedAt *time.Time `json:"completed_at" example:"2025-01-28T12:00:00Z"` // 完成时间

	// 存储信息
	TempDir   string `json:"temp_dir" gorm:"size:500" example:"/tmp/uploads/session_123456"`                          // 临时目录
	FinalPath string `json:"final_path" gorm:"size:500" example:"/data/models/original/2025/01/28/yolo-v5-abc123.pt"` // 最终路径

	// 元数据
	Metadata string `json:"metadata" gorm:"type:text" example:"{}"` // 额外元数据 (JSON)

	// 关联模型
	OriginalModelID *uint `json:"original_model_id" gorm:"index" example:"1"` // 关联的原始模型ID
}

// UploadStatus 上传状态枚举
type UploadStatus string

const (
	UploadStatusInitialized UploadStatus = "initialized" // 已初始化
	UploadStatusUploading   UploadStatus = "uploading"   // 上传中
	UploadStatusCompleted   UploadStatus = "completed"   // 已完成
	UploadStatusFailed      UploadStatus = "failed"      // 失败
	UploadStatusExpired     UploadStatus = "expired"     // 已过期
	UploadStatusCancelled   UploadStatus = "cancelled"   // 已取消
)

// ModelTag 模型标签
type ModelTag struct {
	BaseModel
	Name        string `json:"name" gorm:"size:50;unique;not null" example:"object_detection"` // 标签名称
	Description string `json:"description" gorm:"size:200" example:"目标检测相关模型"`                 // 标签描述
	Color       string `json:"color" gorm:"size:7;default:'#1890ff'" example:"#1890ff"`        // 标签颜色
	Usage       int    `json:"usage" gorm:"default:0" example:"5"`                             // 使用次数
}

// GetTagList 获取标签列表
func (m *OriginalModel) GetTagList() []string {
	if m.Tags == "" {
		return []string{}
	}

	tags := []string{}
	for _, tag := range splitTags(m.Tags) {
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// SetTagList 设置标签列表
func (m *OriginalModel) SetTagList(tags []string) {
	m.Tags = joinTags(tags)
}

// AddTag 添加标签
func (m *OriginalModel) AddTag(tag string) {
	if tag == "" {
		return
	}

	currentTags := m.GetTagList()
	for _, existingTag := range currentTags {
		if existingTag == tag {
			return // 标签已存在
		}
	}

	currentTags = append(currentTags, tag)
	m.SetTagList(currentTags)
}

// RemoveTag 移除标签
func (m *OriginalModel) RemoveTag(tag string) {
	currentTags := m.GetTagList()
	newTags := []string{}

	for _, existingTag := range currentTags {
		if existingTag != tag {
			newTags = append(newTags, existingTag)
		}
	}

	m.SetTagList(newTags)
}

// GetSizeFormatted 获取格式化的文件大小
func (m *OriginalModel) GetSizeFormatted() string {
	return formatFileSize(m.FileSize)
}

// GetUploadedChunkList 获取已上传分片列表
func (u *UploadSession) GetUploadedChunkList() []int {
	if u.UploadedChunks == "" {
		return []int{}
	}

	return parseChunkList(u.UploadedChunks)
}

// SetUploadedChunkList 设置已上传分片列表
func (u *UploadSession) SetUploadedChunkList(chunks []int) {
	u.UploadedChunks = formatChunkList(chunks)
	u.Progress = len(chunks) * 100 / u.TotalChunks
}

// AddUploadedChunk 添加已上传分片
func (u *UploadSession) AddUploadedChunk(chunkIndex int) {
	chunks := u.GetUploadedChunkList()

	// 检查是否已存在
	for _, existing := range chunks {
		if existing == chunkIndex {
			return
		}
	}

	chunks = append(chunks, chunkIndex)
	u.SetUploadedChunkList(chunks)
}

// IsCompleted 检查上传是否完成
func (u *UploadSession) IsCompleted() bool {
	return len(u.GetUploadedChunkList()) >= u.TotalChunks
}

// IsExpired 检查会话是否过期
func (u *UploadSession) IsExpired() bool {
	return time.Now().After(u.ExpiresAt)
}

// MarkAsCompleted 标记为完成
func (u *UploadSession) MarkAsCompleted() {
	u.Status = UploadStatusCompleted
	u.Progress = 100
	now := time.Now()
	u.CompletedAt = &now
}

// MarkAsFailed 标记为失败
func (u *UploadSession) MarkAsFailed(errorMsg string) {
	u.Status = UploadStatusFailed
	u.ErrorMsg = errorMsg
}

// 工具函数

func splitTags(tags string) []string {
	if tags == "" {
		return []string{}
	}

	result := []string{}
	for _, tag := range strings.Split(tags, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func joinTags(tags []string) string {
	validTags := []string{}
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			validTags = append(validTags, trimmed)
		}
	}
	return strings.Join(validTags, ",")
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

func parseChunkList(chunks string) []int {
	if chunks == "" {
		return []int{}
	}

	result := []int{}
	for _, chunk := range strings.Split(chunks, ",") {
		if num, err := strconv.Atoi(strings.TrimSpace(chunk)); err == nil {
			result = append(result, num)
		}
	}
	return result
}

func formatChunkList(chunks []int) string {
	strChunks := make([]string, len(chunks))
	for i, chunk := range chunks {
		strChunks[i] = strconv.Itoa(chunk)
	}
	return strings.Join(strChunks, ",")
}

// ConvertedModel 方法

// GetTagList 获取转换后模型的标签列表
func (cm *ConvertedModel) GetTagList() []string {
	if cm.Tags == "" {
		return []string{}
	}
	return splitTags(cm.Tags)
}

// SetTagList 设置转换后模型的标签列表
func (cm *ConvertedModel) SetTagList(tags []string) {
	cm.Tags = joinTags(tags)
}

// AddTag 添加标签到转换后模型
func (cm *ConvertedModel) AddTag(tag string) {
	if tag == "" {
		return
	}
	currentTags := cm.GetTagList()
	for _, existingTag := range currentTags {
		if existingTag == tag {
			return // 标签已存在
		}
	}
	currentTags = append(currentTags, tag)
	cm.SetTagList(currentTags)
}

// RemoveTag 从转换后模型移除标签
func (cm *ConvertedModel) RemoveTag(tag string) {
	currentTags := cm.GetTagList()
	newTags := []string{}
	for _, existingTag := range currentTags {
		if existingTag != tag {
			newTags = append(newTags, existingTag)
		}
	}
	cm.SetTagList(newTags)
}

// GetSizeFormatted 获取格式化的文件大小
func (cm *ConvertedModel) GetSizeFormatted() string {
	return formatFileSize(cm.FileSize)
}

// IsValidForDeployment 检查模型是否可以部署
func (cm *ConvertedModel) IsValidForDeployment() bool {
	return cm.Status == ConvertedModelStatusCompleted
}

// IncrementAccessCount 增加访问计数
func (cm *ConvertedModel) IncrementAccessCount() {
	cm.AccessCount++
	now := time.Now()
	cm.LastAccessedAt = &now
}

// IncrementDownloadCount 增加下载计数
func (cm *ConvertedModel) IncrementDownloadCount() {
	cm.DownloadCount++
}

// GetInputShapeArray 获取输入形状数组
func (cm *ConvertedModel) GetInputShapeArray() []int {
	// 基于 AI 任务属性构建输入形状数组
	return []int{1, cm.InputChannels, cm.InputHeight, cm.InputWidth}
}

// GetConvertParamsMap 获取转换参数映射
func (cm *ConvertedModel) GetConvertParamsMap() (map[string]interface{}, error) {
	if cm.ConvertParams == "" {
		return make(map[string]interface{}), nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(cm.ConvertParams), &params); err != nil {
		return nil, fmt.Errorf("解析转换参数失败: %w", err)
	}
	return params, nil
}

// SetConvertParamsMap 设置转换参数映射
func (cm *ConvertedModel) SetConvertParamsMap(params map[string]interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("序列化转换参数失败: %w", err)
	}
	cm.ConvertParams = string(data)
	return nil
}

// GenerateModelKey 生成模型键
// 格式: type-targetyoloVersion-targetChip-name
func (cm *ConvertedModel) GenerateModelKey() string {
	return fmt.Sprintf("%s-%s-%s-%s", cm.TaskType, cm.TargetYoloVersion, cm.TargetChip, cm.Name)
}

// SetModelKeyFromParams 根据参数设置模型键
func (cm *ConvertedModel) SetModelKeyFromParams() {
	cm.ModelKey = cm.GenerateModelKey()
}

// GetDisplayKey 获取用于显示的模型键
func (cm *ConvertedModel) GetDisplayKey() string {
	return fmt.Sprintf("%s (%s, %s, %s)", cm.Name, cm.TaskType, cm.TargetYoloVersion, cm.TargetChip)
}
