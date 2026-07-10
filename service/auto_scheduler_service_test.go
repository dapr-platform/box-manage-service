/*
 * @module service/auto_scheduler_service_test
 * @description 自动调度服务单元测试
 * @architecture 测试层
 */

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"box-manage-service/models"
	"box-manage-service/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// MockSchedulePolicyRepository 模拟调度策略仓储
type MockSchedulePolicyRepository struct {
	mock.Mock
}

func (m *MockSchedulePolicyRepository) Create(ctx context.Context, policy *models.SchedulePolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) GetByID(ctx context.Context, id uint) (*models.SchedulePolicy, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) Update(ctx context.Context, policy *models.SchedulePolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) CreateBatch(ctx context.Context, policies []*models.SchedulePolicy) error {
	args := m.Called(ctx, policies)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) UpdateBatch(ctx context.Context, policies []*models.SchedulePolicy) error {
	args := m.Called(ctx, policies)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) Find(ctx context.Context, conditions map[string]interface{}) ([]*models.SchedulePolicy, error) {
	args := m.Called(ctx, conditions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*models.SchedulePolicy, int64, error) {
	args := m.Called(ctx, conditions, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.SchedulePolicy), args.Get(1).(int64), args.Error(2)
}

func (m *MockSchedulePolicyRepository) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	args := m.Called(ctx, conditions)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSchedulePolicyRepository) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	args := m.Called(ctx, conditions)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockSchedulePolicyRepository) FindByName(ctx context.Context, name string) (*models.SchedulePolicy, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) FindByType(ctx context.Context, policyType models.SchedulePolicyType) ([]*models.SchedulePolicy, error) {
	args := m.Called(ctx, policyType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) FindEnabled(ctx context.Context) ([]*models.SchedulePolicy, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) Enable(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) Disable(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) UpdatePriority(ctx context.Context, id uint, priority int) error {
	args := m.Called(ctx, id, priority)
	return args.Error(0)
}

func (m *MockSchedulePolicyRepository) FindEnabledByPriority(ctx context.Context) ([]*models.SchedulePolicy, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) GetDefaultPolicy(ctx context.Context) (*models.SchedulePolicy, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SchedulePolicy), args.Error(1)
}

