/*
 * @module service/box_monitoring
 * @description 高性能盒子监控服务，支持1000+盒子监控，实现定时轮询、状态收集、任务状态监控
 * @architecture 服务层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow 定时轮询 -> 状态收集 -> 任务状态监控 -> 数据存储 -> 离线检测 -> 告警推送
 * @rules 支持高并发轮询、智能批处理、性能监控、故障恢复等功能
 * @dependencies gorm.io/gorm, box-manage-service/client
 * @refs DESIGN-001.md
 */

package service

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	// 并发控制
	MaxConcurrentBoxes int           `json:"max_concurrent_boxes"` // 最大并发盒子数
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"` // 最大并发任务数
	RequestTimeout     time.Duration `json:"request_timeout"`      // 请求超时时间

	// 轮询间隔
	BoxPollingInterval  time.Duration `json:"box_polling_interval"`  // 盒子轮询间隔
	TaskPollingInterval time.Duration `json:"task_polling_interval"` // 任务轮询间隔

	// 批处理配置
	BatchSize    int           `json:"batch_size"`    // 批处理大小
	BatchTimeout time.Duration `json:"batch_timeout"` // 批处理超时

	// 性能配置
	EnableMetrics      bool `json:"enable_metrics"`      // 启用性能指标
	MemoryThreshold    int  `json:"memory_threshold"`    // 内存阈值(MB)
	GoroutineThreshold int  `json:"goroutine_threshold"` // 协程数阈值
}

// DefaultMonitoringConfig 默认监控配置（支持1000盒子）
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		MaxConcurrentBoxes:  100,              // 并发处理100个盒子
		MaxConcurrentTasks:  50,               // 并发处理50个任务状态查询
		RequestTimeout:      10 * time.Second, // 减少超时时间
		BoxPollingInterval:  30 * time.Second, // 盒子状态轮询间隔
		TaskPollingInterval: 60 * time.Second, // 任务状态轮询间隔
		BatchSize:           50,               // 批处理50个
		BatchTimeout:        5 * time.Second,  // 批处理超时
		EnableMetrics:       true,             // 启用性能监控
		MemoryThreshold:     500,              // 500MB内存阈值
		GoroutineThreshold:  1000,             // 1000协程阈值
	}
}

// MonitoringMetrics 监控指标
type MonitoringMetrics struct {
	// 轮询统计
	TotalPolls      int64 `json:"total_polls"`
	SuccessfulPolls int64 `json:"successful_polls"`
	FailedPolls     int64 `json:"failed_polls"`

	// 性能统计
	AverageResponseTime int64 `json:"average_response_time_ms"` // 平均响应时间(毫秒)
	ActiveGoroutines    int   `json:"active_goroutines"`        // 活跃协程数
	MemoryUsageMB       int   `json:"memory_usage_mb"`          // 内存使用(MB)

	// 盒子统计
	OnlineBoxes  int64 `json:"online_boxes"`
	OfflineBoxes int64 `json:"offline_boxes"`
	TotalBoxes   int64 `json:"total_boxes"`

	// 任务统计
	MonitoredTasks int64 `json:"monitored_tasks"`
	RunningTasks   int64 `json:"running_tasks"`
	FailedTasks    int64 `json:"failed_tasks"`

	LastUpdateTime time.Time `json:"last_update_time"`
}

// BoxMonitoringService 高性能盒子监控服务
type BoxMonitoringService struct {
	repoManager repository.RepositoryManager
	config      *MonitoringConfig

	// 定时器
	boxTicker  *time.Ticker
	taskTicker *time.Ticker

	// 控制通道
	stopChan chan struct{}
	wg       sync.WaitGroup

	// 状态管理
	mu      sync.RWMutex
	running bool

	// 性能监控
	metrics      *MonitoringMetrics
	lastPollTime int64 // 原子操作，存储最后轮询时间(unix纳秒)

	// 客户端缓存
	clientCache sync.Map // map[string]*client.BoxClient

	// 工作池
	boxWorkerPool  chan struct{}
	taskWorkerPool chan struct{}

	// 日志服务
	logService SystemLogService
}

// NewBoxMonitoringService 创建高性能盒子监控服务
func NewBoxMonitoringService(repoManager repository.RepositoryManager, config *MonitoringConfig, logService SystemLogService) *BoxMonitoringService {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	service := &BoxMonitoringService{
		repoManager: repoManager,
		config:      config,
		stopChan:    make(chan struct{}),
		metrics:     &MonitoringMetrics{},
		clientCache: sync.Map{},
		logService:  logService,
	}

	// 初始化工作池
	service.boxWorkerPool = make(chan struct{}, config.MaxConcurrentBoxes)
	service.taskWorkerPool = make(chan struct{}, config.MaxConcurrentTasks)

	// 填充工作池
	for i := 0; i < config.MaxConcurrentBoxes; i++ {
		service.boxWorkerPool <- struct{}{}
	}
	for i := 0; i < config.MaxConcurrentTasks; i++ {
		service.taskWorkerPool <- struct{}{}
	}

	log.Printf("[BoxMonitoringService] 高性能监控服务初始化完成 - 最大并发盒子: %d, 最大并发任务: %d",
		config.MaxConcurrentBoxes, config.MaxConcurrentTasks)

	return service
}

