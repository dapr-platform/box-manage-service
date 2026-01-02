// Package repository 系统配置数据访问层
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"box-manage-service/models"

	"gorm.io/gorm"
)

// SystemConfigRepository 系统配置仓库接口
type SystemConfigRepository interface {
	// 基础CRUD
	Create(ctx context.Context, config *models.SystemConfig) error
	GetByID(ctx context.Context, id uint) (*models.SystemConfig, error)
	GetByKey(ctx context.Context, key string) (*models.SystemConfig, error)
	Update(ctx context.Context, config *models.SystemConfig) error
	Delete(ctx context.Context, id uint) error
	
	// 批量操作
	GetAll(ctx context.Context) ([]*models.SystemConfig, error)
	GetByType(ctx context.Context, configType string) ([]*models.SystemConfig, error)
	GetByKeys(ctx context.Context, keys []string) ([]*models.SystemConfig, error)
	SaveOrUpdate(ctx context.Context, config *models.SystemConfig) error
	SaveBatch(ctx context.Context, configs []*models.SystemConfig) error
	
	// 初始化
	InitDefaultConfigs(ctx context.Context) error
}

// systemConfigRepository 系统配置仓库实现
type systemConfigRepository struct {
	db *gorm.DB
}

// NewSystemConfigRepository 创建系统配置仓库
func NewSystemConfigRepository(db *gorm.DB) SystemConfigRepository {
	return &systemConfigRepository{db: db}
}

