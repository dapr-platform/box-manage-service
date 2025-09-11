/*
 * @module config/config
 * @description 统一配置管理，从环境变量加载配置项
 * @architecture 配置层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow 环境变量 -> 配置加载 -> 服务初始化
 * @rules 所有配置使用环境变量，提供默认值，启动时检查和验证
 * @dependencies os, time, strconv
 * @refs DESIGN-003.md, DESIGN-004.md
 */

package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 应用程序统一配置
type Config struct {
	// 应用配置
	App AppConfig `json:"app"`
	// 数据库配置
	Database DatabaseConfig `json:"database"`
	// 模型服务配置
	Model ModelConfig `json:"model"`
	// 转换服务配置
	Conversion ConversionConfig `json:"conversion"`
	// 视频管理配置 - REQ-009
	Video VideoConfig `json:"video"`
	// 服务器配置
	Server ServerConfig `json:"server"`
	// 日志配置
	Log LogConfig `json:"log"`
}

// AppConfig 应用基础配置
type AppConfig struct {
	Name        string `json:"name"`        // 应用名称
	Version     string `json:"version"`     // 应用版本
	Environment string `json:"environment"` // 运行环境 (dev/test/prod)
	Debug       bool   `json:"debug"`       // 调试模式
}

// ModelConfig 模型服务配置
type ModelConfig struct {
	// 存储配置
	StorageBasePath string `json:"storage_base_path"` // 存储基础路径
	TempPath        string `json:"temp_path"`         // 临时文件路径

	// 上传配置
	MaxFileSize          int64         `json:"max_file_size"`          // 最大文件大小 (字节)
	ChunkSize            int64         `json:"chunk_size"`             // 分片大小 (字节)
	SessionTimeout       time.Duration `json:"session_timeout"`        // 会话超时时间
	MaxConcurrentUploads int           `json:"max_concurrent_uploads"` // 最大并发上传数

	// 文件类型配置
	AllowedTypes []string `json:"allowed_types"` // 允许的文件类型

	// 安全配置
	EnableChecksum    bool `json:"enable_checksum"`    // 启用校验和验证
	EnableCompression bool `json:"enable_compression"` // 启用压缩
	EnableVirusScan   bool `json:"enable_virus_scan"`  // 启用病毒扫描

	// 存储策略配置
	HotStorageThreshold  int64 `json:"hot_storage_threshold"`  // 热存储阈值 (字节)
	WarmStorageThreshold int64 `json:"warm_storage_threshold"` // 温存储阈值 (字节)

	// 清理配置
	CleanupInterval        time.Duration `json:"cleanup_interval"`          // 清理间隔
	SessionRetentionDays   int           `json:"session_retention_days"`    // 会话保留天数
	TempFileRetentionHours int           `json:"temp_file_retention_hours"` // 临时文件保留小时数
}

// ConversionConfig 模型转换服务配置
type ConversionConfig struct {
	// 任务配置
	MaxRetries              int           `json:"max_retries"`                // 最大重试次数
	TaskRetentionTime       time.Duration `json:"task_retention_time"`        // 任务保留时间
	FailedTaskRetentionTime time.Duration `json:"failed_task_retention_time"` // 失败任务保留时间
	MaxConcurrentTasks      int           `json:"max_concurrent_tasks"`       // 最大并发任务数

	// 路径配置
	LogPath    string `json:"log_path"`    // 日志文件路径
	OutputPath string `json:"output_path"` // 输出文件路径

	// Docker配置
	DockerHost      string        `json:"docker_host"`       // Docker主机地址
	DockerVersion   string        `json:"docker_version"`    // Docker API版本
	DockerTimeout   time.Duration `json:"docker_timeout"`    // Docker操作超时
	SophonImageRepo string        `json:"sophon_image_repo"` // Sophon镜像仓库
	SophonImageTag  string        `json:"sophon_image_tag"`  // Sophon镜像标签

	// 资源限制
	CPULimit    int64 `json:"cpu_limit"`    // CPU限制(核心数)
	MemoryLimit int64 `json:"memory_limit"` // 内存限制(MB)
	GPULimit    int   `json:"gpu_limit"`    // GPU限制(数量)

	// 性能配置
	QueueBuffer         int           `json:"queue_buffer"`          // 队列缓冲区大小
	WorkerCount         int           `json:"worker_count"`          // 工作节点数量
	HealthCheckInterval time.Duration `json:"health_check_interval"` // 健康检查间隔
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `json:"host"`          // 监听地址
	Port         int           `json:"port"`          // 监听端口
	ReadTimeout  time.Duration `json:"read_timeout"`  // 读取超时
	WriteTimeout time.Duration `json:"write_timeout"` // 写入超时
	IdleTimeout  time.Duration `json:"idle_timeout"`  // 空闲超时

	// CORS配置
	AllowedOrigins []string `json:"allowed_origins"` // 允许的源
	AllowedMethods []string `json:"allowed_methods"` // 允许的方法
	AllowedHeaders []string `json:"allowed_headers"` // 允许的头部
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `json:"level"`       // 日志级别 (debug/info/warn/error)
	Format     string `json:"format"`      // 日志格式 (json/text)
	Output     string `json:"output"`      // 输出目标 (stdout/file)
	FilePath   string `json:"file_path"`   // 日志文件路径
	MaxSize    int    `json:"max_size"`    // 最大文件大小 (MB)
	MaxBackups int    `json:"max_backups"` // 最大备份数
	MaxAge     int    `json:"max_age"`     // 最大保留天数
}