// BoxStatusInfo 盒子状态信息
type BoxStatusInfo struct {
	Service    string                 `json:"service"`
	Status     string                 `json:"status"`
	Timestamp  string                 `json:"timestamp"`
	Version    string                 `json:"version"`
	SystemInfo map[string]interface{} `json:"system_info"`
	Hardware   models.Hardware        `json:"hardware"`  // 硬件信息
	Resources  models.Resources       `json:"resources"` // 资源信息
}

// Start 启动高性能监控服务
func (s *BoxMonitoringService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		log.Println("[BoxMonitoringService] 服务已在运行中")
		return
	}

	s.running = true

	// 记录服务启动日志
	if s.logService != nil {
		s.logService.Info("box_monitoring_service", "服务启动",
			fmt.Sprintf("监控服务启动 - 盒子轮询间隔: %v, 任务轮询间隔: %v",
				s.config.BoxPollingInterval, s.config.TaskPollingInterval))
	}

	// 启动盒子状态轮询
	s.boxTicker = time.NewTicker(s.config.BoxPollingInterval)
	s.wg.Add(1)
	go s.boxPollingLoop()

	// 启动任务状态轮询
	s.taskTicker = time.NewTicker(s.config.TaskPollingInterval)
	s.wg.Add(1)
	go s.taskPollingLoop()

	// 启动性能监控
	if s.config.EnableMetrics {
		s.wg.Add(1)
		go s.metricsLoop()
		log.Println("[BoxMonitoringService] 性能监控已启动")
	} else {
		log.Println("[BoxMonitoringService] 性能监控未启用")
	}

	// 立即执行一次指标更新
	go func() {
		log.Println("[BoxMonitoringService] 执行初始指标更新")
		s.updateMetrics()
		metrics := s.GetMetrics()
		log.Printf("[BoxMonitoringService] 初始指标 - TotalBoxes: %d, OnlineBoxes: %d, TotalPolls: %d",
			metrics.TotalBoxes, metrics.OnlineBoxes, metrics.TotalPolls)
	}()

	log.Printf("[BoxMonitoringService] 高性能监控服务已启动 - 盒子轮询间隔: %v, 任务轮询间隔: %v, 启用指标: %t",
		s.config.BoxPollingInterval, s.config.TaskPollingInterval, s.config.EnableMetrics)
}

// Stop 停止监控服务
func (s *BoxMonitoringService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)

	// 停止定时器
	if s.boxTicker != nil {
		s.boxTicker.Stop()
	}
	if s.taskTicker != nil {
		s.taskTicker.Stop()
	}

	// 等待所有协程结束
	s.wg.Wait()

	// 清理客户端缓存
	s.clientCache.Range(func(key, value interface{}) bool {
		s.clientCache.Delete(key)
		return true
	})

	// 记录服务停止日志
	if s.logService != nil {
		s.logService.Info("box_monitoring_service", "服务停止", "监控服务已停止")
	}

	log.Println("[BoxMonitoringService] 高性能监控服务已停止")
}

// boxPollingLoop 盒子状态轮询循环
func (s *BoxMonitoringService) boxPollingLoop() {
	defer s.wg.Done()
	log.Printf("[BoxMonitoringService] 盒子状态轮询循环已启动")

	for {
		select {
		case <-s.boxTicker.C:
			s.pollAllBoxes()
		case <-s.stopChan:
			log.Printf("[BoxMonitoringService] 盒子状态轮询循环已停止")
			return
		}
	}
}

// taskPollingLoop 任务状态轮询循环
func (s *BoxMonitoringService) taskPollingLoop() {
	defer s.wg.Done()
	log.Printf("[BoxMonitoringService] 任务状态轮询循环已启动")

	for {
		select {
		case <-s.taskTicker.C:
			s.pollAllTaskStatus()
		case <-s.stopChan:
			log.Printf("[BoxMonitoringService] 任务状态轮询循环已停止")
			return
		}
	}
}

// metricsLoop 性能监控循环
func (s *BoxMonitoringService) metricsLoop() {
	defer s.wg.Done()
	metricsTicker := time.NewTicker(30 * time.Second) // 每30秒更新一次指标
	defer metricsTicker.Stop()

	log.Printf("[BoxMonitoringService] 性能监控循环已启动")

	for {
		select {
		case <-metricsTicker.C:
			s.updateMetrics()
		case <-s.stopChan:
			log.Printf("[BoxMonitoringService] 性能监控循环已停止")
			return
		}
	}
}

// pollAllBoxes 高性能轮询所有盒子
func (s *BoxMonitoringService) pollAllBoxes() {
	startTime := time.Now()
	atomic.StoreInt64(&s.lastPollTime, startTime.UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), s.config.BatchTimeout)
	defer cancel()

	boxes, err := s.repoManager.Box().Find(ctx, nil)
	if err != nil {
		log.Printf("[BoxMonitoringService] 获取盒子列表失败: %v", err)
		// 记录获取盒子列表失败的错误日志
		if s.logService != nil {
			s.logService.Error("box_monitoring_service", "获取盒子列表", "获取盒子列表失败", err)
		}
		atomic.AddInt64(&s.metrics.FailedPolls, 1)
		return
	}

	if len(boxes) == 0 {
		return
	}

	log.Printf("[BoxMonitoringService] 开始轮询 %d 个盒子", len(boxes))

	// 批量处理盒子
	s.processBatchBoxes(ctx, boxes)

	// 检查离线盒子
	go s.checkOfflineBoxes() // 异步执行

	// 更新指标
	atomic.AddInt64(&s.metrics.TotalPolls, 1)
	atomic.AddInt64(&s.metrics.SuccessfulPolls, 1)

	duration := time.Since(startTime)
	log.Printf("[BoxMonitoringService] 盒子轮询完成，耗时: %v", duration)
}