func (m *MockSchedulePolicyRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// MockTaskRepository 模拟任务仓储
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, entity *models.Task) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockTaskRepository) GetByID(ctx context.Context, id uint) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, entity *models.Task) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepository) CreateBatch(ctx context.Context, entities []*models.Task) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateBatch(ctx context.Context, entities []*models.Task) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockTaskRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockTaskRepository) Find(ctx context.Context, conditions map[string]interface{}) ([]*models.Task, error) {
	args := m.Called(ctx, conditions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*models.Task, int64, error) {
	args := m.Called(ctx, conditions, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(int64), args.Error(2)
}

func (m *MockTaskRepository) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	args := m.Called(ctx, conditions)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	args := m.Called(ctx, conditions)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockTaskRepository) FindByTaskID(ctx context.Context, taskID string) (*models.Task, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindByExternalID(ctx context.Context, boxID uint, externalID string) (*models.Task, error) {
	args := m.Called(ctx, boxID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindByBoxID(ctx context.Context, boxID uint) ([]*models.Task, error) {
	args := m.Called(ctx, boxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindRunningTasks(ctx context.Context) ([]*models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) UpdateStatus(ctx context.Context, id uint, status models.TaskStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateProgress(ctx context.Context, id uint, totalFrames, inferenceCount, forwardSuccess, forwardFailed int64) error {
	args := m.Called(ctx, id, totalFrames, inferenceCount, forwardSuccess, forwardFailed)
	return args.Error(0)
}

func (m *MockTaskRepository) GetTaskStatistics(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockTaskRepository) GetTaskStatusDistribution(ctx context.Context) (map[models.TaskStatus]int64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[models.TaskStatus]int64), args.Error(1)
}

func (m *MockTaskRepository) GetTasksByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Task, error) {
	args := m.Called(ctx, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindByTags(ctx context.Context, tags []string) ([]*models.Task, error) {
	args := m.Called(ctx, tags)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetAllTags(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockTaskRepository) FindPendingTasks(ctx context.Context, limit int) ([]*models.Task, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindTasksByModel(ctx context.Context, modelKey string) ([]*models.Task, error) {
	args := m.Called(ctx, modelKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindTasksByPriority(ctx context.Context, priority int) ([]*models.Task, error) {
	args := m.Called(ctx, priority)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindAutoScheduleTasks(ctx context.Context, limit int) ([]*models.Task, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindTasksWithAffinityTags(ctx context.Context, affinityTags []string) ([]*models.Task, error) {
	args := m.Called(ctx, affinityTags)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindTasksCompatibleWithBox(ctx context.Context, boxID uint) ([]*models.Task, error) {
	args := m.Called(ctx, boxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) UpdateTaskProgress(ctx context.Context, taskID uint, progress float64) error {
	args := m.Called(ctx, taskID, progress)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateHeartbeat(ctx context.Context, taskID uint, heartbeat time.Time) error {
	args := m.Called(ctx, taskID, heartbeat)
	return args.Error(0)
}

func (m *MockTaskRepository) SetTaskError(ctx context.Context, id uint, errorMsg string) error {
	args := m.Called(ctx, id, errorMsg)
	return args.Error(0)
}

func (m *MockTaskRepository) GetTaskExecutionHistory(ctx context.Context, taskID uint) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskRepository) CreateTaskExecution(ctx context.Context, execution *models.TaskExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateTaskExecution(ctx context.Context, execution *models.TaskExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockTaskRepository) BulkUpdateTaskStatus(ctx context.Context, taskIDs []uint, status models.TaskStatus) error {
	args := m.Called(ctx, taskIDs, status)
	return args.Error(0)
}

func (m *MockTaskRepository) FindTasksWithBox(ctx context.Context, options *repository.QueryOptions) ([]*models.Task, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) SearchTasks(ctx context.Context, keyword string, options *repository.QueryOptions) ([]*models.Task, error) {
	args := m.Called(ctx, keyword, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetTaskPerformanceReport(ctx context.Context, startDate, endDate string) (map[string]interface{}, error) {
	args := m.Called(ctx, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockTaskRepository) GetActiveTasksByBoxID(ctx context.Context, boxID uint) ([]*models.Task, error) {
	args := m.Called(ctx, boxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) IsTaskIDExists(ctx context.Context, taskID string, excludeID ...uint) (bool, error) {
	args := m.Called(ctx, taskID, excludeID)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockTaskRepository) CountTasksByStatus(status models.TaskStatus) (int64, error) {
	args := m.Called(status)
	return args.Get(0).(int64), args.Error(1)
}

// MockBoxRepository 模拟盒子仓储
type MockBoxRepository struct {
	mock.Mock
}

func (m *MockBoxRepository) Create(ctx context.Context, entity *models.Box) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockBoxRepository) GetByID(ctx context.Context, id uint) (*models.Box, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Box), args.Error(1)
}

func (m *MockBoxRepository) Update(ctx context.Context, entity *models.Box) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockBoxRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBoxRepository) CreateBatch(ctx context.Context, entities []*models.Box) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockBoxRepository) UpdateBatch(ctx context.Context, entities []*models.Box) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockBoxRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockBoxRepository) Find(ctx context.Context, conditions map[string]interface{}) ([]*models.Box, error) {
	args := m.Called(ctx, conditions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Box), args.Error(1)
}

func (m *MockBoxRepository) FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*models.Box, int64, error) {
	args := m.Called(ctx, conditions, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*models.Box), args.Get(1).(int64), args.Error(2)
}

func (m *MockBoxRepository) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	args := m.Called(ctx, conditions)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockBoxRepository) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	args := m.Called(ctx, conditions)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockBoxRepository) FindByIPAddress(ctx context.Context, ipAddress string) (*models.Box, error) {
	args := m.Called(ctx, ipAddress)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Box), args.Error(1)
}

func (m *MockBoxRepository) FindByDeviceFingerprint(ctx context.Context, deviceFingerprint string) (*models.Box, error) {
	args := m.Called(ctx, deviceFingerprint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Box), args.Error(1)
}

func (m *MockBoxRepository) FindByStatus(ctx context.Context, status models.BoxStatus) ([]*models.Box, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Box), args.Error(1)
}

func (m *MockBoxRepository) FindOnlineBoxes(ctx context.Context) ([]*models.Box, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Box), args.Error(1)
}

func (m *MockBoxRepository) FindByLocationPattern(ctx context.Context, locationPattern string) ([]*models.Box, error) {
	args := m.Called(ctx, locationPattern)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Box), args.Error(1)
}

func (m *MockBoxRepository) UpdateStatus(ctx context.Context, id uint, status models.BoxStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockBoxRepository) UpdateHeartbeat(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBoxRepository) UpdateResources(ctx context.Context, id uint, resources models.Resources) error {
	args := m.Called(ctx, id, resources)
	return args.Error(0)
}

func (m *MockBoxRepository) GetBoxStatistics(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockBoxRepository) GetBoxStatusDistribution(ctx context.Context) (map[models.BoxStatus]int64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[models.BoxStatus]int64), args.Error(1)
}

func (m *MockBoxRepository) CountOnlineBoxes() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

// MockRepositoryManagerForScheduler 模拟仓储管理器（用于自动调度测试）
type MockRepositoryManagerForScheduler struct {
	mock.Mock
	boxRepo            repository.BoxRepository
	taskRepo           repository.TaskRepository
	schedulePolicyRepo repository.SchedulePolicyRepository
}

func (m *MockRepositoryManagerForScheduler) Box() repository.BoxRepository {
	return m.boxRepo
}

func (m *MockRepositoryManagerForScheduler) Task() repository.TaskRepository {
	return m.taskRepo
}

func (m *MockRepositoryManagerForScheduler) SchedulePolicy() repository.SchedulePolicyRepository {
	return m.schedulePolicyRepo
}

// 其他方法返回 nil
func (m *MockRepositoryManagerForScheduler) OriginalModel() repository.OriginalModelRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) ConvertedModel() repository.ConvertedModelRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) UploadSession() repository.UploadSessionRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) ModelTag() repository.ModelTagRepository { return nil }
func (m *MockRepositoryManagerForScheduler) DeploymentTask() repository.DeploymentRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) ModelDeployment() repository.ModelDeploymentRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) ModelBoxDeployment() repository.ModelBoxDeploymentRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) Upgrade() repository.UpgradeRepository { return nil }
func (m *MockRepositoryManagerForScheduler) UpgradePackage() repository.UpgradePackageRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) BoxHeartbeat() repository.BoxHeartbeatRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) SystemLog() repository.SystemLogRepository { return nil }
func (m *MockRepositoryManagerForScheduler) VideoSource() repository.VideoSourceRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) VideoFile() repository.VideoFileRepository { return nil }
func (m *MockRepositoryManagerForScheduler) ExtractFrame() repository.ExtractFrameRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) ExtractTask() repository.ExtractTaskRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) RecordTask() repository.RecordTaskRepository { return nil }
func (m *MockRepositoryManagerForScheduler) Workflow() repository.WorkflowRepository     { return nil }
func (m *MockRepositoryManagerForScheduler) NodeTemplate() repository.NodeTemplateRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) WorkflowInstance() repository.WorkflowInstanceRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) NodeDefinition() repository.NodeDefinitionRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) NodeInstance() repository.NodeInstanceRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) VariableDefinition() repository.VariableDefinitionRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) VariableInstance() repository.VariableInstanceRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) LineDefinition() repository.LineDefinitionRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) LineInstance() repository.LineInstanceRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) WorkflowLog() repository.WorkflowLogRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) WorkflowSchedule() repository.WorkflowScheduleRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) WorkflowDeployment() repository.WorkflowDeploymentRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) WorkflowScheduleInstance() repository.WorkflowScheduleInstanceRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) TaskScheduleHistory() repository.TaskScheduleHistoryRepository {
	return nil
}
func (m *MockRepositoryManagerForScheduler) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return nil
}
func (m *MockRepositoryManagerForScheduler) DB() *gorm.DB { return nil }

