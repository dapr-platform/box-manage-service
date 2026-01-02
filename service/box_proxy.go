/*
 * @module service/box_proxy
 * @description 盒子代理服务，统一管理与盒子的API调用
 * @architecture 服务层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow API请求 -> 路由到盒子 -> 错误处理 -> 重试机制 -> 响应返回
 * @rules 提供统一的盒子API调用接口、连接池管理、错误处理和重试机制
 * @dependencies gorm.io/gorm, net/http
 * @refs DESIGN-001.md, apis.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// BoxProxyService 盒子代理服务
type BoxProxyService struct {
	repoManager repository.RepositoryManager
	httpClient  *http.Client
}

// NewBoxProxyService 创建盒子代理服务
func NewBoxProxyService(repoManager repository.RepositoryManager) *BoxProxyService {
	return &BoxProxyService{
		repoManager: repoManager,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // 设置为10分钟以支持大模型文件上传
		},
	}
}

// APIResponse 通用API响应（用于向后兼容）
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Code      int         `json:"code,omitempty"`
}

// 健康检查响应
type HealthResponse struct {
	Service   string `json:"service"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

// 系统版本响应
type SystemVersionResponse struct {
	Service    string `json:"service"`
	Version    string `json:"version"`
	BuildTime  string `json:"build_time"`
	ApiVersion string `json:"api_version"`
	Timestamp  string `json:"timestamp"`
}

// Meta响应
type MetaResponse struct {
	Success   bool     `json:"success"`
	Timestamp string   `json:"timestamp"`
	Data      MetaData `json:"data"`
}

type MetaData struct {
	SupportedTypes       []string          `json:"supported_types"`
	SupportedVersions    []string          `json:"supported_versions"`
	SupportedHardware    []string          `json:"supported_hardware"`
	InferenceTypeMapping map[string]string `json:"inference_type_mapping"`
	FileLimits           FileLimits        `json:"file_limits"`
	Defaults             MetaDefaults      `json:"defaults"`
}

type FileLimits struct {
	ModelFileMaxSize      string   `json:"model_file_max_size"`
	ImageFileMaxSize      string   `json:"image_file_max_size"`
	SupportedModelFormats []string `json:"supported_model_formats"`
	SupportedImageFormats []string `json:"supported_image_formats"`
}

type MetaDefaults struct {
	ConfThreshold float64 `json:"conf_threshold"`
	NmsThreshold  float64 `json:"nms_threshold"`
	Type          string  `json:"type"`
	Version       string  `json:"version"`
	Hardware      string  `json:"hardware"`
}

// 模型列表响应
type ModelsResponse struct {
	Success   bool        `json:"success"`
	Timestamp string      `json:"timestamp"`
	Models    []ModelInfo `json:"models"`
}

type ModelInfo struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Version       string  `json:"version"`
	Hardware      string  `json:"hardware"`
	IsLoaded      bool    `json:"is_loaded"`
	ConfThreshold float64 `json:"conf_threshold"`
	NmsThreshold  float64 `json:"nms_threshold"`
	FileSize      int64   `json:"file_size"`
	UploadTime    string  `json:"upload_time"`
	Md5sum        string  `json:"md5sum"`
	BmodelPath    string  `json:"bmodel_path"`
}

// 任务列表响应
type TasksResponse struct {
	Success      bool       `json:"success"`
	Timestamp    string     `json:"timestamp"`
	TotalTasks   int        `json:"total_tasks"`
	RunningTasks int        `json:"running_tasks"`
	Tasks        []TaskInfo `json:"tasks"`
}

type TaskInfo struct {
	TaskID              string `json:"task_id"`
	Status              string `json:"status"`
	AutoStart           bool   `json:"auto_start"`
	RTSPUrl             string `json:"rtsp_url"`
	SkipFrame           int    `json:"skip_frame"`
	InferenceTasksCount int    `json:"inference_tasks_count"`
}

// 任务详情响应
type TaskDetailResponse struct {
	Success   bool           `json:"success"`
	Timestamp string         `json:"timestamp"`
	Task      TaskDetailInfo `json:"task"`
}

type TaskDetailInfo struct {
	TaskID         string                   `json:"taskId"`
	DevID          string                   `json:"devId"`
	RTSPUrl        string                   `json:"rtspUrl"`
	SkipFrame      int                      `json:"skipFrame"`
	AutoStart      bool                     `json:"autoStart"`
	OutputSettings map[string]interface{}   `json:"outputSettings"`
	ROIs           []map[string]interface{} `json:"rois"`
	InferenceTasks []map[string]interface{} `json:"inferenceTasks"`
}

// 任务状态响应
type TaskStatusResponse struct {
	Success   bool      `json:"success"`
	Timestamp string    `json:"timestamp"`
	TaskID    string    `json:"task_id"`
	Stats     TaskStats `json:"stats"`
}

type TaskStats struct {
	Status         string `json:"status"`
	StartTime      int64  `json:"start_time"`
	StopTime       int64  `json:"stop_time"`
	TotalFrames    int64  `json:"total_frames"`
	InferenceCount int64  `json:"inference_count"`
	ForwardSuccess int64  `json:"forward_success"`
	ForwardFailed  int64  `json:"forward_failed"`
	LastError      string `json:"last_error"`
}

// License信息响应
type LicenseInfoResponse struct {
	Success   bool        `json:"success"`
	Timestamp string      `json:"timestamp"`
	License   LicenseInfo `json:"license"`
}

type LicenseInfo struct {
	LicenseID    string `json:"license_id"`
	Edition      string `json:"edition"`
	ValidUntil   string `json:"valid_until"`
	DaysLeft     int    `json:"days_left"`
	IsBuiltinDev bool   `json:"is_builtin_dev"`
	IsValid      bool   `json:"is_valid"`
}

// 日志级别响应
type LogLevelResponse struct {
	Success   bool         `json:"success"`
	Data      LogLevelData `json:"data"`
	Timestamp string       `json:"timestamp"`
}

type LogLevelData struct {
	CurrentLevel    string   `json:"current_level"`
	AvailableLevels []string `json:"available_levels"`
}

// CallBoxAPI 调用盒子API
func (s *BoxProxyService) CallBoxAPI(boxID uint, method, path string, body interface{}) (*APIResponse, error) {
	return s.CallWithRetry(boxID, method, path, body, 3)
}

// CallWithRetry 带重试的API调用
func (s *BoxProxyService) CallWithRetry(boxID uint, method, path string, body interface{}, maxRetries int) (*APIResponse, error) {
	// 获取盒子信息
	ctx := context.Background()
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("查找盒子失败: %w", err)
	}

	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		resp, err := s.callBoxAPIOnce(box, method, path, body)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if i < maxRetries {
			// 等待重试
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("API调用失败，已重试%d次: %w", maxRetries, lastErr)
}

// callBoxAPIOnce 单次API调用
func (s *BoxProxyService) callBoxAPIOnce(box *models.Box, method, path string, body interface{}) (*APIResponse, error) {
	url := fmt.Sprintf("http://%s:%d%s", box.IPAddress, box.Port, path)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 如果盒子有API密钥，则添加X-API-KEY请求头
	if box.ApiKey != "" {
		req.Header.Set("X-API-KEY", box.ApiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		// 如果不是标准格式，尝试直接解析为data
		apiResp = APIResponse{
			Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
			Code:    resp.StatusCode,
		}

		if apiResp.Success {
			json.Unmarshal(respBody, &apiResp.Data)
		} else {
			apiResp.Error = string(respBody)
		}
	}

	if !apiResp.Success && resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("API返回错误: %s", apiResp.Error)
	}

	return &apiResp, nil
}

// uploadModelFile 使用multipart/form-data上传模型文件
func (s *BoxProxyService) uploadModelFile(box *models.Box, modelFilePath, modelType, version, hardware, modelName string, modelData map[string]interface{}) (*APIResponse, error) {
	// 检查文件是否存在
	if _, err := os.Stat(modelFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("模型文件不存在: %s", modelFilePath)
	}

	// 打开文件
	file, err := os.Open(modelFilePath)
	if err != nil {
		return nil, fmt.Errorf("打开模型文件失败: %w", err)
	}
	defer file.Close()

	// 创建multipart form
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// 添加必需字段
	writer.WriteField("type", modelType)
	writer.WriteField("version", version)
	writer.WriteField("hardware", hardware)
	writer.WriteField("model_name", modelName)

	// 添加可选字段
	if confidenceThreshold, ok := modelData["confidence_threshold"]; ok {
		if threshold, ok := confidenceThreshold.(float64); ok {
			writer.WriteField("confidence_threshold", fmt.Sprintf("%.2f", threshold))
		}
	}

	if nmsThreshold, ok := modelData["nms_threshold"]; ok {
		if threshold, ok := nmsThreshold.(float64); ok {
			writer.WriteField("nms_threshold", fmt.Sprintf("%.2f", threshold))
		}
	}

	if md5sum, ok := modelData["md5sum"].(string); ok && md5sum != "" {
		writer.WriteField("md5sum", md5sum)
	}

	// 添加模型文件字段
	part, err := writer.CreateFormFile("model_file", filepath.Base(modelFilePath))
	if err != nil {
		return nil, fmt.Errorf("创建模型文件字段失败: %w", err)
	}

	// 复制文件内容
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 关闭writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭multipart writer失败: %w", err)
	}

	// 创建HTTP请求
	url := fmt.Sprintf("http://%s:%d/api/v1/models/upload", box.IPAddress, box.Port)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 如果盒子有API密钥，则添加X-API-KEY请求头
	if box.ApiKey != "" {
		req.Header.Set("X-API-KEY", box.ApiKey)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		// 如果不是标准格式，尝试直接解析为data
		apiResp = APIResponse{
			Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
			Code:    resp.StatusCode,
		}

		if apiResp.Success {
			json.Unmarshal(respBody, &apiResp.Data)
		} else {
			apiResp.Error = string(respBody)
		}
	}

	if !apiResp.Success && resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("API返回错误: %s", apiResp.Error)
	}

	return &apiResp, nil
}

// callBoxAPIWithType 调用盒子API并解析为指定类型
func (s *BoxProxyService) callBoxAPIWithType(boxID uint, method, path string, body interface{}, respType interface{}) (interface{}, error) {
	// 获取盒子信息
	ctx := context.Background()
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("查找盒子失败: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d%s", box.IPAddress, box.Port, path)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 如果盒子有API密钥，则添加X-API-KEY请求头
	if box.ApiKey != "" {
		req.Header.Set("X-API-KEY", box.ApiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应到指定类型
	if err := json.Unmarshal(respBody, respType); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return respType, nil
}

// 模型管理相关API

// GetModels 获取盒子模型列表
func (s *BoxProxyService) GetModels(boxID uint) (*ModelsResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/models", nil, &ModelsResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*ModelsResponse), nil
}

// UploadModel 上传模型到盒子
func (s *BoxProxyService) UploadModel(boxID uint, modelData map[string]interface{}) (*APIResponse, error) {
	// 获取盒子信息
	ctx := context.Background()
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("查找盒子失败: %w", err)
	}

	// 获取必需参数
	modelFilePath, ok := modelData["model_file"].(string)
	if !ok || modelFilePath == "" {
		return nil, fmt.Errorf("缺少模型文件路径参数")
	}

	modelType, ok := modelData["type"].(string)
	if !ok || modelType == "" {
		return nil, fmt.Errorf("缺少模型类型参数")
	}

	version, ok := modelData["version"].(string)
	if !ok || version == "" {
		return nil, fmt.Errorf("缺少版本参数")
	}

	hardware, ok := modelData["hardware"].(string)
	if !ok || hardware == "" {
		return nil, fmt.Errorf("缺少硬件平台参数")
	}

	modelName, ok := modelData["model_name"].(string)
	if !ok || modelName == "" {
		return nil, fmt.Errorf("缺少模型名称参数")
	}

	// 创建multipart请求
	return s.uploadModelFile(box, modelFilePath, modelType, version, hardware, modelName, modelData)
}

// DeleteModel 删除盒子模型
func (s *BoxProxyService) DeleteModel(boxID uint, modelKey string) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/models/%s", modelKey)
	return s.CallBoxAPI(boxID, "DELETE", path, nil)
}

// ReloadModels 重新加载模型
func (s *BoxProxyService) ReloadModels(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "POST", "/api/v1/models/reload", nil)
}

// 任务管理相关API

// GetTasks 获取盒子任务列表
func (s *BoxProxyService) GetTasks(boxID uint) (*TasksResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/tasks", nil, &TasksResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*TasksResponse), nil
}

// CreateTask 创建任务
func (s *BoxProxyService) CreateTask(boxID uint, taskConfig map[string]interface{}) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "POST", "/api/v1/tasks", taskConfig)
}

// GetTask 获取任务详情
func (s *BoxProxyService) GetTask(boxID uint, taskID string) (*TaskDetailResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/%s", taskID)
	resp, err := s.callBoxAPIWithType(boxID, "GET", path, nil, &TaskDetailResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*TaskDetailResponse), nil
}

// UpdateTask 更新任务
func (s *BoxProxyService) UpdateTask(boxID uint, taskID string, taskConfig map[string]interface{}) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/%s", taskID)
	return s.CallBoxAPI(boxID, "PUT", path, taskConfig)
}

// DeleteTask 删除任务
func (s *BoxProxyService) DeleteTask(boxID uint, taskID string) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/%s", taskID)
	return s.CallBoxAPI(boxID, "DELETE", path, nil)
}

// StartTask 启动任务
func (s *BoxProxyService) StartTask(boxID uint, taskID string) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/%s/start", taskID)
	return s.CallBoxAPI(boxID, "POST", path, nil)
}

// StopTask 停止任务
func (s *BoxProxyService) StopTask(boxID uint, taskID string) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/%s/stop", taskID)
	return s.CallBoxAPI(boxID, "POST", path, nil)
}

// GetTaskStatus 获取任务状态
func (s *BoxProxyService) GetTaskStatus(boxID uint, taskID string) (*TaskStatusResponse, error) {
	path := fmt.Sprintf("/api/v1/tasks/%s/status", taskID)
	resp, err := s.callBoxAPIWithType(boxID, "GET", path, nil, &TaskStatusResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*TaskStatusResponse), nil
}

// 系统信息响应
type SystemInfoResponse struct {
	Success   bool              `json:"success"`
	Timestamp string            `json:"timestamp"`
	Base      SystemBaseInfo    `json:"base"`
	Current   SystemCurrentInfo `json:"current"`
}

type SystemBaseInfo struct {
	CPUCores          int    `json:"cpu_cores"`
	CPULogicalCores   int    `json:"cpu_logical_cores"`
	CPUModelName      string `json:"cpu_model_name"`
	Hostname          string `json:"hostname"`
	KernelArch        string `json:"kernel_arch"`
	KernelVersion     string `json:"kernel_version"`
	OS                string `json:"os"`
	Platform          string `json:"platform"`
	PlatformFamily    string `json:"platform_family"`
	PlatformVersion   string `json:"platform_version"`
	SDKVersion        string `json:"sdk_version"`
	SoftwareBuildTime string `json:"software_build_time"`
	SoftwareVersion   string `json:"software_version"`
}

type SystemCurrentInfo struct {
	BoardTemperature      float64         `json:"board_temperature"`
	CoreTemperature       float64         `json:"core_temperature"`
	CPUPercent            []float64       `json:"cpu_percent"`
	CPUTotal              int             `json:"cpu_total"`
	CPUUsed               float64         `json:"cpu_used"`
	CPUUsedPercent        float64         `json:"cpu_used_percent"`
	DiskData              []DiskUsageInfo `json:"disk_data"`
	IOCount               int64           `json:"io_count"`
	IOReadBytes           int64           `json:"io_read_bytes"`
	IOReadTime            int64           `json:"io_read_time"`
	IOWriteBytes          int64           `json:"io_write_bytes"`
	IOWriteTime           int64           `json:"io_write_time"`
	Load1                 float64         `json:"load1"`
	Load5                 float64         `json:"load5"`
	Load15                float64         `json:"load15"`
	LoadUsagePercent      float64         `json:"load_usage_percent"`
	MemoryAvailable       int64           `json:"memory_available"`
	MemoryTotal           int64           `json:"memory_total"`
	MemoryUsed            int64           `json:"memory_used"`
	MemoryUsedPercent     float64         `json:"memory_used_percent"`
	NetBytesRecv          int64           `json:"net_bytes_recv"`
	NetBytesSent          int64           `json:"net_bytes_sent"`
	NPUMemoryTotal        int             `json:"npu_memory_total"`
	NPUMemoryUsed         int             `json:"npu_memory_used"`
	Procs                 int64           `json:"procs"`
	ShotTime              string          `json:"shot_time"`
	SwapMemoryAvailable   int64           `json:"swap_memory_available"`
	SwapMemoryTotal       int64           `json:"swap_memory_total"`
	SwapMemoryUsed        int64           `json:"swap_memory_used"`
	SwapMemoryUsedPercent float64         `json:"swap_memory_used_percent"`
	TimeSinceUptime       string          `json:"time_since_uptime"`
	TPUUsed               int             `json:"tpu_used"`
	Uptime                int64           `json:"uptime"`
	VPPMemoryTotal        int             `json:"vpp_memory_total"`
	VPPMemoryUsed         int             `json:"vpp_memory_used"`
	VPUMemoryTotal        int             `json:"vpu_memory_total"`
	VPUMemoryUsed         int             `json:"vpu_memory_used"`
}

type DiskUsageInfo struct {
	Device            string  `json:"device"`
	Free              int64   `json:"free"`
	INodesFree        int64   `json:"inodes_free"`
	INodesTotal       int64   `json:"inodes_total"`
	INodesUsed        int64   `json:"inodes_used"`
	INodesUsedPercent float64 `json:"inodes_used_percent"`
	Path              string  `json:"path"`
	Total             int64   `json:"total"`
	Type              string  `json:"type"`
	Used              int64   `json:"used"`
	UsedPercent       float64 `json:"used_percent"`
}

// 系统管理相关API

// GetSystemInfo 获取系统信息
func (s *BoxProxyService) GetSystemInfo(boxID uint) (*SystemInfoResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/system/info", nil, &SystemInfoResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*SystemInfoResponse), nil
}

// GetSystemStatus 获取系统状态
func (s *BoxProxyService) GetSystemStatus(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "GET", "/api/v1/system/status", nil)
}

// GetSystemVersion 获取系统版本
func (s *BoxProxyService) GetSystemVersion(boxID uint) (*SystemVersionResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/system/version", nil, &SystemVersionResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*SystemVersionResponse), nil
}

// UpdateSystem 系统更新
func (s *BoxProxyService) UpdateSystem(boxID uint, updateData map[string]interface{}) (*APIResponse, error) {
	// 获取盒子信息
	ctx := context.Background()
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("查找盒子失败: %w", err)
	}

	// 获取参数
	version, ok := updateData["version"].(string)
	if !ok || version == "" {
		return nil, fmt.Errorf("缺少版本号参数")
	}

	programPath, ok := updateData["program_file"].(string)
	if !ok || programPath == "" {
		return nil, fmt.Errorf("缺少程序文件路径参数")
	}

	// 创建multipart请求
	return s.uploadSystemUpdateFile(box, version, programPath)
}

// GetUpdateStatus 获取更新状态
func (s *BoxProxyService) GetUpdateStatus(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "GET", "/api/v1/system/update/status", nil)
}

// RestartService 重启服务
func (s *BoxProxyService) RestartService(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "POST", "/api/v1/system/restart-service", nil)
}

// RebootSystem 重启系统
func (s *BoxProxyService) RebootSystem(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "POST", "/api/v1/system/reboot", nil)
}

// 日志管理相关API

// SetLogLevel 设置日志级别
func (s *BoxProxyService) SetLogLevel(boxID uint, level string) (*APIResponse, error) {
	data := map[string]interface{}{
		"level": level,
	}
	return s.CallBoxAPI(boxID, "POST", "/api/v1/system/log/level", data)
}

// GetLogLevel 获取日志级别
func (s *BoxProxyService) GetLogLevel(boxID uint) (*LogLevelResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/system/log/level", nil, &LogLevelResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*LogLevelResponse), nil
}

// License管理相关API

// GetLicenseInfo 获取License信息
func (s *BoxProxyService) GetLicenseInfo(boxID uint) (*LicenseInfoResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/system/license/info", nil, &LicenseInfoResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*LicenseInfoResponse), nil
}

// ValidateLicense 验证License
func (s *BoxProxyService) ValidateLicense(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "GET", "/api/v1/system/license/validate", nil)
}

// GetDeviceFingerprint 获取设备指纹
func (s *BoxProxyService) GetDeviceFingerprint(boxID uint) (*APIResponse, error) {
	return s.CallBoxAPI(boxID, "GET", "/api/v1/system/device-fingerprint", nil)
}

// 检测和分析相关API

// DetectImage 图像检测
func (s *BoxProxyService) DetectImage(boxID uint, modelKey string, imageData []byte) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/detect/%s", modelKey)
	// 注意：这里需要处理multipart/form-data，实际实现时需要调整
	return s.CallBoxAPI(boxID, "POST", path, imageData)
}

// SegmentImage 图像分割
func (s *BoxProxyService) SegmentImage(boxID uint, modelKey string, imageData []byte) (*APIResponse, error) {
	path := fmt.Sprintf("/api/v1/segment/%s", modelKey)
	// 注意：这里需要处理multipart/form-data，实际实现时需要调整
	return s.CallBoxAPI(boxID, "POST", path, imageData)
}

// GetMetadata 获取系统元数据
func (s *BoxProxyService) GetMetadata(boxID uint) (*MetaResponse, error) {
	resp, err := s.callBoxAPIWithType(boxID, "GET", "/api/v1/meta", nil, &MetaResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*MetaResponse), nil
}

// RTSP相关API

// GetRTSPSnapshot 获取RTSP截图
func (s *BoxProxyService) GetRTSPSnapshot(boxID uint, rtspURL string) (*APIResponse, error) {
	data := map[string]interface{}{
		"rtsp_url": rtspURL,
	}
	return s.CallBoxAPI(boxID, "POST", "/api/v1/rtsp/snapshot", data)
}

// uploadSystemUpdateFile 使用multipart/form-data上传系统更新文件
func (s *BoxProxyService) uploadSystemUpdateFile(box *models.Box, version, programPath string) (*APIResponse, error) {
	// 检查文件是否存在
	if _, err := os.Stat(programPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("程序文件不存在: %s", programPath)
	}

	// 打开文件
	file, err := os.Open(programPath)
	if err != nil {
		return nil, fmt.Errorf("打开程序文件失败: %w", err)
	}
	defer file.Close()

	// 创建multipart form
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// 添加版本号字段
	if err := writer.WriteField("version", version); err != nil {
		return nil, fmt.Errorf("添加版本号字段失败: %w", err)
	}

	// 添加程序文件字段
	part, err := writer.CreateFormFile("program", filepath.Base(programPath))
	if err != nil {
		return nil, fmt.Errorf("创建程序文件字段失败: %w", err)
	}

	// 复制文件内容
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 关闭writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭multipart writer失败: %w", err)
	}

	// 创建HTTP请求
	url := fmt.Sprintf("http://%s:%d/api/v1/system/update/upload", box.IPAddress, box.Port)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		// 如果不是标准格式，尝试直接解析为data
		apiResp = APIResponse{
			Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
			Code:    resp.StatusCode,
		}

		if apiResp.Success {
			json.Unmarshal(respBody, &apiResp.Data)
		} else {
			apiResp.Error = string(respBody)
		}
	}

	if !apiResp.Success && resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("API返回错误: %s", apiResp.Error)
	}

	return &apiResp, nil
}
