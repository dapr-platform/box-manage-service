/*
 * @module service/sse_service
 * @description Server-Sent Events (SSE) 服务 - 实现实时状态同步和事件推送
 * @architecture 服务层
 * @documentReference REQ-003: 模型转换功能, REQ-005: 任务管理功能
 * @stateFlow 客户端连接 -> 流订阅 -> 实时推送 -> 连接管理
 * @rules 支持转换任务状态、任务状态、盒子状态、系统事件的实时推送
 * @dependencies net/http, sync, context
 * @refs REQ-003.md, REQ-005.md
 */

package service

import (
	"box-manage-service/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEEventType SSE事件类型
type SSEEventType string

const (
	// 转换相关事件
	SSEEventTypeConversionStarted   SSEEventType = "conversion_started"
	SSEEventTypeConversionCompleted SSEEventType = "conversion_completed"
	SSEEventTypeConversionFailed    SSEEventType = "conversion_failed"
	SSEEventTypeConversionUpdate    SSEEventType = "conversion_task_update"

	// 抽帧任务相关事件
	SSEEventTypeExtractTaskCreated   SSEEventType = "extract_task_created"
	SSEEventTypeExtractTaskStarted   SSEEventType = "extract_task_started"
	SSEEventTypeExtractTaskCompleted SSEEventType = "extract_task_completed"
	SSEEventTypeExtractTaskFailed    SSEEventType = "extract_task_failed"
	SSEEventTypeExtractTaskStopped   SSEEventType = "extract_task_stopped"
	SSEEventTypeExtractTaskUpdate    SSEEventType = "extract_task_update"

	// 录制任务相关事件
	SSEEventTypeRecordTaskCreated   SSEEventType = "record_task_created"
	SSEEventTypeRecordTaskStarted   SSEEventType = "record_task_started"
	SSEEventTypeRecordTaskCompleted SSEEventType = "record_task_completed"
	SSEEventTypeRecordTaskFailed    SSEEventType = "record_task_failed"
	SSEEventTypeRecordTaskStopped   SSEEventType = "record_task_stopped"
	SSEEventTypeRecordTaskUpdate    SSEEventType = "record_task_update"

	// 部署任务相关事件
	SSEEventTypeDeploymentTaskCreated   SSEEventType = "deployment_task_created"
	SSEEventTypeDeploymentTaskDeployed  SSEEventType = "deployment_task_deployed"
	SSEEventTypeDeploymentTaskCompleted SSEEventType = "deployment_task_completed"
	SSEEventTypeDeploymentTaskFailed    SSEEventType = "deployment_task_failed"
	SSEEventTypeDeploymentTaskPaused    SSEEventType = "deployment_task_paused"
	SSEEventTypeDeploymentTaskResumed   SSEEventType = "deployment_task_resumed"
	SSEEventTypeDeploymentTaskCancelled SSEEventType = "deployment_task_cancelled"
	SSEEventTypeDeploymentTaskUpdate    SSEEventType = "deployment_task_update"

	// 批量部署任务相关事件
	SSEEventTypeBatchDeploymentCreated   SSEEventType = "batch_deployment_created"
	SSEEventTypeBatchDeploymentStarted   SSEEventType = "batch_deployment_started"
	SSEEventTypeBatchDeploymentProgress  SSEEventType = "batch_deployment_progress"
	SSEEventTypeBatchDeploymentCompleted SSEEventType = "batch_deployment_completed"
	SSEEventTypeBatchDeploymentFailed    SSEEventType = "batch_deployment_failed"
	SSEEventTypeBatchDeploymentCancelled SSEEventType = "batch_deployment_cancelled"
	SSEEventTypeBatchDeploymentUpdate    SSEEventType = "batch_deployment_update"

	// 模型部署任务相关事件
	SSEEventTypeModelDeploymentTaskCreated   SSEEventType = "model_deployment_task_created"
	SSEEventTypeModelDeploymentTaskStarted   SSEEventType = "model_deployment_task_started"
	SSEEventTypeModelDeploymentTaskProgress  SSEEventType = "model_deployment_task_progress"
	SSEEventTypeModelDeploymentTaskCompleted SSEEventType = "model_deployment_task_completed"
	SSEEventTypeModelDeploymentTaskFailed    SSEEventType = "model_deployment_task_failed"
	SSEEventTypeModelDeploymentTaskCancelled SSEEventType = "model_deployment_task_cancelled"
	SSEEventTypeModelDeploymentTaskUpdate    SSEEventType = "model_deployment_task_update"

	// 盒子相关事件
	SSEEventTypeBoxOnline  SSEEventType = "box_online"
	SSEEventTypeBoxOffline SSEEventType = "box_offline"
	SSEEventTypeBoxUpdate  SSEEventType = "box_update"

	// 模型相关事件
	SSEEventTypeModelDeployed SSEEventType = "model_deployed"

	// 扫描发现事件
	SSEEventTypeDiscoveryStarted   SSEEventType = "discovery_started"
	SSEEventTypeDiscoveryCompleted SSEEventType = "discovery_completed"
	SSEEventTypeDiscoveryFailed    SSEEventType = "discovery_failed"
	SSEEventTypeDiscoveryProgress  SSEEventType = "discovery_progress"
	SSEEventTypeDiscoveryResult    SSEEventType = "discovery_result"

	// 系统事件
	SSEEventTypeSystemError SSEEventType = "system_error"
	SSEEventTypeHeartbeat   SSEEventType = "heartbeat"
	SSEEventTypeConnected   SSEEventType = "connected"
)

// SSEMessage SSE消息格式
type SSEMessage struct {
	Event     SSEEventType `json:"event"`
	Data      interface{}  `json:"data"`
	Timestamp time.Time    `json:"timestamp"`
	ID        string       `json:"id,omitempty"`
}

// SystemEvent 系统事件
type SystemEvent struct {
	Type      SystemEventType        `json:"type"`
	Level     EventLevel             `json:"level"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	SourceID  string                 `json:"source_id"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SystemEventType 系统事件类型
type SystemEventType string

const (
	SystemEventTypeConversionStarted   SystemEventType = "conversion_started"
	SystemEventTypeConversionCompleted SystemEventType = "conversion_completed"
	SystemEventTypeConversionFailed    SystemEventType = "conversion_failed"
	SystemEventTypeTaskCreated         SystemEventType = "task_created"
	SystemEventTypeTaskDeployed        SystemEventType = "task_deployed"
	SystemEventTypeTaskCompleted       SystemEventType = "task_completed"
	SystemEventTypeTaskFailed          SystemEventType = "task_failed"
	SystemEventTypeBoxOnline           SystemEventType = "box_online"
	SystemEventTypeBoxOffline          SystemEventType = "box_offline"
	SystemEventTypeModelDeployed       SystemEventType = "model_deployed"
	SystemEventTypeSystemError         SystemEventType = "system_error"
	SystemEventTypeDiscoveryStarted    SystemEventType = "discovery_started"
	SystemEventTypeDiscoveryProgress   SystemEventType = "discovery_progress"
	SystemEventTypeDiscoveryCompleted  SystemEventType = "discovery_completed"
	SystemEventTypeDiscoveryFailed     SystemEventType = "discovery_failed"
)

// EventLevel 事件级别
type EventLevel string

const (
	EventLevelInfo    EventLevel = "info"
	EventLevelWarning EventLevel = "warning"
	EventLevelError   EventLevel = "error"
	EventLevelSuccess EventLevel = "success"
)

// SSEClient SSE客户端连接
type SSEClient struct {
	ID             string
	UserID         string
	Channel        string // conversion_tasks, tasks, boxes, system
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	Flusher        http.Flusher
	Context        context.Context
	Cancel         context.CancelFunc
	ConnectedAt    time.Time
	LastPingAt     time.Time
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	TotalConnections     int            `json:"total_connections"`
	ConnectionsByChannel map[string]int `json:"connections_by_channel"`
	ConnectionsByUserID  map[string]int `json:"connections_by_user_id"`
	ActiveSince          time.Time      `json:"active_since"`
}

// DiscoveryProgress 扫描进度信息
type DiscoveryProgress struct {
	ScanID        string    `json:"scan_id"`        // 扫描任务ID
	IPRange       string    `json:"ip_range"`       // 扫描范围
	Port          int       `json:"port"`           // 扫描端口
	TotalIPs      int       `json:"total_ips"`      // 总IP数量
	ScannedIPs    int       `json:"scanned_ips"`    // 已扫描IP数量
	FoundBoxes    int       `json:"found_boxes"`    // 发现的盒子数量
	Status        string    `json:"status"`         // 扫描状态：scanning, completed, failed
	Progress      float64   `json:"progress"`       // 进度百分比 0-100
	CurrentIP     string    `json:"current_ip"`     // 当前扫描的IP
	StartTime     time.Time `json:"start_time"`     // 开始时间
	UpdateTime    time.Time `json:"update_time"`    // 更新时间
	EstimatedTime int       `json:"estimated_time"` // 预计剩余时间(秒)
	ErrorMessage  string    `json:"error_message"`  // 错误信息
}

// DiscoveryResult 扫描结果
type DiscoveryResult struct {
	ScanID          string        `json:"scan_id"`          // 扫描任务ID
	IPRange         string        `json:"ip_range"`         // 扫描范围
	Port            int           `json:"port"`             // 扫描端口
	Status          string        `json:"status"`           // 完成状态：completed, failed
	TotalIPs        int           `json:"total_ips"`        // 总IP数量
	ScannedIPs      int           `json:"scanned_ips"`      // 已扫描IP数量
	FoundBoxes      int           `json:"found_boxes"`      // 发现的盒子数量
	NewBoxes        int           `json:"new_boxes"`        // 新盒子数量
	ExistingBoxes   int           `json:"existing_boxes"`   // 已存在盒子数量
	DiscoveredBoxes []interface{} `json:"discovered_boxes"` // 发现的盒子列表
	StartTime       time.Time     `json:"start_time"`       // 开始时间
	EndTime         time.Time     `json:"end_time"`         // 结束时间
	Duration        int           `json:"duration"`         // 扫描耗时(秒)
	ErrorMessage    string        `json:"error_message"`    // 错误信息
}

// sseService SSE服务实现
type sseService struct {
	clients     map[string]*SSEClient
	clientMutex sync.RWMutex
	channels    map[string][]*SSEClient // channel -> clients
	startTime   time.Time
}

// NewSSEService 创建SSE服务实例
func NewSSEService() SSEService {
	return &sseService{
		clients:   make(map[string]*SSEClient),
		channels:  make(map[string][]*SSEClient),
		startTime: time.Now(),
	}
}

// HandleConversionEvents 处理转换事件流
func (s *sseService) HandleConversionEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "conversion")
}

