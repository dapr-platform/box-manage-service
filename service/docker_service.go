/*
 * @module service/docker_service
 * @description Docker模型转换服务，负责与外部转换服务API交互
 * @architecture 服务层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow 上传模型 -> 执行转换 -> 下载结果
 * @rules 遵循外部API协议，处理异步转换流程
 * @dependencies net/http, encoding/json
 * @refs DESIGN-000.md
 */

package service

import (
	"box-manage-service/models"
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
	"strings"
	"time"
)

// dockerService Docker转换服务实现
type dockerService struct {
	baseURL    string
	httpClient *http.Client
}

// ConversionStatus 转换状态响应
type ConversionStatus struct {
	TaskID      string     `json:"task_id"`
	Status      string     `json:"status"` // pending, processing, completed, failed
	ModelType   string     `json:"model_type"`
	InputShape  string     `json:"input_shape"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	OutputFile  string     `json:"output_file,omitempty"`
	Logs        []string   `json:"logs,omitempty"`
}

// DockerResponse 通用Docker API响应
type DockerResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ConvertRequest 转换请求结构
type ConvertRequest struct {
	TaskID       string `json:"task_id"`
	ModelType    string `json:"model_type"`
	InputShape   string `json:"input_shape"`
	ModelPath    string `json:"model_path"`
	ChipType     string `json:"chip_type"`
	Quantization string `json:"quantization"`
}

// NewDockerService 创建Docker服务实例
func NewDockerService(baseURL string) DockerService {
	if baseURL == "" {
		baseURL = "http://182.92.117.41:49000"
	}

	return &dockerService{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Minute, // 长时间转换任务需要更长的超时
		},
	}
}

// UploadModel 上传模型文件到转换服务
func (s *dockerService) UploadModel(ctx context.Context, task *models.ConversionTask, inputPath string) (string, error) {
	log.Printf("[DockerService] UploadModel started - TaskID: %s, InputPath: %s, BaseURL: %s", task.TaskID, inputPath, s.baseURL)

	// 检查原始模型文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		log.Printf("[DockerService] Model file not found - TaskID: %s, Path: %s", task.TaskID, inputPath)
		return "", fmt.Errorf("模型文件不存在: %s", inputPath)
	}

	// 打开文件
	log.Printf("[DockerService] Opening model file - TaskID: %s, Path: %s", task.TaskID, inputPath)
	file, err := os.Open(inputPath)
	if err != nil {
		log.Printf("[DockerService] Failed to open model file - TaskID: %s, Error: %v", task.TaskID, err)
		return "", fmt.Errorf("打开模型文件失败: %w", err)
	}
	defer file.Close()

	// 创建multipart form请求
	log.Printf("[DockerService] Creating multipart form request - TaskID: %s", task.TaskID)
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// 从TargetFormat推断模型类型，默认为"detection"
	modelType := string(task.OriginalModel.TaskType)
	// 添加表单字段
	_ = writer.WriteField("model_type", modelType)

	// 从Parameters中获取input_shape（通常使用宽度作为单一参数）
	inputShape := "640" // 默认值
	if len(task.Parameters.InputShape) >= 4 {
		// InputShape格式: [batch, channels, height, width]
		inputShape = fmt.Sprintf("%d", task.Parameters.InputShape[3]) // 使用width
		log.Printf("[DockerService] Using input width from parameters - TaskID: %s, Shape: %v, Width: %s",
			task.TaskID, task.Parameters.InputShape, inputShape)
	} else if len(task.Parameters.InputShape) >= 3 {
		// 如果没有batch维度: [channels, height, width]
		inputShape = fmt.Sprintf("%d", task.Parameters.InputShape[2]) // 使用width
		log.Printf("[DockerService] Using input width from parameters (no batch) - TaskID: %s, Shape: %v, Width: %s",
			task.TaskID, task.Parameters.InputShape, inputShape)
	}
	_ = writer.WriteField("input_shape", inputShape)
	chipType := task.Parameters.TargetChip
	_ = writer.WriteField("chip_type", chipType)

	// 添加 quantization 参数
	quantization := task.Parameters.Quantization
	if quantization == "" {
		quantization = "F16" // 默认值
	}
	_ = writer.WriteField("quantization", quantization)

	log.Printf("[DockerService] UploadModel - TaskID: %s, ModelType: %s, InputShape: %s, ChipType: %s, Quantization: %s",
		task.TaskID, modelType, inputShape, chipType, quantization)

	// 添加文件字段
	log.Printf("[DockerService] Creating form file field - TaskID: %s, FileName: %s", task.TaskID, filepath.Base(inputPath))
	part, err := writer.CreateFormFile("model_file", filepath.Base(inputPath))
	if err != nil {
		log.Printf("[DockerService] Failed to create form file field - TaskID: %s, Error: %v", task.TaskID, err)
		return "", fmt.Errorf("创建文件字段失败: %w", err)
	}

	// 复制文件内容
	log.Printf("[DockerService] Starting file content copy - TaskID: %s, SourcePath: %s", task.TaskID, inputPath)
	bytesWritten, err := io.Copy(part, file)
	if err != nil {
		log.Printf("[DockerService] Failed to copy file content - TaskID: %s, Error: %v", task.TaskID, err)
		return "", fmt.Errorf("复制文件内容失败: %w", err)
	}
	log.Printf("[DockerService] File content copied successfully - TaskID: %s, BytesWritten: %d", task.TaskID, bytesWritten)

	// 关闭writer
	log.Printf("[DockerService] Closing multipart writer - TaskID: %s", task.TaskID)
	if err := writer.Close(); err != nil {
		log.Printf("[DockerService] Failed to close multipart writer - TaskID: %s, Error: %v", task.TaskID, err)
		return "", fmt.Errorf("关闭writer失败: %w", err)
	}

	// 创建HTTP请求
	uploadURL := s.baseURL + "/api/upload"
	log.Printf("[DockerService] Creating HTTP upload request - TaskID: %s, URL: %s, RequestSize: %d bytes",
		task.TaskID, uploadURL, b.Len())
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &b)
	if err != nil {
		log.Printf("[DockerService] Failed to create HTTP request - TaskID: %s, Error: %v", task.TaskID, err)
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	log.Printf("[DockerService] HTTP request headers set - TaskID: %s, ContentType: %s",
		task.TaskID, writer.FormDataContentType())

	// 发送请求
	log.Printf("[DockerService] Sending HTTP upload request - TaskID: %s, URL: %s", task.TaskID, uploadURL)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[DockerService] HTTP upload request failed - TaskID: %s, URL: %s, Error: %v", task.TaskID, uploadURL, err)
		return "", fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("[DockerService] HTTP upload request sent successfully - TaskID: %s, StatusCode: %d", task.TaskID, resp.StatusCode)

	// 读取响应内容
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[DockerService] Failed to read response body - TaskID: %s, Error: %v", task.TaskID, err)
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	log.Printf("[DockerService] Upload response received - TaskID: %s, Status: %d, Body: %s",
		task.TaskID, resp.StatusCode, string(bodyBytes))

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("[DockerService] HTTP error - TaskID: %s, Status: %d, URL: %s",
			task.TaskID, resp.StatusCode, req.URL.String())

		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("上传接口不存在 (404): %s", req.URL.String())
		}
		return "", fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var dockerResp DockerResponse
	if err := json.Unmarshal(bodyBytes, &dockerResp); err != nil {
		log.Printf("[DockerService] Failed to unmarshal response - TaskID: %s, Body: %s, Error: %v",
			task.TaskID, string(bodyBytes), err)

		// 尝试解析为通用map以获取更多信息
		var genericResp map[string]interface{}
		if err2 := json.Unmarshal(bodyBytes, &genericResp); err2 == nil {
			log.Printf("[DockerService] Generic response parsed - TaskID: %s, Response: %+v", task.TaskID, genericResp)

			// 检查是否是成功响应
			if success, ok := genericResp["success"].(bool); ok && success {
				if data, ok := genericResp["data"].(map[string]interface{}); ok {
					if modelPath, ok := data["model_path"].(string); ok {
						log.Printf("[DockerService] Model path extracted from generic response - TaskID: %s, ModelPath: %s",
							task.TaskID, modelPath)
						return modelPath, nil
					}
				}
			}

			// 检查错误信息
			if errorMsg, ok := genericResp["error"].(string); ok {
				return "", fmt.Errorf("上传失败: %s", errorMsg)
			}
		}

		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if !dockerResp.Success {
		log.Printf("[DockerService] Upload failed - TaskID: %s, Error: %s", task.TaskID, dockerResp.Error)
		return "", fmt.Errorf("上传失败: %s", dockerResp.Error)
	}

	// 从响应中获取模型路径和任务ID
	data, ok := dockerResp.Data.(map[string]interface{})
	if !ok {
		log.Printf("[DockerService] Invalid data format in response - TaskID: %s, Data: %+v", task.TaskID, dockerResp.Data)
		return "", fmt.Errorf("响应数据格式错误")
	}

	modelPath, ok := data["model_path"].(string)
	if !ok {
		log.Printf("[DockerService] Missing model_path in response - TaskID: %s, Data: %+v", task.TaskID, data)
		return "", fmt.Errorf("响应中缺少model_path")
	}

	// 获取远程任务ID（重要：用于后续convert API调用）
	remoteTaskID, _ := data["task_id"].(string)
	log.Printf("[DockerService] Model uploaded successfully - TaskID: %s, ModelPath: %s, RemoteTaskID: %s",
		task.TaskID, modelPath, remoteTaskID)

	// 返回格式：modelPath|remoteTaskID，这样调用方可以解析出两个值
	return fmt.Sprintf("%s|%s", modelPath, remoteTaskID), nil
}

// StartConversion 开始模型转换
func (s *dockerService) StartConversion(ctx context.Context, task *models.ConversionTask, modelPath string, remoteTaskId string) error {
	log.Printf("[DockerService] StartConversion started - TaskID: %s, ModelPath: %s, RemoteTaskID: %s",
		task.TaskID, modelPath, remoteTaskId)

	// 获取 quantization 参数
	quantization := task.Parameters.Quantization
	if quantization == "" {
		quantization = "F16" // 默认值
	}

	// 构建转换请求 - 使用远程任务ID
	convertReq := ConvertRequest{
		TaskID:       remoteTaskId,                        // 使用上传时返回的远程任务ID
		ModelType:    string(task.OriginalModel.TaskType), // 使用原始模型的任务类型
		ModelPath:    modelPath,
		ChipType:     strings.ToLower(task.Parameters.TargetChip),
		Quantization: quantization,
	}

	// 从Parameters中获取input_shape（通常使用宽度作为单一参数）
	if len(task.Parameters.InputShape) >= 4 {
		// InputShape格式: [batch, channels, height, width]
		convertReq.InputShape = fmt.Sprintf("%d", task.Parameters.InputShape[3]) // 使用width
		log.Printf("[DockerService] Using input width from parameters - TaskID: %s, Shape: %v, Width: %s",
			task.TaskID, task.Parameters.InputShape, convertReq.InputShape)
	} else if len(task.Parameters.InputShape) >= 3 {
		// 如果没有batch维度: [channels, height, width]
		convertReq.InputShape = fmt.Sprintf("%d", task.Parameters.InputShape[2]) // 使用width
		log.Printf("[DockerService] Using input width from parameters (no batch) - TaskID: %s, Shape: %v, Width: %s",
			task.TaskID, task.Parameters.InputShape, convertReq.InputShape)
	} else {
		convertReq.InputShape = "640" // 默认输入尺寸
		log.Printf("[DockerService] Using default input shape - TaskID: %s, Width: %s",
			task.TaskID, convertReq.InputShape)
	}

	log.Printf("[DockerService] Conversion request prepared - TaskID: %s, RemoteTaskID: %s, ModelType: %s, ChipType: %s, InputShape: %s, Quantization: %s",
		task.TaskID, remoteTaskId, convertReq.ModelType, convertReq.ChipType, convertReq.InputShape, convertReq.Quantization)

	// 序列化请求
	reqBody, err := json.Marshal(convertReq)
	if err != nil {
		return fmt.Errorf("序列化转换请求失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/convert", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[DockerService] Failed to read response body - TaskID: %s, Error: %v", task.TaskID, err)
		return fmt.Errorf("读取响应失败: %w", err)
	}

	log.Printf("[DockerService] Conversion response received - TaskID: %s, Status: %d, Body: %s",
		task.TaskID, resp.StatusCode, string(bodyBytes))

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("[DockerService] HTTP error - TaskID: %s, Status: %d, URL: %s",
			task.TaskID, resp.StatusCode, req.URL.String())

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("转换接口不存在 (404): %s", req.URL.String())
		}
		return fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var dockerResp DockerResponse
	if err := json.Unmarshal(bodyBytes, &dockerResp); err != nil {
		log.Printf("[DockerService] Failed to unmarshal response - TaskID: %s, Body: %s, Error: %v",
			task.TaskID, string(bodyBytes), err)

		// 尝试解析为通用map以获取更多信息
		var genericResp map[string]interface{}
		if err2 := json.Unmarshal(bodyBytes, &genericResp); err2 == nil {
			log.Printf("[DockerService] Generic response parsed - TaskID: %s, Response: %+v", task.TaskID, genericResp)

			// 检查是否是成功响应
			if success, ok := genericResp["success"].(bool); ok && success {
				log.Printf("[DockerService] Conversion started successfully - TaskID: %s", task.TaskID)
				return nil
			}

			// 检查错误信息
			if errorMsg, ok := genericResp["error"].(string); ok {
				return fmt.Errorf("启动转换失败: %s", errorMsg)
			}
		}

		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !dockerResp.Success {
		log.Printf("[DockerService] Conversion start failed - TaskID: %s, Error: %s", task.TaskID, dockerResp.Error)
		return fmt.Errorf("启动转换失败: %s", dockerResp.Error)
	}

	log.Printf("[DockerService] Conversion started successfully - TaskID: %s", task.TaskID)
	return nil
}

// StopConversion 停止转换
func (s *dockerService) StopConversion(ctx context.Context, taskID string) error {
	log.Printf("[DockerService] StopConversion started - TaskID: %s", taskID)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/stop/"+taskID, nil)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[DockerService] Failed to send stop request - TaskID: %s, Error: %v", taskID, err)
		return fmt.Errorf("发送停止请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[DockerService] Failed to read response body - TaskID: %s, Error: %v", taskID, err)
		return fmt.Errorf("读取响应失败: %w", err)
	}

	log.Printf("[DockerService] Stop response received - TaskID: %s, Status: %d, Body: %s",
		taskID, resp.StatusCode, string(bodyBytes))

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("[DockerService] HTTP error - TaskID: %s, Status: %d, URL: %s",
			taskID, resp.StatusCode, req.URL.String())

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("停止接口不存在 (404): %s", req.URL.String())
		}
		return fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var dockerResp DockerResponse
	if err := json.Unmarshal(bodyBytes, &dockerResp); err != nil {
		log.Printf("[DockerService] Failed to unmarshal response - TaskID: %s, Body: %s, Error: %v",
			taskID, string(bodyBytes), err)
		// 即使解析失败，如果HTTP状态码是200，我们仍然认为停止成功
		return nil
	}

	if !dockerResp.Success {
		return fmt.Errorf("停止转换失败: %s", dockerResp.Error)
	}

	log.Printf("[DockerService] StopConversion completed successfully - TaskID: %s", taskID)
	return nil
}

// GetConversionStatus 获取转换状态
func (s *dockerService) GetConversionStatus(ctx context.Context, taskID string) (*ConversionStatus, error) {
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/api/status/"+taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var dockerResp DockerResponse
	if err := json.Unmarshal(bodyBytes, &dockerResp); err != nil {
		// 尝试解析为通用map以获取更多信息
		var genericResp map[string]interface{}
		if err2 := json.Unmarshal(bodyBytes, &genericResp); err2 == nil {
			// 检查是否是成功响应
			if success, ok := genericResp["success"].(bool); ok && success {
				// 直接从data字段构建ConversionStatus
				if data, ok := genericResp["data"].(map[string]interface{}); ok {
					statusData, err := json.Marshal(data)
					if err != nil {
						return nil, fmt.Errorf("序列化状态数据失败: %w", err)
					}

					var status ConversionStatus
					if err := json.Unmarshal(statusData, &status); err != nil {
						return nil, fmt.Errorf("反序列化状态数据失败: %w", err)
					}

					return &status, nil
				}
			}

			// 检查错误信息
			if errorMsg, ok := genericResp["error"].(string); ok {
				return nil, fmt.Errorf("获取状态失败: %s", errorMsg)
			}
		}

		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !dockerResp.Success {
		return nil, fmt.Errorf("获取状态失败: %s", dockerResp.Error)
	}

	// 将Data转换为ConversionStatus
	statusData, err := json.Marshal(dockerResp.Data)
	if err != nil {
		return nil, fmt.Errorf("序列化状态数据失败: %w", err)
	}

	var status ConversionStatus
	if err := json.Unmarshal(statusData, &status); err != nil {
		return nil, fmt.Errorf("反序列化状态数据失败: %w", err)
	}

	return &status, nil
}

// DownloadResult 下载转换结果
func (s *dockerService) DownloadResult(ctx context.Context, taskID string, outputPath string) error {
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/api/download/"+taskID, nil)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建输出文件
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	// 复制文件内容
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	return nil
}
