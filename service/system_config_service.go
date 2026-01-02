// Package service 系统配置服务层
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"box-manage-service/models"
	"box-manage-service/repository"
)

// SystemConfigService 系统配置服务接口
type SystemConfigService interface {
	// 获取配置
	GetAllConfigs(ctx context.Context) (*models.AllSystemConfigs, error)
	GetConfigByType(ctx context.Context, configType string) (map[string]interface{}, error)
	GetConfigByKey(ctx context.Context, key string) (interface{}, error)
	GetConfigMetadata(ctx context.Context) ([]models.ConfigMetadata, error)
	
	// 更新配置
	UpdateConfig(ctx context.Context, key string, value interface{}) error
	UpdateConfigs(ctx context.Context, configs map[string]interface{}) error
	UpdateConfigByType(ctx context.Context, configType string, values map[string]interface{}) error
	
	// 重置配置
	ResetToDefault(ctx context.Context, key string) error
	ResetAllToDefault(ctx context.Context) error
	
	// 初始化
	InitializeConfigs(ctx context.Context) error
	
	// 缓存相关
	RefreshCache(ctx context.Context) error
	GetCachedConfig(key string) (interface{}, bool)
}

// systemConfigService 系统配置服务实现
type systemConfigService struct {
	repo  repository.SystemConfigRepository
	cache map[string]interface{}
	mu    sync.RWMutex
}

// NewSystemConfigService 创建系统配置服务
func NewSystemConfigService(repo repository.SystemConfigRepository) SystemConfigService {
	return &systemConfigService{
		repo:  repo,
		cache: make(map[string]interface{}),
	}
}

// GetAllConfigs 获取所有配置
func (s *systemConfigService) GetAllConfigs(ctx context.Context) (*models.AllSystemConfigs, error) {
	configs, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取配置列表失败: %w", err)
	}
	
	// 从数据库配置构建聚合配置结构
	result := models.GetDefaultAllConfigs()
	
	for _, config := range configs {
		if err := s.applyConfigToResult(config, &result); err != nil {
			log.Printf("应用配置 %s 失败: %v", config.ConfigKey, err)
		}
	}
	
	return &result, nil
}

// GetConfigByType 根据类型获取配置
func (s *systemConfigService) GetConfigByType(ctx context.Context, configType string) (map[string]interface{}, error) {
	configs, err := s.repo.GetByType(ctx, configType)
	if err != nil {
		return nil, fmt.Errorf("获取类型 %s 配置失败: %w", configType, err)
	}
	
	result := make(map[string]interface{})
	for _, config := range configs {
		var value interface{}
		if err := json.Unmarshal([]byte(config.ConfigValue), &value); err != nil {
			log.Printf("解析配置 %s 失败: %v", config.ConfigKey, err)
			continue
		}
		result[config.ConfigKey] = value
	}
	
	return result, nil
}

// GetConfigByKey 根据键获取配置
func (s *systemConfigService) GetConfigByKey(ctx context.Context, key string) (interface{}, error) {
	// 先从缓存获取
	if value, ok := s.GetCachedConfig(key); ok {
		return value, nil
	}
	
	config, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取配置 %s 失败: %w", key, err)
	}
	
	var value interface{}
	if err := json.Unmarshal([]byte(config.ConfigValue), &value); err != nil {
		return nil, fmt.Errorf("解析配置值失败: %w", err)
	}
	
	// 更新缓存
	s.mu.Lock()
	s.cache[key] = value
	s.mu.Unlock()
	
	return value, nil
}

// GetConfigMetadata 获取配置元数据
func (s *systemConfigService) GetConfigMetadata(ctx context.Context) ([]models.ConfigMetadata, error) {
	return models.GetConfigMetadataList(), nil
}

// UpdateConfig 更新单个配置
func (s *systemConfigService) UpdateConfig(ctx context.Context, key string, value interface{}) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置值失败: %w", err)
	}
	
	// 获取现有配置
	existing, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}
	
	existing.ConfigValue = string(valueJSON)
	if err := s.repo.Update(ctx, existing); err != nil {
		return fmt.Errorf("更新配置失败: %w", err)
	}
	
	// 更新缓存
	s.mu.Lock()
	s.cache[key] = value
	s.mu.Unlock()
	
	log.Printf("配置 %s 已更新", key)
	return nil
}