// VideoConfig 视频管理配置
type VideoConfig struct {
	ZLMediaKit struct {
		Host         string        `json:"host"`          // ZLMediaKit服务器地址
		Port         int           `json:"port"`          // ZLMediaKit端口
		Secret       string        `json:"secret"`        // API密钥
		Timeout      time.Duration `json:"timeout"`       // 请求超时时间
		DefaultApp   string        `json:"default_app"`   // 默认应用名
		DefaultVhost string        `json:"default_vhost"` // 默认虚拟主机
		RTSPPrefix   string        `json:"rtsp_prefix"`   // RTSP访问前缀
	} `json:"zlmediakit"`

	FFmpeg struct {
		FFmpegPath  string        `json:"ffmpeg_path"`  // FFmpeg可执行文件路径
		FFprobePath string        `json:"ffprobe_path"` // FFprobe可执行文件路径
		Timeout     time.Duration `json:"timeout"`      // 处理超时时间
		Quality     int           `json:"quality"`      // 默认质量参数
		WorkerCount int           `json:"worker_count"` // 并发处理数量
	} `json:"ffmpeg"`

	Storage struct {
		BasePath      string `json:"base_path"`      // 存储基础路径
		FramePath     string `json:"frame_path"`     // 帧存储路径
		RecordPath    string `json:"record_path"`    // 录制文件路径
		TempPath      string `json:"temp_path"`      // 临时文件路径
		ConvertedPath string `json:"converted_path"` // 转换后文件路径
	} `json:"storage"`

	Extract struct {
		DefaultFrameCount int    `json:"default_frame_count"` // 默认提取帧数
		DefaultQuality    int    `json:"default_quality"`     // 默认图片质量
		MaxFrameCount     int    `json:"max_frame_count"`     // 最大帧数限制
		DefaultFormat     string `json:"default_format"`      // 默认图片格式
	} `json:"extract"`

	Record struct {
		DefaultDuration int    `json:"default_duration"` // 默认录制时长（秒）
		MaxDuration     int    `json:"max_duration"`     // 最大录制时长（秒）
		DefaultFormat   string `json:"default_format"`   // 默认录制格式
		DefaultCodec    string `json:"default_codec"`    // 默认编码格式
	} `json:"record"`

	// 播放URL配置
	PlayURL struct {
		StreamPrefix string `json:"stream_prefix"` // 流播放URL前缀
		FilePrefix   string `json:"file_prefix"`   // 文件播放URL前缀
		RoutePrefix  string `json:"route_prefix"`  // 路由前缀，录制时需要去除
	} `json:"play_url"`
}

