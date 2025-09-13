/*
 * @module service/system_log_service
 * @description 系统日志服务，提供统一的日志记录和管理功能
 * @architecture 服务层
 * @documentReference DESIGN-000.md
 * @stateFlow 日志记录 -> 存储 -> 查询 -> 清理
 * @rules 提供统一的日志记录接口，支持自动清理过期日志
 * @dependencies box-manage-service/repository, box-manage-service/models
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

// SystemLogService 系统日志服务接口
type SystemLogService interface {
	// 日志记录方法
	Debug(source, title, message string, opts ...LogOption) error
	Info(source, title, message string, opts ...LogOption) error
	Warn(source, title, message string, opts ...LogOption) error
	Error(source, title, message string, err error, opts ...LogOption) error
	Fatal(source, title, message string, err error, opts ...LogOption) error

	// 便捷方法
	LogServiceError(serviceName, operation string, err error, opts ...LogOption) error
	LogServiceInfo(serviceName, operation, message string, opts ...LogOption) error
	LogServiceWarn(serviceName, operation, message string, opts ...LogOption) error

	// 查询方法
	GetLogs(ctx context.Context, req *GetLogsRequest) (*GetLogsResponse, error)
	GetLogsBySource(ctx context.Context, source string, limit int) ([]*models.SystemLog, error)
	GetLogsByLevel(ctx context.Context, level models.LogLevel, limit int) ([]*models.SystemLog, error)
	GetLogStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error)

	// 清理方法
	CleanupOldLogs(ctx context.Context) (int64, error)
	CleanupLogsByLevel(ctx context.Context, level models.LogLevel, olderThanDays int) (int64, error)

	// 启动清理任务
	StartCleanupScheduler(ctx context.Context)
}

// LogOption 日志选项
type LogOption func(*LogOptions)

// LogOptions 日志选项结构
type LogOptions struct {
	SourceID  string
	UserID    *uint
	RequestID string
	Metadata  map[string]interface{}
	Details   string
}

// WithSourceID 设置来源ID
func WithSourceID(sourceID string) LogOption {
	return func(opts *LogOptions) {
		opts.SourceID = sourceID
	}
}

// WithUserID 设置用户ID
func WithUserID(userID uint) LogOption {
	return func(opts *LogOptions) {
		opts.UserID = &userID
	}
}

// WithRequestID 设置请求ID
func WithRequestID(requestID string) LogOption {
	return func(opts *LogOptions) {
		opts.RequestID = requestID
	}
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]interface{}) LogOption {
	return func(opts *LogOptions) {
		opts.Metadata = metadata
	}
}

// WithDetails 设置详细信息
func WithDetails(details string) LogOption {
	return func(opts *LogOptions) {
		opts.Details = details
	}
}

// GetLogsRequest 获取日志请求
type GetLogsRequest struct {
	Level     *models.LogLevel `json:"level"`
	Source    string           `json:"source"`
	SourceID  string           `json:"source_id"`
	UserID    *uint            `json:"user_id"`
	RequestID string           `json:"request_id"`
	StartTime *time.Time       `json:"start_time"`
	EndTime   *time.Time       `json:"end_time"`
	Keyword   string           `json:"keyword"`
	Page      int              `json:"page"`
	PageSize  int              `json:"page_size"`
}

// GetLogsResponse 获取日志响应
type GetLogsResponse struct {
	Logs     []*models.SystemLog `json:"logs"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// systemLogService 系统日志服务实现
type systemLogService struct {
	logRepo       repository.SystemLogRepository
	cleanupDays   int           // 日志保留天数，默认7天
	cleanupTicker *time.Ticker  // 清理定时器
	cleanupStop   chan struct{} // 停止清理信号
}

// NewSystemLogService 创建系统日志服务实例
func NewSystemLogService(logRepo repository.SystemLogRepository) SystemLogService {
	return &systemLogService{
		logRepo:     logRepo,
		cleanupDays: 7, // 默认保留7天
		cleanupStop: make(chan struct{}),
	}
}

// Debug 记录调试日志
func (s *systemLogService) Debug(source, title, message string, opts ...LogOption) error {
	return s.createLog(models.LogLevelDebug, source, title, message, nil, opts...)
}

// Info 记录信息日志
func (s *systemLogService) Info(source, title, message string, opts ...LogOption) error {
	return s.createLog(models.LogLevelInfo, source, title, message, nil, opts...)
}

// Warn 记录警告日志
func (s *systemLogService) Warn(source, title, message string, opts ...LogOption) error {
	return s.createLog(models.LogLevelWarn, source, title, message, nil, opts...)
}

// Error 记录错误日志
func (s *systemLogService) Error(source, title, message string, err error, opts ...LogOption) error {
	var details string
	if err != nil {
		details = err.Error()
		// 获取调用栈信息
		if stack := s.getStackTrace(); stack != "" {
			details += "\n\nStack Trace:\n" + stack
		}
		message = fmt.Sprintf("%s: %v", message, err)
	}

	// 添加详细信息到选项中
	opts = append(opts, WithDetails(details))

	return s.createLog(models.LogLevelError, source, title, message, nil, opts...)
}

// Fatal 记录致命错误日志
func (s *systemLogService) Fatal(source, title, message string, err error, opts ...LogOption) error {
	var details string
	if err != nil {
		details = err.Error()
		// 获取调用栈信息
		if stack := s.getStackTrace(); stack != "" {
			details += "\n\nStack Trace:\n" + stack
		}
		message = fmt.Sprintf("%s: %v", message, err)
	}

	// 添加详细信息到选项中
	opts = append(opts, WithDetails(details))

	return s.createLog(models.LogLevelFatal, source, title, message, nil, opts...)
}

// LogServiceError 记录服务错误
func (s *systemLogService) LogServiceError(serviceName, operation string, err error, opts ...LogOption) error {
	title := fmt.Sprintf("%s操作失败", operation)
	message := fmt.Sprintf("服务 %s 在执行 %s 时发生错误", serviceName, operation)
	return s.Error(serviceName, title, message, err, opts...)
}

// LogServiceInfo 记录服务信息
func (s *systemLogService) LogServiceInfo(serviceName, operation, message string, opts ...LogOption) error {
	title := fmt.Sprintf("%s操作完成", operation)
	fullMessage := fmt.Sprintf("服务 %s 执行 %s: %s", serviceName, operation, message)
	return s.Info(serviceName, title, fullMessage, opts...)
}

// LogServiceWarn 记录服务警告
func (s *systemLogService) LogServiceWarn(serviceName, operation, message string, opts ...LogOption) error {
	title := fmt.Sprintf("%s操作警告", operation)
	fullMessage := fmt.Sprintf("服务 %s 执行 %s 时出现警告: %s", serviceName, operation, message)
	return s.Warn(serviceName, title, fullMessage, opts...)
}

// createLog 创建日志记录
func (s *systemLogService) createLog(level models.LogLevel, source, title, message string, err error, opts ...LogOption) error {
	// 应用选项
	options := &LogOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// 创建日志记录
	logEntry := &models.SystemLog{
		Level:     level,
		Source:    source,
		SourceID:  options.SourceID,
		Title:     title,
		Message:   message,
		Details:   options.Details,
		UserID:    options.UserID,
		RequestID: options.RequestID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 序列化元数据
	if len(options.Metadata) > 0 {
		if metadataJSON, err := json.Marshal(options.Metadata); err == nil {
			jsonStr := string(metadataJSON)
			logEntry.Metadata = &jsonStr
		} else {
			// 如果序列化失败，记录错误但不阻止日志保存
			log.Printf("[SystemLogService] Failed to marshal metadata: %v", err)
			logEntry.Metadata = nil
		}
	} else {
		// 当 metadata 为 nil 或空时，设置为 nil（数据库中为 NULL）
		logEntry.Metadata = nil
	}

	// 保存到数据库
	ctx := context.Background()
	if err := s.logRepo.Create(ctx, logEntry); err != nil {
		// 如果数据库保存失败，至少输出到标准日志
		log.Printf("[%s] %s - %s: %s", strings.ToUpper(string(level)), source, title, message)
		return fmt.Errorf("保存日志到数据库失败: %w", err)
	}

	return nil
}

// GetLogs 获取日志列表
func (s *systemLogService) GetLogs(ctx context.Context, req *GetLogsRequest) (*GetLogsResponse, error) {
	// 构建过滤条件
	filters := make(map[string]interface{})

	if req.Level != nil {
		filters["level"] = *req.Level
	}
	if req.Source != "" {
		filters["source"] = req.Source
	}
	if req.SourceID != "" {
		filters["source_id"] = req.SourceID
	}
	if req.UserID != nil {
		filters["user_id"] = *req.UserID
	}
	if req.RequestID != "" {
		filters["request_id"] = req.RequestID
	}
	if req.StartTime != nil {
		filters["start_time"] = *req.StartTime
	}
	if req.EndTime != nil {
		filters["end_time"] = *req.EndTime
	}
	if req.Keyword != "" {
		filters["keyword"] = req.Keyword
	}

	// 设置分页参数
	pagination := &repository.PaginationOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
	}
	if pagination.Page <= 0 {
		pagination.Page = 1
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 100
	}

	// 查询日志
	logs, total, err := s.logRepo.FindWithPagination(ctx, filters, pagination)
	if err != nil {
		return nil, fmt.Errorf("查询日志失败: %w", err)
	}

	return &GetLogsResponse{
		Logs:     logs,
		Total:    total,
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
	}, nil
}

// GetLogsBySource 根据来源获取日志
func (s *systemLogService) GetLogsBySource(ctx context.Context, source string, limit int) ([]*models.SystemLog, error) {
	return s.logRepo.FindBySource(ctx, source, limit)
}

// GetLogsByLevel 根据级别获取日志
func (s *systemLogService) GetLogsByLevel(ctx context.Context, level models.LogLevel, limit int) ([]*models.SystemLog, error) {
	return s.logRepo.FindByLevel(ctx, level, limit)
}

// GetLogStatistics 获取日志统计
func (s *systemLogService) GetLogStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error) {
	return s.logRepo.GetStatistics(ctx, startTime, endTime)
}

// CleanupOldLogs 清理过期日志
func (s *systemLogService) CleanupOldLogs(ctx context.Context) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -s.cleanupDays)
	deletedCount, err := s.logRepo.DeleteOlderThan(ctx, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("清理过期日志失败: %w", err)
	}

	log.Printf("[SystemLogService] 清理了 %d 条超过 %d 天的日志", deletedCount, s.cleanupDays)
	return deletedCount, nil
}

// CleanupLogsByLevel 根据级别清理日志
func (s *systemLogService) CleanupLogsByLevel(ctx context.Context, level models.LogLevel, olderThanDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -olderThanDays)
	deletedCount, err := s.logRepo.DeleteByLevel(ctx, level, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("清理 %s 级别日志失败: %w", level, err)
	}

	log.Printf("[SystemLogService] 清理了 %d 条超过 %d 天的 %s 级别日志", deletedCount, olderThanDays, level)
	return deletedCount, nil
}

// StartCleanupScheduler 启动日志清理调度器
func (s *systemLogService) StartCleanupScheduler(ctx context.Context) {
	// 每天凌晨2点执行清理
	s.cleanupTicker = time.NewTicker(24 * time.Hour)

	// 立即执行一次清理
	go func() {
		if _, err := s.CleanupOldLogs(ctx); err != nil {
			log.Printf("[SystemLogService] 启动时清理日志失败: %v", err)
		}
	}()

	// 启动定时清理
	go func() {
		for {
			select {
			case <-s.cleanupTicker.C:
				if _, err := s.CleanupOldLogs(ctx); err != nil {
					log.Printf("[SystemLogService] 定时清理日志失败: %v", err)
				}
			case <-s.cleanupStop:
				s.cleanupTicker.Stop()
				return
			case <-ctx.Done():
				s.cleanupTicker.Stop()
				return
			}
		}
	}()

	log.Println("[SystemLogService] 日志清理调度器已启动")
}

// getStackTrace 获取调用栈信息
func (s *systemLogService) getStackTrace() string {
	const maxStackSize = 4096
	buf := make([]byte, maxStackSize)
	n := runtime.Stack(buf, false)

	// 过滤掉当前函数和日志服务相关的栈帧
	stack := string(buf[:n])
	lines := strings.Split(stack, "\n")

	var filteredLines []string
	skip := true
	for _, line := range lines {
		if skip && (strings.Contains(line, "system_log_service.go") ||
			strings.Contains(line, "runtime.") ||
			strings.Contains(line, "systemLogService")) {
			continue
		}
		skip = false
		filteredLines = append(filteredLines, line)

		// 只保留前几层调用栈
		if len(filteredLines) > 10 {
			break
		}
	}

	return strings.Join(filteredLines, "\n")
}