// UpdateConfigs 批量更新配置
func (s *systemConfigService) UpdateConfigs(ctx context.Context, configs map[string]interface{}) error {
	for key, value := range configs {
		if err := s.UpdateConfig(ctx, key, value); err != nil {
			return fmt.Errorf("更新配置 %s 失败: %w", key, err)
		}
	}
	return nil
}

// UpdateConfigByType 按类型更新配置
func (s *systemConfigService) UpdateConfigByType(ctx context.Context, configType string, values map[string]interface{}) error {
	// 获取该类型的所有配置键
	existingConfigs, err := s.repo.GetByType(ctx, configType)
	if err != nil {
		return fmt.Errorf("获取类型 %s 配置失败: %w", configType, err)
	}
	
	existingKeys := make(map[string]bool)
	for _, config := range existingConfigs {
		existingKeys[config.ConfigKey] = true
	}
	
	// 只更新存在的配置
	for key, value := range values {
		if existingKeys[key] {
			if err := s.UpdateConfig(ctx, key, value); err != nil {
				return fmt.Errorf("更新配置 %s 失败: %w", key, err)
			}
		}
	}
	
	return nil
}

// ResetToDefault 重置单个配置为默认值
func (s *systemConfigService) ResetToDefault(ctx context.Context, key string) error {
	// 获取默认值
	defaults := models.GetDefaultAllConfigs()
	defaultValue, err := s.getDefaultValueByKey(key, defaults)
	if err != nil {
		return fmt.Errorf("获取默认值失败: %w", err)
	}
	
	return s.UpdateConfig(ctx, key, defaultValue)
}

// ResetAllToDefault 重置所有配置为默认值
func (s *systemConfigService) ResetAllToDefault(ctx context.Context) error {
	return s.repo.InitDefaultConfigs(ctx)
}

// InitializeConfigs 初始化配置
func (s *systemConfigService) InitializeConfigs(ctx context.Context) error {
	if err := s.repo.InitDefaultConfigs(ctx); err != nil {
		return fmt.Errorf("初始化默认配置失败: %w", err)
	}
	
	// 刷新缓存
	return s.RefreshCache(ctx)
}

// RefreshCache 刷新缓存
func (s *systemConfigService) RefreshCache(ctx context.Context) error {
	configs, err := s.repo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("刷新缓存失败: %w", err)
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.cache = make(map[string]interface{})
	for _, config := range configs {
		var value interface{}
		if err := json.Unmarshal([]byte(config.ConfigValue), &value); err != nil {
			log.Printf("解析配置 %s 失败: %v", config.ConfigKey, err)
			continue
		}
		s.cache[config.ConfigKey] = value
	}
	
	log.Printf("配置缓存已刷新，共 %d 条配置", len(s.cache))
	return nil
}

// GetCachedConfig 从缓存获取配置
func (s *systemConfigService) GetCachedConfig(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.cache[key]
	return value, ok
}