// HandleExtractTaskEvents 处理抽帧任务事件流
func (s *sseService) HandleExtractTaskEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "extract-tasks")
}

// HandleRecordTaskEvents 处理录制任务事件流
func (s *sseService) HandleRecordTaskEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "record-tasks")
}

// HandleDeploymentTaskEvents 处理部署任务事件流
func (s *sseService) HandleDeploymentTaskEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "deployment-tasks")
}

// HandleBatchDeploymentEvents 处理批量部署事件流
func (s *sseService) HandleBatchDeploymentEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "batch-deployments")
}

// HandleModelDeploymentEvents 处理模型部署事件流
func (s *sseService) HandleModelDeploymentEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "model-deployments")
}

// HandleBoxEvents 处理盒子事件流
func (s *sseService) HandleBoxEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "boxes")
}

// HandleModelEvents 处理模型事件流
func (s *sseService) HandleModelEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "models")
}

// HandleDiscoveryEvents 处理扫描事件流
func (s *sseService) HandleDiscoveryEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "discovery")
}

// HandleSystemEvents 处理系统事件流
func (s *sseService) HandleSystemEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "system")
}

// handleSSEConnection 通用SSE连接处理
func (s *sseService) handleSSEConnection(w http.ResponseWriter, r *http.Request, channel string) error {
	// 检查是否支持SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("server does not support Server-Sent Events")
	}

	// 设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// 创建客户端上下文
	ctx, cancel := context.WithCancel(r.Context())

	// 创建客户端
	client := &SSEClient{
		ID:             s.generateClientID(),
		UserID:         r.URL.Query().Get("user_id"),
		Channel:        channel,
		ResponseWriter: w,
		Request:        r,
		Flusher:        flusher,
		Context:        ctx,
		Cancel:         cancel,
		ConnectedAt:    time.Now(),
		LastPingAt:     time.Now(),
	}

	// 注册客户端
	s.registerClient(client)
	defer s.unregisterClient(client.ID)

	log.Printf("[SSE] Client connected - ID: %s, Channel: %s, UserID: %s", client.ID, channel, client.UserID)

	// 发送连接确认消息
	s.sendToClient(client, SSEMessage{
		Event: SSEEventTypeConnected,
		Data: map[string]interface{}{
			"type":      "connected",
			"channel":   channel,
			"client_id": client.ID,
			"message":   fmt.Sprintf("已连接到%s事件流", channel),
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("connect_%s_%d", client.ID, time.Now().Unix()),
	})

	// 启动心跳
	go s.heartbeatLoop(client)

	// 保持连接直到客户端断开
	<-ctx.Done()

	log.Printf("[SSE] Client disconnected - ID: %s, Channel: %s", client.ID, channel)
	return nil
}