// LoadConfig 从环境变量加载配置
func LoadConfig() (*Config, error) {
	config := &Config{
		App:        loadAppConfig(),
		Database:   *LoadDatabaseConfig(), // 使用现有的数据库配置
		Model:      loadModelConfig(),
		Conversion: loadConversionConfig(),
		Video:      loadVideoConfig(), // REQ-009: 加载视频管理配置
		Server:     loadServerConfig(),
		Log:        loadLogConfig(),
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// 创建必要的目录
	if err := config.CreateDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	log.Printf("Configuration loaded successfully for environment: %s", config.App.Environment)
	return config, nil
}

// loadAppConfig 加载应用配置
func loadAppConfig() AppConfig {
	return AppConfig{
		Name:        getEnv("APP_NAME", "box-manage-service"),
		Version:     getEnv("APP_VERSION", "1.0.0"),
		Environment: getEnv("APP_ENV", "dev"),
		Debug:       getEnvAsBool("APP_DEBUG", true),
	}
}

// loadModelConfig 加载模型服务配置
func loadModelConfig() ModelConfig {
	return ModelConfig{
		// 存储配置
		StorageBasePath: getEnv("MODEL_STORAGE_BASE_PATH", "./data/models"),
		TempPath:        getEnv("MODEL_TEMP_PATH", "./temp/uploads"),

		// 上传配置
		MaxFileSize:          getEnvAsInt64("MODEL_MAX_FILE_SIZE", 10*1024*1024*1024), // 10GB
		ChunkSize:            getEnvAsInt64("MODEL_CHUNK_SIZE", 5*1024*1024),          // 5MB
		SessionTimeout:       getEnvAsDuration("MODEL_SESSION_TIMEOUT", "24h"),
		MaxConcurrentUploads: getEnvAsInt("MODEL_MAX_CONCURRENT_UPLOADS", 10),

		// 文件类型配置
		AllowedTypes: getEnvAsStringSlice("MODEL_ALLOWED_TYPES", []string{".pt", ".onnx"}),

		// 安全配置
		EnableChecksum:    getEnvAsBool("MODEL_ENABLE_CHECKSUM", true),
		EnableCompression: getEnvAsBool("MODEL_ENABLE_COMPRESSION", false),
		EnableVirusScan:   getEnvAsBool("MODEL_ENABLE_VIRUS_SCAN", false),

		// 存储策略配置
		HotStorageThreshold:  getEnvAsInt64("MODEL_HOT_STORAGE_THRESHOLD", 100*1024*1024),   // 100MB
		WarmStorageThreshold: getEnvAsInt64("MODEL_WARM_STORAGE_THRESHOLD", 1024*1024*1024), // 1GB

		// 清理配置
		CleanupInterval:        getEnvAsDuration("MODEL_CLEANUP_INTERVAL", "1h"),
		SessionRetentionDays:   getEnvAsInt("MODEL_SESSION_RETENTION_DAYS", 7),
		TempFileRetentionHours: getEnvAsInt("MODEL_TEMP_FILE_RETENTION_HOURS", 24),
	}
}

// loadConversionConfig 加载转换服务配置
func loadConversionConfig() ConversionConfig {
	return ConversionConfig{
		// 任务配置
		MaxRetries:              getEnvAsInt("CONVERSION_MAX_RETRIES", 3),
		TaskRetentionTime:       getEnvAsDuration("CONVERSION_TASK_RETENTION_TIME", "7d"),
		FailedTaskRetentionTime: getEnvAsDuration("CONVERSION_FAILED_TASK_RETENTION_TIME", "3d"),
		MaxConcurrentTasks:      getEnvAsInt("CONVERSION_MAX_CONCURRENT_TASKS", 3),

		// 路径配置
		LogPath:    getEnv("CONVERSION_LOG_PATH", "./data/logs/conversion"),
		OutputPath: getEnv("CONVERSION_OUTPUT_PATH", "./data/models/converted"),

		// Docker配置
		DockerHost:      getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		DockerVersion:   getEnv("DOCKER_VERSION", "1.41"),
		DockerTimeout:   getEnvAsDuration("DOCKER_TIMEOUT", "30s"),
		SophonImageRepo: getEnv("SOPHON_IMAGE_REPO", "sophon/model-converter"),
		SophonImageTag:  getEnv("SOPHON_IMAGE_TAG", "latest"),

		// 资源限制
		CPULimit:    getEnvAsInt64("CONVERSION_CPU_LIMIT", 2),       // 2核心
		MemoryLimit: getEnvAsInt64("CONVERSION_MEMORY_LIMIT", 4096), // 4GB
		GPULimit:    getEnvAsInt("CONVERSION_GPU_LIMIT", 1),         // 1个GPU

		// 性能配置
		QueueBuffer:         getEnvAsInt("CONVERSION_QUEUE_BUFFER", 100),
		WorkerCount:         getEnvAsInt("CONVERSION_WORKER_COUNT", 2),
		HealthCheckInterval: getEnvAsDuration("CONVERSION_HEALTH_CHECK_INTERVAL", "30s"),
	}
}

// loadServerConfig 加载服务器配置
func loadServerConfig() ServerConfig {
	return ServerConfig{
		Host:         getEnv("SERVER_HOST", "0.0.0.0"),
		Port:         getEnvAsInt("SERVER_PORT", 80),
		ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", "30s"),
		WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", "30s"),
		IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", "120s"),

		// CORS配置
		AllowedOrigins: getEnvAsStringSlice("SERVER_ALLOWED_ORIGINS", []string{"*"}),
		AllowedMethods: getEnvAsStringSlice("SERVER_ALLOWED_METHODS",
			[]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		AllowedHeaders: getEnvAsStringSlice("SERVER_ALLOWED_HEADERS",
			[]string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"}),
	}
}

// loadLogConfig 加载日志配置
func loadLogConfig() LogConfig {
	return LogConfig{
		Level:      getEnv("LOG_LEVEL", "info"),
		Format:     getEnv("LOG_FORMAT", "json"),
		Output:     getEnv("LOG_OUTPUT", "stdout"),
		FilePath:   getEnv("LOG_FILE_PATH", "./logs/app.log"),
		MaxSize:    getEnvAsInt("LOG_MAX_SIZE", 100), // 100MB
		MaxBackups: getEnvAsInt("LOG_MAX_BACKUPS", 3),
		MaxAge:     getEnvAsInt("LOG_MAX_AGE", 7), // 7天
	}
}

// loadVideoConfig 加载视频管理配置
func loadVideoConfig() VideoConfig {
	config := VideoConfig{}

	// ZLMediaKit配置
	config.ZLMediaKit.Host = getEnv("VIDEO_ZLMEDIAKIT_HOST", "zlmediakit")
	config.ZLMediaKit.Port = getEnvAsInt("VIDEO_ZLMEDIAKIT_PORT", 80)
	config.ZLMediaKit.Secret = getEnv("VIDEO_ZLMEDIAKIT_SECRET", "l7NUsKO0eViYfwkAeXCCBAkBT5jbEOzw")
	config.ZLMediaKit.Timeout = time.Duration(getEnvAsInt("VIDEO_ZLMEDIAKIT_TIMEOUT", 30)) * time.Second
	config.ZLMediaKit.DefaultApp = getEnv("VIDEO_ZLMEDIAKIT_DEFAULT_APP", "live")
	config.ZLMediaKit.DefaultVhost = getEnv("VIDEO_ZLMEDIAKIT_DEFAULT_VHOST", "__defaultVhost__")
	config.ZLMediaKit.RTSPPrefix = getEnv("VIDEO_ZLMEDIAKIT_RTSP_PREFIX", "rtsp://zlmediakit:554")

	// FFmpeg配置
	config.FFmpeg.FFmpegPath = getEnv("VIDEO_FFMPEG_PATH", "/usr/bin/ffmpeg")
	config.FFmpeg.FFprobePath = getEnv("VIDEO_FFPROBE_PATH", "/usr/bin/ffprobe")
	config.FFmpeg.Timeout = time.Duration(getEnvAsInt("VIDEO_FFMPEG_TIMEOUT", 300)) * time.Second
	config.FFmpeg.Quality = getEnvAsInt("VIDEO_FFMPEG_QUALITY", 23)
	config.FFmpeg.WorkerCount = getEnvAsInt("VIDEO_FFMPEG_WORKER_COUNT", 2)

	// 存储配置
	config.Storage.BasePath = getEnv("VIDEO_STORAGE_BASE_PATH", "./data/video")
	config.Storage.FramePath = getEnv("VIDEO_STORAGE_FRAME_PATH", "./data/video/frames")
	config.Storage.RecordPath = getEnv("VIDEO_STORAGE_RECORD_PATH", "./data/video/records")
	config.Storage.TempPath = getEnv("VIDEO_STORAGE_TEMP_PATH", "./data/video/temp")
	config.Storage.ConvertedPath = getEnv("VIDEO_STORAGE_CONVERTED_PATH", "./data/video/converted")

	// 抽帧配置
	config.Extract.DefaultFrameCount = getEnvAsInt("VIDEO_EXTRACT_DEFAULT_FRAME_COUNT", 10)
	config.Extract.DefaultQuality = getEnvAsInt("VIDEO_EXTRACT_DEFAULT_QUALITY", 2)
	config.Extract.MaxFrameCount = getEnvAsInt("VIDEO_EXTRACT_MAX_FRAME_COUNT", 1000)
	config.Extract.DefaultFormat = getEnv("VIDEO_EXTRACT_DEFAULT_FORMAT", "jpg")

	// 录制配置
	config.Record.DefaultDuration = getEnvAsInt("VIDEO_RECORD_DEFAULT_DURATION", 60)
	config.Record.MaxDuration = getEnvAsInt("VIDEO_RECORD_MAX_DURATION", 3600)
	config.Record.DefaultFormat = getEnv("VIDEO_RECORD_DEFAULT_FORMAT", "mp4")
	config.Record.DefaultCodec = getEnv("VIDEO_RECORD_DEFAULT_CODEC", "h264")

	// 播放URL配置
	config.PlayURL.StreamPrefix = getEnv("VIDEO_PLAY_URL_STREAM_PREFIX", "/stream/live")
	config.PlayURL.FilePrefix = getEnv("VIDEO_PLAY_URL_FILE_PREFIX", "/stream/record/videos/converted")
	config.PlayURL.RoutePrefix = getEnv("VIDEO_PLAY_URL_ROUTE_PREFIX", "/stream")

	return config
}

// Validate 验证配置项的有效性
func (c *Config) Validate() error {
	// 验证应用配置
	if c.App.Name == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	// 验证环境
	validEnvs := []string{"dev", "test", "prod"}
	if !contains(validEnvs, c.App.Environment) {
		return fmt.Errorf("invalid environment: %s, must be one of %v", c.App.Environment, validEnvs)
	}

	// 验证服务器端口
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// 验证模型配置
	if c.Model.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be positive")
	}

	if c.Model.ChunkSize <= 0 || c.Model.ChunkSize > c.Model.MaxFileSize {
		return fmt.Errorf("invalid chunk size: %d", c.Model.ChunkSize)
	}

	// 验证存储路径
	if c.Model.StorageBasePath == "" {
		return fmt.Errorf("storage base path cannot be empty")
	}

	if c.Model.TempPath == "" {
		return fmt.Errorf("temp path cannot be empty")
	}

	// 验证文件类型
	if len(c.Model.AllowedTypes) == 0 {
		return fmt.Errorf("allowed types cannot be empty")
	}

	// 验证日志级别
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.Log.Level) {
		return fmt.Errorf("invalid log level: %s, must be one of %v", c.Log.Level, validLogLevels)
	}

	// 验证视频配置
	if c.Video.ZLMediaKit.Host == "" {
		return fmt.Errorf("ZLMediaKit host cannot be empty")
	}
	if c.Video.ZLMediaKit.Port <= 0 {
		return fmt.Errorf("ZLMediaKit port must be positive")
	}
	if c.Video.Storage.BasePath == "" {
		return fmt.Errorf("video storage base path cannot be empty")
	}

	return nil
}

// CreateDirectories 创建必要的目录
func (c *Config) CreateDirectories() error {
	directories := []string{
		c.Model.StorageBasePath,
		c.Model.TempPath,
		c.Model.StorageBasePath + "/original",
		c.Model.StorageBasePath + "/converted",
		c.Model.StorageBasePath + "/backup",
		c.Model.TempPath + "/sessions",
		c.Conversion.LogPath,
		c.Conversion.OutputPath,
		// 视频相关目录
		c.Video.Storage.BasePath,
		c.Video.Storage.FramePath,
		c.Video.Storage.RecordPath,
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// IsDevelopment 检查是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "dev"
}

// IsProduction 检查是否为生产环境
func (c *Config) IsProduction() bool {
	return c.App.Environment == "prod"
}

// GetServerAddress 获取服务器地址
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// 环境变量辅助函数

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	duration, _ := time.ParseDuration(defaultValue)
	return duration
}

func getEnvAsStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// 支持逗号分隔的字符串
		return strings.Split(strings.TrimSpace(value), ",")
	}
	return defaultValue
}