// applyConfigToResult 将配置应用到结果结构
func (s *systemConfigService) applyConfigToResult(config *models.SystemConfig, result *models.AllSystemConfigs) error {
	var value interface{}
	if err := json.Unmarshal([]byte(config.ConfigValue), &value); err != nil {
		return err
	}
	
	switch config.ConfigKey {
	// 盒子配置
	case models.ConfigKeyBoxHeartbeatTimeout:
		result.Box.HeartbeatTimeoutSeconds = toInt(value)
	case models.ConfigKeyBoxMonitoringInterval:
		result.Box.MonitoringIntervalSeconds = toInt(value)
	case models.ConfigKeyBoxAutoOfflineMinutes:
		result.Box.AutoOfflineMinutes = toInt(value)
	case models.ConfigKeyBoxMaxConcurrentUpload:
		result.Box.MaxConcurrentModelUpload = toInt(value)
	case models.ConfigKeyBoxDiscoveryScanRange:
		result.Box.DiscoveryScanRange = toStringSlice(value)
	
	// 任务配置
	case models.ConfigKeyTaskAutoScheduleEnabled:
		result.Task.AutoScheduleEnabled = toBool(value)
	case models.ConfigKeyTaskScheduleIntervalSec:
		result.Task.ScheduleIntervalSeconds = toInt(value)
	case models.ConfigKeyTaskMaxConcurrentPerBox:
		result.Task.MaxConcurrentPerBox = toInt(value)
	case models.ConfigKeyTaskDefaultPriority:
		result.Task.DefaultPriority = toInt(value)
	case models.ConfigKeyTaskRetryMaxAttempts:
		result.Task.RetryMaxAttempts = toInt(value)
	case models.ConfigKeyTaskDeploymentTimeoutSec:
		result.Task.DeploymentTimeoutSeconds = toInt(value)
	
	// 模型配置
	case models.ConfigKeyModelAllowedTypes:
		result.Model.AllowedFileTypes = toStringSlice(value)
	case models.ConfigKeyModelMaxFileSizeMB:
		result.Model.MaxFileSizeMB = toInt64(value)
	case models.ConfigKeyModelChunkSizeMB:
		result.Model.ChunkSizeMB = toInt(value)
	case models.ConfigKeyModelSessionTimeout:
		result.Model.SessionTimeoutHours = toInt(value)
	case models.ConfigKeyModelCleanupDays:
		result.Model.CleanupDays = toInt(value)
	
	// 转换配置
	case models.ConfigKeyConversionTargetChips:
		result.Conversion.TargetChips = toStringSlice(value)
	case models.ConfigKeyConversionYoloVersions:
		result.Conversion.YoloVersions = toStringSlice(value)
	case models.ConfigKeyConversionQuantizations:
		result.Conversion.QuantizationTypes = toStringSlice(value)
	case models.ConfigKeyConversionMaxConcurrent:
		result.Conversion.MaxConcurrentTasks = toInt(value)
	case models.ConfigKeyConversionDefaultTimeout:
		result.Conversion.DefaultTimeoutHours = toInt(value)
	
	// 视频配置
	case models.ConfigKeyVideoExtractDefaultCount:
		result.Video.ExtractDefaultFrameCount = toInt(value)
	case models.ConfigKeyVideoExtractMaxCount:
		result.Video.ExtractMaxFrameCount = toInt(value)
	case models.ConfigKeyVideoExtractDefaultQuality:
		result.Video.ExtractDefaultQuality = toInt(value)
	case models.ConfigKeyVideoRecordDefaultDuration:
		result.Video.RecordDefaultDuration = toInt(value)
	case models.ConfigKeyVideoRecordMaxDuration:
		result.Video.RecordMaxDuration = toInt(value)
	case models.ConfigKeyVideoRecordDefaultFormat:
		result.Video.RecordDefaultFormat = toString(value)
	
	// 系统配置
	case models.ConfigKeySystemLogRetentionDays:
		result.System.LogRetentionDays = toInt(value)
	case models.ConfigKeySystemNotificationEnabled:
		result.System.NotificationEnabled = toBool(value)
	case models.ConfigKeySystemMaintenanceMode:
		result.System.MaintenanceMode = toBool(value)
	case models.ConfigKeySystemDefaultPageSize:
		result.System.DefaultPageSize = toInt(value)
	}
	
	return nil
}

