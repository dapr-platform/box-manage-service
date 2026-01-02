/*
 * @module client/box_client
 * @description 盒子API客户端，用于调用盒子上的API接口
 * @architecture 客户端层
 * @documentReference REQ-004: 任务调度系统
 * @stateFlow HTTP请求 -> 盒子API -> 响应处理
 * @rules 遵循盒子API规范，处理网络错误和重试
 * @dependencies net/http, encoding/json
 * @refs apis.md, task_req.md
 */

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// BoxClient 盒子API客户端
type BoxClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewBoxClient 创建新的盒子客户端
func NewBoxClient(host string, port int) *BoxClient {
	return &BoxClient{
		baseURL:    fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{Timeout: 10 * time.Minute}, // 设置为10分钟以支持大模型文件上传
	}
}

// BoxResponse 盒子API通用响应
type BoxResponse struct {
	Success   bool        `json:"success"`
	Timestamp string      `json:"timestamp,omitempty"`
	Message   string      `json:"message,omitempty"`
	Error     string      `json:"error,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// BoxTaskConfig 盒子任务配置（对应apis.md中的任务配置格式）
type BoxTaskConfig struct {
	TaskID                string             `json:"taskId"`
	DevID                 string             `json:"devId"`
	RTSPUrl               string             `json:"rtspUrl"`
	SkipFrame             int                `json:"skipFrame"`
	AutoStart             bool               `json:"autoStart"`
	OutputSettings        BoxOutputSettings  `json:"outputSettings"`
	ROIs                  []BoxROIConfig     `json:"rois"`
	InferenceTasks        []BoxInferenceTask `json:"inferenceTasks"`
	UseROItoInference     bool               `json:"useROItoInference"`
	TaskLevelForwardInfos []BoxForwardInfo   `json:"taskLevelForwardInfos"`
}

// BoxOutputSettings 盒子输出设置
type BoxOutputSettings struct {
	SendFullImage int `json:"sendFullImage"` // 0=否，1=是
}

// BoxROIConfig 盒子ROI配置
type BoxROIConfig struct {
	ID     int            `json:"id"`
	Name   string         `json:"name"`
	Width  int            `json:"width"`
	Height int            `json:"height"`
	X      int            `json:"x"`
	Y      int            `json:"y"`
	Areas  []BoxAreaPoint `json:"areas"`
}

// BoxAreaPoint ROI区域坐标点
type BoxAreaPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// BoxInferenceTask 盒子推理任务配置
type BoxInferenceTask struct {
	Type            string           `json:"type"`            // detection, segmentation, classification, custom
	ModelKey        string           `json:"modelKey"`        // 使用modelKey而不是modelName
	Threshold       float64          `json:"threshold"`       // 置信度阈值
	SendSSEImage    bool             `json:"sendSSEImage"`    // 是否发送SSE图像
	BusinessProcess string           `json:"businessProcess"` // Lua脚本路径
	RTSPPushUrl     string           `json:"rtspPushUrl"`     // RTSP推流地址
	ROIIds          []int            `json:"roiIds"`          // 关联的ROI ID列表
	ForwardInfos    []BoxForwardInfo `json:"forwardInfos"`    // 结果转发配置列表
}

// BoxForwardInfo 盒子结果转发配置
type BoxForwardInfo struct {
	Enabled  bool   `json:"enabled"` // 是否启用
	Type     string `json:"type"`    // mqtt, http_post, websocket
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Topic    string `json:"topic"` // MQTT主题或HTTP路径
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Health 检查盒子健康状态
func (c *BoxClient) Health(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/api/v1/health", nil)
	return err
}

// CreateTask 创建任务
func (c *BoxClient) CreateTask(ctx context.Context, task *BoxTaskConfig) error {
	log.Printf("[BoxClient] CreateTask started - TaskID: %s, DevID: %s, RTSPUrl: %s, AutoStart: %t",
		task.TaskID, task.DevID, task.RTSPUrl, task.AutoStart)

	resp, err := c.doRequest(ctx, "POST", "/api/v1/tasks", task)
	if err != nil {
		log.Printf("[BoxClient] CreateTask request failed - TaskID: %s, Error: %v", task.TaskID, err)
		return err
	}

	// 解析响应检查是否真正成功
	var boxResp BoxResponse
	if err := json.Unmarshal(resp, &boxResp); err != nil {
		log.Printf("[BoxClient] CreateTask response parse failed - TaskID: %s, Response: %s, Error: %v",
			task.TaskID, string(resp), err)
		return fmt.Errorf("解析创建任务响应失败: %w", err)
	}

	// 检查业务成功标志
	if !boxResp.Success {
		log.Printf("[BoxClient] CreateTask business logic failed - TaskID: %s, Error: %s, Message: %s",
			task.TaskID, boxResp.Error, boxResp.Message)
		return fmt.Errorf("创建任务失败: %s", boxResp.Error)
	}

	log.Printf("[BoxClient] CreateTask succeeded - TaskID: %s, Message: %s", task.TaskID, boxResp.Message)
	return nil
}

// GetTask 获取任务详情
func (c *BoxClient) GetTask(ctx context.Context, taskID string) (*BoxTaskConfig, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/tasks/%s", taskID), nil)
	if err != nil {
		return nil, err
	}

	var boxResp struct {
		BoxResponse
		Task *BoxTaskConfig `json:"task"`
	}

	if err := json.Unmarshal(resp, &boxResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !boxResp.Success {
		return nil, fmt.Errorf("box API error: %s", boxResp.Error)
	}

	return boxResp.Task, nil
}

// UpdateTask 更新任务
func (c *BoxClient) UpdateTask(ctx context.Context, taskID string, task *BoxTaskConfig) error {
	log.Printf("[BoxClient] UpdateTask started - TaskID: %s", taskID)

	resp, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/api/v1/tasks/%s", taskID), task)
	if err != nil {
		log.Printf("[BoxClient] UpdateTask request failed - TaskID: %s, Error: %v", taskID, err)
		return err
	}

	// 解析响应检查是否真正成功
	var boxResp BoxResponse
	if err := json.Unmarshal(resp, &boxResp); err != nil {
		log.Printf("[BoxClient] UpdateTask response parse failed - TaskID: %s, Response: %s, Error: %v",
			taskID, string(resp), err)
		return fmt.Errorf("解析更新任务响应失败: %w", err)
	}

	if !boxResp.Success {
		log.Printf("[BoxClient] UpdateTask business logic failed - TaskID: %s, Error: %s", taskID, boxResp.Error)
		return fmt.Errorf("更新任务失败: %s", boxResp.Error)
	}

	log.Printf("[BoxClient] UpdateTask succeeded - TaskID: %s", taskID)
	return nil
}

// DeleteTask 删除任务
func (c *BoxClient) DeleteTask(ctx context.Context, taskID string) error {
	log.Printf("[BoxClient] DeleteTask started - TaskID: %s", taskID)

	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/tasks/%s", taskID), nil)
	if err != nil {
		log.Printf("[BoxClient] DeleteTask request failed - TaskID: %s, Error: %v", taskID, err)
		return err
	}

	// 解析响应检查是否真正成功
	var boxResp BoxResponse
	if err := json.Unmarshal(resp, &boxResp); err != nil {
		log.Printf("[BoxClient] DeleteTask response parse failed - TaskID: %s, Response: %s, Error: %v",
			taskID, string(resp), err)
		return fmt.Errorf("解析删除任务响应失败: %w", err)
	}

	if !boxResp.Success {
		log.Printf("[BoxClient] DeleteTask business logic failed - TaskID: %s, Error: %s", taskID, boxResp.Error)
		return fmt.Errorf("删除任务失败: %s", boxResp.Error)
	}

	log.Printf("[BoxClient] DeleteTask succeeded - TaskID: %s", taskID)
	return nil
}

// StartTask 启动任务
func (c *BoxClient) StartTask(ctx context.Context, taskID string) error {
	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/tasks/%s/start", taskID), nil)
	return err
}

// StopTask 停止任务
func (c *BoxClient) StopTask(ctx context.Context, taskID string) error {
	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/tasks/%s/stop", taskID), nil)
	return err
}

// GetTaskStatus 获取任务状态
func (c *BoxClient) GetTaskStatus(ctx context.Context, taskID string) (*BoxTaskStatusResponse, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/tasks/%s/status", taskID), nil)
	if err != nil {
		return nil, err
	}

	var boxResp struct {
		BoxResponse
		Status *BoxTaskStatusResponse `json:"status"`
	}

	if err := json.Unmarshal(resp, &boxResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !boxResp.Success {
		return nil, fmt.Errorf("box API error: %s", boxResp.Error)
	}

	return boxResp.Status, nil
}

// BoxTaskStatusResponse 盒子任务状态响应
type BoxTaskStatusResponse struct {
	Status     string             `json:"status"`
	Progress   float64            `json:"progress"`
	Error      string             `json:"error,omitempty"`
	Message    string             `json:"message,omitempty"`
	Statistics *BoxTaskStatistics `json:"statistics,omitempty"`
}

// BoxTaskStatistics 盒子任务统计信息
type BoxTaskStatistics struct {
	FPS             float64 `json:"fps"`
	AverageLatency  float64 `json:"average_latency"`
	ProcessedFrames int64   `json:"processed_frames"`
	TotalFrames     int64   `json:"total_frames"`
	InferenceCount  int64   `json:"inference_count"`
	ForwardSuccess  int64   `json:"forward_success"`
	ForwardFailed   int64   `json:"forward_failed"`
}

// GetSystemMeta 获取系统元信息
func (c *BoxClient) GetSystemMeta(ctx context.Context) (*BoxSystemMeta, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/meta", nil)
	if err != nil {
		return nil, err
	}

	var boxResp struct {
		BoxResponse
		Data *BoxSystemMeta `json:"data"`
	}

	if err := json.Unmarshal(resp, &boxResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !boxResp.Success {
		return nil, fmt.Errorf("box API error: %s", boxResp.Error)
	}

	return boxResp.Data, nil
}

// GetSystemVersion 获取系统版本信息
func (c *BoxClient) GetSystemVersion(ctx context.Context) (*BoxVersionResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/system/version", nil)
	if err != nil {
		return nil, err
	}

	var versionResp BoxVersionResponse
	if err := json.Unmarshal(resp, &versionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version response: %w", err)
	}

	return &versionResp, nil
}

// GetSystemInfo 获取系统信息
func (c *BoxClient) GetSystemInfo(ctx context.Context) (*BoxSystemInfoResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/system/info", nil)
	if err != nil {
		return nil, err
	}

	var infoResp BoxSystemInfoResponse
	if err := json.Unmarshal(resp, &infoResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system info response: %w", err)
	}

	if !infoResp.Success {
		return nil, fmt.Errorf("box API error: failed to get system info")
	}

	return &infoResp, nil
}

// BoxModel 盒子上的模型信息
type BoxModel struct {
	BModelPath    string  `json:"bmodel_path"`
	ConfThreshold float64 `json:"conf_threshold"`
	FileSize      int64   `json:"file_size"`
	Hardware      string  `json:"hardware"`
	IsLoaded      bool    `json:"is_loaded"`
	MD5Sum        string  `json:"md5sum"`
	ModelKey      string  `json:"model_key"`
	Name          string  `json:"name"`
	NMSThreshold  float64 `json:"nms_threshold"`
	Type          string  `json:"type"`
	UploadTime    string  `json:"upload_time"`
	Version       string  `json:"version"`
}

// GetInstalledModels 获取盒子上已安装的模型列表
func (c *BoxClient) GetInstalledModels(ctx context.Context) ([]BoxModel, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/models", nil)
	if err != nil {
		return nil, err
	}

	var boxResp struct {
		BoxResponse
		Models []BoxModel `json:"models"`
	}

	if err := json.Unmarshal(resp, &boxResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !boxResp.Success {
		return nil, fmt.Errorf("box API error: %s", boxResp.Error)
	}

	return boxResp.Models, nil
}

// GetInstalledModelKeys 获取盒子上已安装的模型Key列表（为了向后兼容）
func (c *BoxClient) GetInstalledModelKeys(ctx context.Context) ([]string, error) {
	models, err := c.GetInstalledModels(ctx)
	if err != nil {
		return nil, err
	}

	modelKeys := make([]string, len(models))
	for i, model := range models {
		modelKeys[i] = model.ModelKey
	}

	return modelKeys, nil
}

// BoxSystemMeta 盒子系统元信息
type BoxSystemMeta struct {
	Defaults             BoxDefaults       `json:"defaults"`
	FileLimits           BoxFileLimits     `json:"file_limits"`
	InferenceTypeMapping map[string]string `json:"inference_type_mapping"`
	SupportedHardware    []string          `json:"supported_hardware"`
	SupportedTypes       []string          `json:"supported_types"`
	SupportedVersions    []string          `json:"supported_versions"`
}

// BoxDefaults 盒子默认配置
type BoxDefaults struct {
	ConfThreshold float64 `json:"conf_threshold"`
	Hardware      string  `json:"hardware"`
	NMSThreshold  float64 `json:"nms_threshold"`
	Type          string  `json:"type"`
	Version       string  `json:"version"`
}

// BoxFileLimits 文件限制
type BoxFileLimits struct {
	ImageFileMaxSize      string   `json:"image_file_max_size"`
	ModelFileMaxSize      string   `json:"model_file_max_size"`
	SupportedImageFormats []string `json:"supported_image_formats"`
	SupportedModelFormats []string `json:"supported_model_formats"`
}

// BoxVersionResponse 盒子版本信息响应
type BoxVersionResponse struct {
	APIVersion string `json:"api_version"`
	BuildTime  string `json:"build_time"`
	Service    string `json:"service"`
	Timestamp  string `json:"timestamp"`
	Version    string `json:"version"`
}

// BoxSystemInfoResponse 盒子系统信息响应
type BoxSystemInfoResponse struct {
	Base      BoxBaseInfo    `json:"base"`
	Current   BoxCurrentInfo `json:"current"`
	Success   bool           `json:"success"`
	Timestamp string         `json:"timestamp"`
}

// BoxBaseInfo 基础系统信息
type BoxBaseInfo struct {
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

// BoxCurrentInfo 当前系统状态信息
type BoxCurrentInfo struct {
	BoardTemperature      float64       `json:"board_temperature"`
	CoreTemperature       float64       `json:"core_temperature"`
	CPUPercent            []float64     `json:"cpu_percent"`
	CPUTotal              int           `json:"cpu_total"`
	CPUUsed               float64       `json:"cpu_used"`
	CPUUsedPercent        float64       `json:"cpu_used_percent"`
	DiskData              []BoxDiskInfo `json:"disk_data"`
	IOCount               int64         `json:"io_count"`
	IOReadBytes           int64         `json:"io_read_bytes"`
	IOReadTime            int64         `json:"io_read_time"`
	IOWriteBytes          int64         `json:"io_write_bytes"`
	IOWriteTime           int64         `json:"io_write_time"`
	Load1                 float64       `json:"load1"`
	Load15                float64       `json:"load15"`
	Load5                 float64       `json:"load5"`
	LoadUsagePercent      float64       `json:"load_usage_percent"`
	MemoryAvailable       int64         `json:"memory_available"`
	MemoryTotal           int64         `json:"memory_total"`
	MemoryUsed            int64         `json:"memory_used"`
	MemoryUsedPercent     float64       `json:"memory_used_percent"`
	NetBytesRecv          int64         `json:"net_bytes_recv"`
	NetBytesSent          int64         `json:"net_bytes_sent"`
	NPUMemoryTotal        int64         `json:"npu_memory_total"`
	NPUMemoryUsed         int64         `json:"npu_memory_used"`
	Procs                 int           `json:"procs"`
	ShotTime              string        `json:"shot_time"`
	SwapMemoryAvailable   int64         `json:"swap_memory_available"`
	SwapMemoryTotal       int64         `json:"swap_memory_total"`
	SwapMemoryUsed        int64         `json:"swap_memory_used"`
	SwapMemoryUsedPercent float64       `json:"swap_memory_used_percent"`
	TimeSinceUptime       string        `json:"time_since_uptime"`
	TPUUsed               int           `json:"tpu_used"`
	Uptime                int64         `json:"uptime"`
	VPPMemoryTotal        int64         `json:"vpp_memory_total"`
	VPPMemoryUsed         int64         `json:"vpp_memory_used"`
	VPUMemoryTotal        int64         `json:"vpu_memory_total"`
	VPUMemoryUsed         int64         `json:"vpu_memory_used"`
}

// BoxDiskInfo 磁盘信息
type BoxDiskInfo struct {
	Device            string  `json:"device"`
	Free              int64   `json:"free"`
	InodesFree        int64   `json:"inodes_free"`
	InodesTotal       int64   `json:"inodes_total"`
	InodesUsed        int64   `json:"inodes_used"`
	InodesUsedPercent float64 `json:"inodes_used_percent"`
	Path              string  `json:"path"`
	Total             int64   `json:"total"`
	Type              string  `json:"type"`
	Used              int64   `json:"used"`
	UsedPercent       float64 `json:"used_percent"`
}

// doRequest 执行HTTP请求
func (c *BoxClient) doRequest(ctx context.Context, method, path string, data interface{}) ([]byte, error) {
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request data: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var boxResp BoxResponse
		if json.Unmarshal(respBody, &boxResp) == nil && boxResp.Error != "" {
			return nil, fmt.Errorf("box API error [%d]: %s", resp.StatusCode, boxResp.Error)
		}
		return nil, fmt.Errorf("HTTP error [%d]: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ModelUploadRequest 模型上传请求
type ModelUploadRequest struct {
	ModelName  string  `json:"model_name"`
	Type       string  `json:"type"`                           // detection, segmentation等
	Version    string  `json:"version"`                        // 版本号
	Hardware   string  `json:"hardware"`                       // 硬件平台
	FilePath   string  `json:"file_path"`                      // 本地文件路径
	MD5Sum     string  `json:"md5sum"`                         // MD5校验和
	Confidence float64 `json:"confidence_threshold,omitempty"` // 置信度阈值
	NMS        float64 `json:"nms_threshold,omitempty"`        // NMS阈值
}

// UploadModel 上传模型到盒子
func (c *BoxClient) UploadModel(ctx context.Context, req *ModelUploadRequest) error {
	log.Printf("[BoxClient] UploadModel started - ModelName: %s, Type: %s, Hardware: %s",
		req.ModelName, req.Type, req.Hardware)

	// 检查文件是否存在
	if _, err := os.Stat(req.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("模型文件不存在: %s", req.FilePath)
	}

	// 打开文件
	file, err := os.Open(req.FilePath)
	if err != nil {
		return fmt.Errorf("打开模型文件失败: %w", err)
	}
	defer file.Close()

	// 创建multipart/form-data请求
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// 添加文件字段 - 盒子期望字段名为 "model"
	part, err := writer.CreateFormFile("model", filepath.Base(req.FilePath))
	if err != nil {
		return fmt.Errorf("创建文件字段失败: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	// 添加其他字段
	writer.WriteField("model_name", req.ModelName)
	writer.WriteField("type", req.Type)
	writer.WriteField("version", req.Version)
	writer.WriteField("hardware", req.Hardware)

	if req.MD5Sum != "" {
		writer.WriteField("md5sum", req.MD5Sum)
	}

	if req.Confidence > 0 {
		writer.WriteField("confidence_threshold", fmt.Sprintf("%.2f", req.Confidence))
	}

	if req.NMS > 0 {
		writer.WriteField("nms_threshold", fmt.Sprintf("%.2f", req.NMS))
	}

	writer.Close()

	// 创建HTTP请求
	url := fmt.Sprintf("%s/api/v1/models/upload", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var boxResp BoxResponse
		if json.Unmarshal(respBody, &boxResp) == nil && boxResp.Error != "" {
			return fmt.Errorf("模型上传失败 [%d]: %s", resp.StatusCode, boxResp.Error)
		}
		return fmt.Errorf("HTTP错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[BoxClient] Model %s uploaded successfully", req.ModelName)
	return nil
}
