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
	"box-manage-service/repository"
	"context"
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
func AutoMigrate(db *gorm.DB, resetNodeTemplate bool) error {
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
		// 权限体系扩展表（沿用 postgrest schema）
		&models.Menu{},
		&models.RoleMenu{},

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
	if err := initFromSQL(db, resetNodeTemplate); err != nil {
		log.Printf("Warning: SQL initialization failed: %v", err)
		// 不返回错误，只记录警告
	}

	// 初始化菜单权限数据（幂等）
	if err := initMenuPermissions(db); err != nil {
		log.Printf("Warning: menu permission initialization failed: %v", err)
	}
	if err := patchPostgRESTTokenFunctions(db); err != nil {
		log.Printf("Warning: PostgREST token function patch failed: %v", err)
	}

	return nil
}

// preMigration 在 GORM AutoMigrate 之前执行的预迁移
// 处理旧表升级到新版本时的兼容问题（如新增 NOT NULL 列时已有数据）
func preMigration(db *gorm.DB) error {
	log.Println("Running pre-migration patches...")

	patchPostgRESTSchema := `
CREATE SCHEMA IF NOT EXISTS postgrest;

CREATE TABLE IF NOT EXISTS postgrest.roles (
  role_name text primary key,
  description text,
  display_name text,
  is_system_role boolean default false,
  created_at timestamp with time zone default now(),
  updated_at timestamp with time zone default now()
);

INSERT INTO postgrest.roles (role_name, description, display_name, is_system_role) VALUES
  ('admin', '系统管理员，拥有所有权限', '系统管理员', true),
  ('user', '普通用户，基本读写权限', '普通用户', true),
  ('readonly', '只读用户，仅查看权限', '只读用户', true),
  ('guest', '访客用户，最小权限', '访客用户', true)
ON CONFLICT (role_name) DO NOTHING;
`
	if err := db.Exec(patchPostgRESTSchema).Error; err != nil {
		return fmt.Errorf("patch postgrest schema failed: %w", err)
	}

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
func initFromSQL(db *gorm.DB, resetNodeTemplate bool) error {
	log.Println("Executing SQL initialization scripts...")

	// 0. 如果配置了重置内置节点数据，先清理再重新初始化
	if resetNodeTemplate {
		log.Println("RESET_NODE_TEMPLATE_DATA=true，清理内置节点模板数据...")
		if err := db.Exec("DELETE FROM node_templates WHERE is_system = true").Error; err != nil {
			log.Printf("Warning: failed to clear node_templates: %v", err)
		}
		if err := db.Exec("DELETE FROM variable_definitions WHERE workflow_id = 0").Error; err != nil {
			log.Printf("Warning: failed to clear variable_definitions: %v", err)
		}
		log.Println("内置节点数据已清理")
	}

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

func initMenuPermissions(db *gorm.DB) error {
	log.Println("Initializing menu permission data...")

	menuRepo := repository.NewMenuRepository(db)
	ctx := context.Background()

	menus := []models.Menu{
		{ResourceID: "dashboard", Name: "Dashboard", Title: "仪表板", Path: "/dashboard", Icon: "📊", SortOrder: 10, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "boxes", Name: "BoxManagement", Title: "盒子管理", Icon: "📦", SortOrder: 20, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "boxes-list", ParentID: "boxes", Name: "BoxList", Title: "盒子列表", Path: "/boxes", SortOrder: 21, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "boxes-add", ParentID: "boxes", Name: "BoxAdd", Title: "添加盒子", Path: "/boxes/add", SortOrder: 22, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "boxes-detail", ParentID: "boxes", Name: "BoxDetail", Title: "设备详情", Path: "/boxes/detail/:id", SortOrder: 23, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "boxes-detail-alias", ParentID: "boxes", Name: "BoxDetailAlias", Title: "设备详情", Path: "/boxes/:id", SortOrder: 24, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "boxes-edit", ParentID: "boxes", Name: "BoxEdit", Title: "编辑设备", Path: "/boxes/edit/:id", SortOrder: 25, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "boxes-discover", ParentID: "boxes", Name: "BoxDiscover", Title: "设备发现", Path: "/boxes/discover", SortOrder: 26, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "upgrade-packages-list", ParentID: "boxes", Name: "UpgradePackageList", Title: "升级包列表", Path: "/upgrade-packages", SortOrder: 27, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "upgrade-packages-create", ParentID: "boxes", Name: "UpgradePackageCreate", Title: "创建升级包", Path: "/upgrade-packages/create", SortOrder: 28, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "upgrade-packages-detail", ParentID: "boxes", Name: "UpgradePackageDetail", Title: "升级包详情", Path: "/upgrade-packages/detail/:id", SortOrder: 29, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "upgrades-list", ParentID: "boxes", Name: "UpgradeTaskList", Title: "升级任务列表", Path: "/upgrades", SortOrder: 30, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "upgrades-detail", ParentID: "boxes", Name: "UpgradeTaskDetail", Title: "升级任务详情", Path: "/upgrades/:id", SortOrder: 31, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "upgrades-detail-alias", ParentID: "boxes", Name: "UpgradeTaskDetailAlias", Title: "升级任务详情", Path: "/upgrades/detail/:id", SortOrder: 32, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "models", Name: "ModelManagement", Title: "模型管理", Icon: "🧪", SortOrder: 40, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "models-original", ParentID: "models", Name: "ModelList", Title: "原始模型", Path: "/models", SortOrder: 41, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "models-converted", ParentID: "models", Name: "ConvertedModelList", Title: "转换后模型", Path: "/models/converted", SortOrder: 42, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "models-conversion-tasks", ParentID: "models", Name: "ConversionTaskList", Title: "转换任务", Path: "/models/conversion-tasks", SortOrder: 43, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "models-conversion-task-detail", ParentID: "models", Name: "ConversionTaskDetail", Title: "转换任务详情", Path: "/models/conversion-tasks/:id", SortOrder: 44, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "models-deployment-tasks", ParentID: "models", Name: "ModelDeploymentTaskList", Title: "模型部署任务", Path: "/models/deployment-tasks", SortOrder: 45, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "models-deployment-task-detail", ParentID: "models", Name: "ModelDeploymentTaskDetail", Title: "部署任务详情", Path: "/models/deployment-tasks/:id", SortOrder: 46, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "models-upload", ParentID: "models", Name: "ModelUpload", Title: "上传模型", Path: "/models/upload", SortOrder: 47, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "models-detail", ParentID: "models", Name: "ModelDetail", Title: "模型详情", Path: "/models/detail/:id", SortOrder: 48, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "models-edit", ParentID: "models", Name: "ModelEdit", Title: "编辑模型", Path: "/models/edit/:id", SortOrder: 49, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "models-converted-detail", ParentID: "models", Name: "ConvertedModelDetail", Title: "转换后模型详情", Path: "/models/converted/:id", SortOrder: 50, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "videos", Name: "VideoManagement", Title: "视频管理", Icon: "📹", SortOrder: 60, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "video-sources", ParentID: "videos", Name: "VideoSources", Title: "视频源管理", Path: "/videos/sources", SortOrder: 61, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "video-source-detail", ParentID: "videos", Name: "VideoSourceDetail", Title: "视频源详情", Path: "/videos/sources/:id", SortOrder: 62, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "video-files", ParentID: "videos", Name: "VideoFiles", Title: "视频文件管理", Path: "/videos/files", SortOrder: 63, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "video-file-detail", ParentID: "videos", Name: "VideoFileDetail", Title: "视频文件详情", Path: "/videos/files/:id", SortOrder: 64, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "video-extract-tasks", ParentID: "videos", Name: "VideoExtractTasks", Title: "抽帧任务管理", Path: "/videos/extract-tasks", SortOrder: 65, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "video-extract-task-frames", ParentID: "videos", Name: "VideoExtractTaskFrames", Title: "抽帧结果", Path: "/videos/extract-tasks/:id/frames", SortOrder: 66, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "video-record-tasks", ParentID: "videos", Name: "VideoRecordTasks", Title: "录制任务管理", Path: "/videos/record-tasks", SortOrder: 67, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "tasks", Name: "TaskManagement", Title: "任务管理", Icon: "📅", SortOrder: 70, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "tasks-list", ParentID: "tasks", Name: "TaskList", Title: "任务列表", Path: "/tasks", SortOrder: 71, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "tasks-deployment", ParentID: "tasks", Name: "DeploymentTaskList", Title: "部署任务", Path: "/tasks/deployment", SortOrder: 72, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "tasks-create", ParentID: "tasks", Name: "TaskCreate", Title: "创建任务", Path: "/tasks/create", SortOrder: 73, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "tasks-detail", ParentID: "tasks", Name: "TaskDetail", Title: "任务详情", Path: "/tasks/detail/:id", SortOrder: 74, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "tasks-deployment-detail", ParentID: "tasks", Name: "DeploymentTaskDetail", Title: "部署任务详情", Path: "/tasks/deployment/detail/:id", SortOrder: 75, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "schedule", Name: "ScheduleManagement", Title: "调度管理", Icon: "🔄", SortOrder: 80, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "schedule-policies", ParentID: "schedule", Name: "SchedulePolicyList", Title: "调度策略", Path: "/schedule/policies", SortOrder: 81, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "schedule-auto-scheduler", ParentID: "schedule", Name: "AutoScheduler", Title: "自动调度控制", Path: "/schedule/auto-scheduler", SortOrder: 82, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow", Name: "WorkflowManagement", Title: "业务编排", Icon: "🔗", SortOrder: 90, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow-list", ParentID: "workflow", Name: "WorkflowList", Title: "编排列表", Path: "/workflow", SortOrder: 91, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow-instances", ParentID: "workflow", Name: "WorkflowInstanceList", Title: "实例列表", Path: "/workflow/instances", SortOrder: 92, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow-instance-detail", ParentID: "workflow", Name: "WorkflowInstanceDetail", Title: "实例详情", Path: "/workflow/instance/:id", SortOrder: 93, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "workflow-schedules", ParentID: "workflow", Name: "ScheduleList", Title: "调度管理", Path: "/workflow/schedules", SortOrder: 94, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow-schedule-instances", ParentID: "workflow", Name: "ScheduleInstanceList", Title: "调度实例", Path: "/workflow/schedule/:id/instances", SortOrder: 95, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "workflow-deployments", ParentID: "workflow", Name: "DeploymentList", Title: "部署管理", Path: "/workflow/deployments", SortOrder: 96, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow-deployment-detail", ParentID: "workflow", Name: "DeploymentDetail", Title: "部署任务详情", Path: "/workflow/deployments/:id", SortOrder: 97, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "workflow-node-templates", ParentID: "workflow", Name: "NodeTemplateList", Title: "节点管理", Path: "/workflow/node-templates", SortOrder: 98, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "workflow-editor", ParentID: "workflow", Name: "WorkflowEditor", Title: "编排编辑器", Path: "/workflow/editor", SortOrder: 99, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "workflow-editor-with-id", ParentID: "workflow", Name: "WorkflowEditorWithID", Title: "编辑流程", Path: "/workflow/editor/:id", SortOrder: 100, IsSystem: true, IsEnabled: true, IsVisible: false},
		{ResourceID: "users", Name: "UserManagement", Title: "用户管理", Icon: "👥", SortOrder: 110, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "users-list", ParentID: "users", Name: "UserList", Title: "用户列表", Path: "/users", SortOrder: 111, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "users-roles", ParentID: "users", Name: "RoleList", Title: "角色管理", Path: "/users/roles", SortOrder: 112, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "system", Name: "SystemManagement", Title: "系统管理", Icon: "⚙️", SortOrder: 120, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "system-config", ParentID: "system", Name: "SystemConfig", Title: "系统配置", Path: "/system/config", SortOrder: 121, IsSystem: true, IsEnabled: true, IsVisible: true},
		{ResourceID: "system-logs", ParentID: "system", Name: "SystemLogs", Title: "系统日志", Path: "/system/logs", SortOrder: 122, IsSystem: true, IsEnabled: true, IsVisible: true},
	}

	for i := range menus {
		menu := menus[i]
		if err := db.WithContext(ctx).
			Where("resource_id = ?", menu.ResourceID).
			Assign(map[string]interface{}{
				"parent_id":  menu.ParentID,
				"name":       menu.Name,
				"title":      menu.Title,
				"path":       menu.Path,
				"icon":       menu.Icon,
				"component":  menu.Component,
				"sort_order": menu.SortOrder,
				"is_enabled": menu.IsEnabled,
				"is_visible": menu.IsVisible,
				"is_system":  menu.IsSystem,
				"remark":     menu.Remark,
				"updated_at": time.Now(),
			}).
			FirstOrCreate(&menu).Error; err != nil {
			return fmt.Errorf("upsert menu %s failed: %w", menu.ResourceID, err)
		}
	}

	staleSystemMenuIDs := []string{
		"tasks.list", "tasks.deployment", "tasks.create",
		"schedule.policies", "schedule.auto",
		"boxes.list", "boxes.add", "boxes.discover",
		"upgrade_packages", "upgrade_packages.list", "upgrade_packages.create",
		"upgrades", "upgrades.list",
		"models.original", "models.converted", "models.conversion_tasks", "models.deployment_tasks",
		"videos.sources", "videos.files", "videos.extract_tasks", "videos.record_tasks",
		"workflow.list", "workflow.instances", "workflow.schedules", "workflow.deployments", "workflow.node_templates",
		"users.list", "users.roles",
		"system.config", "system.logs",
	}
	if err := db.WithContext(ctx).
		Model(&models.Menu{}).
		Where("is_system = ? AND resource_id IN ?", true, staleSystemMenuIDs).
		Updates(map[string]interface{}{
			"is_enabled": false,
			"is_visible": false,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("disable stale system menus failed: %w", err)
	}

	if err := menuRepo.GrantAllMenusToRole(ctx, "admin", "system"); err != nil {
		return fmt.Errorf("grant admin menus failed: %w", err)
	}

	businessMenus := []string{
		"dashboard",
		"boxes", "boxes-list", "boxes-add", "boxes-detail", "boxes-detail-alias", "boxes-edit", "boxes-discover",
		"upgrade-packages-list", "upgrade-packages-create", "upgrade-packages-detail",
		"upgrades-list", "upgrades-detail", "upgrades-detail-alias",
		"models", "models-original", "models-converted", "models-conversion-tasks", "models-conversion-task-detail",
		"models-deployment-tasks", "models-deployment-task-detail", "models-upload", "models-detail", "models-edit", "models-converted-detail",
		"videos", "video-sources", "video-source-detail", "video-files", "video-file-detail", "video-extract-tasks", "video-extract-task-frames", "video-record-tasks",
		"tasks", "tasks-list", "tasks-deployment", "tasks-create", "tasks-detail", "tasks-deployment-detail",
		"schedule", "schedule-policies", "schedule-auto-scheduler",
		"workflow", "workflow-list", "workflow-instances", "workflow-instance-detail", "workflow-schedules", "workflow-schedule-instances",
		"workflow-deployments", "workflow-deployment-detail", "workflow-node-templates", "workflow-editor", "workflow-editor-with-id",
	}
	if err := seedRoleMenusIfEmpty(ctx, db, menuRepo, "user", businessMenus); err != nil {
		return fmt.Errorf("seed user menus failed: %w", err)
	}

	readonlyMenus := []string{
		"dashboard",
		"boxes", "boxes-list", "boxes-detail", "boxes-detail-alias", "boxes-discover",
		"upgrade-packages-list", "upgrade-packages-detail",
		"upgrades-list", "upgrades-detail", "upgrades-detail-alias",
		"models", "models-original", "models-converted", "models-conversion-tasks", "models-conversion-task-detail",
		"models-deployment-tasks", "models-deployment-task-detail", "models-detail", "models-converted-detail",
		"videos", "video-sources", "video-source-detail", "video-files", "video-file-detail", "video-extract-tasks", "video-extract-task-frames", "video-record-tasks",
		"tasks", "tasks-list", "tasks-detail", "tasks-deployment", "tasks-deployment-detail",
		"schedule", "schedule-policies", "schedule-auto-scheduler",
		"workflow", "workflow-list", "workflow-instances", "workflow-instance-detail", "workflow-schedules", "workflow-schedule-instances",
		"workflow-deployments", "workflow-deployment-detail", "workflow-node-templates",
	}
	if err := seedRoleMenusIfEmpty(ctx, db, menuRepo, "readonly", readonlyMenus); err != nil {
		return fmt.Errorf("seed readonly menus failed: %w", err)
	}

	if err := seedRoleMenusIfEmpty(ctx, db, menuRepo, "guest", []string{"dashboard"}); err != nil {
		return fmt.Errorf("seed guest menus failed: %w", err)
	}

	grantSQL := `
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'authenticator') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON postgrest.menus TO authenticator;
    GRANT SELECT, INSERT, UPDATE, DELETE ON postgrest.role_menus TO authenticator;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA postgrest TO authenticator;
  END IF;
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'admin') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON postgrest.menus TO admin;
    GRANT SELECT, INSERT, UPDATE, DELETE ON postgrest.role_menus TO admin;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA postgrest TO admin;
  END IF;
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'user') THEN
    GRANT SELECT ON postgrest.menus TO "user";
    GRANT SELECT ON postgrest.role_menus TO "user";
  END IF;
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'readonly') THEN
    GRANT SELECT ON postgrest.menus TO readonly;
    GRANT SELECT ON postgrest.role_menus TO readonly;
  END IF;
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'guest') THEN
    GRANT SELECT ON postgrest.menus TO guest;
    GRANT SELECT ON postgrest.role_menus TO guest;
  END IF;
END $$;
`
	if err := db.Exec(grantSQL).Error; err != nil {
		return fmt.Errorf("grant menu permissions failed: %w", err)
	}

	log.Println("Menu permission data initialized")
	return nil
}

func seedRoleMenusIfEmpty(ctx context.Context, db *gorm.DB, menuRepo repository.MenuRepository, roleName string, resourceIDs []string) error {
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.RoleMenu{}).
		Where("role_name = ?", roleName).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return menuRepo.ReplaceRoleMenus(ctx, roleName, resourceIDs, "system")
}

func patchPostgRESTTokenFunctions(db *gorm.DB) error {
	log.Println("Patching PostgREST token functions with menu permissions...")

	sql := `
CREATE OR REPLACE FUNCTION postgrest.get_menu_ids_for_roles(role_names text[])
RETURNS json AS $$
  SELECT COALESCE(
    json_agg(json_build_object('resource_id', resource_id) ORDER BY sort_order, resource_id),
    '[]'::json
  )
  FROM (
    SELECT DISTINCT m.resource_id, m.sort_order
    FROM postgrest.menus m
    LEFT JOIN postgrest.role_menus rm ON rm.resource_id = m.resource_id
    WHERE m.is_enabled = true
      AND (
        'admin' = ANY(COALESCE(role_names, ARRAY[]::text[]))
        OR rm.role_name = ANY(COALESCE(role_names, ARRAY[]::text[]))
      )
  ) authorized_menus;
$$ LANGUAGE sql STABLE SECURITY DEFINER;

CREATE OR REPLACE FUNCTION postgrest.get_menus_for_roles(role_names text[])
RETURNS json AS $$
  SELECT COALESCE(
    json_agg(
      json_build_object(
        'id', id,
        'resource_id', resource_id,
        'parent_id', parent_id,
        'name', name,
        'title', title,
        'path', path,
        'icon', icon,
        'component', component,
        'sort_order', sort_order,
        'is_enabled', is_enabled,
        'is_visible', is_visible,
        'is_system', is_system,
        'remark', remark,
        'created_at', created_at,
        'updated_at', updated_at
      )
      ORDER BY sort_order, resource_id
    ),
    '[]'::json
  )
  FROM (
    SELECT DISTINCT
      m.id,
      m.resource_id,
      m.parent_id,
      m.name,
      m.title,
      m.path,
      m.icon,
      m.component,
      m.sort_order,
      m.is_enabled,
      m.is_visible,
      m.is_system,
      m.remark,
      m.created_at,
      m.updated_at
    FROM postgrest.menus m
    LEFT JOIN postgrest.role_menus rm ON rm.resource_id = m.resource_id
    WHERE m.is_enabled = true
      AND m.is_visible = true
      AND (
        'admin' = ANY(COALESCE(role_names, ARRAY[]::text[]))
        OR rm.role_name = ANY(COALESCE(role_names, ARRAY[]::text[]))
      )
  ) authorized_menus;
$$ LANGUAGE sql STABLE SECURITY DEFINER;

CREATE OR REPLACE FUNCTION postgrest.get_token(
  username text,
  password text
)
RETURNS json AS $$
DECLARE
  user_record record;
  jwt_secret text;
  jwt_exp_setting text;
  jwt_refresh_exp_setting text;
  jwt_exp_seconds integer;
  jwt_refresh_exp_seconds integer;
  access_token text;
  refresh_token text;
  refresh_token_id uuid;
  user_roles text[];
  user_permissions text[];
  user_menu_ids json;
  user_menus json;
  is_postgres_superuser boolean := false;
  is_superuser boolean := false;
  db_auth_result boolean := false;
BEGIN
  PERFORM set_config('search_path', 'postgrest,extensions,public', true);
  jwt_secret := current_setting('app.settings.jwt_secret', false);

  BEGIN
    jwt_exp_setting := current_setting('app.settings.jwt_exp', true);
    IF jwt_exp_setting IS NULL OR jwt_exp_setting = '' THEN
      jwt_exp_seconds := 900;
    ELSE
      jwt_exp_seconds := jwt_exp_setting::integer;
    END IF;
  EXCEPTION WHEN OTHERS THEN
    jwt_exp_seconds := 900;
  END;

  BEGIN
    jwt_refresh_exp_setting := current_setting('app.settings.jwt_refresh_exp', true);
    IF jwt_refresh_exp_setting IS NULL OR jwt_refresh_exp_setting = '' THEN
      jwt_refresh_exp_seconds := 604800;
    ELSE
      jwt_refresh_exp_seconds := jwt_refresh_exp_setting::integer;
    END IF;
  EXCEPTION WHEN OTHERS THEN
    jwt_refresh_exp_seconds := 604800;
  END;

  IF get_token.username = 'postgres' THEN
    is_postgres_superuser := true;

    BEGIN
      SELECT EXISTS(
        SELECT 1 FROM pg_authid
        WHERE rolname = 'postgres'
          AND rolpassword = 'md5' || md5(get_token.password || 'postgres')
      ) INTO db_auth_result;

      IF NOT db_auth_result THEN
        SELECT EXISTS(
          SELECT 1 FROM pg_authid
          WHERE rolname = 'postgres'
            AND rolpassword = crypt(get_token.password, rolpassword)
        ) INTO db_auth_result;
      END IF;
    EXCEPTION WHEN OTHERS THEN
      RETURN json_build_object(
        'success', false,
        'message', '无法验证postgres用户密码: ' || sqlerrm,
        'token', null
      );
    END;

    IF NOT db_auth_result THEN
      RETURN json_build_object(
        'success', false,
        'message', 'postgres用户密码错误',
        'token', null
      );
    END IF;

    user_roles := ARRAY['admin', 'postgres_superuser'];
    user_permissions := ARRAY[
      'system.admin', 'users.select', 'users.insert', 'users.update', 'users.delete',
      'roles.manage', 'permissions.manage', 'data.read', 'data.write', 'data.delete',
      'schema.create', 'schema.delete', 'database.admin'
    ];
    user_menu_ids := postgrest.get_menu_ids_for_roles(user_roles);
    user_menus := postgrest.get_menus_for_roles(user_roles);

    SELECT sign(row_to_json(r), jwt_secret) INTO access_token
    FROM (
      SELECT
        'admin' AS role,
        'postgres' AS username,
        user_roles AS roles,
        user_permissions AS permissions,
        user_menu_ids AS menu_ids,
        extract(epoch FROM now())::integer + jwt_exp_seconds AS exp,
        extract(epoch FROM now())::integer AS iat,
        'postgrest' AS iss,
        'access' AS token_type,
        true AS is_superuser
    ) r;

    refresh_token_id := gen_random_uuid();
    SELECT sign(row_to_json(r), jwt_secret) INTO refresh_token
    FROM (
      SELECT
        'postgres' AS username,
        extract(epoch FROM now())::integer + jwt_refresh_exp_seconds AS exp,
        extract(epoch FROM now())::integer AS iat,
        'postgrest' AS iss,
        'refresh' AS token_type,
        refresh_token_id AS jti
    ) r;

    BEGIN
      INSERT INTO postgrest.refresh_tokens (id, username, token_hash, expires_at)
      VALUES (
        refresh_token_id,
        'postgres',
        encode(digest(refresh_token, 'sha256'), 'hex'),
        to_timestamp(extract(epoch FROM now()) + jwt_refresh_exp_seconds)
      );
    EXCEPTION WHEN foreign_key_violation THEN
      NULL;
    END;

    RETURN json_build_object(
      'success', true,
      'message', 'postgres超级用户登录成功',
      'access_token', access_token,
      'refresh_token', refresh_token,
      'access_expires_in', jwt_exp_seconds,
      'refresh_expires_in', jwt_refresh_exp_seconds,
      'username', 'postgres',
      'roles', user_roles,
      'permissions', user_permissions,
      'menu_ids', user_menu_ids,
      'menus', user_menus,
      'is_superuser', true,
      'user_info', json_build_object(
        'username', 'postgres',
        'email', 'postgres@system.local',
        'full_name', 'PostgreSQL 超级用户',
        'display_name', 'PostgreSQL 超级用户',
        'is_active', true,
        'created_at', now(),
        'updated_at', now(),
        'menu_ids', user_menu_ids,
        'menus', user_menus
      )
    );
  END IF;

  SELECT * INTO user_record
  FROM postgrest.users u
  WHERE u.username = get_token.username
    AND u.is_active = true;

  IF NOT found THEN
    RETURN json_build_object(
      'success', false,
      'message', '用户不存在或已被禁用',
      'token', null
    );
  END IF;

  IF user_record.password_hash != crypt(password, user_record.password_hash) THEN
    RETURN json_build_object(
      'success', false,
      'message', '密码错误',
      'token', null
    );
  END IF;

  SELECT array_agg(ur.role_name) INTO user_roles
  FROM postgrest.user_roles ur
  WHERE ur.username = get_token.username
    AND ur.is_active = true
    AND (ur.expires_at IS NULL OR ur.expires_at > now());

  SELECT array_agg(DISTINCT rp.permission_name) INTO user_permissions
  FROM postgrest.user_roles ur
  JOIN postgrest.role_permissions rp ON ur.role_name = rp.role_name
  WHERE ur.username = get_token.username
    AND ur.is_active = true
    AND (ur.expires_at IS NULL OR ur.expires_at > now());

  IF get_token.username = 'admin' THEN
    is_superuser := true;
  END IF;

  user_menu_ids := postgrest.get_menu_ids_for_roles(user_roles);
  user_menus := postgrest.get_menus_for_roles(user_roles);

  SELECT sign(row_to_json(r), jwt_secret) INTO access_token
  FROM (
    SELECT
      coalesce(user_roles[1], 'anon') AS role,
      get_token.username AS username,
      user_roles AS roles,
      user_permissions AS permissions,
      user_menu_ids AS menu_ids,
      extract(epoch FROM now())::integer + jwt_exp_seconds AS exp,
      extract(epoch FROM now())::integer AS iat,
      'postgrest' AS iss,
      'access' AS token_type,
      is_superuser
  ) r;

  refresh_token_id := gen_random_uuid();
  SELECT sign(row_to_json(r), jwt_secret) INTO refresh_token
  FROM (
    SELECT
      get_token.username AS username,
      extract(epoch FROM now())::integer + jwt_refresh_exp_seconds AS exp,
      extract(epoch FROM now())::integer AS iat,
      'postgrest' AS iss,
      'refresh' AS token_type,
      refresh_token_id AS jti
  ) r;

  INSERT INTO postgrest.refresh_tokens (id, username, token_hash, expires_at)
  VALUES (
    refresh_token_id,
    get_token.username,
    encode(digest(refresh_token, 'sha256'), 'hex'),
    to_timestamp(extract(epoch FROM now()) + jwt_refresh_exp_seconds)
  );

  RETURN json_build_object(
    'success', true,
    'message', '登录成功',
    'access_token', access_token,
    'refresh_token', refresh_token,
    'access_expires_in', jwt_exp_seconds,
    'refresh_expires_in', jwt_refresh_exp_seconds,
    'username', get_token.username,
    'roles', user_roles,
    'permissions', user_permissions,
    'menu_ids', user_menu_ids,
    'menus', user_menus,
    'is_superuser', is_superuser,
    'user_info', json_build_object(
      'username', user_record.username,
      'email', user_record.email,
      'full_name', user_record.full_name,
      'display_name', user_record.display_name,
      'is_active', user_record.is_active,
      'created_at', user_record.created_at,
      'updated_at', user_record.updated_at,
      'menu_ids', user_menu_ids,
      'menus', user_menus
    )
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'anon') THEN
    GRANT EXECUTE ON FUNCTION postgrest.get_token(text, text) TO anon;
    GRANT EXECUTE ON FUNCTION postgrest.get_menu_ids_for_roles(text[]) TO anon;
    GRANT EXECUTE ON FUNCTION postgrest.get_menus_for_roles(text[]) TO anon;
  END IF;
  IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'authenticator') THEN
    GRANT EXECUTE ON FUNCTION postgrest.get_token(text, text) TO authenticator;
    GRANT EXECUTE ON FUNCTION postgrest.get_menu_ids_for_roles(text[]) TO authenticator;
    GRANT EXECUTE ON FUNCTION postgrest.get_menus_for_roles(text[]) TO authenticator;
  END IF;
END $$;
`
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("patch get_token with menu permissions failed: %w", err)
	}

	log.Println("PostgREST token functions patched")
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