// processBatchBoxes 批量处理盒子
func (s *BoxMonitoringService) processBatchBoxes(ctx context.Context, boxes []*models.Box) {
	total := len(boxes)
	batchSize := s.config.BatchSize

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := boxes[i:end]
		s.processBatch(ctx, batch)

		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			log.Printf("[BoxMonitoringService] 批量处理被取消")
			return
		default:
		}
	}
}

// processBatch 处理一批盒子
func (s *BoxMonitoringService) processBatch(ctx context.Context, boxes []*models.Box) {
	var wg sync.WaitGroup

	for _, box := range boxes {
		// 获取工作池令牌
		select {
		case <-s.boxWorkerPool:
			wg.Add(1)
			go func(box *models.Box) {
				defer wg.Done()
				defer func() {
					s.boxWorkerPool <- struct{}{} // 归还令牌
				}()

				if err := s.pollBox(ctx, box); err != nil {
					log.Printf("[BoxMonitoringService] 轮询盒子失败 %s (%s:%d): %v",
						box.Name, box.IPAddress, box.Port, err)
				}
			}(box)
		case <-ctx.Done():
			// 上下文取消，不再处理新的盒子
			wg.Wait()
			return
		}
	}

	wg.Wait()
}

// pollBox 使用BoxClient轮询单个盒子（优化版本）
func (s *BoxMonitoringService) pollBox(ctx context.Context, box *models.Box) error {
	startTime := time.Now()

	// 获取或创建BoxClient
	boxClient := s.getOrCreateBoxClient(box)

	// 检查盒子健康状态（使用较短超时）
	healthCtx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()

	if err := boxClient.Health(healthCtx); err != nil {
		return s.markBoxOffline(box, fmt.Errorf("健康检查失败: %w", err))
	}

	// 收集系统元信息、版本信息和系统信息（每次都获取）
	var metaInfo *client.BoxSystemMeta
	var versionInfo *client.BoxVersionResponse
	var systemInfo *client.BoxSystemInfoResponse

	// 为信息获取使用更长的超时时间
	infoCtx, infoCancel := context.WithTimeout(ctx, 20*time.Second)
	defer infoCancel()

	// 获取系统元信息
	if meta, err := boxClient.GetSystemMeta(infoCtx); err != nil {
		log.Printf("[BoxMonitoringService] 获取盒子 %s 元信息失败: %v", box.Name, err)
		// 记录错误日志
		if s.logService != nil {
			s.logService.Error("box_monitoring_service", "获取盒子元信息",
				fmt.Sprintf("盒子[%s]获取元信息失败", box.Name), err,
				WithSourceID(fmt.Sprintf("box_%d", box.ID)))
		}
	} else {
		metaInfo = meta
		log.Printf("[BoxMonitoringService] 成功获取盒子 %s 元信息", box.Name)
	}

	// 获取版本信息
	if version, err := boxClient.GetSystemVersion(infoCtx); err != nil {
		log.Printf("[BoxMonitoringService] 获取盒子 %s 版本信息失败: %v", box.Name, err)
		// 记录错误日志
		if s.logService != nil {
			s.logService.Error("box_monitoring_service", "获取盒子版本信息",
				fmt.Sprintf("盒子[%s]获取版本信息失败", box.Name), err,
				WithSourceID(fmt.Sprintf("box_%d", box.ID)))
		}
	} else {
		versionInfo = version
		log.Printf("[BoxMonitoringService] 成功获取盒子 %s 版本信息: %s", box.Name, version.Version)
	}

	// 获取系统信息
	if sysInfo, err := boxClient.GetSystemInfo(infoCtx); err != nil {
		log.Printf("[BoxMonitoringService] 获取盒子 %s 系统信息失败: %v", box.Name, err)
		// 记录错误日志
		if s.logService != nil {
			s.logService.Error("box_monitoring_service", "获取盒子系统信息",
				fmt.Sprintf("盒子[%s]获取系统信息失败", box.Name), err,
				WithSourceID(fmt.Sprintf("box_%d", box.ID)))
		}
		// 系统信息获取失败不应该阻止其他信息的更新
	} else {
		systemInfo = sysInfo
		log.Printf("[BoxMonitoringService] 成功获取盒子 %s 系统信息，CPU使用率: %.2f%%",
			box.Name, sysInfo.Current.CPUUsedPercent)
	}

	// 更新盒子状态
	box.Status = models.BoxStatusOnline
	box.UpdateHeartbeat()

	// 更新元信息、版本信息和系统信息（如果获取到）
	if metaInfo != nil || versionInfo != nil || systemInfo != nil {
		s.updateBoxWithAllInfo(box, metaInfo, versionInfo, systemInfo)
	}

	// 批量保存（减少数据库操作）
	if err := s.saveBatchBoxData(ctx, box, nil); err != nil {
		log.Printf("[BoxMonitoringService] 保存盒子数据失败 %s: %v", box.Name, err)
		return err
	}

	// 记录响应时间
	responseTime := time.Since(startTime).Milliseconds()
	s.updateResponseTime(responseTime)

	return nil
}