// =================== 转换相关事件广播方法 ===================

// BroadcastConversionStarted 广播转换开始事件
func (s *sseService) BroadcastConversionStarted(taskID string, modelName string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":    taskID,
		"model_name": modelName,
		"timestamp":  time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeConversionStarted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("conversion_started_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("conversion", message)
}

// BroadcastConversionCompleted 广播转换完成事件
func (s *sseService) BroadcastConversionCompleted(taskID string, modelName string, outputPath string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":     taskID,
		"model_name":  modelName,
		"output_path": outputPath,
		"timestamp":   time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeConversionCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("conversion_completed_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("conversion", message)
}

// BroadcastConversionFailed 广播转换失败事件
func (s *sseService) BroadcastConversionFailed(taskID string, modelName string, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":    taskID,
		"model_name": modelName,
		"error":      errorMsg,
		"timestamp":  time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeConversionFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("conversion_failed_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("conversion", message)
}

// BroadcastConversionTaskUpdate 广播转换任务状态更新
func (s *sseService) BroadcastConversionTaskUpdate(task *models.ConversionTask) error {
	message := SSEMessage{
		Event:     SSEEventTypeConversionUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("conv_task_%s_%d", task.TaskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("conversion", message)
}

// =================== 抽帧任务相关事件广播方法 ===================

// BroadcastExtractTaskCreated 广播抽帧任务创建事件
func (s *sseService) BroadcastExtractTaskCreated(task *models.ExtractTask, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task":      task,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeExtractTaskCreated,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("extract_task_created_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("extract-tasks", message)
}

// BroadcastExtractTaskStarted 广播抽帧任务开始事件
func (s *sseService) BroadcastExtractTaskStarted(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeExtractTaskStarted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("extract_task_started_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("extract-tasks", message)
}

// BroadcastExtractTaskCompleted 广播抽帧任务完成事件
func (s *sseService) BroadcastExtractTaskCompleted(taskID uint, result string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"result":    result,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeExtractTaskCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("extract_task_completed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("extract-tasks", message)
}

// BroadcastExtractTaskFailed 广播抽帧任务失败事件
func (s *sseService) BroadcastExtractTaskFailed(taskID uint, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeExtractTaskFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("extract_task_failed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("extract-tasks", message)
}

// BroadcastExtractTaskStopped 广播抽帧任务停止事件
func (s *sseService) BroadcastExtractTaskStopped(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeExtractTaskStopped,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("extract_task_stopped_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("extract-tasks", message)
}

// BroadcastExtractTaskUpdate 广播抽帧任务状态更新
func (s *sseService) BroadcastExtractTaskUpdate(task *models.ExtractTask) error {
	message := SSEMessage{
		Event:     SSEEventTypeExtractTaskUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("extract_task_update_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("extract-tasks", message)
}

// =================== 盒子相关事件广播方法 ===================

// BroadcastBoxOnline 广播盒子上线事件
func (s *sseService) BroadcastBoxOnline(boxID uint, boxName string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"box_id":    boxID,
		"box_name":  boxName,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBoxOnline,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("box_online_%d_%d", boxID, time.Now().Unix()),
	}

	return s.broadcastToChannel("boxes", message)
}

// BroadcastBoxOffline 广播盒子离线事件
func (s *sseService) BroadcastBoxOffline(boxID uint, boxName string, reason string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"box_id":    boxID,
		"box_name":  boxName,
		"reason":    reason,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBoxOffline,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("box_offline_%d_%d", boxID, time.Now().Unix()),
	}

	return s.broadcastToChannel("boxes", message)
}

// BroadcastBoxUpdate 广播盒子状态更新
func (s *sseService) BroadcastBoxUpdate(box *models.Box) error {
	message := SSEMessage{
		Event:     SSEEventTypeBoxUpdate,
		Data:      box,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("box_%d_%d", box.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("boxes", message)
}

// =================== 模型相关事件广播方法 ===================

// BroadcastModelDeployed 广播模型部署事件
func (s *sseService) BroadcastModelDeployed(modelKey string, boxID uint, boxName string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"model_key": modelKey,
		"box_id":    boxID,
		"box_name":  boxName,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeployed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployed_%s_%d_%d", modelKey, boxID, time.Now().Unix()),
	}

	return s.broadcastToChannel("models", message)
}

// =================== 扫描发现事件广播方法 ===================

// BroadcastDiscoveryStarted 广播扫描开始事件
func (s *sseService) BroadcastDiscoveryStarted(scanID string, ipRange string, port int, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"scan_id":   scanID,
		"ip_range":  ipRange,
		"port":      port,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDiscoveryStarted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("discovery_started_%s_%d", scanID, time.Now().Unix()),
	}

	return s.broadcastToChannel("discovery", message)
}

// BroadcastDiscoveryCompleted 广播扫描完成事件
func (s *sseService) BroadcastDiscoveryCompleted(scanID string, foundBoxes int, newBoxes int, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"scan_id":     scanID,
		"found_boxes": foundBoxes,
		"new_boxes":   newBoxes,
		"timestamp":   time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDiscoveryCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("discovery_completed_%s_%d", scanID, time.Now().Unix()),
	}

	return s.broadcastToChannel("discovery", message)
}

// BroadcastDiscoveryFailed 广播扫描失败事件
func (s *sseService) BroadcastDiscoveryFailed(scanID string, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"scan_id":   scanID,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDiscoveryFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("discovery_failed_%s_%d", scanID, time.Now().Unix()),
	}

	return s.broadcastToChannel("discovery", message)
}

// BroadcastDiscoveryProgress 广播扫描进度
func (s *sseService) BroadcastDiscoveryProgress(progress *DiscoveryProgress) error {
	message := SSEMessage{
		Event:     SSEEventTypeDiscoveryProgress,
		Data:      progress,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("discovery_progress_%s_%d", progress.ScanID, time.Now().Unix()),
	}

	return s.broadcastToChannel("discovery", message)
}

// BroadcastDiscoveryResult 广播扫描结果
func (s *sseService) BroadcastDiscoveryResult(result *DiscoveryResult) error {
	message := SSEMessage{
		Event:     SSEEventTypeDiscoveryResult,
		Data:      result,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("discovery_result_%s_%d", result.ScanID, time.Now().Unix()),
	}

	return s.broadcastToChannel("discovery", message)
}

// =================== 系统错误事件广播方法 ===================

// BroadcastSystemError 广播系统错误事件
func (s *sseService) BroadcastSystemError(source string, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"source":    source,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeSystemError,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("system_error_%s_%d", source, time.Now().Unix()),
	}

	return s.broadcastToChannel("system", message)
}

// GetConnectionStats 获取连接统计
func (s *sseService) GetConnectionStats() *ConnectionStats {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()

	channelStats := make(map[string]int)
	userStats := make(map[string]int)

	for _, client := range s.clients {
		channelStats[client.Channel]++
		if client.UserID != "" {
			userStats[client.UserID]++
		}
	}

	return &ConnectionStats{
		TotalConnections:     len(s.clients),
		ConnectionsByChannel: channelStats,
		ConnectionsByUserID:  userStats,
		ActiveSince:          s.startTime,
	}
}

// CloseAllConnections 关闭所有连接
func (s *sseService) CloseAllConnections() error {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	for _, client := range s.clients {
		client.Cancel()
	}

	s.clients = make(map[string]*SSEClient)
	s.channels = make(map[string][]*SSEClient)

	log.Println("[SSE] All connections closed")
	return nil
}

// 私有方法

// registerClient 注册客户端
func (s *sseService) registerClient(client *SSEClient) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	s.clients[client.ID] = client

	// 添加到频道
	if s.channels[client.Channel] == nil {
		s.channels[client.Channel] = make([]*SSEClient, 0)
	}
	s.channels[client.Channel] = append(s.channels[client.Channel], client)
}

// unregisterClient 注销客户端
func (s *sseService) unregisterClient(clientID string) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	client, exists := s.clients[clientID]
	if !exists {
		return
	}

	// 从频道中移除
	if clients, exists := s.channels[client.Channel]; exists {
		for i, c := range clients {
			if c.ID == clientID {
				s.channels[client.Channel] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}

	delete(s.clients, clientID)
}

// broadcastToChannel 向指定频道广播消息
func (s *sseService) broadcastToChannel(channel string, message SSEMessage) error {
	s.clientMutex.RLock()
	clients := s.channels[channel]
	s.clientMutex.RUnlock()

	if len(clients) == 0 {
		log.Printf("[SSE] No clients subscribed to channel %s, message not sent - Event: %s, ID: %s",
			channel, message.Event, message.ID)
		return nil
	}

	log.Printf("[SSE] Broadcasting to channel %s - %d clients, Event: %s", channel, len(clients), message.Event)

	for _, client := range clients {
		go s.sendToClient(client, message)
	}

	return nil
}

// sendToClient 向单个客户端发送消息
func (s *sseService) sendToClient(client *SSEClient, message SSEMessage) {
	select {
	case <-client.Context.Done():
		return
	default:
	}

	// 序列化消息数据
	data, err := json.Marshal(message.Data)
	if err != nil {
		log.Printf("[SSE] Failed to marshal message data for client %s: %v", client.ID, err)
		return
	}

	// 发送SSE格式的消息
	if message.ID != "" {
		fmt.Fprintf(client.ResponseWriter, "id: %s\n", message.ID)
	}
	fmt.Fprintf(client.ResponseWriter, "event: %s\n", message.Event)
	fmt.Fprintf(client.ResponseWriter, "data: %s\n\n", string(data))

	client.Flusher.Flush()
	client.LastPingAt = time.Now()
}

// heartbeatLoop 心跳循环
func (s *sseService) heartbeatLoop(client *SSEClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-client.Context.Done():
			return
		case <-ticker.C:
			heartbeatMsg := SSEMessage{
				Event: SSEEventTypeHeartbeat,
				Data: map[string]interface{}{
					"timestamp":   time.Now(),
					"server_time": time.Now().Unix(),
					"client_id":   client.ID,
					"channel":     client.Channel,
				},
				Timestamp: time.Now(),
				ID:        fmt.Sprintf("heartbeat_%s_%d", client.ID, time.Now().Unix()),
			}

			s.sendToClient(client, heartbeatMsg)
		}
	}
}

// generateClientID 生成客户端ID
func (s *sseService) generateClientID() string {
	return fmt.Sprintf("sse_%d", time.Now().UnixNano())
}

// =================== 临时的旧方法（为了兼容性） ===================

// BroadcastTaskCreated 广播任务创建事件（临时方法）
func (s *sseService) BroadcastTaskCreated(task *models.Task, metadata map[string]interface{}) error {
	// 重定向到部署任务创建事件
	return s.BroadcastDeploymentTaskCreated(task, metadata)
}

// BroadcastTaskCompleted 广播任务完成事件（临时方法）
func (s *sseService) BroadcastTaskCompleted(taskID uint, result string, metadata map[string]interface{}) error {
	// 重定向到部署任务完成事件
	return s.BroadcastDeploymentTaskCompleted(taskID, result, metadata)
}

// BroadcastTaskFailed 广播任务失败事件（临时方法）
func (s *sseService) BroadcastTaskFailed(taskID uint, errorMsg string, metadata map[string]interface{}) error {
	// 重定向到部署任务失败事件
	return s.BroadcastDeploymentTaskFailed(taskID, errorMsg, metadata)
}

// BroadcastTaskDeployed 广播任务部署事件（临时方法）
func (s *sseService) BroadcastTaskDeployed(taskID uint, boxID uint, metadata map[string]interface{}) error {
	// 重定向到部署任务部署事件
	return s.BroadcastDeploymentTaskDeployed(taskID, boxID, metadata)
}

// =================== 录制任务相关事件广播方法 ===================

// BroadcastRecordTaskCreated 广播录制任务创建事件
func (s *sseService) BroadcastRecordTaskCreated(task *models.RecordTask, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task":      task,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeRecordTaskCreated,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("record_task_created_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("record-tasks", message)
}

// BroadcastRecordTaskStarted 广播录制任务开始事件
func (s *sseService) BroadcastRecordTaskStarted(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeRecordTaskStarted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("record_task_started_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("record-tasks", message)
}

// BroadcastRecordTaskCompleted 广播录制任务完成事件
func (s *sseService) BroadcastRecordTaskCompleted(taskID uint, result string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"result":    result,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeRecordTaskCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("record_task_completed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("record-tasks", message)
}

// BroadcastRecordTaskFailed 广播录制任务失败事件
func (s *sseService) BroadcastRecordTaskFailed(taskID uint, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeRecordTaskFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("record_task_failed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("record-tasks", message)
}

// BroadcastRecordTaskStopped 广播录制任务停止事件
func (s *sseService) BroadcastRecordTaskStopped(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeRecordTaskStopped,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("record_task_stopped_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("record-tasks", message)
}

// BroadcastRecordTaskUpdate 广播录制任务状态更新
func (s *sseService) BroadcastRecordTaskUpdate(task *models.RecordTask) error {
	message := SSEMessage{
		Event:     SSEEventTypeRecordTaskUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("record_task_update_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("record-tasks", message)
}

// =================== 部署任务相关事件广播方法 ===================

// BroadcastDeploymentTaskCreated 广播部署任务创建事件
func (s *sseService) BroadcastDeploymentTaskCreated(task *models.Task, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task":      task,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskCreated,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_created_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// BroadcastDeploymentTaskDeployed 广播部署任务部署事件
func (s *sseService) BroadcastDeploymentTaskDeployed(taskID uint, boxID uint, metadata map[string]interface{}) error {
	log.Printf("[SSE] BroadcastDeploymentTaskDeployed called - TaskID: %d, BoxID: %d", taskID, boxID)

	data := map[string]interface{}{
		"task_id":   taskID,
		"box_id":    boxID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskDeployed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_deployed_%d_%d_%d", taskID, boxID, time.Now().Unix()),
	}

	err := s.broadcastToChannel("deployment-tasks", message)
	if err != nil {
		log.Printf("[SSE] BroadcastDeploymentTaskDeployed failed - TaskID: %d, BoxID: %d, Error: %v", taskID, boxID, err)
	}
	return err
}

// BroadcastDeploymentTaskCompleted 广播部署任务完成事件
func (s *sseService) BroadcastDeploymentTaskCompleted(taskID uint, result string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"result":    result,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_completed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// BroadcastDeploymentTaskFailed 广播部署任务失败事件
func (s *sseService) BroadcastDeploymentTaskFailed(taskID uint, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_failed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// BroadcastDeploymentTaskPaused 广播部署任务暂停事件
func (s *sseService) BroadcastDeploymentTaskPaused(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskPaused,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_paused_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// BroadcastDeploymentTaskResumed 广播部署任务恢复事件
func (s *sseService) BroadcastDeploymentTaskResumed(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskResumed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_resumed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// BroadcastDeploymentTaskCancelled 广播部署任务取消事件
func (s *sseService) BroadcastDeploymentTaskCancelled(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskCancelled,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_cancelled_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// BroadcastDeploymentTaskUpdate 广播部署任务状态更新
func (s *sseService) BroadcastDeploymentTaskUpdate(task *models.Task) error {
	message := SSEMessage{
		Event:     SSEEventTypeDeploymentTaskUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("deployment_task_update_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("deployment-tasks", message)
}

// =================== 批量部署任务相关事件广播方法 ===================

// BroadcastBatchDeploymentCreated 广播批量部署任务创建事件
func (s *sseService) BroadcastBatchDeploymentCreated(task *models.DeploymentTask, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task":      task,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentCreated,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_created_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// BroadcastBatchDeploymentStarted 广播批量部署任务开始事件
func (s *sseService) BroadcastBatchDeploymentStarted(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentStarted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_started_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// BroadcastBatchDeploymentProgress 广播批量部署任务进度事件
func (s *sseService) BroadcastBatchDeploymentProgress(taskID uint, progress float64, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"progress":  progress,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentProgress,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_progress_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// BroadcastBatchDeploymentCompleted 广播批量部署任务完成事件
func (s *sseService) BroadcastBatchDeploymentCompleted(taskID uint, result string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"result":    result,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_completed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// BroadcastBatchDeploymentFailed 广播批量部署任务失败事件
func (s *sseService) BroadcastBatchDeploymentFailed(taskID uint, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_failed_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// BroadcastBatchDeploymentCancelled 广播批量部署任务取消事件
func (s *sseService) BroadcastBatchDeploymentCancelled(taskID uint, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentCancelled,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_cancelled_%d_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// BroadcastBatchDeploymentUpdate 广播批量部署任务状态更新
func (s *sseService) BroadcastBatchDeploymentUpdate(task *models.DeploymentTask) error {
	message := SSEMessage{
		Event:     SSEEventTypeBatchDeploymentUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("batch_deployment_update_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("batch-deployments", message)
}

// =================== 模型部署任务相关事件广播方法 ===================

// BroadcastModelDeploymentTaskCreated 广播模型部署任务创建事件
func (s *sseService) BroadcastModelDeploymentTaskCreated(task *models.ModelDeploymentTask, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task":      task,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskCreated,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_created_%s_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}

// BroadcastModelDeploymentTaskStarted 广播模型部署任务开始事件
func (s *sseService) BroadcastModelDeploymentTaskStarted(taskID string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskStarted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_started_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}

// BroadcastModelDeploymentTaskProgress 广播模型部署任务进度事件
func (s *sseService) BroadcastModelDeploymentTaskProgress(taskID string, progress int, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"progress":  progress,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskProgress,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_progress_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}

// BroadcastModelDeploymentTaskCompleted 广播模型部署任务完成事件
func (s *sseService) BroadcastModelDeploymentTaskCompleted(taskID string, result string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"result":    result,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskCompleted,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_completed_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}

// BroadcastModelDeploymentTaskFailed 广播模型部署任务失败事件
func (s *sseService) BroadcastModelDeploymentTaskFailed(taskID string, errorMsg string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"error":     errorMsg,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskFailed,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_failed_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}

// BroadcastModelDeploymentTaskCancelled 广播模型部署任务取消事件
func (s *sseService) BroadcastModelDeploymentTaskCancelled(taskID string, metadata map[string]interface{}) error {
	data := map[string]interface{}{
		"task_id":   taskID,
		"timestamp": time.Now(),
	}
	if metadata != nil {
		for k, v := range metadata {
			data[k] = v
		}
	}

	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskCancelled,
		Data:      data,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_cancelled_%s_%d", taskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}

// BroadcastModelDeploymentTaskUpdate 广播模型部署任务状态更新
func (s *sseService) BroadcastModelDeploymentTaskUpdate(task *models.ModelDeploymentTask) error {
	message := SSEMessage{
		Event:     SSEEventTypeModelDeploymentTaskUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("model_deployment_task_update_%s_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("model-deployments", message)
}
