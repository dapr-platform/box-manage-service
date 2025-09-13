package service

import (
	"box-manage-service/models"
	"box-manage-service/modules/ffmpeg"
	"context"
	"net/http"
	"time"
)

// ZLMediaKitService ZLMediaKit服务接口
type ZLMediaKitService interface {
	AddStreamProxy(ctx context.Context, req *AddStreamProxyRequest) (*StreamInfo, error)
	CloseStream(ctx context.Context, app, stream string) error
	StartRecord(ctx context.Context, req *ZLMStartRecordRequest) error
	StopRecord(ctx context.Context, req *ZLMStopRecordRequest) error
	GetAllStreams() ([]*StreamInfo, error)
}

// FFmpegService FFmpeg服务接口
type FFmpegService interface {
	ExtractFrames(ctx context.Context, req *ffmpeg.ExtractFramesRequest) (*FFmpegExtractResult, error)
	ConvertVideo(ctx context.Context, req *ConvertVideoRequest) error
}

// VideoSourceService 视频源服务接口
type VideoSourceService interface {
	CreateVideoSource(ctx context.Context, req *CreateVideoSourceRequest) (*models.VideoSource, error)
	GetVideoSource(ctx context.Context, id uint) (*models.VideoSource, error)
	GetVideoSources(ctx context.Context, req *GetVideoSourcesRequest) ([]*models.VideoSource, int64, error)
	UpdateVideoSource(ctx context.Context, id uint, req *UpdateVideoSourceRequest) (*models.VideoSource, error)
	DeleteVideoSource(ctx context.Context, id uint) error
	StartVideoSource(ctx context.Context, id uint) error
	SyncToZLMediaKit(ctx context.Context) error
	TakeScreenshot(ctx context.Context, req *ScreenshotRequest) (*ScreenshotResponse, error)

	// 监控相关方法
	StartMonitoring()
	StopMonitoring()
	GetMonitoringStatus() map[string]interface{}
}

// VideoFileService 视频文件服务接口
type VideoFileService interface {
	UploadVideoFile(ctx context.Context, req *UploadVideoFileRequest) (*models.VideoFile, error)
	GetVideoFiles(ctx context.Context, req *GetVideoFilesRequest) ([]*models.VideoFile, int64, error)
	GetVideoFile(ctx context.Context, id uint) (*models.VideoFile, error)
	UpdateVideoFile(ctx context.Context, id uint, req *UpdateVideoFileRequest) (*models.VideoFile, error)
	DeleteVideoFile(ctx context.Context, id uint) error
	DownloadVideoFile(ctx context.Context, id uint) (*DownloadInfo, error)
	ConvertVideoFile(ctx context.Context, id uint) error
}

// ExtractTaskService 抽帧任务服务接口
type ExtractTaskService interface {
	CreateExtractTask(ctx context.Context, req *CreateExtractTaskRequest) (*models.ExtractTask, error)
	GetExtractTask(ctx context.Context, id uint) (*models.ExtractTask, error)
	GetExtractTasks(ctx context.Context, req *GetExtractTasksRequest) ([]*models.ExtractTask, int64, error)
	StartExtractTask(ctx context.Context, id uint) error
	StopExtractTask(ctx context.Context, id uint) error
	DeleteExtractTask(ctx context.Context, id uint) error
	GetTaskFrames(ctx context.Context, taskID string, page, pageSize int) ([]*models.ExtractFrame, int64, error)
	GetFrameImage(ctx context.Context, frameID uint) (*FrameImageInfo, error)
	DownloadTaskImages(ctx context.Context, taskID uint) (*TaskImagesDownloadInfo, error)
}

// RecordTaskService 录制任务服务接口
type RecordTaskService interface {
	CreateRecordTask(ctx context.Context, req *CreateRecordTaskRequest) (*models.RecordTask, error)
	GetRecordTasks(ctx context.Context, req *GetRecordTasksRequest) ([]*models.RecordTask, int64, error)
	GetRecordTask(ctx context.Context, id uint) (*models.RecordTask, error)
	StartRecordTask(ctx context.Context, id uint) error
	StopRecordTask(ctx context.Context, id uint) error
	DeleteRecordTask(ctx context.Context, id uint) error
	DownloadRecord(ctx context.Context, id uint) (*DownloadInfo, error)
	GetTaskStatistics(ctx context.Context, userID *uint) (*RecordTaskStatistics, error)
}