// getOrCreateBoxClient 获取或创建BoxClient（带缓存）
func (s *BoxMonitoringService) getOrCreateBoxClient(box *models.Box) *client.BoxClient {
	clientKey := fmt.Sprintf("%s:%d", box.IPAddress, box.Port)

	if cachedClient, ok := s.clientCache.Load(clientKey); ok {
		return cachedClient.(*client.BoxClient)
	}

	// 创建新的客户端
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
	s.clientCache.Store(clientKey, boxClient)

	return boxClient
}

// updateBoxWithAllInfo 使用元信息、版本信息和系统信息更新盒子
func (s *BoxMonitoringService) updateBoxWithAllInfo(box *models.Box, meta *client.BoxSystemMeta, version *client.BoxVersionResponse, systemInfo *client.BoxSystemInfoResponse) {
	// 更新元信息
	if meta != nil {
		// 更新盒子的Meta信息
		box.Meta.SupportedTypes = meta.SupportedTypes
		box.Meta.SupportedVersions = meta.SupportedVersions
		box.Meta.SupportedHardware = meta.SupportedHardware
		box.Meta.InferenceMapping = meta.InferenceTypeMapping

		// 更新文件限制信息
		box.Meta.FileLimits.ModelFileMaxSize = meta.FileLimits.ModelFileMaxSize
		box.Meta.FileLimits.ImageFileMaxSize = meta.FileLimits.ImageFileMaxSize
		box.Meta.FileLimits.SupportedModelFormats = meta.FileLimits.SupportedModelFormats
		box.Meta.FileLimits.SupportedImageFormats = meta.FileLimits.SupportedImageFormats

		// 更新默认配置
		box.Meta.Defaults.ConfThreshold = meta.Defaults.ConfThreshold
		box.Meta.Defaults.NmsThreshold = meta.Defaults.NMSThreshold
		box.Meta.Defaults.Type = meta.Defaults.Type
		box.Meta.Defaults.Version = meta.Defaults.Version
		box.Meta.Defaults.Hardware = meta.Defaults.Hardware

		log.Printf("[BoxMonitoringService] 更新盒子 %s 的元信息，支持硬件: %v, 支持类型: %v",
			box.Name, meta.SupportedHardware, meta.SupportedTypes)
	}

	// 更新版本信息
	if version != nil {
		// 更新软件版本信息到盒子的Version字段
		box.Version = version.Version
		box.BuildTime = version.BuildTime

		log.Printf("[BoxMonitoringService] 更新盒子 %s 的版本信息，服务: %s, 版本: %s, 构建时间: %s",
			box.Name, version.Service, version.Version, version.BuildTime)
	}

	// 更新系统信息
	if systemInfo != nil {
		// 更新硬件信息
		box.Hardware.CPUCores = systemInfo.Base.CPUCores
		box.Hardware.CPULogicalCores = systemInfo.Base.CPULogicalCores
		box.Hardware.CPUModelName = systemInfo.Base.CPUModelName
		box.Hardware.Hostname = systemInfo.Base.Hostname
		box.Hardware.KernelArch = systemInfo.Base.KernelArch
		box.Hardware.KernelVersion = systemInfo.Base.KernelVersion
		box.Hardware.OS = systemInfo.Base.OS
		box.Hardware.Platform = systemInfo.Base.Platform
		box.Hardware.PlatformFamily = systemInfo.Base.PlatformFamily
		box.Hardware.PlatformVersion = systemInfo.Base.PlatformVersion
		box.Hardware.SDKVersion = systemInfo.Base.SDKVersion
		box.Hardware.SoftwareBuildTime = systemInfo.Base.SoftwareBuildTime
		box.Hardware.SoftwareVersion = systemInfo.Base.SoftwareVersion

		// 更新资源使用情况
		box.Resources.BoardTemperature = systemInfo.Current.BoardTemperature
		box.Resources.CoreTemperature = systemInfo.Current.CoreTemperature
		box.Resources.CPUPercent = systemInfo.Current.CPUPercent
		box.Resources.CPUTotal = systemInfo.Current.CPUTotal
		box.Resources.CPUUsed = systemInfo.Current.CPUUsed
		box.Resources.CPUUsedPercent = systemInfo.Current.CPUUsedPercent
		box.Resources.IOCount = systemInfo.Current.IOCount
		box.Resources.IOReadBytes = systemInfo.Current.IOReadBytes
		box.Resources.IOReadTime = systemInfo.Current.IOReadTime
		box.Resources.IOWriteBytes = systemInfo.Current.IOWriteBytes
		box.Resources.IOWriteTime = systemInfo.Current.IOWriteTime
		box.Resources.Load1 = systemInfo.Current.Load1
		box.Resources.Load5 = systemInfo.Current.Load5
		box.Resources.Load15 = systemInfo.Current.Load15
		box.Resources.LoadUsagePercent = systemInfo.Current.LoadUsagePercent
		box.Resources.MemoryAvailable = systemInfo.Current.MemoryAvailable
		box.Resources.MemoryTotal = systemInfo.Current.MemoryTotal
		box.Resources.MemoryUsed = systemInfo.Current.MemoryUsed
		box.Resources.MemoryUsedPercent = systemInfo.Current.MemoryUsedPercent
		box.Resources.NetBytesRecv = systemInfo.Current.NetBytesRecv
		box.Resources.NetBytesSent = systemInfo.Current.NetBytesSent
		box.Resources.Procs = int64(systemInfo.Current.Procs)
		box.Resources.ShotTime = systemInfo.Current.ShotTime
		box.Resources.SwapMemoryAvailable = systemInfo.Current.SwapMemoryAvailable
		box.Resources.SwapMemoryTotal = systemInfo.Current.SwapMemoryTotal
		box.Resources.SwapMemoryUsed = systemInfo.Current.SwapMemoryUsed
		box.Resources.SwapMemoryUsedPercent = systemInfo.Current.SwapMemoryUsedPercent
		box.Resources.TimeSinceUptime = systemInfo.Current.TimeSinceUptime
		box.Resources.Uptime = systemInfo.Current.Uptime

		// 更新AI加速器资源
		box.Resources.NPUMemoryTotal = int(systemInfo.Current.NPUMemoryTotal)
		box.Resources.NPUMemoryUsed = int(systemInfo.Current.NPUMemoryUsed)
		box.Resources.VPUMemoryTotal = int(systemInfo.Current.VPUMemoryTotal)
		box.Resources.VPUMemoryUsed = int(systemInfo.Current.VPUMemoryUsed)
		box.Resources.VPPMemoryTotal = int(systemInfo.Current.VPPMemoryTotal)
		box.Resources.VPPMemoryUsed = int(systemInfo.Current.VPPMemoryUsed)
		box.Resources.TPUUsed = systemInfo.Current.TPUUsed

		// 转换磁盘信息
		box.Resources.DiskData = make([]models.DiskUsage, len(systemInfo.Current.DiskData))
		for i, disk := range systemInfo.Current.DiskData {
			box.Resources.DiskData[i] = models.DiskUsage{
				Device:            disk.Device,
				Free:              disk.Free,
				INodesFree:        disk.InodesFree,
				INodesTotal:       disk.InodesTotal,
				INodesUsed:        disk.InodesUsed,
				INodesUsedPercent: disk.InodesUsedPercent,
				Path:              disk.Path,
				Total:             disk.Total,
				Type:              disk.Type,
				Used:              disk.Used,
				UsedPercent:       disk.UsedPercent,
			}
		}

		log.Printf("[BoxMonitoringService] 更新盒子 %s 的系统信息，CPU使用率: %.2f%%, 内存使用率: %.2f%%, 温度: %.1f°C",
			box.Name, systemInfo.Current.CPUUsedPercent, systemInfo.Current.MemoryUsedPercent, systemInfo.Current.CoreTemperature)
	}
}

