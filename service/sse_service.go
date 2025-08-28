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
	SSEEventTypeConversionTaskUpdate SSEEventType = "conversion_task_update"
	SSEEventTypeTaskUpdate           SSEEventType = "task_update"
	SSEEventTypeBoxUpdate            SSEEventType = "box_update"
	SSEEventTypeSystemEvent          SSEEventType = "system_event"
	SSEEventTypeHeartbeat            SSEEventType = "heartbeat"
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

// HandleConversionTaskEvents 处理转换任务事件流
func (s *sseService) HandleConversionTaskEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "conversion_tasks")
}

// HandleTaskEvents 处理任务事件流
func (s *sseService) HandleTaskEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "tasks")
}

// HandleBoxEvents 处理盒子事件流
func (s *sseService) HandleBoxEvents(w http.ResponseWriter, r *http.Request) error {
	return s.handleSSEConnection(w, r, "boxes")
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
		Event: SSEEventTypeSystemEvent,
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

// BroadcastConversionTaskUpdate 广播转换任务状态更新
func (s *sseService) BroadcastConversionTaskUpdate(task *models.ConversionTask) error {
	message := SSEMessage{
		Event:     SSEEventTypeConversionTaskUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("conv_task_%s_%d", task.TaskID, time.Now().Unix()),
	}

	return s.broadcastToChannel("conversion_tasks", message)
}

// BroadcastTaskUpdate 广播任务状态更新
func (s *sseService) BroadcastTaskUpdate(task *models.Task) error {
	message := SSEMessage{
		Event:     SSEEventTypeTaskUpdate,
		Data:      task,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("task_%d_%d", task.ID, time.Now().Unix()),
	}

	return s.broadcastToChannel("tasks", message)
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

// BroadcastSystemEvent 广播系统事件
func (s *sseService) BroadcastSystemEvent(event *SystemEvent) error {
	message := SSEMessage{
		Event:     SSEEventTypeSystemEvent,
		Data:      event,
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("event_%s_%d", event.Type, time.Now().Unix()),
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
		return nil
	}

	log.Printf("[SSE] Broadcasting to channel %s - %d clients", channel, len(clients))

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