// getDefaultValueByKey 根据键获取默认值
func (s *systemConfigService) getDefaultValueByKey(key string, defaults models.AllSystemConfigs) (interface{}, error) {
	switch key {
	// 盒子配置
	case models.ConfigKeyBoxHeartbeatTimeout:
		return defaults.Box.HeartbeatTimeoutSeconds, nil
	case models.ConfigKeyBoxMonitoringInterval:
		return defaults.Box.MonitoringIntervalSeconds, nil
	case models.ConfigKeyBoxAutoOfflineMinutes:
		return defaults.Box.AutoOfflineMinutes, nil
	case models.ConfigKeyBoxMaxConcurrentUpload:
		return defaults.Box.MaxConcurrentModelUpload, nil
	case models.ConfigKeyBoxDiscoveryScanRange:
		return defaults.Box.DiscoveryScanRange, nil
	
	// 任务配置
	case models.ConfigKeyTaskAutoScheduleEnabled:
		return defaults.Task.AutoScheduleEnabled, nil
	case models.ConfigKeyTaskScheduleIntervalSec:
		return defaults.Task.ScheduleIntervalSeconds, nil
	case models.ConfigKeyTaskMaxConcurrentPerBox:
		return defaults.Task.MaxConcurrentPerBox, nil
	case models.ConfigKeyTaskDefaultPriority:
		return defaults.Task.DefaultPriority, nil
	case models.ConfigKeyTaskRetryMaxAttempts:
		return defaults.Task.RetryMaxAttempts, nil
	case models.ConfigKeyTaskDeploymentTimeoutSec:
		return defaults.Task.DeploymentTimeoutSeconds, nil
	
	// 模型配置
	case models.ConfigKeyModelAllowedTypes:
		return defaults.Model.AllowedFileTypes, nil
	case models.ConfigKeyModelMaxFileSizeMB:
		return defaults.Model.MaxFileSizeMB, nil
	case models.ConfigKeyModelChunkSizeMB:
		return defaults.Model.ChunkSizeMB, nil
	case models.ConfigKeyModelSessionTimeout:
		return defaults.Model.SessionTimeoutHours, nil
	case models.ConfigKeyModelCleanupDays:
		return defaults.Model.CleanupDays, nil
	
	// 转换配置
	case models.ConfigKeyConversionTargetChips:
		return defaults.Conversion.TargetChips, nil
	case models.ConfigKeyConversionYoloVersions:
		return defaults.Conversion.YoloVersions, nil
	case models.ConfigKeyConversionQuantizations:
		return defaults.Conversion.QuantizationTypes, nil
	case models.ConfigKeyConversionMaxConcurrent:
		return defaults.Conversion.MaxConcurrentTasks, nil
	case models.ConfigKeyConversionDefaultTimeout:
		return defaults.Conversion.DefaultTimeoutHours, nil
	
	// 视频配置
	case models.ConfigKeyVideoExtractDefaultCount:
		return defaults.Video.ExtractDefaultFrameCount, nil
	case models.ConfigKeyVideoExtractMaxCount:
		return defaults.Video.ExtractMaxFrameCount, nil
	case models.ConfigKeyVideoExtractDefaultQuality:
		return defaults.Video.ExtractDefaultQuality, nil
	case models.ConfigKeyVideoRecordDefaultDuration:
		return defaults.Video.RecordDefaultDuration, nil
	case models.ConfigKeyVideoRecordMaxDuration:
		return defaults.Video.RecordMaxDuration, nil
	case models.ConfigKeyVideoRecordDefaultFormat:
		return defaults.Video.RecordDefaultFormat, nil
	
	// 系统配置
	case models.ConfigKeySystemLogRetentionDays:
		return defaults.System.LogRetentionDays, nil
	case models.ConfigKeySystemNotificationEnabled:
		return defaults.System.NotificationEnabled, nil
	case models.ConfigKeySystemMaintenanceMode:
		return defaults.System.MaintenanceMode, nil
	case models.ConfigKeySystemDefaultPageSize:
		return defaults.System.DefaultPageSize, nil
	
	default:
		return nil, fmt.Errorf("未知的配置键: %s", key)
	}
}

// ==================== 类型转换辅助函数 ====================

func toInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	default:
		return 0
	}
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int64:
		return val
	default:
		return 0
	}
}

func toBool(v interface{}) bool {
	if val, ok := v.(bool); ok {
		return val
	}
	return false
}

func toString(v interface{}) string {
	if val, ok := v.(string); ok {
		return val
	}
	return ""
}

func toStringSlice(v interface{}) []string {
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, len(arr))
		for i, item := range arr {
			if str, ok := item.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}

