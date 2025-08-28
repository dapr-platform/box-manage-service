package main

import (
	"box-manage-service/api"
	"box-manage-service/config"
	_ "box-manage-service/docs"
	"box-manage-service/service"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	daprd "github.com/dapr/go-sdk/service/http"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

var (
	BASE_CONTEXT = ""
)

func init() {
	if val := os.Getenv("BASE_CONTEXT"); val != "" {
		BASE_CONTEXT = val
	}
}

// @title AI盒子管理系统 API
// @version 1.0
// @description AI盒子管理系统后端服务，提供盒子管理、模型管理、任务管理、用户管理等功能
// @BasePath /swagger/box-manage-service
func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 打印配置信息（在非生产环境）
	if !cfg.IsProduction() {
		cfg.PrintConfig()
	}

	// 初始化数据库连接
	db, err := config.InitDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 确保在程序退出时关闭数据库连接
	defer func() {
		if err := config.CloseDatabase(db); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// 执行数据库迁移
	if err := config.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	mux := chi.NewRouter()

	var conversionService service.ConversionService

	// 如果有BASE_CONTEXT，则在该路径下挂载所有路由
	if BASE_CONTEXT != "" {
		mux.Route(BASE_CONTEXT, func(r chi.Router) {
			// 创建子路由器并初始化路由，传递数据库连接和配置
			subMux := r.(*chi.Mux)
			conversionService = api.InitRoute(subMux, db, cfg)
			r.Handle("/metrics", promhttp.Handler())
			r.Handle("/swagger*", httpSwagger.WrapHandler)
		})
	} else {
		conversionService = api.InitRoute(mux, db, cfg)
		mux.Handle("/metrics", promhttp.Handler())
		mux.Handle("/swagger*", httpSwagger.WrapHandler)
	}

	// 程序启动时恢复转换任务
	if conversionService != nil {
		ctx := context.Background()
		log.Printf("Starting conversion service initialization...")

		// 检查运行中任务的超时情况
		if err := conversionService.CheckRunningTasksTimeout(ctx); err != nil {
			log.Printf("Failed to check running tasks timeout: %v", err)
		}

		// 恢复等待中的转换任务
		if err := conversionService.RecoverPendingTasks(ctx); err != nil {
			log.Printf("Failed to recover pending conversion tasks: %v", err)
		}

		log.Printf("Conversion service initialization completed")
	}

	// 使用配置中的端口，如果有LISTEN_PORT环境变量则覆盖
	port := cfg.Server.Port
	if val := os.Getenv("LISTEN_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			port = p
		}
	}

	s := daprd.NewServiceWithMux(":"+strconv.Itoa(port), mux)
	log.Printf("Starting server on port %d", port)

	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error: %v", err)
	}
}
