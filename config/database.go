/*
 * @module config/database
 * @description 数据库配置和初始化
 * @architecture 配置层
 * @documentReference DESIGN-000.md
 * @stateFlow 配置加载 -> 数据库连接 -> 迁移 -> 服务使用
 * @rules 根据GORM最佳实践配置数据库连接池、日志等
 * @dependencies gorm.io/gorm, gorm.io/driver/postgres
 * @refs context7 GORM最佳实践
 */

package config

import (
	"box-manage-service/models"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// DatabaseConfig 数据库配置结构
type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	SSLMode      string
	TimeZone     string
	MaxIdleConns int
	MaxOpenConns int
	MaxLifetime  time.Duration
	LogLevel     logger.LogLevel
}

// LoadDatabaseConfig 从环境变量加载数据库配置
func LoadDatabaseConfig() *DatabaseConfig {
	config := &DatabaseConfig{
		Host:         getEnv("DB_HOST", "localhost"),
		Port:         getEnvAsInt("DB_PORT", 5432),
		User:         getEnv("DB_USER", "postgres"),
		Password:     getEnv("DB_PASSWORD", "postgres"),
		DBName:       getEnv("DB_NAME", "box_manage"),
		SSLMode:      getEnv("DB_SSLMODE", "disable"),
		TimeZone:     getEnv("DB_TIMEZONE", "Asia/Shanghai"),
		MaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		MaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 100),
		MaxLifetime:  getEnvAsDuration("DB_MAX_LIFETIME", "1h"),
		LogLevel:     getLogLevel(getEnv("DB_LOG_LEVEL", "warn")),
	}

	return config
}

// BuildDSN 构建PostgreSQL数据源名称
func (c *DatabaseConfig) BuildDSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		c.Host, c.User, c.Password, c.DBName, c.Port, c.SSLMode, c.TimeZone)
}

// InitDatabase 初始化数据库连接
func InitDatabase() (*gorm.DB, error) {
	config := LoadDatabaseConfig()

	// 配置GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(config.LogLevel),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false, // 使用复数表名
		},
	}

	// 连接数据库
	db, err := gorm.Open(postgres.Open(config.BuildDSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取底层的sql.DB对象来配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(config.MaxLifetime)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Successfully connected to database: %s@%s:%d/%s",
		config.User, config.Host, config.Port, config.DBName)

	return db, nil
}

// AutoMigrate 执行数据库迁移
func AutoMigrate(db *gorm.DB) error {
	log.Println("Starting database migration...")

	// 按依赖顺序迁移模型
	models := []interface{}{
		// 基础表
		&models.Box{},
		&models.BoxHeartbeat{},
		&models.BoxModel{}, // 盒子-模型关联表

		// 任务相关表
		&models.Task{},
		&models.TaskExecution{},  // 任务执行记录表
		&models.DeploymentTask{}, // 部署任务表

		// 转换任务表
		&models.ConversionTask{}, // 模型转换任务表

		// 升级相关表
		&models.UpgradePackage{},   // 先创建升级包表
		&models.UpgradeTask{},      // 再创建升级任务表（依赖升级包）
		&models.BatchUpgradeTask{}, // 批量升级任务表
		&models.UpgradeVersion{},   // 升级版本记录表

		// 原始模型管理相关表 (REQ-002)
		&models.ModelTag{},       // 模型标签表
		&models.OriginalModel{},  // 原始模型表
		&models.ConvertedModel{}, // 转换后模型表（依赖原始模型）
		&models.UploadSession{},  // 上传会话表

		// 模型部署相关表（按依赖顺序）
		&models.ModelDeploymentTask{}, // 模型部署任务表（先创建）
		&models.ModelDeploymentItem{}, // 模型部署项表（依赖任务表）
		&models.ModelBoxDeployment{},  // 模型-盒子部署关联表

		// 视频管理相关表 (REQ-009)
		&models.VideoSource{},  // 视频源表
		&models.VideoFile{},    // 视频文件表
		&models.ExtractFrame{}, // 帧提取表
		&models.ExtractTask{},  // 抽帧任务表
		&models.RecordTask{},   // 录制任务表
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	log.Println("Database migration completed successfully")
	return nil
}

// CloseDatabase 关闭数据库连接
func CloseDatabase(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// 辅助函数已在config.go中定义

func getLogLevel(level string) logger.LogLevel {
	switch level {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}