// saveBatchBoxData 批量保存盒子数据
func (s *BoxMonitoringService) saveBatchBoxData(ctx context.Context, box *models.Box, heartbeat *models.BoxHeartbeat) error {
	// 保存盒子状态
	if err := s.repoManager.Box().Update(ctx, box); err != nil {
		return fmt.Errorf("更新盒子状态失败: %w", err)
	}

	// 保存心跳记录（如果提供）
	if heartbeat != nil {
		if err := s.repoManager.BoxHeartbeat().Create(ctx, heartbeat); err != nil {
			log.Printf("[BoxMonitoringService] 保存心跳记录失败: %v", err)
		}
	}

	return nil
}

// updateResponseTime 更新平均响应时间
func (s *BoxMonitoringService) updateResponseTime(responseTime int64) {
	// 使用简单的移动平均算法
	oldAvg := atomic.LoadInt64(&s.metrics.AverageResponseTime)
	newAvg := (oldAvg + responseTime) / 2
	atomic.StoreInt64(&s.metrics.AverageResponseTime, newAvg)
}

// pollAllTaskStatus 轮询所有运行中任务的状态
func (s *BoxMonitoringService) pollAllTaskStatus() {
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.BatchTimeout)
	defer cancel()

	// 获取所有运行中的任务
	runningTasks, err := s.repoManager.Task().FindByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		log.Printf("[BoxMonitoringService] 获取运行中任务失败: %v", err)
		return
	}

	if len(runningTasks) == 0 {
		return
	}

	log.Printf("[BoxMonitoringService] 开始监控 %d 个运行中任务的状态", len(runningTasks))

	// 批量处理任务
	s.processBatchTasks(ctx, runningTasks)

	duration := time.Since(startTime)
	log.Printf("[BoxMonitoringService] 任务状态监控完成，耗时: %v", duration)
}

// processBatchTasks 批量处理任务状态监控
func (s *BoxMonitoringService) processBatchTasks(ctx context.Context, tasks []*models.Task) {
	total := len(tasks)
	batchSize := s.config.BatchSize

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}

		batch := tasks[i:end]
		s.processTaskBatch(ctx, batch)

		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			log.Printf("[BoxMonitoringService] 任务批量处理被取消")
			return
		default:
		}
	}
}

