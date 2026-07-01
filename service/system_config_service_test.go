package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"box-manage-service/models"

	"github.com/stretchr/testify/assert"
)

type mockSystemConfigRepository struct {
	configs map[string]*models.SystemConfig
}

func newMockSystemConfigRepository(values map[string]interface{}) *mockSystemConfigRepository {
	repo := &mockSystemConfigRepository{
		configs: make(map[string]*models.SystemConfig),
	}
	for key, value := range values {
		valueJSON, _ := json.Marshal(value)
		repo.configs[key] = &models.SystemConfig{
			ConfigKey:   key,
			ConfigValue: string(valueJSON),
			ConfigType:  models.ConfigTypeTask,
			IsSystem:    true,
		}
	}
	return repo
}

func (m *mockSystemConfigRepository) Create(ctx context.Context, config *models.SystemConfig) error {
	m.configs[config.ConfigKey] = config
	return nil
}

func (m *mockSystemConfigRepository) GetByID(ctx context.Context, id uint) (*models.SystemConfig, error) {
	for _, config := range m.configs {
		if config.ID == id {
			return config, nil
		}
	}
	return nil, fmt.Errorf("config not found")
}

func (m *mockSystemConfigRepository) GetByKey(ctx context.Context, key string) (*models.SystemConfig, error) {
	config, ok := m.configs[key]
	if !ok {
		return nil, fmt.Errorf("config not found")
	}
	return config, nil
}

func (m *mockSystemConfigRepository) Update(ctx context.Context, config *models.SystemConfig) error {
	m.configs[config.ConfigKey] = config
	return nil
}

func (m *mockSystemConfigRepository) Delete(ctx context.Context, id uint) error {
	for key, config := range m.configs {
		if config.ID == id {
			delete(m.configs, key)
			return nil
		}
	}
	return nil
}

func (m *mockSystemConfigRepository) GetAll(ctx context.Context) ([]*models.SystemConfig, error) {
	configs := make([]*models.SystemConfig, 0, len(m.configs))
	for _, config := range m.configs {
		configs = append(configs, config)
	}
	return configs, nil
}

func (m *mockSystemConfigRepository) GetByType(ctx context.Context, configType string) ([]*models.SystemConfig, error) {
	configs := make([]*models.SystemConfig, 0)
	for _, config := range m.configs {
		if config.ConfigType == configType {
			configs = append(configs, config)
		}
	}
	return configs, nil
}

func (m *mockSystemConfigRepository) GetByKeys(ctx context.Context, keys []string) ([]*models.SystemConfig, error) {
	configs := make([]*models.SystemConfig, 0, len(keys))
	for _, key := range keys {
		if config, ok := m.configs[key]; ok {
			configs = append(configs, config)
		}
	}
	return configs, nil
}

func (m *mockSystemConfigRepository) SaveOrUpdate(ctx context.Context, config *models.SystemConfig) error {
	m.configs[config.ConfigKey] = config
	return nil
}

func (m *mockSystemConfigRepository) SaveBatch(ctx context.Context, configs []*models.SystemConfig) error {
	for _, config := range configs {
		m.configs[config.ConfigKey] = config
	}
	return nil
}

func (m *mockSystemConfigRepository) InitDefaultConfigs(ctx context.Context) error {
	return nil
}

type mockAutoSchedulerService struct {
	running  bool
	interval time.Duration
	starts   int
	stops    int
}

func (m *mockAutoSchedulerService) Start(ctx context.Context) error {
	if m.running {
		return fmt.Errorf("自动调度器已在运行")
	}
	m.running = true
	m.starts++
	return nil
}

func (m *mockAutoSchedulerService) Stop() error {
	if !m.running {
		return fmt.Errorf("自动调度器未运行")
	}
	m.running = false
	m.stops++
	return nil
}

func (m *mockAutoSchedulerService) IsRunning() bool {
	return m.running
}

func (m *mockAutoSchedulerService) GetStatus(ctx context.Context) *AutoSchedulerStatus {
	return &AutoSchedulerStatus{IsRunning: m.running}
}

func (m *mockAutoSchedulerService) TriggerSchedule(ctx context.Context) (*AutoScheduleResult, error) {
	return nil, nil
}

func (m *mockAutoSchedulerService) TriggerScheduleWithPolicy(ctx context.Context, policyID uint) (*AutoScheduleResult, error) {
	return nil, nil
}

func (m *mockAutoSchedulerService) OnNewTask(ctx context.Context, taskID uint) error {
	return nil
}

func (m *mockAutoSchedulerService) OnBoxOnline(ctx context.Context, boxID uint) error {
	return nil
}

func (m *mockAutoSchedulerService) SetDefaultInterval(interval time.Duration) {
	m.interval = interval
}

func TestSystemConfigService_AutoSchedulerConfigLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newMockSystemConfigRepository(map[string]interface{}{
		models.ConfigKeyTaskAutoScheduleEnabled: false,
		models.ConfigKeyTaskScheduleIntervalSec: 70,
	})
	autoScheduler := &mockAutoSchedulerService{}
	configService := NewSystemConfigServiceWithAutoScheduler(repo, autoScheduler)

	err := configService.ApplyAutoSchedulerConfig(ctx)
	assert.NoError(t, err)
	assert.False(t, autoScheduler.running)
	assert.Equal(t, 70*time.Second, autoScheduler.interval)

	err = configService.UpdateConfig(ctx, models.ConfigKeyTaskAutoScheduleEnabled, true)
	assert.NoError(t, err)
	assert.True(t, autoScheduler.running)
	assert.Equal(t, 1, autoScheduler.starts)
	assert.Equal(t, 70*time.Second, autoScheduler.interval)

	err = configService.UpdateConfig(ctx, models.ConfigKeyTaskScheduleIntervalSec, 15)
	assert.NoError(t, err)
	assert.True(t, autoScheduler.running)
	assert.Equal(t, 1, autoScheduler.starts)
	assert.Equal(t, 15*time.Second, autoScheduler.interval)

	err = configService.UpdateConfig(ctx, models.ConfigKeyTaskAutoScheduleEnabled, false)
	assert.NoError(t, err)
	assert.False(t, autoScheduler.running)
	assert.Equal(t, 1, autoScheduler.stops)
}
