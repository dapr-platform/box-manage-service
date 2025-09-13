/*
 * @module repository/interfaces
 * @description Repository接口定义，实现数据访问层的抽象
 * @architecture 数据访问层
 * @documentReference DESIGN-000.md
 * @stateFlow 业务层 -> Repository接口 -> 具体实现 -> 数据库
 * @rules 遵循GORM最佳实践，实现CRUD操作、分页、条件查询等通用功能
 * @dependencies gorm.io/gorm
 * @refs context7 GORM最佳实践
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// BaseRepository 基础Repository接口，定义通用CRUD操作
type BaseRepository[T any] interface {
	// 基础CRUD操作
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id uint) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error

	// 批量操作
	CreateBatch(ctx context.Context, entities []*T) error
	UpdateBatch(ctx context.Context, entities []*T) error
	DeleteBatch(ctx context.Context, ids []uint) error

	// 查询操作
	Find(ctx context.Context, conditions map[string]interface{}) ([]*T, error)
	FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*T, int64, error)
	Count(ctx context.Context, conditions map[string]interface{}) (int64, error)
	Exists(ctx context.Context, conditions map[string]interface{}) (bool, error)
}

// BoxRepository 盒子Repository接口
type BoxRepository interface {
	BaseRepository[models.Box]

	// 盒子特定查询
	FindByIPAddress(ctx context.Context, ipAddress string) (*models.Box, error)
	FindByStatus(ctx context.Context, status models.BoxStatus) ([]*models.Box, error)
	FindOnlineBoxes(ctx context.Context) ([]*models.Box, error)
	FindByLocationPattern(ctx context.Context, locationPattern string) ([]*models.Box, error)

	// 盒子状态管理
	UpdateStatus(ctx context.Context, id uint, status models.BoxStatus) error
	UpdateHeartbeat(ctx context.Context, id uint) error
	UpdateResources(ctx context.Context, id uint, resources models.Resources) error

	// 统计查询
	GetBoxStatistics(ctx context.Context) (map[string]interface{}, error)
	GetBoxStatusDistribution(ctx context.Context) (map[models.BoxStatus]int64, error)

	// 任务调度服务需要的方法
	CountOnlineBoxes() (int64, error)
}

// OriginalModelRepository 原始模型Repository接口
type OriginalModelRepository interface {
	BaseRepository[models.OriginalModel]

	// 基础查询
	FindByName(ctx context.Context, name string) ([]*models.OriginalModel, error)
	FindByType(ctx context.Context, modelType models.OriginalModelType) ([]*models.OriginalModel, error)
	FindByStatus(ctx context.Context, status models.OriginalModelStatus) ([]*models.OriginalModel, error)
	FindByUserID(ctx context.Context, userID uint) ([]*models.OriginalModel, error)
	FindByMD5(ctx context.Context, md5 string) (*models.OriginalModel, error)

	// 搜索和筛选
	SearchModels(ctx context.Context, keyword string, options *QueryOptions) ([]*models.OriginalModel, error)
	FindByTags(ctx context.Context, tags []string) ([]*models.OriginalModel, error)
	FindPublicModels(ctx context.Context, options *QueryOptions) ([]*models.OriginalModel, error)

	// 版本管理
	FindByNameAndVersion(ctx context.Context, name, version string) (*models.OriginalModel, error)
	GetModelVersions(ctx context.Context, name string) ([]*models.OriginalModel, error)
	GetLatestVersion(ctx context.Context, name string) (*models.OriginalModel, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.OriginalModelStatus) error
	UpdateUploadProgress(ctx context.Context, id uint, progress int) error
	MarkAsValidated(ctx context.Context, id uint, validationMsg string) error
	MarkAsInvalid(ctx context.Context, id uint, validationMsg string) error

	// 存储管理
	UpdateStorageClass(ctx context.Context, id uint, storageClass string) error
	UpdateLastAccessed(ctx context.Context, id uint) error
	IncrementDownloadCount(ctx context.Context, id uint) error

	// 统计查询
	GetModelStatistics(ctx context.Context) (map[string]interface{}, error)
	GetStorageStatistics(ctx context.Context) (map[string]interface{}, error)
	GetUserModelStatistics(ctx context.Context, userID uint) (map[string]interface{}, error)

	// 批量操作
	BulkUpdateStatus(ctx context.Context, ids []uint, status models.OriginalModelStatus) error
	BulkDelete(ctx context.Context, ids []uint) error

	// 清理操作
	CleanupFailedUploads(ctx context.Context, olderThan time.Time) error
	ArchiveOldModels(ctx context.Context, olderThan time.Time) error

	// 关联加载
	LoadConvertedModels(ctx context.Context, model *models.OriginalModel) error
}

// UploadSessionRepository 上传会话Repository接口
type UploadSessionRepository interface {
	BaseRepository[models.UploadSession]

	// 会话查询
	FindBySessionID(ctx context.Context, sessionID string) (*models.UploadSession, error)
	FindByUserID(ctx context.Context, userID uint) ([]*models.UploadSession, error)
	FindActiveByUserID(ctx context.Context, userID uint) ([]*models.UploadSession, error)
	FindExpiredSessions(ctx context.Context) ([]*models.UploadSession, error)

	// 会话状态管理
	UpdateStatus(ctx context.Context, sessionID string, status models.UploadStatus) error
	UpdateProgress(ctx context.Context, sessionID string, progress int) error
	AddUploadedChunk(ctx context.Context, sessionID string, chunkIndex int) error
	MarkAsCompleted(ctx context.Context, sessionID string, finalPath string, modelID uint) error
	MarkAsFailed(ctx context.Context, sessionID string, errorMsg string) error

	// 清理操作
	CleanupExpiredSessions(ctx context.Context) error
	CleanupCompletedSessions(ctx context.Context, olderThan time.Time) error

	// 统计查询
	GetUploadStatistics(ctx context.Context) (map[string]interface{}, error)
}

// ModelTagRepository 模型标签Repository接口
type ModelTagRepository interface {
	BaseRepository[models.ModelTag]

	// 标签查询
	FindByName(ctx context.Context, name string) (*models.ModelTag, error)
	FindPopularTags(ctx context.Context, limit int) ([]*models.ModelTag, error)
	SearchTags(ctx context.Context, keyword string) ([]*models.ModelTag, error)

	// 标签统计
	IncrementUsage(ctx context.Context, id uint) error
	DecrementUsage(ctx context.Context, id uint) error
	UpdateUsageCount(ctx context.Context, id uint, count int) error

	// 标签管理
	GetOrCreateTag(ctx context.Context, name string) (*models.ModelTag, error)
	CleanupUnusedTags(ctx context.Context) error
}

// TaskRepository 任务Repository接口
type TaskRepository interface {
	BaseRepository[models.Task]

	// 任务特定查询
	FindByTaskID(ctx context.Context, taskID string) (*models.Task, error)
	FindByExternalID(ctx context.Context, boxID uint, externalID string) (*models.Task, error)
	FindByBoxID(ctx context.Context, boxID uint) ([]*models.Task, error)
	FindByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error)
	FindRunningTasks(ctx context.Context) ([]*models.Task, error)

	// 任务状态管理
	UpdateStatus(ctx context.Context, id uint, status models.TaskStatus) error
	UpdateProgress(ctx context.Context, id uint, totalFrames, inferenceCount, forwardSuccess, forwardFailed int64) error

	// 任务统计
	GetTaskStatistics(ctx context.Context) (map[string]interface{}, error)
	GetTaskStatusDistribution(ctx context.Context) (map[models.TaskStatus]int64, error)
	GetTasksByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Task, error)

	// REQ-005 新增方法
	// 标签相关查询
	FindByTags(ctx context.Context, tags []string) ([]*models.Task, error)
	GetAllTags(ctx context.Context) ([]string, error)

	// 任务调度相关
	FindPendingTasks(ctx context.Context, limit int) ([]*models.Task, error)
	FindTasksByModel(ctx context.Context, modelKey string) ([]*models.Task, error)
	FindTasksByPriority(ctx context.Context, priority int) ([]*models.Task, error)

	// 自动调度相关
	FindAutoScheduleTasks(ctx context.Context, limit int) ([]*models.Task, error)
	FindTasksWithAffinityTags(ctx context.Context, affinityTags []string) ([]*models.Task, error)
	FindTasksCompatibleWithBox(ctx context.Context, boxID uint) ([]*models.Task, error)

	// 任务状态扩展
	UpdateTaskProgress(ctx context.Context, taskID uint, progress float64) error
	UpdateHeartbeat(ctx context.Context, taskID uint, heartbeat time.Time) error
	SetTaskError(ctx context.Context, id uint, errorMsg string) error

	// 任务执行相关
	GetTaskExecutionHistory(ctx context.Context, taskID uint) ([]*models.TaskExecution, error)
	CreateTaskExecution(ctx context.Context, execution *models.TaskExecution) error
	UpdateTaskExecution(ctx context.Context, execution *models.TaskExecution) error

	// 批量操作扩展
	BulkUpdateTaskStatus(ctx context.Context, taskIDs []uint, status models.TaskStatus) error
	FindTasksWithBox(ctx context.Context, options *QueryOptions) ([]*models.Task, error)
	SearchTasks(ctx context.Context, keyword string, options *QueryOptions) ([]*models.Task, error)

	// 性能报告
	GetTaskPerformanceReport(ctx context.Context, startDate, endDate string) (map[string]interface{}, error)
	GetActiveTasksByBoxID(ctx context.Context, boxID uint) ([]*models.Task, error)

	// 任务ID重复检查
	IsTaskIDExists(ctx context.Context, taskID string, excludeID ...uint) (bool, error)

	// 任务调度服务需要的方法
	CountTasksByStatus(status models.TaskStatus) (int64, error)
}

// UpgradeRepository 升级Repository接口
type UpgradeRepository interface {
	BaseRepository[models.UpgradeTask]

	// 升级任务特定查询
	FindByBoxID(ctx context.Context, boxID uint) ([]*models.UpgradeTask, error)
	FindByStatus(ctx context.Context, status models.UpgradeStatus) ([]*models.UpgradeTask, error)
	FindPendingUpgrades(ctx context.Context) ([]*models.UpgradeTask, error)
	FindByBatchID(ctx context.Context, batchID uint) ([]*models.UpgradeTask, error)

	// 升级状态管理
	UpdateStatus(ctx context.Context, id uint, status models.UpgradeStatus) error
	UpdateProgress(ctx context.Context, id uint, progress int) error
	SetError(ctx context.Context, id uint, errorMsg, errorDetail string) error

	// 批量升级
	CreateBatchUpgrade(ctx context.Context, batch *models.BatchUpgradeTask) error
	GetBatchUpgrade(ctx context.Context, id uint) (*models.BatchUpgradeTask, error)
	UpdateBatchStatus(ctx context.Context, id uint, status models.UpgradeStatus) error

	// 版本管理
	RecordVersion(ctx context.Context, task *models.UpgradeTask) error
	GetVersionByBoxIDAndVersion(ctx context.Context, boxID uint, version string) (*models.UpgradeVersion, error)
	GetCurrentVersion(ctx context.Context, boxID uint) (*models.UpgradeVersion, error)

	// 扩展查询
	UpdateTasksStatusByBatch(ctx context.Context, batchID uint, status models.UpgradeStatus) error
	GetTaskWithBox(ctx context.Context, taskID uint) (*models.UpgradeTask, error)
	FindUpgradeTasksWithOptions(ctx context.Context, conditions map[string]interface{}, page, pageSize int, preloads []string) ([]*models.UpgradeTask, int64, error)
}

// BoxHeartbeatRepository 盒子心跳Repository接口
type BoxHeartbeatRepository interface {
	BaseRepository[models.BoxHeartbeat]

	// 心跳特定查询
	FindByBoxID(ctx context.Context, boxID uint, limit int) ([]*models.BoxHeartbeat, error)
	FindByBoxIDAndDateRange(ctx context.Context, boxID uint, startDate, endDate string) ([]*models.BoxHeartbeat, error)
	GetLatestHeartbeat(ctx context.Context, boxID uint) (*models.BoxHeartbeat, error)

	// 心跳统计
	GetHeartbeatStatistics(ctx context.Context, boxID uint) (map[string]interface{}, error)
	CleanupOldHeartbeats(ctx context.Context, daysToKeep int) error
}

// RepositoryManager Repository管理器接口
type RepositoryManager interface {
	// 获取各个Repository实例
	Box() BoxRepository
	OriginalModel() OriginalModelRepository
	UploadSession() UploadSessionRepository
	ModelTag() ModelTagRepository
	Task() TaskRepository

	// 部署任务相关
	DeploymentTask() DeploymentRepository
	ModelDeployment() ModelDeploymentRepository
	ModelBoxDeployment() ModelBoxDeploymentRepository

	Upgrade() UpgradeRepository
	UpgradePackage() UpgradePackageRepository
	BoxHeartbeat() BoxHeartbeatRepository

	// 视频管理相关Repository - REQ-009
	VideoSource() VideoSourceRepository
	VideoFile() VideoFileRepository
	ExtractFrame() ExtractFrameRepository
	ExtractTask() ExtractTaskRepository
	RecordTask() RecordTaskRepository

	// 事务管理
	Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error
	DB() *gorm.DB
}

// PaginationOptions 分页选项
type PaginationOptions struct {
	Page     int `json:"page" validate:"min=1"`
	PageSize int `json:"page_size" validate:"min=1,max=100"`
}

// SortOptions 排序选项
type SortOptions struct {
	Field string `json:"field"`
	Order string `json:"order"` // asc, desc
}

// QueryOptions 查询选项
type QueryOptions struct {
	Pagination *PaginationOptions `json:"pagination,omitempty"`
	Sort       *SortOptions       `json:"sort,omitempty"`
	Preload    []string           `json:"preload,omitempty"`
}

// PaginatedResult 分页结果
type PaginatedResult[T any] struct {
	Data       []*T  `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// UpgradePackageRepository 升级包Repository接口
type UpgradePackageRepository interface {
	BaseRepository[models.UpgradePackage]

	// 根据名称和版本查找升级包
	FindByNameAndVersion(ctx context.Context, name, version string) (*models.UpgradePackage, error)

	// 根据版本查找升级包
	FindByVersion(ctx context.Context, version string) ([]*models.UpgradePackage, error)

	// 根据类型查找升级包
	FindByType(ctx context.Context, packageType models.UpgradePackageType) ([]*models.UpgradePackage, error)

	// 根据状态查找升级包
	FindByStatus(ctx context.Context, status models.PackageStatus) ([]*models.UpgradePackage, error)

	// 获取可用的升级包列表(status=ready且未删除)
	FindAvailable(ctx context.Context) ([]*models.UpgradePackage, error)

	// 获取最新版本的升级包
	FindLatestByType(ctx context.Context, packageType models.UpgradePackageType) (*models.UpgradePackage, error)

	// 检查版本是否存在
	VersionExists(ctx context.Context, version string) (bool, error)

	// 检查版本和类型的组合是否存在
	VersionAndTypeExists(ctx context.Context, version string, packageType models.UpgradePackageType) (bool, error)

	// 根据版本和类型查找升级包
	FindByVersionAndType(ctx context.Context, version string, packageType models.UpgradePackageType) (*models.UpgradePackage, error)

	// 获取升级包统计信息
	GetStatistics(ctx context.Context) (map[string]interface{}, error)

	// 批量更新状态
	BatchUpdateStatus(ctx context.Context, ids []uint, status models.PackageStatus) error
}

// DeploymentRepository 部署任务Repository接口
type DeploymentRepository interface {
	BaseRepository[models.DeploymentTask]

	// 基础查询
	FindByStatus(ctx context.Context, status models.DeploymentTaskStatus) ([]*models.DeploymentTask, error)

	FindByCreatedBy(ctx context.Context, userID uint) ([]*models.DeploymentTask, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.DeploymentTaskStatus) error
	UpdateProgress(ctx context.Context, id uint, progress float64, message string) error

	// 运行时管理
	FindRunningTasks(ctx context.Context) ([]*models.DeploymentTask, error)
	FindPendingTasks(ctx context.Context, limit int) ([]*models.DeploymentTask, error)

	// 日志和结果
	AddExecutionLog(ctx context.Context, id uint, log models.DeploymentLog) error
	AddResult(ctx context.Context, id uint, result models.DeploymentResult) error
	GetLogs(ctx context.Context, id uint, limit int) ([]models.DeploymentLog, error)
	GetResults(ctx context.Context, id uint) ([]models.DeploymentResult, error)

	// 统计和监控
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
	GetStatusDistribution(ctx context.Context) (map[models.DeploymentTaskStatus]int64, error)
	GetPerformanceReport(ctx context.Context, startDate, endDate string) (map[string]interface{}, error)

	// 查询操作
	SearchTasks(ctx context.Context, keyword string, options *QueryOptions) ([]*models.DeploymentTask, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.DeploymentTask, error)
	FindByPriority(ctx context.Context, priority int) ([]*models.DeploymentTask, error)

	// 清理操作
	CleanupCompletedTasks(ctx context.Context, olderThan time.Time) (int64, error)
	CleanupLogs(ctx context.Context, deploymentID uint, keepLatest int) error
}
