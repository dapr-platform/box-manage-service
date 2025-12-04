/*
 * @module api/routes
 * @description API路由配置模块，负责初始化和配置所有HTTP路由
 * @architecture RESTful API架构
 * @documentReference REQ-001, REQ-002, REQ-003, REQ-004
 * @stateFlow 无状态HTTP请求处理
 * @rules 遵循RESTful API设计规范，统一错误处理和响应格式
 * @dependencies github.com/go-chi/chi/v5, github.com/go-chi/cors, github.com/go-chi/render
 * @refs DESIGN-000.md
 */

package api

import (
	"box-manage-service/api/controllers"
	"box-manage-service/client"
	"box-manage-service/config"
	"box-manage-service/modules/ffmpeg"
	"box-manage-service/repository"
	"box-manage-service/service"
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"gorm.io/gorm"
)

// InitRoute 初始化所有API路由
func InitRoute(r *chi.Mux, db *gorm.DB, cfg *config.Config) service.ConversionService {
	var conversionService service.ConversionService

	// 基础中间件
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// CORS配置
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.Server.AllowedOrigins,
		AllowedMethods:   cfg.Server.AllowedMethods,
		AllowedHeaders:   cfg.Server.AllowedHeaders,
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 健康检查
	healthController := controllers.NewHealthController()
	r.Get("/health", healthController.Health)
	r.Get("/ready", healthController.Ready)

	// 注意：用户管理和角色管理通过前端直接调用PostgREST API处理

	// 创建SSE服务（全局服务，不依赖数据库）
	sseService := service.NewSSEService()

	// 初始化服务和Repository（全局共享）
	var discoveryService *service.BoxDiscoveryService
	var monitoringService *service.BoxMonitoringService
	var proxyService *service.BoxProxyService
	var upgradeService *service.UpgradeService
	var taskSyncService *service.TaskSyncService
	var systemLogService service.SystemLogService
	var repoManager repository.RepositoryManager

	// 共享的Repository实例
	var taskRepo repository.TaskRepository
	var boxRepo repository.BoxRepository
	var videoSourceRepo repository.VideoSourceRepository
	var videoFileRepo repository.VideoFileRepository
	var modelRepo repository.OriginalModelRepository
	var convertedModelRepo repository.ConvertedModelRepository
	var conversionRepo repository.ConversionTaskRepository
	var recordTaskRepo repository.RecordTaskRepository
	var extractTaskRepo repository.ExtractTaskRepository
	var extractFrameRepo repository.ExtractFrameRepository

	// 共享的服务实例
	var taskExecutorService service.TaskExecutorService
	var taskDeploymentService service.TaskDeploymentService
	var taskSchedulerService service.TaskSchedulerService
	var modelDependencyService service.ModelDependencyService
	var videoSourceService service.VideoSourceService
	var videoFileService service.VideoFileService
	var extractTaskService service.ExtractTaskService
	var recordTaskService service.RecordTaskService

	if db != nil {
		// 创建Repository管理器和所有Repository实例
		repoManager = repository.NewRepositoryManager(db)
		taskRepo = repository.NewTaskRepository(db)
		boxRepo = repository.NewBoxRepository(db)
		videoSourceRepo = repository.NewVideoSourceRepository(db)
		videoFileRepo = repository.NewVideoFileRepository(db)
		modelRepo = repository.NewOriginalModelRepository(db)
		convertedModelRepo = repository.NewConvertedModelRepository(db)
		conversionRepo = repository.NewConversionTaskRepository(db)
		recordTaskRepo = repository.NewRecordTaskRepository(db)
		extractTaskRepo = repository.NewExtractTaskRepository(db)
		extractFrameRepo = repository.NewExtractFrameRepository(db)

		// 创建系统日志服务
		systemLogService = service.NewSystemLogService(repoManager.SystemLog())

		// 创建基础服务层实例
		discoveryService = service.NewBoxDiscoveryService(repoManager, systemLogService)
		monitoringService = service.NewBoxMonitoringService(repoManager, nil, systemLogService)
		proxyService = service.NewBoxProxyService(repoManager)
		upgradeService = service.NewUpgradeService(repoManager, proxyService, monitoringService, sseService)
		taskSyncService = service.NewTaskSyncService(repoManager, proxyService)

		// 创建ZLMediaKit客户端和FFmpeg模块（共享）
		zlmClient := client.NewZLMediaKitClient(
			cfg.Video.ZLMediaKit.Host,
			cfg.Video.ZLMediaKit.Port,
			cfg.Video.ZLMediaKit.Secret,
		)
		ffmpegModule := ffmpeg.NewFFmpegModule(
			cfg.Video.FFmpeg.FFmpegPath,
			cfg.Video.FFmpeg.FFprobePath,
			cfg.Video.FFmpeg.Timeout,
		)

		// 创建任务相关服务
		taskDeploymentService = service.NewTaskDeploymentService(taskRepo, boxRepo, videoSourceRepo, modelRepo, convertedModelRepo, cfg.Video, systemLogService, sseService)
		taskSchedulerService = service.NewTaskSchedulerService(taskRepo, boxRepo, taskDeploymentService)
		modelDependencyService = service.NewModelDependencyService(taskRepo, boxRepo, convertedModelRepo)
		taskExecutorService = service.NewTaskExecutorService(taskRepo, boxRepo, taskSchedulerService, taskDeploymentService, modelDependencyService)

		// 创建视频相关服务
		videoSourceService = service.NewVideoSourceService(videoSourceRepo, videoFileRepo, zlmClient, cfg.Video, ffmpegModule)
		videoFileService = service.NewVideoFileService(videoFileRepo, videoSourceRepo, ffmpegModule, cfg.Video.Storage.BasePath)
		// 抽帧服务需要两个路径：FramePath用于存储抽帧结果，BasePath用于解析视频文件输入路径
		extractTaskService = service.NewExtractTaskService(extractTaskRepo, extractFrameRepo, videoSourceRepo, videoFileRepo, ffmpegModule, cfg.Video.Storage.FramePath, cfg.Video.Storage.BasePath, sseService)
		recordTaskService = service.NewRecordTaskService(recordTaskRepo, videoSourceRepo, ffmpegModule, zlmClient, cfg.Video.Storage.RecordPath, cfg.Video, sseService)

		// 启动核心服务
		go func() {
			log.Println("启动高性能盒子监控服务...")
			monitoringService.Start()
		}()

		go func() {
			log.Println("启动任务执行器服务...")
			if err := taskExecutorService.StartExecutor(context.Background()); err != nil {
				log.Printf("启动任务执行器失败: %v", err)
			}
		}()

		// 启动日志清理调度器
		go systemLogService.StartCleanupScheduler(context.Background())

		// AI盒子管理 (REQ-001: 盒子管理功能)
		r.Route("/api/v1/boxes", func(r chi.Router) {
			boxController := controllers.NewBoxController(discoveryService, monitoringService, proxyService, upgradeService, taskSyncService, sseService)
			upgradeController := controllers.NewUpgradeController(upgradeService)

			// 盒子基础管理
			r.Post("/", boxController.AddBox)
			r.Get("/", boxController.GetBoxes)
			r.Get("/{id}", boxController.GetBox)
			r.Put("/{id}", boxController.UpdateBox)
			r.Delete("/{id}", boxController.DeleteBox)

			// 盒子发现
			r.Post("/discover", boxController.DiscoverBoxes)
			r.Get("/discover/{scan_id}", boxController.GetScanTask)
			r.Post("/discover/{scan_id}/cancel", boxController.CancelScanTask)

			// 盒子状态监控
			r.Get("/{id}/status", boxController.GetBoxStatus)
			r.Post("/{id}/refresh", boxController.RefreshBoxStatus)
			r.Get("/{id}/status/history", boxController.GetBoxStatusHistory)
			r.Get("/overview", boxController.GetBoxesOverview)

			// 盒子代理API（转发到盒子）
			r.Get("/{id}/models", boxController.GetBoxModels)
			r.Get("/{id}/tasks", boxController.GetBoxTasks)

			// 任务同步
			r.Post("/{id}/sync-tasks", boxController.SyncBoxTasks)

			// 单个盒子升级（移到盒子路由组内）
			r.Post("/{id}/upgrade", upgradeController.UpgradeBox)
			r.Post("/{id}/rollback", upgradeController.RollbackBox)

			// 批量升级
			r.Post("/batch/upgrade", upgradeController.BatchUpgrade)
		})

		// 升级管理（非盒子相关的路由）
		r.Route("/api/v1", func(r chi.Router) {
			upgradeController := controllers.NewUpgradeController(upgradeService)
			upgradePackageController := controllers.NewUpgradePackageController(repoManager, "./uploads/packages")

			// 升级任务管理
			r.Get("/upgrades", upgradeController.GetUpgradeTasks)
			r.Get("/upgrades/{id}", upgradeController.GetUpgradeTask)
			r.Post("/upgrades/{id}/cancel", upgradeController.CancelUpgradeTask)
			r.Post("/upgrades/{id}/retry", upgradeController.RetryUpgradeTask)
			r.Delete("/upgrades/{id}", upgradeController.DeleteUpgradeTask)

			// 升级包管理
			r.Post("/upgrade-packages", upgradePackageController.CreatePackage)
			r.Get("/upgrade-packages", upgradePackageController.GetPackages)
			r.Get("/upgrade-packages/statistics", upgradePackageController.GetStatistics)
			r.Get("/upgrade-packages/{id}", upgradePackageController.GetPackage)
			r.Delete("/upgrade-packages/{id}", upgradePackageController.DeletePackage)
			r.Post("/upgrade-packages/{id}/upload", upgradePackageController.UploadFile)
			r.Put("/upgrade-packages/{id}/status", upgradePackageController.UpdatePackageStatus)
			r.Get("/upgrade-packages/{id}/download/{file_type}", upgradePackageController.DownloadFile)
		})
	} else {
		log.Println("Warning: Database not initialized, skipping service routes")

		// 提供基本的占位路由以避免编译错误
		r.Route("/api/v1/boxes", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Service not initialized", http.StatusServiceUnavailable)
			})
		})
	}

	// 原始模型管理 (REQ-002: 原始模型管理)
	if db != nil {
		// 使用共享的Repository实例
		uploadRepo := repository.NewUploadSessionRepository(db)
		tagRepo := repository.NewModelTagRepository(db)

		// 模型服务配置 (使用统一配置)
		modelConfig := &service.ModelServiceConfig{
			StorageBasePath:   cfg.Model.StorageBasePath,
			TempPath:          cfg.Model.TempPath,
			MaxFileSize:       cfg.Model.MaxFileSize,
			ChunkSize:         cfg.Model.ChunkSize,
			SessionTimeout:    cfg.Model.SessionTimeout,
			AllowedTypes:      cfg.Model.AllowedTypes,
			EnableChecksum:    cfg.Model.EnableChecksum,
			EnableCompression: cfg.Model.EnableCompression,
		}

		modelService := service.NewModelService(modelRepo, uploadRepo, tagRepo, modelConfig)

		r.Route("/api/models", func(r chi.Router) {
			modelController := controllers.NewModelController(modelService)

			// 文件上传管理
			r.Post("/upload", modelController.UploadModel) // 简化为直接上传

			// 模型基础管理
			r.Get("/", modelController.GetModels)
			r.Get("/{id}", modelController.GetModel)
			r.Put("/{id}", modelController.UpdateModel)
			r.Delete("/{id}", modelController.DeleteModel)

			// 模型下载和验证
			r.Get("/{id}/download", modelController.DownloadModel)
			r.Post("/{id}/validate", modelController.ValidateModel)

			// 搜索和版本管理
			r.Get("/search", modelController.SearchModels)
			r.Get("/versions", modelController.GetModelVersions)

			// 统计信息
			r.Get("/statistics", modelController.GetModelStatistics)
			r.Get("/storage/statistics", modelController.GetStorageStatistics)
		})

		// 转换后模型管理 (REQ-004: 转换后模型管理)
		modelBoxDeploymentRepo := repository.NewModelBoxDeploymentRepository(db)
		convertedModelConfig := &service.ConvertedModelConfig{
			StorageBasePath:    cfg.Model.StorageBasePath,
			MaxFileSize:        cfg.Model.MaxFileSize,
			DeleteFileOnRemove: true,
		}
		convertedModelService := service.NewConvertedModelService(convertedModelRepo, modelRepo, convertedModelConfig)

		r.Route("/api/converted-models", func(r chi.Router) {
			convertedModelController := controllers.NewConvertedModelController(convertedModelService, modelBoxDeploymentRepo)

			// 转换后模型基础管理
			r.Post("/", convertedModelController.CreateConvertedModel)
			r.Get("/", convertedModelController.GetConvertedModels)
			r.Get("/{id}", convertedModelController.GetConvertedModel)
			r.Put("/{id}", convertedModelController.UpdateConvertedModel)
			r.Delete("/{id}", convertedModelController.DeleteConvertedModel)

			// 搜索
			r.Get("/search", convertedModelController.SearchConvertedModels)

			// 下载
			r.Get("/{id}/download", convertedModelController.DownloadConvertedModel)

			// 统计
			r.Get("/statistics", convertedModelController.GetConvertedModelStatistics)

			// 根据原始模型获取
			r.Get("/original/{originalModelId}", convertedModelController.GetConvertedModelsByOriginal)

			// 部署信息查询
			r.Get("/{id}/deployments", convertedModelController.GetModelDeployments)
			r.Get("/key/{modelKey}/deployments", convertedModelController.GetModelDeploymentsByKey)
		})
	}

	// 模型转换管理 (REQ-003: 模型转换功能)
	if db != nil {

		conversionService = service.NewConversionService(conversionRepo, modelRepo, convertedModelRepo, &cfg.Conversion, sseService, systemLogService)

		r.Route("/api/conversion", func(r chi.Router) {
			conversionController := controllers.NewConversionController(conversionService)

			// 转换任务管理
			r.Post("/tasks", conversionController.CreateConversionTask)
			r.Get("/tasks", conversionController.GetConversionTasks)
			r.Get("/tasks/{taskId}", conversionController.GetConversionTask)
			r.Delete("/tasks/{taskId}", conversionController.DeleteConversionTask)

			// 手动控制
			r.Post("/tasks/{taskId}/start", conversionController.StartConversion)
			r.Post("/tasks/{taskId}/stop", conversionController.StopConversion)

			// 进度和日志
			r.Get("/tasks/{taskId}/progress", conversionController.GetConversionProgress)
			r.Get("/tasks/{taskId}/logs", conversionController.GetConversionLogs)

			// 统计
			r.Get("/statistics", conversionController.GetConversionStatistics)
		})

		// SSE事件流 (Server-Sent Events)
		r.Route("/api/sse", func(r chi.Router) {
			sseController := controllers.NewSSEController(sseService)

			// 专用事件流端点
			r.Get("/conversion", sseController.HandleConversionEvents)             // 转换相关事件
			r.Get("/extract-tasks", sseController.HandleExtractTaskEvents)         // 抽帧任务事件
			r.Get("/record-tasks", sseController.HandleRecordTaskEvents)           // 录制任务事件
			r.Get("/deployment-tasks", sseController.HandleDeploymentTaskEvents)   // 部署任务事件
			r.Get("/batch-deployments", sseController.HandleBatchDeploymentEvents) // 批量部署事件
			r.Get("/model-deployments", sseController.HandleModelDeploymentEvents) // 模型部署任务事件
			r.Get("/boxes", sseController.HandleBoxEvents)                         // 盒子相关事件
			r.Get("/models", sseController.HandleModelEvents)                      // 模型相关事件
			r.Get("/discovery", sseController.HandleDiscoveryEvents)               // 扫描发现事件
			r.Get("/system", sseController.HandleSystemEvents)                     // 系统错误事件

			// 管理端点
			r.Get("/stats", sseController.GetConnectionStats)
			r.Post("/close-all", sseController.CloseAllConnections)
		})

	}

	// 任务管理 (REQ-005: 任务管理功能)
	if db != nil {
		// 使用共享的服务实例
		deploymentTaskRepo := repository.NewDeploymentRepository(db)
		deploymentTaskService := service.NewDeploymentTaskService(deploymentTaskRepo, taskRepo, taskDeploymentService)

		// 创建模型部署Repository和服务
		modelDeploymentRepo := repository.NewModelDeploymentRepository(db)
		modelBoxDeploymentRepo := repository.NewModelBoxDeploymentRepository(db)
		modelDeploymentService := service.NewModelDeploymentService(modelDeploymentRepo, convertedModelRepo, boxRepo, modelBoxDeploymentRepo)

		r.Route("/api/v1/tasks", func(r chi.Router) {
			taskController := controllers.NewTaskController(taskRepo, boxRepo, taskSchedulerService, taskDeploymentService)

			// 任务基础管理
			r.Post("/", taskController.CreateTask)
			r.Get("/", taskController.GetTasks)
			r.Get("/{id}", taskController.GetTask)
			r.Put("/{id}", taskController.UpdateTask)
			r.Delete("/{id}", taskController.DeleteTask)

			// 任务控制操作
			r.Post("/{id}/start", taskController.StartTask)
			r.Post("/{id}/stop", taskController.StopTask)
			r.Post("/{id}/pause", taskController.PauseTask)
			r.Post("/{id}/resume", taskController.ResumeTask)
			r.Post("/{id}/retry", taskController.RetryTask)

			// 任务下发和部署
			r.Post("/{id}/deploy", taskController.DeployTask)
			r.Post("/batch/deploy", taskController.BatchDeploy)

			// 任务状态和监控
			r.Get("/{id}/status", taskController.GetTaskStatus)
			r.Get("/{id}/logs", taskController.GetTaskLogs)
			r.Get("/{id}/sse", taskController.GetTaskSSEStream) // SSE图像流代理
			// r.Get("/{id}/execution-history", taskController.GetTaskExecutionHistory) // TODO: 实现此方法

			// 任务统计和查询
			r.Get("/statistics", taskController.GetTaskStatistics)
			r.Get("/tags", taskController.GetTaskTags)

			// 批量操作
			r.Post("/batch/start", taskController.BatchStart)
			r.Post("/batch/stop", taskController.BatchStop)
			r.Post("/batch/delete", taskController.BatchDelete)

			// 任务调度相关
			r.Post("/schedule/auto", taskController.TriggerAutoSchedule)
			r.Post("/{id}/schedule", taskController.ScheduleTask)
			r.Get("/{id}/compatible-boxes", taskController.GetCompatibleBoxes)
			r.Get("/auto-schedule", taskController.GetAutoScheduleTasks)
			r.Get("/affinity", taskController.GetTasksByAffinityTags)
			r.Get("/box/{boxId}/compatible", taskController.GetTasksByBox)
			r.Put("/{id}/progress", taskController.UpdateTaskProgress)
		})

		// 部署任务管理路由
		r.Route("/api/v1/deployments", func(r chi.Router) {
			deploymentTaskController := controllers.NewDeploymentTaskController(deploymentTaskService)

			// 基础CRUD
			r.Get("/", deploymentTaskController.GetDeploymentTasks)
			r.Post("/", deploymentTaskController.CreateDeploymentTask)
			r.Get("/{id}", deploymentTaskController.GetDeploymentTask)
			r.Delete("/{id}", deploymentTaskController.DeleteDeploymentTask)

			// 控制操作
			r.Post("/{id}/start", deploymentTaskController.StartDeploymentTask)
			r.Post("/{id}/cancel", deploymentTaskController.CancelDeploymentTask)

			// 监控操作
			r.Get("/{id}/logs", deploymentTaskController.GetDeploymentTaskLogs)
			r.Get("/{id}/results", deploymentTaskController.GetDeploymentTaskResults)
			r.Get("/{id}/progress", deploymentTaskController.GetDeploymentTaskProgress)

			// 批量操作
			r.Post("/batch", deploymentTaskController.BatchCreateDeployment)

			// 统计和清理
			r.Get("/statistics", deploymentTaskController.GetDeploymentStatistics)
			r.Post("/cleanup", deploymentTaskController.CleanupOldTasks)
		})

		// 模型部署管理路由
		r.Route("/api/v1/model-deployment", func(r chi.Router) {
			modelDeploymentController := controllers.NewModelDeploymentController(modelDeploymentService)

			// 部署任务管理
			r.Post("/tasks", modelDeploymentController.CreateDeploymentTask)
			r.Get("/tasks", modelDeploymentController.GetDeploymentTasks)
			r.Get("/tasks/{taskId}", modelDeploymentController.GetDeploymentTask)
			r.Delete("/tasks/{taskId}", modelDeploymentController.DeleteDeploymentTask)

			// 部署控制
			r.Post("/tasks/{taskId}/start", modelDeploymentController.StartDeployment)
			r.Post("/tasks/{taskId}/cancel", modelDeploymentController.CancelDeployment)

			// 部署项管理
			r.Get("/tasks/{taskId}/items", modelDeploymentController.GetDeploymentItems)
		})

		// TODO: 任务调度管理 - 需要实现SchedulerController
		/*
			r.Route("/api/v1/scheduler", func(r chi.Router) {
				schedulerController := controllers.NewSchedulerController(taskSchedulerService)

				// 调度操作
				r.Post("/schedule/{taskId}", schedulerController.ScheduleTask)
				r.Post("/schedule/pending", schedulerController.SchedulePendingTasks)

				// 调度状态
				r.Get("/queue/status", schedulerController.GetQueueStatus)
				r.Get("/find-best-box/{taskId}", schedulerController.FindBestBox)

				// 调度器控制
				r.Post("/start", schedulerController.StartScheduler)
				r.Post("/stop", schedulerController.StopScheduler)
			})
		*/

		// TODO: 模型依赖管理 - 需要实现DependencyController
		/*
			r.Route("/api/v1/dependencies", func(r chi.Router) {
				dependencyController := controllers.NewDependencyController(modelDependencyService)

				// 依赖检查
				r.Get("/tasks/{taskId}/check", dependencyController.CheckTaskDependency)
				r.Get("/boxes/{boxId}/models/{modelKey}/compatibility", dependencyController.CheckCompatibility)
				r.Get("/boxes/{boxId}/models/{modelKey}/status", dependencyController.GetDeploymentStatus)

				// 模型部署
				r.Post("/boxes/{boxId}/models/{modelKey}/deploy", dependencyController.DeployModel)
				r.Post("/boxes/{boxId}/models/{modelKey}/ensure", dependencyController.EnsureModelAvailability)

				// 模型信息
				r.Get("/boxes/{boxId}/models", dependencyController.GetBoxModels)
				r.Get("/models/{modelKey}/usage", dependencyController.GetModelUsage)
			})
		*/

	} else {
		// 数据库未初始化时的占位路由
		r.Route("/api/v1/tasks", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Task service not initialized", http.StatusServiceUnavailable)
			})
		})
	}

	// 视频管理 (REQ-009: 视频管理功能)
	if db != nil {
		// 使用共享的服务实例

		r.Route("/api/v1/video", func(r chi.Router) {
			// 视频源管理
			r.Route("/sources", func(r chi.Router) {
				videoSourceController := controllers.NewVideoSourceController(videoSourceService)

				r.Post("/", videoSourceController.CreateVideoSource)
				r.Get("/", videoSourceController.GetVideoSources)
				r.Get("/{id}", videoSourceController.GetVideoSource)
				r.Put("/{id}", videoSourceController.UpdateVideoSource)
				r.Delete("/{id}", videoSourceController.DeleteVideoSource)
				r.Post("/{id}/screenshot", videoSourceController.TakeScreenshot)

				// 监控管理
				r.Route("/monitoring", func(r chi.Router) {
					r.Get("/status", videoSourceController.GetMonitoringStatus)
					r.Post("/start", videoSourceController.StartMonitoring)
					r.Post("/stop", videoSourceController.StopMonitoring)
				})
			})

			// 视频文件管理
			r.Route("/files", func(r chi.Router) {
				videoFileController := controllers.NewVideoFileController(videoFileService)

				r.Post("/upload", videoFileController.UploadVideoFile)
				r.Get("/", videoFileController.GetVideoFiles)
				r.Get("/{id}", videoFileController.GetVideoFile)
				r.Put("/{id}", videoFileController.UpdateVideoFile)
				r.Delete("/{id}", videoFileController.DeleteVideoFile)
				r.Get("/{id}/download", videoFileController.DownloadVideoFile)
				r.Post("/{id}/convert", videoFileController.ConvertVideoFile)
			})

			// 抽帧任务管理
			r.Route("/extract-tasks", func(r chi.Router) {
				extractTaskController := controllers.NewExtractTaskController(extractTaskService)

				r.Post("/", extractTaskController.CreateExtractTask)
				r.Get("/", extractTaskController.GetExtractTasks)
				r.Get("/{id}", extractTaskController.GetExtractTask)
				r.Post("/{id}/start", extractTaskController.StartExtractTask)
				r.Post("/{id}/stop", extractTaskController.StopExtractTask)
				r.Get("/{id}/frames", extractTaskController.GetTaskFrames)
				r.Get("/{id}/download", extractTaskController.DownloadTaskImages)
				r.Delete("/{id}", extractTaskController.DeleteExtractTask)
			})

			// 抽帧结果管理
			r.Route("/extract-frames", func(r chi.Router) {
				extractTaskController := controllers.NewExtractTaskController(extractTaskService)

				r.Get("/{frame_id}/info", extractTaskController.GetFrameImageInfo)
				r.Get("/{frame_id}/image", extractTaskController.GetFrameImage)
			})

			// 录制任务管理
			r.Route("/record-tasks", func(r chi.Router) {
				recordTaskController := controllers.NewRecordTaskController(recordTaskService)

				r.Post("/", recordTaskController.CreateRecordTask)
				r.Get("/", recordTaskController.GetRecordTasks)
				r.Get("/statistics", recordTaskController.GetTaskStatistics)
				r.Get("/{id}", recordTaskController.GetRecordTask)
				r.Post("/{id}/start", recordTaskController.StartRecordTask)
				r.Post("/{id}/stop", recordTaskController.StopRecordTask)
				r.Get("/{id}/download", recordTaskController.DownloadRecord)
				r.Delete("/{id}", recordTaskController.DeleteRecordTask)
			})
		})
	} else {
		// 数据库未初始化时的占位路由
		r.Route("/api/v1/video", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Video service not initialized", http.StatusServiceUnavailable)
			})
		})
	}

	// 系统监控
	r.Route("/monitoring", func(r chi.Router) {
		// 创建监控控制器实例，使用共享的服务实例
		var monitoringController *controllers.MonitoringController
		if db != nil {
			monitoringController = controllers.NewMonitoringController(
				monitoringService,      // 使用已启动的监控服务实例
				conversionService,      // 使用共享的转换服务
				recordTaskService,      // 使用共享的录制服务
				videoSourceService,     // 使用共享的视频源服务
				taskExecutorService,    // 使用共享的任务执行器服务
				modelDependencyService, // 使用共享的模型依赖服务
				sseService,             // 使用共享的SSE服务
				systemLogService,       // 使用共享的系统日志服务
			)
		} else {
			// 如果数据库未初始化，创建一个空的控制器
			monitoringController = controllers.NewMonitoringController(nil, nil, nil, nil, nil, nil, nil, nil)
		}

		// 系统概览
		r.Get("/overview", monitoringController.GetSystemOverview)

		// 系统指标
		r.Get("/metrics", monitoringController.GetSystemMetrics)
		r.Get("/performance", monitoringController.GetPerformanceMetrics)
		r.Get("/resource-usage", monitoringController.GetResourceUsage)

		// 分类指标
		r.Get("/boxes/{id}/metrics", monitoringController.GetBoxMetrics)
		r.Get("/tasks/metrics", monitoringController.GetTaskMetrics)

		// 日志管理
		r.Get("/logs", monitoringController.GetSystemLogs)
		r.Get("/logs/statistics", monitoringController.GetLogStatistics)

		// 健康检查
		r.Get("/health-check", monitoringController.HealthCheck)
	})

	// 配置管理
	r.Route("/config", func(r chi.Router) {
		configController := controllers.NewConfigController()

		r.Get("/system", configController.GetSystemConfig)
		r.Put("/system", configController.UpdateSystemConfig)
		r.Get("/deployment", configController.GetDeploymentConfig)
		r.Put("/deployment", configController.UpdateDeploymentConfig)
	})

	return conversionService
}