// MockTaskSchedulerService 模拟任务调度服务
type MockTaskSchedulerService struct {
	mock.Mock
}

func (m *MockTaskSchedulerService) ScheduleAutoTasks(ctx context.Context) (*ScheduleResult, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ScheduleResult), args.Error(1)
}

func (m *MockTaskSchedulerService) ScheduleTask(ctx context.Context, taskID uint) (*TaskScheduleResult, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TaskScheduleResult), args.Error(1)
}

func (m *MockTaskSchedulerService) ScheduleTaskWithPolicy(ctx context.Context, taskID uint, policy *models.SchedulePolicy) (*TaskScheduleResult, error) {
	args := m.Called(ctx, taskID, policy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TaskScheduleResult), args.Error(1)
}

func (m *MockTaskSchedulerService) FindCompatibleBoxes(ctx context.Context, taskID uint) ([]*BoxScore, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*BoxScore), args.Error(1)
}

func (m *MockTaskSchedulerService) FindCompatibleBoxesWithPolicy(ctx context.Context, taskID uint, policy *models.SchedulePolicy) ([]*BoxScore, error) {
	args := m.Called(ctx, taskID, policy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*BoxScore), args.Error(1)
}

// TestAutoSchedulerService_StartStop 测试自动调度器启动和停止
func TestAutoSchedulerService_StartStop(t *testing.T) {
	// 创建模拟对象
	mockPolicyRepo := new(MockSchedulePolicyRepository)
	mockTaskRepo := new(MockTaskRepository)
	mockBoxRepo := new(MockBoxRepository)
	mockSchedulerService := new(MockTaskSchedulerService)

	// 创建模拟仓储管理器
	mockRepoManager := &MockRepositoryManagerForScheduler{
		schedulePolicyRepo: mockPolicyRepo,
		taskRepo:           mockTaskRepo,
		boxRepo:            mockBoxRepo,
	}

	// 创建自动调度服务
	svc := NewAutoSchedulerService(mockRepoManager, mockSchedulerService, nil)

	// 测试初始状态
	assert.False(t, svc.IsRunning())

	// 测试启动
	ctx := context.Background()
	mockPolicyRepo.On("GetDefaultPolicy", mock.Anything).Return(nil, nil)
	mockPolicyRepo.On("FindEnabled", mock.Anything).Return([]*models.SchedulePolicy{}, nil)
	mockTaskRepo.On("FindAutoScheduleTasks", mock.Anything, 1000).Return([]*models.Task{}, nil)
	mockBoxRepo.On("FindOnlineBoxes", mock.Anything).Return([]*models.Box{}, nil)

	err := svc.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, svc.IsRunning())

	// 测试重复启动
	err = svc.Start(ctx)
	assert.Error(t, err)

	// 测试停止
	err = svc.Stop()
	assert.NoError(t, err)
	assert.False(t, svc.IsRunning())

	// 测试重复停止
	err = svc.Stop()
	assert.Error(t, err)
}