// 检查切片中是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// PrintConfig 打印配置信息（隐藏敏感信息）
func (c *Config) PrintConfig() {
	log.Println("=== Application Configuration ===")
	log.Printf("App Name: %s", c.App.Name)
	log.Printf("Version: %s", c.App.Version)
	log.Printf("Environment: %s", c.App.Environment)
	log.Printf("Debug Mode: %v", c.App.Debug)
	log.Printf("Server: %s", c.GetServerAddress())
	log.Printf("Database: %s@%s:%d/%s", c.Database.User, c.Database.Host, c.Database.Port, c.Database.DBName)
	log.Printf("Model Storage: %s", c.Model.StorageBasePath)
	log.Printf("Temp Path: %s", c.Model.TempPath)
	log.Printf("Max File Size: %d MB", c.Model.MaxFileSize/1024/1024)
	log.Printf("Chunk Size: %d MB", c.Model.ChunkSize/1024/1024)
	log.Printf("Allowed Types: %v", c.Model.AllowedTypes)
	log.Printf("Log Level: %s", c.Log.Level)
	log.Printf("Video Storage: %s", c.Video.Storage.BasePath)
	log.Printf("ZLMediaKit Host: %s:%d", c.Video.ZLMediaKit.Host, c.Video.ZLMediaKit.Port)
	log.Println("================================")
}