// ConversionService 模型转换服务接口
type ConversionService interface {
	// 转换任务管理
	CreateConversionTask(ctx context.Context, req *CreateConversionTaskRequest) ([]*models.ConversionTask, error)
	GetConversionTask(ctx context.Context, taskID string) (*models.ConversionTask, error)
	GetConversionTasks(ctx context.Context, req *GetConversionTasksRequest) (*GetConversionTasksResponse, error)
	DeleteConversionTask(ctx context.Context, taskID string) error

	// 转换执行
	StartConversion(ctx context.Context, taskID string) error
	StopConversion(ctx context.Context, taskID string) error
	CompleteConversion(ctx context.Context, taskID string, outputPath string) error
	FailConversion(ctx context.Context, taskID string, errorMsg string) error

	// 进度和状态
	UpdateConversionProgress(ctx context.Context, taskID string, progress int) error
	GetConversionProgress(ctx context.Context, taskID string) (*ConversionProgress, error)
	GetConversionLogs(ctx context.Context, taskID string) ([]string, error)

	// 统计
	GetConversionStatistics(ctx context.Context, userID *uint) (*ConversionStatistics, error)

	// 清理操作
	CleanupFailedTasks(ctx context.Context) (int64, error)

	// 服务管理
	RecoverPendingTasks(ctx context.Context) error
	CheckRunningTasksTimeout(ctx context.Context) error
}

// ConvertedModelService 转换后模型服务接口
type ConvertedModelService interface {
	// 基本CRUD操作
	CreateConvertedModel(ctx context.Context, req *CreateConvertedModelRequest) (*models.ConvertedModel, error)
	GetConvertedModel(ctx context.Context, id uint) (*models.ConvertedModel, error)
	UpdateConvertedModel(ctx context.Context, id uint, req *UpdateConvertedModelRequest) error
	DeleteConvertedModel(ctx context.Context, id uint) error

	// 查询操作
	GetConvertedModelList(ctx context.Context, req *GetConvertedModelListRequest) (*GetConvertedModelListResponse, error)
	GetConvertedModelsByOriginal(ctx context.Context, originalModelID uint) ([]*models.ConvertedModel, error)
	GetConvertedModelByName(ctx context.Context, name string) (*models.ConvertedModel, error)
	GetConvertedModelsByChip(ctx context.Context, chip string) ([]*models.ConvertedModel, error)

	// 搜索
	SearchConvertedModels(ctx context.Context, req *SearchConvertedModelRequest) (*SearchConvertedModelResponse, error)

	// 文件操作
	DownloadConvertedModel(ctx context.Context, id uint) (*DownloadInfo, error)

	// 统计
	GetConvertedModelStatistics(ctx context.Context, userID *uint) (*ConvertedModelStatistics, error)
}

// DockerService Docker转换服务接口
type DockerService interface {
	UploadModel(ctx context.Context, task *models.ConversionTask, inputPath string) (string, error)
	StartConversion(ctx context.Context, task *models.ConversionTask, modelPath string, remoteTaskId string) error
	StopConversion(ctx context.Context, taskID string) error
	GetConversionStatus(ctx context.Context, taskID string) (*ConversionStatus, error)
	DownloadResult(ctx context.Context, taskID string, outputPath string) error
}

// ModelService 原始模型服务接口
type ModelService interface {
	// 直接上传（简化版本）
	DirectUpload(ctx context.Context, req *DirectUploadRequest) (*models.OriginalModel, error)

	// 分片上传（保留用于大文件，可选）
	InitializeUpload(ctx context.Context, req *InitUploadRequest) (*InitUploadResponse, error)
	UploadChunk(ctx context.Context, req *UploadChunkRequest) error
	CompleteUpload(ctx context.Context, sessionID string) (*models.OriginalModel, error)
	CancelUpload(ctx context.Context, sessionID string) error

	// 模型管理
	GetModelList(ctx context.Context, req *GetModelListRequest) (*GetModelListResponse, error)
	GetModelDetail(ctx context.Context, modelID uint) (*models.OriginalModel, error)
	UpdateModel(ctx context.Context, modelID uint, req *UpdateModelRequest) error
	DeleteModel(ctx context.Context, modelID uint) error
	DownloadModel(ctx context.Context, modelID uint) (*DownloadModelResponse, error)

	// 搜索和筛选
	SearchModels(ctx context.Context, req *SearchModelRequest) (*SearchModelResponse, error)
	GetModelVersions(ctx context.Context, modelName string) ([]*models.OriginalModel, error)

	// 验证和统计
	ValidateModel(ctx context.Context, modelID uint) error
	GetModelStatistics(ctx context.Context) (*ModelStatisticsResponse, error)
	GetStorageStatistics(ctx context.Context) (*StorageStatisticsResponse, error)

	// 维护操作
	CleanupExpiredSessions(ctx context.Context) error
	ArchiveOldModels(ctx context.Context, olderThanDays int) error
}