// TestAutoSchedulerService_GetStatus 测试获取状态
func TestAutoSchedulerService_GetStatus(t *testing.T) {
	mockPolicyRepo := new(MockSchedulePolicyRepository)
	mockTaskRepo := new(MockTaskRepository)
	mockBoxRepo := new(MockBoxRepository)
	mockSchedulerService := new(MockTaskSchedulerService)

	mockRepoManager := &MockRepositoryManagerForScheduler{
		schedulePolicyRepo: mockPolicyRepo,
		taskRepo:           mockTaskRepo,
		boxRepo:            mockBoxRepo,
	}

	svc := NewAutoSchedulerService(mockRepoManager, mockSchedulerService, nil)

	ctx := context.Background()

	// 设置模拟返回值
	mockPolicyRepo.On("FindEnabled", mock.Anything).Return([]*models.SchedulePolicy{
		{Name: "policy1", IsEnabled: true},
		{Name: "policy2", IsEnabled: true},
	}, nil)
	mockTaskRepo.On("FindAutoScheduleTasks", mock.Anything, 1000).Return([]*models.Task{
		{Name: "task1"},
	}, nil)
	mockBoxRepo.On("FindOnlineBoxes", mock.Anything).Return([]*models.Box{
		{Name: "box1"},
		{Name: "box2"},
	}, nil)

	status := svc.GetStatus(ctx)

	assert.NotNil(t, status)
	assert.False(t, status.IsRunning)
	assert.Equal(t, 2, status.ActivePolicies)
	assert.Equal(t, 1, status.PendingTasks)
	assert.Equal(t, 2, status.AvailableBoxes)
}