// processTaskBatch 处理一批任务
func (s *BoxMonitoringService) processTaskBatch(ctx context.Context, tasks []*models.Task) {
	var wg sync.WaitGroup

	for _, task := range tasks {
		// 跳过没有分配到盒子的任务
		if task.BoxID == nil {
			continue
		}

		// 获取工作池令牌
		select {
		case <-s.taskWorkerPool:
			wg.Add(1)
			go func(task *models.Task) {
				defer wg.Done()
				defer func() {
					s.taskWorkerPool <- struct{}{} // 归还令牌
				}()

				if err := s.pollTaskStatus(ctx, task); err != nil {
					log.Printf("[BoxMonitoringService] 监控任务状态失败 TaskID: %s, Error: %v",
						task.TaskID, err)
				}
			}(task)
		case <-ctx.Done():
			// 上下文取消，不再处理新的任务
			wg.Wait()
			return
		}
	}

	wg.Wait()
}

// pollTaskStatus 轮询单个任务状态
func (s *BoxMonitoringService) pollTaskStatus(ctx context.Context, task *models.Task) error {
	// 获取盒子信息
	box, err := s.repoManager.Box().GetByID(ctx, *task.BoxID)
	if err != nil {
		return fmt.Errorf("获取任务盒子信息失败: %w", err)
	}

	// 获取BoxClient
	boxClient := s.getOrCreateBoxClient(box)

	// 获取任务状态
	taskCtx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()

	boxTaskStatus, err := boxClient.GetTaskStatus(taskCtx, task.TaskID)
	if err != nil {
		// 如果获取状态失败，可能任务已经停止或盒子离线
		log.Printf("[BoxMonitoringService] 获取任务状态失败 TaskID: %s, BoxID: %d, Error: %v",
			task.TaskID, *task.BoxID, err)

		// 记录任务状态获取失败的错误日志
		if s.logService != nil {
			s.logService.Error("box_monitoring_service", "获取任务状态",
				fmt.Sprintf("任务[%s]获取状态失败", task.TaskID), err,
				WithSourceID(fmt.Sprintf("task_%s", task.TaskID)))
		}

		// 检查是否需要更新任务状态为失败
		if s.shouldMarkTaskFailed(err) {
			task.Fail(fmt.Sprintf("无法获取任务状态: %v", err))
			s.repoManager.Task().Update(ctx, task)

			// 记录任务标记为失败的日志
			if s.logService != nil {
				s.logService.Warn("box_monitoring_service", "任务状态更新",
					fmt.Sprintf("任务[%s]已标记为失败", task.TaskID),
					WithSourceID(fmt.Sprintf("task_%s", task.TaskID)))
			}
		}

		return err
	}

	// 更新任务状态和统计信息
	s.updateTaskFromBoxStatus(task, boxTaskStatus)

	// 保存任务更新
	if err := s.repoManager.Task().Update(ctx, task); err != nil {
		log.Printf("[BoxMonitoringService] 保存任务状态失败 TaskID: %s, Error: %v", task.TaskID, err)
		return err
	}

	return nil
}

// shouldMarkTaskFailed 判断是否应该标记任务为失败
func (s *BoxMonitoringService) shouldMarkTaskFailed(err error) bool {
	// 这里可以根据错误类型判断
	// 比如网络错误可能是临时的，不应该立即标记失败
	// 而任务不存在的错误应该标记为失败
	errorStr := err.Error()

	// 任务不存在
	if strings.Contains(errorStr, "not found") || strings.Contains(errorStr, "404") {
		return true
	}

	// 其他情况暂时不标记为失败，给任务一些恢复时间
	return false
}

// updateTaskFromBoxStatus 根据盒子返回的状态更新任务
func (s *BoxMonitoringService) updateTaskFromBoxStatus(task *models.Task, boxStatus *client.BoxTaskStatusResponse) {
	// 更新任务状态
	switch boxStatus.Status {
	case "running":
		task.Status = models.TaskStatusRunning
	case "completed":
		task.Status = models.TaskStatusCompleted
		now := time.Now()
		task.StopTime = &now
	case "failed", "error":
		task.Status = models.TaskStatusFailed
		task.LastError = boxStatus.Error
		now := time.Now()
		task.StopTime = &now
	case "paused":
		task.Status = models.TaskStatusPaused
	}

	// 更新进度
	task.UpdateProgress(boxStatus.Progress)

	// 更新统计信息
	if boxStatus.Statistics != nil {
		task.UpdateStats(
			boxStatus.Statistics.TotalFrames,
			boxStatus.Statistics.InferenceCount,
			boxStatus.Statistics.ForwardSuccess,
			boxStatus.Statistics.ForwardFailed,
		)
	}

	// 更新心跳
	task.UpdateHeartbeat()
}

// updateMetrics 更新性能指标
func (s *BoxMonitoringService) updateMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 更新内存使用情况
	s.metrics.MemoryUsageMB = int(memStats.Alloc / 1024 / 1024)

	// 更新协程数量
	s.metrics.ActiveGoroutines = runtime.NumGoroutine()

	// 更新盒子统计
	ctx := context.Background()
	s.updateBoxMetrics(ctx)

	// 更新任务统计
	s.updateTaskMetrics(ctx)

	// 更新时间戳
	s.metrics.LastUpdateTime = time.Now()

	// 检查性能阈值
	s.checkPerformanceThresholds()
}