// SSEService Server-Sent Events服务接口
type SSEService interface {
	// 连接处理
	HandleConversionTaskEvents(w http.ResponseWriter, r *http.Request) error
	HandleTaskEvents(w http.ResponseWriter, r *http.Request) error
	HandleBoxEvents(w http.ResponseWriter, r *http.Request) error
	HandleSystemEvents(w http.ResponseWriter, r *http.Request) error
	HandleDiscoveryEvents(w http.ResponseWriter, r *http.Request) error

	// 事件发送
	BroadcastConversionTaskUpdate(task *models.ConversionTask) error
	BroadcastTaskUpdate(task *models.Task) error
	BroadcastBoxUpdate(box *models.Box) error
	BroadcastSystemEvent(event *SystemEvent) error
	BroadcastDiscoveryProgress(progress *DiscoveryProgress) error
	BroadcastDiscoveryResult(result *DiscoveryResult) error

	// 连接管理
	GetConnectionStats() *ConnectionStats
	CloseAllConnections() error
}

// ModelDependencyService 模型依赖检查服务接口
type ModelDependencyService interface {
	// CheckTaskModelDependency 检查任务的模型依赖
	CheckTaskModelDependency(ctx context.Context, taskID uint) (*DependencyCheckResult, error)

	// CheckBoxModelCompatibility 检查盒子与模型的兼容性
	CheckBoxModelCompatibility(ctx context.Context, boxID uint, modelKey string) (*CompatibilityResult, error)

	// GetModelDeploymentStatus 获取模型在指定盒子上的部署状态
	GetModelDeploymentStatus(ctx context.Context, boxID uint, modelKey string) (*DeploymentStatus, error)

	// DeployModelToBox 将模型部署到指定盒子
	DeployModelToBox(ctx context.Context, boxID uint, modelKey string) (*DeploymentResult, error)

	// EnsureModelAvailability 确保模型在盒子上可用（如果未部署则自动部署）
	EnsureModelAvailability(ctx context.Context, boxID uint, modelKey string) error

	// GetBoxModels 获取盒子上所有已部署的模型
	GetBoxModels(ctx context.Context, boxID uint) ([]*BoxModel, error)

	// GetModelUsage 获取模型的使用情况统计
	GetModelUsage(ctx context.Context, modelKey string) (*ModelUsageStats, error)
}

// TaskSchedulerService 任务调度服务接口
type TaskSchedulerService interface {
	// ScheduleAutoTasks 调度所有启用自动调度的任务
	ScheduleAutoTasks(ctx context.Context) (*ScheduleResult, error)

	// ScheduleTask 调度单个任务到最适合的盒子
	ScheduleTask(ctx context.Context, taskID uint) (*TaskScheduleResult, error)

	// FindCompatibleBoxes 查找与任务兼容的盒子
	FindCompatibleBoxes(ctx context.Context, taskID uint) ([]*BoxScore, error)
}

// ScheduleResult 批量调度结果
type ScheduleResult struct {
	TotalTasks     int                   `json:"total_tasks"`
	ScheduledTasks int                   `json:"scheduled_tasks"`
	FailedTasks    int                   `json:"failed_tasks"`
	TaskResults    []*TaskScheduleResult `json:"task_results"`
	Summary        map[string]int        `json:"summary"`
}

// TaskScheduleResult 单个任务调度结果
type TaskScheduleResult struct {
	TaskID     uint        `json:"task_id"`
	TaskName   string      `json:"task_name"`
	BoxID      *uint       `json:"box_id,omitempty"`
	BoxName    string      `json:"box_name,omitempty"`
	Score      float64     `json:"score,omitempty"`
	Success    bool        `json:"success"`
	Reason     string      `json:"reason"`
	Candidates []*BoxScore `json:"candidates,omitempty"`
}

// BoxScore 盒子评分
type BoxScore struct {
	BoxID       uint     `json:"box_id"`
	BoxName     string   `json:"box_name"`
	Score       float64  `json:"score"`
	Reasons     []string `json:"reasons"`
	IsOnline    bool     `json:"is_online"`
	HasCapacity bool     `json:"has_capacity"`
	TagMatches  int      `json:"tag_matches"`
}

// TaskDeploymentService 任务部署服务接口
type TaskDeploymentService interface {
	// 基础部署操作（同步）
	DeployTask(ctx context.Context, taskID uint, boxID uint) (*DeploymentResponse, error)
	DeployTaskWithLogging(ctx context.Context, taskID uint, boxID uint, deploymentTask *models.DeploymentTask) (*DeploymentResponse, error)
	UndeployTask(ctx context.Context, taskID uint, boxID uint) error
	RedeployTask(ctx context.Context, taskID uint, oldBoxID *uint, newBoxID uint) (*DeploymentResponse, error)

	// 批量部署操作（同步）
	BatchDeployTask(ctx context.Context, taskID uint, boxIDs []uint) ([]*DeploymentResponse, error)

	// 盒子任务管理
	UpdateTaskOnBox(ctx context.Context, taskID uint, boxID uint) (*DeploymentResponse, error)
	StopTaskOnBox(ctx context.Context, taskID uint, boxID uint) error
	GetTaskStatusFromBox(ctx context.Context, taskID uint, boxID uint) (*BoxTaskStatus, error)

	// 验证任务配置
	ValidateTaskConfig(ctx context.Context, task *models.Task) error
}

