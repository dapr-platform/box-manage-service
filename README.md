# AI 盒子管理系统后端服务

AI 盒子管理系统后端服务，提供盒子管理、模型管理、任务管理、视频管理等功能。

## 项目概述

基于 DESIGN-000.md 设计文档实现的 AI 盒子管理系统后端，采用前后端分离架构，后端使用 Golang + PostgreSQL + PostgREST RBAC 权限系统。

## 功能特性

- **盒子管理**: AI 盒子注册、监控、控制
- **模型管理**: 模型上传、转换、部署、版本管理
- **任务管理**: 任务调度、监控、控制
- **视频管理**: 视频上传、处理、分析
- **监控告警**: 系统监控、性能指标、告警管理
- **权限管理**: 基于 PostgREST RBAC 的完整权限控制

## 技术栈

- **编程语言**: Go 1.23.1
- **Web 框架**: Chi Router
- **数据库**: PostgreSQL 14+
- **ORM**: GORM
- **权限管理**: PostgREST RBAC
- **API 文档**: Swagger
- **容器**: Docker

## 项目结构

```
box-manage-service/
├── api/                    # API层
│   ├── controllers/        # 控制器
│   └── routes.go          # 路由配置
├── models/                # 数据模型
├── service/               # 业务服务层
├── docs/                  # API文档
├── Dockerfile            # Docker构建文件
├── go.mod               # Go模块文件
├── main.go              # 主入口文件
└── README.md            # 项目说明
```

## 快速开始

### 环境要求

- Go 1.23.1+
- PostgreSQL 14+
- PostgREST (用于权限管理)

### 环境变量配置

```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=box_management
DB_SSLMODE=disable

# PostgREST配置
POSTGREST_URL=http://localhost:3000
JWT_SECRET=your_jwt_secret

# 服务配置
LISTEN_PORT=80
BASE_CONTEXT=/api/v1
```

### 运行服务

```bash
# 克隆项目
git clone <repository-url>
cd box-manage-service

# 安装依赖
go mod tidy

# 运行服务
go run main.go
```

### Docker 运行

```bash
# 构建镜像
docker build -t box-manage-service .

# 运行容器
docker run -p 80:80 \
  -e DB_HOST=your_db_host \
  -e DB_PASSWORD=your_password \
  box-manage-service
```

## API 文档

服务启动后，可以通过以下地址访问 API 文档：

- Swagger UI: http://localhost/swagger/index.html
- API JSON: http://localhost/swagger/doc.json

## 主要 API 端点

### 盒子管理

- `GET /boxes` - 获取盒子列表
- `POST /boxes` - 创建盒子
- `GET /boxes/{id}` - 获取盒子详情
- `PUT /boxes/{id}` - 更新盒子信息
- `DELETE /boxes/{id}` - 删除盒子
- `POST /boxes/heartbeat` - 盒子心跳
- `GET /boxes/{id}/status` - 获取盒子状态
- `GET /boxes/{id}/resources` - 获取资源使用情况

### 模型管理

- `GET /models` - 获取模型列表
- `POST /models` - 创建模型
- `POST /models/upload` - 上传模型文件
- `POST /models/{id}/convert` - 转换模型
- `POST /models/{id}/deploy` - 部署模型

### 任务管理

- `GET /tasks` - 获取任务列表
- `POST /tasks` - 创建任务
- `POST /tasks/{id}/start` - 启动任务
- `POST /tasks/{id}/stop` - 停止任务
- `GET /tasks/{id}/status` - 获取任务状态

### 监控管理

- `GET /monitoring/metrics` - 获取系统指标
- `GET /monitoring/alerts` - 获取告警列表
- `GET /monitoring/logs` - 获取系统日志

## 权限系统

本系统使用 PostgREST RBAC 权限系统，支持：

- 基于角色的权限控制
- 细粒度资源访问控制
- JWT Token 认证
- 行级安全策略

### 主要权限类型

- `box:*` - 盒子管理权限
- `model:*` - 模型管理权限
- `task:*` - 任务管理权限
- `video:*` - 视频管理权限
- `system:*` - 系统管理权限

## 开发指南

### 添加新的 API 端点

1. 在`api/controllers/`中创建控制器
2. 在`api/routes.go`中注册路由
3. 在`models/`中定义数据模型
4. 在`service/`中实现业务逻辑
5. 添加 Swagger 注释

### 数据库迁移

```bash
# 自动迁移将在服务启动时执行
# 手动执行迁移（如果需要）
go run scripts/migrate.go
```

## 部署指南

### 单机部署

使用 Docker Compose 进行单机部署，配置文件参考`deploy/docker-compose.yml`

### 集群部署

支持 Kubernetes 集群部署，配置文件参考`deploy/k8s/`

## 监控运维

### 健康检查

- `/health` - 基础健康检查
- `/ready` - 就绪检查
- `/metrics` - Prometheus 指标

### 日志管理

- 结构化日志输出
- 支持多种日志级别
- 集成 ELK 日志收集

## 注意事项

1. **权限验证**: 所有 API 调用都需要通过 PostgREST RBAC 权限验证
2. **数据安全**: 敏感数据加密存储，传输使用 HTTPS
3. **性能优化**: 支持分页查询、缓存优化、连接池管理
4. **错误处理**: 统一的错误响应格式和错误码

## 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 联系方式

如有问题或建议，请通过以下方式联系：

- 项目 Issues: [GitHub Issues](https://github.com/your-org/box-manage-service/issues)
- 邮箱: your-email@example.com



# box-manage-service