// Create 创建配置
func (r *systemConfigRepository) Create(ctx context.Context, config *models.SystemConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// GetByID 根据ID获取配置
func (r *systemConfigRepository) GetByID(ctx context.Context, id uint) (*models.SystemConfig, error) {
	var config models.SystemConfig
	if err := r.db.WithContext(ctx).First(&config, id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetByKey 根据配置键获取配置
func (r *systemConfigRepository) GetByKey(ctx context.Context, key string) (*models.SystemConfig, error) {
	var config models.SystemConfig
	if err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// Update 更新配置
func (r *systemConfigRepository) Update(ctx context.Context, config *models.SystemConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// Delete 删除配置
func (r *systemConfigRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SystemConfig{}, id).Error
}

// GetAll 获取所有配置
func (r *systemConfigRepository) GetAll(ctx context.Context) ([]*models.SystemConfig, error) {
	var configs []*models.SystemConfig
	if err := r.db.WithContext(ctx).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetByType 根据类型获取配置
func (r *systemConfigRepository) GetByType(ctx context.Context, configType string) ([]*models.SystemConfig, error) {
	var configs []*models.SystemConfig
	if err := r.db.WithContext(ctx).Where("config_type = ?", configType).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetByKeys 根据多个配置键获取配置
func (r *systemConfigRepository) GetByKeys(ctx context.Context, keys []string) ([]*models.SystemConfig, error) {
	var configs []*models.SystemConfig
	if err := r.db.WithContext(ctx).Where("config_key IN ?", keys).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// SaveOrUpdate 保存或更新配置
func (r *systemConfigRepository) SaveOrUpdate(ctx context.Context, config *models.SystemConfig) error {
	var existing models.SystemConfig
	err := r.db.WithContext(ctx).Where("config_key = ?", config.ConfigKey).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(config).Error
	}
	if err != nil {
		return err
	}
	// 更新现有配置
	config.ID = existing.ID
	return r.db.WithContext(ctx).Save(config).Error
}

// SaveBatch 批量保存配置
func (r *systemConfigRepository) SaveBatch(ctx context.Context, configs []*models.SystemConfig) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, config := range configs {
			var existing models.SystemConfig
			err := tx.Where("config_key = ?", config.ConfigKey).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(config).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				config.ID = existing.ID
				if err := tx.Save(config).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// InitDefaultConfigs 初始化默认配置
func (r *systemConfigRepository) InitDefaultConfigs(ctx context.Context) error {
	defaults := models.GetDefaultAllConfigs()
	
	configs := []*models.SystemConfig{
		// 盒子管理配置
		r.createConfigEntry(models.ConfigKeyBoxHeartbeatTimeout, defaults.Box.HeartbeatTimeoutSeconds, models.ConfigTypeBox, "心跳超时时间（秒）", true),
		r.createConfigEntry(models.ConfigKeyBoxMonitoringInterval, defaults.Box.MonitoringIntervalSeconds, models.ConfigTypeBox, "监控刷新间隔（秒）", true),
		r.createConfigEntry(models.ConfigKeyBoxAutoOfflineMinutes, defaults.Box.AutoOfflineMinutes, models.ConfigTypeBox, "自动离线判定时间（分钟）", true),
		r.createConfigEntry(models.ConfigKeyBoxMaxConcurrentUpload, defaults.Box.MaxConcurrentModelUpload, models.ConfigTypeBox, "最大并发模型上传数", true),
		r.createConfigEntry(models.ConfigKeyBoxDiscoveryScanRange, defaults.Box.DiscoveryScanRange, models.ConfigTypeBox, "发现扫描IP范围", true),
		
		// 任务调度配置
		r.createConfigEntry(models.ConfigKeyTaskAutoScheduleEnabled, defaults.Task.AutoScheduleEnabled, models.ConfigTypeTask, "自动调度开关", true),
		r.createConfigEntry(models.ConfigKeyTaskScheduleIntervalSec, defaults.Task.ScheduleIntervalSeconds, models.ConfigTypeTask, "调度间隔（秒）", true),
		r.createConfigEntry(models.ConfigKeyTaskMaxConcurrentPerBox, defaults.Task.MaxConcurrentPerBox, models.ConfigTypeTask, "每盒子最大并发任务数", true),
		r.createConfigEntry(models.ConfigKeyTaskDefaultPriority, defaults.Task.DefaultPriority, models.ConfigTypeTask, "默认任务优先级", true),
		r.createConfigEntry(models.ConfigKeyTaskRetryMaxAttempts, defaults.Task.RetryMaxAttempts, models.ConfigTypeTask, "任务重试最大次数", true),
		r.createConfigEntry(models.ConfigKeyTaskDeploymentTimeoutSec, defaults.Task.DeploymentTimeoutSeconds, models.ConfigTypeTask, "部署超时时间（秒）", true),
		
		// 模型管理配置
		r.createConfigEntry(models.ConfigKeyModelAllowedTypes, defaults.Model.AllowedFileTypes, models.ConfigTypeModel, "允许的文件类型", true),
		r.createConfigEntry(models.ConfigKeyModelMaxFileSizeMB, defaults.Model.MaxFileSizeMB, models.ConfigTypeModel, "最大文件大小（MB）", true),
		r.createConfigEntry(models.ConfigKeyModelChunkSizeMB, defaults.Model.ChunkSizeMB, models.ConfigTypeModel, "分片大小（MB）", true),
		r.createConfigEntry(models.ConfigKeyModelSessionTimeout, defaults.Model.SessionTimeoutHours, models.ConfigTypeModel, "会话超时时间（小时）", true),
		r.createConfigEntry(models.ConfigKeyModelCleanupDays, defaults.Model.CleanupDays, models.ConfigTypeModel, "清理天数", true),
		
		// 模型转换配置
		r.createConfigEntry(models.ConfigKeyConversionTargetChips, defaults.Conversion.TargetChips, models.ConfigTypeConversion, "支持的目标芯片", true),
		r.createConfigEntry(models.ConfigKeyConversionYoloVersions, defaults.Conversion.YoloVersions, models.ConfigTypeConversion, "支持的YOLO版本", true),
		r.createConfigEntry(models.ConfigKeyConversionQuantizations, defaults.Conversion.QuantizationTypes, models.ConfigTypeConversion, "量化类型", true),
		r.createConfigEntry(models.ConfigKeyConversionMaxConcurrent, defaults.Conversion.MaxConcurrentTasks, models.ConfigTypeConversion, "最大并发转换任务数", true),
		r.createConfigEntry(models.ConfigKeyConversionDefaultTimeout, defaults.Conversion.DefaultTimeoutHours, models.ConfigTypeConversion, "默认转换超时（小时）", true),
		
		// 视频处理配置
		r.createConfigEntry(models.ConfigKeyVideoExtractDefaultCount, defaults.Video.ExtractDefaultFrameCount, models.ConfigTypeVideo, "默认抽帧数量", true),
		r.createConfigEntry(models.ConfigKeyVideoExtractMaxCount, defaults.Video.ExtractMaxFrameCount, models.ConfigTypeVideo, "最大抽帧数量", true),
		r.createConfigEntry(models.ConfigKeyVideoExtractDefaultQuality, defaults.Video.ExtractDefaultQuality, models.ConfigTypeVideo, "默认抽帧质量", true),
		r.createConfigEntry(models.ConfigKeyVideoRecordDefaultDuration, defaults.Video.RecordDefaultDuration, models.ConfigTypeVideo, "默认录制时长（秒）", true),
		r.createConfigEntry(models.ConfigKeyVideoRecordMaxDuration, defaults.Video.RecordMaxDuration, models.ConfigTypeVideo, "最大录制时长（秒）", true),
		r.createConfigEntry(models.ConfigKeyVideoRecordDefaultFormat, defaults.Video.RecordDefaultFormat, models.ConfigTypeVideo, "默认录制格式", true),
		
		// 系统通用配置
		r.createConfigEntry(models.ConfigKeySystemLogRetentionDays, defaults.System.LogRetentionDays, models.ConfigTypeSystem, "日志保留天数", true),
		r.createConfigEntry(models.ConfigKeySystemNotificationEnabled, defaults.System.NotificationEnabled, models.ConfigTypeSystem, "通知开关", true),
		r.createConfigEntry(models.ConfigKeySystemMaintenanceMode, defaults.System.MaintenanceMode, models.ConfigTypeSystem, "维护模式", true),
		r.createConfigEntry(models.ConfigKeySystemDefaultPageSize, defaults.System.DefaultPageSize, models.ConfigTypeSystem, "默认分页大小", true),
	}
	
	// 批量保存，如果配置已存在则跳过
	for _, config := range configs {
		var existing models.SystemConfig
		err := r.db.WithContext(ctx).Where("config_key = ?", config.ConfigKey).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := r.db.WithContext(ctx).Create(config).Error; err != nil {
				return fmt.Errorf("初始化配置 %s 失败: %w", config.ConfigKey, err)
			}
		} else if err != nil {
			return fmt.Errorf("查询配置 %s 失败: %w", config.ConfigKey, err)
		}
		// 配置已存在，跳过
	}
	
	return nil
}

// createConfigEntry 创建配置条目
func (r *systemConfigRepository) createConfigEntry(key string, value interface{}, configType, description string, isSystem bool) *models.SystemConfig {
	valueJSON, _ := json.Marshal(value)
	return &models.SystemConfig{
		ConfigKey:   key,
		ConfigValue: string(valueJSON),
		ConfigType:  configType,
		Description: description,
		IsSystem:    isSystem,
	}
}