// DeploymentTaskService 部署任务管理服务接口
type DeploymentTaskService interface {
	// 部署任务管理
	CreateDeploymentTask(ctx context.Context, req *CreateDeploymentTaskRequest) (*models.DeploymentTask, error)
	GetDeploymentTask(ctx context.Context, id uint) (*models.DeploymentTask, error)
	GetDeploymentTasks(ctx context.Context, filters *DeploymentTaskFilters) ([]*models.DeploymentTask, error)
	UpdateDeploymentTask(ctx context.Context, id uint, updates map[string]interface{}) error
	DeleteDeploymentTask(ctx context.Context, id uint) error

	// 部署任务控制
	StartDeploymentTask(ctx context.Context, id uint) error
	CancelDeploymentTask(ctx context.Context, id uint, reason string) error

	// 部署任务监控
	GetDeploymentTaskLogs(ctx context.Context, id uint, limit int) ([]models.DeploymentLog, error)
	GetDeploymentTaskResults(ctx context.Context, id uint) ([]models.DeploymentResult, error)
	GetDeploymentTaskProgress(ctx context.Context, id uint) (*DeploymentProgress, error)

	// 批量操作
	BatchCreateDeployment(ctx context.Context, taskIDs []uint, boxIDs []uint, config *models.DeploymentConfig) (*models.DeploymentTask, error)

	// 统计和清理
	GetDeploymentStatistics(ctx context.Context) (map[string]interface{}, error)
	CleanupOldTasks(ctx context.Context, olderThan time.Time) (int64, error)
}

// CreateDeploymentTaskRequest 创建部署任务请求
type CreateDeploymentTaskRequest struct {
	Name             string                   `json:"name" validate:"required"`
	Description      string                   `json:"description"`
	TaskIDs          []uint                   `json:"task_ids" validate:"required,min=1"`
	TargetBoxIDs     []uint                   `json:"target_box_ids" validate:"required,min=1"`
	Priority         int                      `json:"priority"`
	DeploymentConfig *models.DeploymentConfig `json:"deployment_config"`
	CreatedBy        uint                     `json:"created_by"`
}

// DeploymentTaskFilters 部署任务过滤条件
type DeploymentTaskFilters struct {
	Status    *models.DeploymentTaskStatus `json:"status"`
	CreatedBy *uint                        `json:"created_by"`
	Priority  *int                         `json:"priority"`
	StartDate *time.Time                   `json:"start_date"`
	EndDate   *time.Time                   `json:"end_date"`
	Keyword   string                       `json:"keyword"`
	Limit     int                          `json:"limit"`
	Offset    int                          `json:"offset"`
}

// DeploymentProgress 部署进度
type DeploymentProgress struct {
	DeploymentID   uint    `json:"deployment_id"`
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	FailedTasks    int     `json:"failed_tasks"`
	SkippedTasks   int     `json:"skipped_tasks"`
	Progress       float64 `json:"progress"`
	Message        string  `json:"message"`
	EstimatedTime  string  `json:"estimated_time"`
}

// StartupRecoveryService 程序启动恢复服务接口
type StartupRecoveryService interface {
	// RecoverOnStartup 程序启动时执行恢复操作
	RecoverOnStartup(ctx context.Context) error
}

// TaskExecutorService 任务执行服务接口
type TaskExecutorService interface {
	// ExecuteTask 执行单个任务（完整流程）
	ExecuteTask(ctx context.Context, taskID uint) (*ExecutionResult, error)

	// ExecuteTaskOnBox 在指定盒子上执行任务
	ExecuteTaskOnBox(ctx context.Context, taskID uint, boxID uint) (*ExecutionResult, error)

	// StartTaskExecution 启动任务执行
	StartTaskExecution(ctx context.Context, taskID uint) (*ExecutionSession, error)

	// MonitorTaskExecution 监控任务执行状态
	MonitorTaskExecution(ctx context.Context, sessionID string) (*ExecutionStatus, error)

	// StopTaskExecution 停止任务执行
	StopTaskExecution(ctx context.Context, sessionID string) error

	// GetExecutionHistory 获取任务执行历史
	GetExecutionHistory(ctx context.Context, taskID uint) ([]*ExecutionRecord, error)

	// StartExecutor 启动执行器
	StartExecutor(ctx context.Context) error

	// StopExecutor 停止执行器
	StopExecutor() error

	// GetExecutorStatus 获取执行器状态
	GetExecutorStatus() *ExecutorStatus
}
