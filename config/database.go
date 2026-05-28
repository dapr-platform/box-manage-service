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
	"box-manage-service/migrations"
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
	Host          string
	Port          int
	User          string
	Password      string
	DBName        string
	SSLMode       string
	TimeZone      string
	MaxIdleConns  int
	MaxOpenConns  int
	MaxLifetime   time.Duration
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration // 慢 SQL 阈值
}

// LoadDatabaseConfig 从环境变量加载数据库配置
func LoadDatabaseConfig() *DatabaseConfig {
	config := &DatabaseConfig{
		Host:          getEnv("DB_HOST", "localhost"),
		Port:          getEnvAsInt("DB_PORT", 5432),
		User:          getEnv("DB_USER", "postgres"),
		Password:      getEnv("DB_PASSWORD", "postgres"),
		DBName:        getEnv("DB_NAME", "box_manage"),
		SSLMode:       getEnv("DB_SSLMODE", "disable"),
		TimeZone:      getEnv("DB_TIMEZONE", "Asia/Shanghai"),
		MaxIdleConns:  getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		MaxOpenConns:  getEnvAsInt("DB_MAX_OPEN_CONNS", 100),
		MaxLifetime:   getEnvAsDuration("DB_MAX_LIFETIME", "1h"),
		LogLevel:      getLogLevel(getEnv("DB_LOG_LEVEL", "warn")),
		SlowThreshold: getEnvAsDuration("DB_SLOW_THRESHOLD", "200ms"), // 默认 200ms
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

	// 配置 GORM
	gormConfig := &gorm.Config{
		Logger: logger.New(
			log.New(log.Writer(), "\r\n", log.LstdFlags), // io writer
			logger.Config{
				SlowThreshold:             config.SlowThreshold, // 慢 SQL 阈值
				LogLevel:                  config.LogLevel,      // Log level
				IgnoreRecordNotFoundError: true,                 // Ignore ErrRecordNotFound error by logger
				ParameterizedQueries:      false,                // Don't include params in the SQL log
				Colorful:                  true,                 // Disable color
			},
		),
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

	// 在 GORM AutoMigrate 之前先执行 pre-migration，
	// 修复历史遗留表结构问题（如新增 NOT NULL 列但表中已有数据），
	// 避免 GORM 自动 ALTER TABLE 时因 NULL 值约束失败
	if err := preMigration(db); err != nil {
		log.Printf("Warning: pre-migration failed: %v", err)
		// 不返回错误，让后续的 AutoMigrate 继续执行
	}

	// 按依赖顺序迁移模型
	models := []interface{}{
		// 基础表
		&models.Box{},
		&models.BoxHeartbeat{},
		&models.BoxModel{}, // 盒子-模型关联表

		// 系统配置表
		&models.SystemConfig{}, // 系统配置表

		// 系统日志表
		&models.SystemLog{}, // 系统日志表

		// 调度策略表
		&models.SchedulePolicy{}, // 调度策略表

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

		// 业务编排引擎相关表（按依赖顺序）
		&models.Workflow{},                 // 工作流定义表
		&models.NodeTemplate{},             // 节点模板表
		&models.NodeTemplateMeta{},         // 节点模板下发版本元信息（单行）
		&models.NodeDefinition{},           // 节点定义表（依赖 Workflow 和 NodeTemplate）
		&models.VariableDefinition{},       // 变量定义表（依赖 Workflow 和 NodeTemplate）
		&models.LineDefinition{},           // 连接线定义表（依赖 Workflow）
		&models.WorkflowDeployment{},       // 工作流部署表（依赖 Workflow）
		&models.WorkflowSchedule{},         // 工作流调度配置表（依赖 Workflow）
		&models.WorkflowScheduleInstance{}, // 工作流调度实例表（依赖 WorkflowSchedule）
		&models.WorkflowInstance{},         // 工作流实例表（依赖 Workflow）
		&models.NodeInstance{},             // 节点实例表（依赖 WorkflowInstance 和 NodeDefinition）
		&models.VariableInstance{},         // 变量实例表（依赖 WorkflowInstance 和 VariableDefinition）
		&models.LineInstance{},             // 连接线实例表（依赖 WorkflowInstance）
		&models.WorkflowLog{},              // 工作流日志表（依赖 WorkflowInstance）
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	log.Println("Database migration completed successfully")

	// 执行数据迁移：同步旧状态到新状态字段
	if err := migrateTaskStatus(db); err != nil {
		log.Printf("Warning: Task status migration failed: %v", err)
		// 不返回错误，只记录警告，因为这是可选的迁移
	}

	// 初始化系统预置节点模板（执行 SQL 文件）
	if err := initFromSQL(db); err != nil {
		log.Printf("Warning: SQL initialization failed: %v", err)
		// 不返回错误，只记录警告
	}

	return nil
}

// preMigration 在 GORM AutoMigrate 之前执行的预迁移
// 处理旧表升级到新版本时的兼容问题（如新增 NOT NULL 列时已有数据）
func preMigration(db *gorm.DB) error {
	log.Println("Running pre-migration patches...")

	// 修复 workflow_logs 表：新增的列要求 NOT NULL，
	// 但旧表已有数据时 GORM 无法直接添加 NOT NULL 列
	// 解决方案：先添加允许 NULL 的列 -> 填充默认值 -> 再设为 NOT NULL
	patchWorkflowLogs := `
DO $$
DECLARE
    col_def RECORD;
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'workflow_logs'
    ) THEN
        -- 需要补齐的 NOT NULL 列：(列名, 数据类型, 默认值)
        FOR col_def IN
            SELECT * FROM (VALUES
                ('level',              'varchar(20)',  'info'),
                ('node_instance_id',   'varchar(100)', ''),
                ('message',            'text',         ''),
                ('log_type',           'varchar(20)',  'node')
            ) AS t(col_name, col_type, default_val)
        LOOP
            -- 1. 如果列不存在，先添加（允许 NULL）
            IF NOT EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'public'
                  AND table_name = 'workflow_logs'
                  AND column_name = col_def.col_name
            ) THEN
                EXECUTE format('ALTER TABLE workflow_logs ADD COLUMN %I %s', col_def.col_name, col_def.col_type);
            END IF;

            -- 2. 给已有 NULL 数据填充默认值
            EXECUTE format('UPDATE workflow_logs SET %I = %L WHERE %I IS NULL', col_def.col_name, col_def.default_val, col_def.col_name);

            -- 3. 将列设为 NOT NULL，与模型定义保持一致
            EXECUTE format('ALTER TABLE workflow_logs ALTER COLUMN %I SET NOT NULL', col_def.col_name);
        END LOOP;

        -- 清理旧 type 列（模型已改为 log_type，旧 type 列不再使用）
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = 'workflow_logs'
              AND column_name = 'type'
        ) THEN
            ALTER TABLE workflow_logs DROP COLUMN type;
        END IF;
    END IF;
END $$;
`
	if err := db.Exec(patchWorkflowLogs).Error; err != nil {
		return fmt.Errorf("patch workflow_logs failed: %w", err)
	}
	log.Println("Pre-migration patches applied")
	return nil
}

// migrateTaskStatus 迁移任务状态数据：根据旧的 Status 字段同步新的 ScheduleStatus 和 RunStatus 字段
func migrateTaskStatus(db *gorm.DB) error {
	log.Println("Starting task status migration...")

	// 更新 ScheduleStatus：如果 BoxID 不为空，则为 assigned，否则为 unassigned
	result := db.Exec(`
		UPDATE tasks 
		SET schedule_status = CASE 
			WHEN box_id IS NOT NULL THEN 'assigned' 
			ELSE 'unassigned' 
		END
		WHERE schedule_status IS NULL OR schedule_status = ''
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to migrate schedule_status: %w", result.Error)
	}
	log.Printf("Updated %d tasks schedule_status", result.RowsAffected)

	// 更新 RunStatus：根据 Status 字段判断
	result = db.Exec(`
		UPDATE tasks 
		SET run_status = CASE 
			WHEN status = 'running' THEN 'running' 
			ELSE 'stopped' 
		END
		WHERE run_status IS NULL OR run_status = ''
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to migrate run_status: %w", result.Error)
	}
	log.Printf("Updated %d tasks run_status", result.RowsAffected)

	log.Println("Task status migration completed")
	return nil
}

// initFromSQL 执行 SQL 文件进行系统初始化
func initFromSQL(db *gorm.DB) error {
	log.Println("Executing SQL initialization scripts...")

	// 1. 执行 create_workflow_tables.sql（幂等，每次启动都执行）
	// 创建表、索引、触发器、系统配置等
	if err := db.Exec(migrations.CreateWorkflowTablesSQL).Error; err != nil {
		log.Printf("Warning: create_workflow_tables.sql execution had issues: %v", err)
		// 不返回错误，因为部分语句可能因为 GORM 已经创建了表而跳过
	}
	log.Println("create_workflow_tables.sql executed")

	// 2. 执行 node_template_init_data.sql（幂等，使用 WHERE NOT EXISTS 保证不重复插入）
	if err := db.Exec(migrations.NodeTemplateInitDataSQL).Error; err != nil {
		return fmt.Errorf("node_template_init_data.sql execution failed: %w", err)
	}
	log.Println("node_template_init_data.sql executed")

	log.Println("SQL initialization completed")
	return nil
}

// initNodeTemplates 初始化系统预置节点模板（已废弃，保留作为备用）
func initNodeTemplates(db *gorm.DB) error {
	log.Println("Initializing system node templates...")

	// 检查是否已经初始化过
	var count int64
	if err := db.Model(&models.NodeTemplate{}).Where("is_system = ?", true).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check existing templates: %w", err)
	}

	if count > 0 {
		log.Printf("System node templates already initialized (%d templates found), skipping...", count)
		return nil
	}

	// 系统预置节点模板
	templates := []models.NodeTemplate{
		// 逻辑控制类节点
		{
			TypeKey:     "start",
			TypeName:    "开始节点",
			Category:    "logic",
			GroupType:   models.NodeGroupTypeSingle,
			Description: "工作流的起始节点",
			Icon:        "icon-start",
			IsSystem:    true,
			IsEnabled:   true,
			SortOrder:   1,
		},
		{
			TypeKey:     "end",
			TypeName:    "结束节点",
			Category:    "logic",
			GroupType:   models.NodeGroupTypeSingle,
			Description: "工作流的结束节点",
			Icon:        "icon-end",
			IsSystem:    true,
			IsEnabled:   true,
			SortOrder:   2,
		},
		{
			TypeKey:      "concurrency_start",
			TypeName:     "并发开始",
			Category:     "logic",
			GroupType:    models.NodeGroupTypePaired,
			StartNodeKey: "concurrency_start",
			EndNodeKey:   "concurrency_end",
			Description:  "标记并发执行区域的开始",
			Icon:         "icon-concurrency",
			IsSystem:     true,
			IsEnabled:    true,
			SortOrder:    3,
		},
		{
			TypeKey:      "concurrency_end",
			TypeName:     "并发结束",
			Category:     "logic",
			GroupType:    models.NodeGroupTypePaired,
			StartNodeKey: "concurrency_start",
			EndNodeKey:   "concurrency_end",
			Description:  "标记并发执行区域的结束，等待所有并发分支完成",
			Icon:         "icon-concurrency",
			IsSystem:     true,
			IsEnabled:    true,
			SortOrder:    4,
		},
		{
			TypeKey:      "loop_start",
			TypeName:     "循环开始",
			Category:     "logic",
			GroupType:    models.NodeGroupTypePaired,
			StartNodeKey: "loop_start",
			EndNodeKey:   "loop_end",
			Description:  "标记循环区域的开始",
			Icon:         "icon-loop",
			IsSystem:     true,
			IsEnabled:    true,
			SortOrder:    5,
		},
		{
			TypeKey:      "loop_end",
			TypeName:     "循环结束",
			Category:     "logic",
			GroupType:    models.NodeGroupTypePaired,
			StartNodeKey: "loop_start",
			EndNodeKey:   "loop_end",
			Description:  "标记循环区域的结束，判断是否继续循环",
			Icon:         "icon-loop",
			IsSystem:     true,
			IsEnabled:    true,
			SortOrder:    6,
		},
		// 业务执行类节点
		{
			TypeKey:     "kvm",
			TypeName:    "KVM接入节点",
			Category:    "business",
			GroupType:   models.NodeGroupTypeSingle,
			Description: "连接和控制KVM设备",
			Icon:        "icon-kvm",
			IsSystem:    true,
			IsEnabled:   true,
			SortOrder:   10,
		},
		{
			TypeKey:     "reasoning",
			TypeName:    "Reasoning推理节点",
			Category:    "business",
			GroupType:   models.NodeGroupTypeSingle,
			Description: "调用AI模型进行推理计算",
			Icon:        "icon-ai",
			IsSystem:    true,
			IsEnabled:   true,
			SortOrder:   11,
		},
		{
			TypeKey:     "python_script",
			TypeName:    "PythonScript脚本节点",
			Category:    "business",
			GroupType:   models.NodeGroupTypeSingle,
			Description: "执行自定义Python脚本",
			Icon:        "icon-python",
			IsSystem:    true,
			IsEnabled:   true,
			SortOrder:   12,
		},
		{
			TypeKey:     "mqtt",
			TypeName:    "MQTT推送节点",
			Category:    "business",
			GroupType:   models.NodeGroupTypeSingle,
			Description: "向MQTT服务器推送消息",
			Icon:        "icon-mqtt",
			IsSystem:    true,
			IsEnabled:   true,
			SortOrder:   13,
		},
	}

	// 批量插入
	if err := db.Create(&templates).Error; err != nil {
		return fmt.Errorf("failed to create node templates: %w", err)
	}

	log.Printf("Successfully initialized %d system node templates", len(templates))
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