// TestAutoSchedulerService_TriggerSchedule 测试手动触发调度
func TestAutoSchedulerService_TriggerSchedule(t *testing.T) {
	mockPolicyRepo := new(MockSchedulePolicyRepository)
	mockTaskRepo := new(MockTaskRepository)
	mockBoxRepo := new(MockBoxRepository)
	mockSchedulerService := new(MockTaskSchedulerService)

	mockRepoManager := &MockRepositoryManagerForScheduler{
		schedulePolicyRepo: mockPolicyRepo,
		taskRepo:           mockTaskRepo,
		boxRepo:            mockBoxRepo,
	}

	svc := NewAutoSchedulerService(mockRepoManager, mockSchedulerService, nil)

	ctx := context.Background()

	// 设置无启用的策略（返回错误触发默认调度路径）
	mockPolicyRepo.On("GetDefaultPolicy", mock.Anything).Return(nil, fmt.Errorf("no default policy"))
	mockTaskRepo.On("FindAutoScheduleTasks", mock.Anything, 50).Return([]*models.Task{
		{Name: "task1"},
		{Name: "task2"},
	}, nil)

	mockSchedulerService.On("ScheduleAutoTasks", mock.Anything).Return(&ScheduleResult{
		TotalTasks:     2,
		ScheduledTasks: 1,
		FailedTasks:    1,
		TaskResults:    []*TaskScheduleResult{},
	}, nil)

	result, err := svc.TriggerSchedule(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalTasks)
	assert.Equal(t, 1, result.ScheduledTasks)
	assert.Equal(t, 1, result.FailedTasks)
}

// TestAutoSchedulerService_SetDefaultInterval 测试设置默认间隔
func TestAutoSchedulerService_SetDefaultInterval(t *testing.T) {
	mockPolicyRepo := new(MockSchedulePolicyRepository)
	mockTaskRepo := new(MockTaskRepository)
	mockBoxRepo := new(MockBoxRepository)
	mockSchedulerService := new(MockTaskSchedulerService)

	mockRepoManager := &MockRepositoryManagerForScheduler{
		schedulePolicyRepo: mockPolicyRepo,
		taskRepo:           mockTaskRepo,
		boxRepo:            mockBoxRepo,
	}

	svc := NewAutoSchedulerService(mockRepoManager, mockSchedulerService, nil)

	// 设置新的间隔
	newInterval := 120 * time.Second
	svc.SetDefaultInterval(newInterval)

	// 验证设置生效（通过状态检查）
	ctx := context.Background()
	mockPolicyRepo.On("FindEnabled", mock.Anything).Return([]*models.SchedulePolicy{}, nil)
	mockTaskRepo.On("FindAutoScheduleTasks", mock.Anything, 1000).Return([]*models.Task{}, nil)
	mockBoxRepo.On("FindOnlineBoxes", mock.Anything).Return([]*models.Box{}, nil)

	status := svc.GetStatus(ctx)
	assert.NotNil(t, status)
	assert.Equal(t, 120, status.ScheduleIntervalSec)
}