// updateBoxMetrics 更新盒子相关指标
func (s *BoxMonitoringService) updateBoxMetrics(ctx context.Context) {
	boxes, err := s.repoManager.Box().Find(ctx, nil)
	if err != nil {
		log.Printf("[BoxMonitoringService] 更新盒子指标失败: %v", err)
		// 记录错误日志
		if s.logService != nil {
			s.logService.Error("box_monitoring_service", "更新盒子指标失败", "无法从数据库获取盒子列表", err)
		}
		return
	}

	log.Printf("[BoxMonitoringService] 从数据库获取到 %d 个盒子", len(boxes))

	var online, offline int64
	for _, box := range boxes {
		if box.Status == models.BoxStatusOnline {
			online++
		} else {
			offline++
		}
		log.Printf("[BoxMonitoringService] 盒子 %s (ID: %d) 状态: %s", box.Name, box.ID, box.Status)
	}

	atomic.StoreInt64(&s.metrics.OnlineBoxes, online)
	atomic.StoreInt64(&s.metrics.OfflineBoxes, offline)
	atomic.StoreInt64(&s.metrics.TotalBoxes, int64(len(boxes)))

	log.Printf("[BoxMonitoringService] 更新盒子指标完成 - 总数: %d, 在线: %d, 离线: %d", len(boxes), online, offline)
}

// updateTaskMetrics 更新任务相关指标
func (s *BoxMonitoringService) updateTaskMetrics(ctx context.Context) {
	// 获取运行中的任务数量
	runningTasks, err := s.repoManager.Task().FindByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		log.Printf("[BoxMonitoringService] 获取运行中任务失败: %v", err)
		return
	}

	// 获取失败的任务数量
	failedTasks, err := s.repoManager.Task().FindByStatus(ctx, models.TaskStatusFailed)
	if err != nil {
		log.Printf("[BoxMonitoringService] 获取失败任务失败: %v", err)
		return
	}

	atomic.StoreInt64(&s.metrics.MonitoredTasks, int64(len(runningTasks)))
	atomic.StoreInt64(&s.metrics.RunningTasks, int64(len(runningTasks)))
	atomic.StoreInt64(&s.metrics.FailedTasks, int64(len(failedTasks)))
}

// checkPerformanceThresholds 检查性能阈值
func (s *BoxMonitoringService) checkPerformanceThresholds() {
	// 检查内存使用
	if s.metrics.MemoryUsageMB > s.config.MemoryThreshold {
		log.Printf("[BoxMonitoringService] 警告: 内存使用超过阈值 %dMB > %dMB",
			s.metrics.MemoryUsageMB, s.config.MemoryThreshold)

		// 触发垃圾回收
		runtime.GC()
		log.Printf("[BoxMonitoringService] 已触发垃圾回收")
	}

	// 检查协程数量
	if s.metrics.ActiveGoroutines > s.config.GoroutineThreshold {
		log.Printf("[BoxMonitoringService] 警告: 协程数量超过阈值 %d > %d",
			s.metrics.ActiveGoroutines, s.config.GoroutineThreshold)
	}
}

// GetMetrics 获取当前性能指标
func (s *BoxMonitoringService) GetMetrics() *MonitoringMetrics {
	// 返回指标的副本，避免并发问题
	return &MonitoringMetrics{
		TotalPolls:          atomic.LoadInt64(&s.metrics.TotalPolls),
		SuccessfulPolls:     atomic.LoadInt64(&s.metrics.SuccessfulPolls),
		FailedPolls:         atomic.LoadInt64(&s.metrics.FailedPolls),
		AverageResponseTime: atomic.LoadInt64(&s.metrics.AverageResponseTime),
		ActiveGoroutines:    s.metrics.ActiveGoroutines,
		MemoryUsageMB:       s.metrics.MemoryUsageMB,
		OnlineBoxes:         atomic.LoadInt64(&s.metrics.OnlineBoxes),
		OfflineBoxes:        atomic.LoadInt64(&s.metrics.OfflineBoxes),
		TotalBoxes:          atomic.LoadInt64(&s.metrics.TotalBoxes),
		MonitoredTasks:      atomic.LoadInt64(&s.metrics.MonitoredTasks),
		RunningTasks:        atomic.LoadInt64(&s.metrics.RunningTasks),
		FailedTasks:         atomic.LoadInt64(&s.metrics.FailedTasks),
		LastUpdateTime:      s.metrics.LastUpdateTime,
	}
}

// checkOfflineBoxes 检查离线的盒子（异步优化版本）
func (s *BoxMonitoringService) checkOfflineBoxes() {
	ctx := context.Background()
	threshold := time.Now().Add(-5 * time.Minute)

	// 构建查询条件
	filter := map[string]interface{}{
		"status": models.BoxStatusOnline,
	}

	boxes, err := s.repoManager.Box().Find(ctx, filter)
	if err != nil {
		log.Printf("[BoxMonitoringService] 检查离线盒子失败: %v", err)
		return
	}

	// 过滤出超时的盒子
	var timeoutBoxes []*models.Box
	for _, box := range boxes {
		if box.LastHeartbeat == nil || box.LastHeartbeat.Before(threshold) {
			timeoutBoxes = append(timeoutBoxes, box)
		}
	}

	if len(timeoutBoxes) == 0 {
		return
	}

	log.Printf("[BoxMonitoringService] 发现 %d 个超时盒子", len(timeoutBoxes))

	// 批量更新离线状态
	for _, box := range timeoutBoxes {
		box.Status = models.BoxStatusOffline
		if err := s.repoManager.Box().Update(ctx, box); err != nil {
			log.Printf("[BoxMonitoringService] 标记盒子 %s 为离线失败: %v", box.Name, err)
		} else {
			log.Printf("[BoxMonitoringService] 盒子 %s 因超时已自动标记为离线", box.Name)
		}
	}
}

