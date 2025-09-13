# 监控 API 测试示例

## API 端点列表

### 1. 系统概览

```bash
GET /monitoring/overview
```

返回系统的整体概览，包括盒子状态、任务统计、模型信息、视频处理等。

### 2. 系统指标

```bash
GET /monitoring/metrics
```

返回详细的系统监控指标。

### 3. 性能指标

```bash
GET /monitoring/performance
```

返回系统性能相关指标。

### 4. 资源使用情况

```bash
GET /monitoring/resource-usage
```

返回资源使用情况统计。

### 5. 盒子指标

```bash
GET /monitoring/boxes/{id}/metrics
```

获取指定盒子的监控指标。

### 6. 任务指标

```bash
GET /monitoring/tasks/metrics
```

获取任务相关的统计指标。

### 7. 告警管理

```bash
GET /monitoring/alerts
POST /monitoring/alerts/{id}/acknowledge
```

获取告警列表和确认告警。

### 8. 系统日志

```bash
GET /monitoring/logs
```

获取系统日志。

### 9. 健康检查

```bash
GET /monitoring/health-check
```

系统健康检查。

## 响应示例

### 系统概览响应示例

```json
{
  "code": 0,
  "message": "获取系统概览成功",
  "data": {
    "timestamp": "2025-01-26T12:00:00Z",
    "boxes": {
      "total": 10,
      "online": 8,
      "offline": 2,
      "distribution": {
        "BM1684X": 5,
        "BM1688": 3,
        "CPU": 2
      },
      "resources": {
        "avg_cpu_usage": 45.2,
        "avg_memory_usage": 67.8,
        "avg_temperature": 42.5,
        "total_memory_gb": 128.0
      }
    },
    "tasks": {
      "total": 25,
      "running": 8,
      "completed": 15,
      "failed": 2,
      "pending": 0,
      "performance": {
        "avg_fps": 25.5,
        "avg_latency_ms": 150.2,
        "total_processed_frames": 1250000,
        "total_inference_count": 125000,
        "success_rate": 96.5
      }
    },
    "models": {
      "original": 50,
      "converted": 120,
      "conversion_tasks": {
        "total": 30,
        "running": 2,
        "completed": 25,
        "failed": 3,
        "success_rate": 89.3
      },
      "deployments": 85,
      "storage_usage_mb": 25600
    },
    "videos": {
      "sources": 15,
      "files": 200,
      "record_tasks": {
        "total": 12,
        "recording": 3,
        "completed": 8,
        "failed": 1
      },
      "extract_tasks": {
        "total": 25,
        "running": 2,
        "completed": 20,
        "failed": 3
      },
      "monitoring": true
    },
    "system": {
      "memory_usage_mb": 2048,
      "active_goroutines": 150,
      "average_response_time_ms": 25,
      "total_polls": 15000,
      "successful_polls": 14850,
      "failed_polls": 150,
      "cpu_usage_percent": 35.5
    },
    "services": {
      "box_monitoring": true,
      "task_executor": true,
      "video_monitoring": true,
      "sse_connections": 5
    }
  },
  "timestamp": "2025-01-26T12:00:00Z"
}
```

## 测试命令

```bash
# 测试系统概览
curl -X GET "http://localhost:9000/monitoring/overview" -H "accept: application/json"

# 测试系统指标
curl -X GET "http://localhost:9000/monitoring/metrics" -H "accept: application/json"

# 测试性能指标
curl -X GET "http://localhost:9000/monitoring/performance" -H "accept: application/json"

# 测试健康检查
curl -X GET "http://localhost:9000/monitoring/health-check" -H "accept: application/json"
```