// markBoxOffline 标记盒子为离线（保持向后兼容）
func (s *BoxMonitoringService) markBoxOffline(box *models.Box, err error) error {
	box.Status = models.BoxStatusOffline

	// 保存到数据库
	ctx := context.Background()
	if dbErr := s.repoManager.Box().Update(ctx, box); dbErr != nil {
		return fmt.Errorf("标记盒子离线失败: %w", dbErr)
	}

	log.Printf("[BoxMonitoringService] 盒子 %s (%s:%d) 已标记为离线: %v",
		box.Name, box.IPAddress, box.Port, err)
	return err
}

// RefreshBoxStatus 手动刷新盒子状态（保持向后兼容）
func (s *BoxMonitoringService) RefreshBoxStatus(boxID uint) error {
	ctx := context.Background()
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return fmt.Errorf("查找盒子失败: %w", err)
	}

	return s.pollBox(ctx, box)
}

// GetBoxStatusHistory 获取盒子状态历史（保持向后兼容）
func (s *BoxMonitoringService) GetBoxStatusHistory(boxID uint, startTime, endTime time.Time, limit int) ([]*models.BoxHeartbeat, error) {
	ctx := context.Background()

	heartbeats, err := s.repoManager.BoxHeartbeat().FindByBoxID(ctx, boxID, 1000)
	if err != nil {
		return nil, err
	}

	// 时间过滤
	var filteredHeartbeats []*models.BoxHeartbeat
	for _, heartbeat := range heartbeats {
		if !startTime.IsZero() && heartbeat.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && heartbeat.Timestamp.After(endTime) {
			continue
		}
		filteredHeartbeats = append(filteredHeartbeats, heartbeat)
	}

	// 限制数量
	if limit > 0 && len(filteredHeartbeats) > limit {
		filteredHeartbeats = filteredHeartbeats[:limit]
	}

	return filteredHeartbeats, nil
}

// GetSystemOverview 获取系统概览（增强版本）
func (s *BoxMonitoringService) GetSystemOverview() (map[string]interface{}, error) {
	// 直接使用缓存的指标
	metrics := s.GetMetrics()

	// 添加调试日志
	log.Printf("[BoxMonitoringService] GetSystemOverview - IsRunning: %t, TotalBoxes: %d, OnlineBoxes: %d, TotalPolls: %d",
		s.IsRunning(), metrics.TotalBoxes, metrics.OnlineBoxes, metrics.TotalPolls)

	// 如果指标为空，尝试立即更新一次
	if metrics.TotalBoxes == 0 && metrics.TotalPolls == 0 {
		log.Printf("[BoxMonitoringService] 指标为空，尝试立即更新")
		s.updateMetrics()
		metrics = s.GetMetrics()
		log.Printf("[BoxMonitoringService] 更新后指标 - TotalBoxes: %d, OnlineBoxes: %d", metrics.TotalBoxes, metrics.OnlineBoxes)
	}

	overview := map[string]interface{}{
		"boxes": map[string]interface{}{
			"total":   metrics.TotalBoxes,
			"online":  metrics.OnlineBoxes,
			"offline": metrics.OfflineBoxes,
		},
		"tasks": map[string]interface{}{
			"monitored": metrics.MonitoredTasks,
			"running":   metrics.RunningTasks,
			"failed":    metrics.FailedTasks,
		},
		"performance": map[string]interface{}{
			"memory_usage_mb":       metrics.MemoryUsageMB,
			"active_goroutines":     metrics.ActiveGoroutines,
			"average_response_time": metrics.AverageResponseTime,
			"total_polls":           metrics.TotalPolls,
			"successful_polls":      metrics.SuccessfulPolls,
			"failed_polls":          metrics.FailedPolls,
		},
		"timestamp": metrics.LastUpdateTime.Format(time.RFC3339),
	}

	return overview, nil
}

// IsRunning 检查监控服务是否正在运行
func (s *BoxMonitoringService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetConfig 获取当前配置
func (s *BoxMonitoringService) GetConfig() *MonitoringConfig {
	return s.config
}

// UpdateConfig 更新配置（需要重启服务才能生效）
func (s *BoxMonitoringService) UpdateConfig(config *MonitoringConfig) error {
	if s.IsRunning() {
		return fmt.Errorf("无法在服务运行时更新配置，请先停止服务")
	}

	s.config = config

	// 重新初始化工作池
	s.boxWorkerPool = make(chan struct{}, config.MaxConcurrentBoxes)
	s.taskWorkerPool = make(chan struct{}, config.MaxConcurrentTasks)

	// 填充工作池
	for i := 0; i < config.MaxConcurrentBoxes; i++ {
		s.boxWorkerPool <- struct{}{}
	}
	for i := 0; i < config.MaxConcurrentTasks; i++ {
		s.taskWorkerPool <- struct{}{}
	}

	log.Printf("[BoxMonitoringService] 配置已更新 - 最大并发盒子: %d, 最大并发任务: %d",
		config.MaxConcurrentBoxes, config.MaxConcurrentTasks)

	return nil
}
