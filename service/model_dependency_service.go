/*
 * @module service/model_dependency_service
 * @description 模型依赖检查服务 - 检查模型部署状态和依赖关系
 * @architecture 服务层
 * @documentReference REQ-005: 任务管理功能, DESIGN-007: 模型下发机制设计
 * @stateFlow 任务创建 -> 模型依赖检查 -> 模型下发 -> 任务调度
 * @rules 确保任务执行前所需模型已正确部署到目标盒子
 * @dependencies repository, models
 * @refs DESIGN-007.md, REQ-005.md
 */

package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"box-manage-service/models"
	"box-manage-service/repository"
)

// DependencyCheckResult 依赖检查结果
type DependencyCheckResult struct {
	TaskID        uint              `json:"task_id"`
	TaskName      string            `json:"task_name"`
	RequiredModel string            `json:"required_model"`
	BoxID         uint              `json:"box_id"`
	BoxName       string            `json:"box_name"`
	IsAvailable   bool              `json:"is_available"`
	Status        DependencyStatus  `json:"status"`
	Issues        []DependencyIssue `json:"issues"`
	Suggestions   []string          `json:"suggestions"`
	CheckedAt     time.Time         `json:"checked_at"`
}

// DependencyStatus 依赖状态
type DependencyStatus string

const (
	DependencyStatusSatisfied    DependencyStatus = "satisfied"    // 依赖满足
	DependencyStatusMissing      DependencyStatus = "missing"      // 模型缺失
	DependencyStatusIncompatible DependencyStatus = "incompatible" // 不兼容
	DependencyStatusDeploying    DependencyStatus = "deploying"    // 正在部署
	DependencyStatusError        DependencyStatus = "error"        // 错误状态
)

// DependencyIssue 依赖问题
type DependencyIssue struct {
	Type        IssueType `json:"type"`
	Severity    Severity  `json:"severity"`
	Description string    `json:"description"`
	Solution    string    `json:"solution"`
}

// IssueType 问题类型
type IssueType string

const (
	IssueTypeModelNotFound        IssueType = "model_not_found"       // 模型未找到
	IssueTypeVersionMismatch      IssueType = "version_mismatch"      // 版本不匹配
	IssueTypeHardwareIncompatible IssueType = "hardware_incompatible" // 硬件不兼容
	IssueTypeInsufficientResource IssueType = "insufficient_resource" // 资源不足
	IssueTypeDeploymentFailed     IssueType = "deployment_failed"     // 部署失败
)

// Severity 严重程度
type Severity string

const (
	SeverityLow      Severity = "low"      // 低
	SeverityMedium   Severity = "medium"   // 中
	SeverityHigh     Severity = "high"     // 高
	SeverityCritical Severity = "critical" // 严重
)

// CompatibilityResult 兼容性检查结果
type CompatibilityResult struct {
	BoxID           uint                 `json:"box_id"`
	ModelKey        string               `json:"model_key"`
	IsCompatible    bool                 `json:"is_compatible"`
	Compatibility   CompatibilityDetails `json:"compatibility"`
	Requirements    ModelRequirements    `json:"requirements"`
	BoxCapabilities BoxCapabilities      `json:"box_capabilities"`
	CheckedAt       time.Time            `json:"checked_at"`
}

// CompatibilityDetails 兼容性详情
type CompatibilityDetails struct {
	Hardware     bool `json:"hardware"`      // 硬件兼容性
	Memory       bool `json:"memory"`        // 内存充足性
	Storage      bool `json:"storage"`       // 存储充足性
	Framework    bool `json:"framework"`     // 框架兼容性
	Version      bool `json:"version"`       // 版本兼容性
	OverallScore int  `json:"overall_score"` // 总体兼容性分数 (0-100)
}

// ModelRequirements 模型要求
type ModelRequirements struct {
	MinMemoryGB         float64  `json:"min_memory_gb"`
	MinStorageGB        float64  `json:"min_storage_gb"`
	RequiredGPU         bool     `json:"required_gpu"`
	SupportedFrameworks []string `json:"supported_frameworks"`
	MinFrameworkVersion string   `json:"min_framework_version"`
}

// BoxCapabilities 盒子能力
type BoxCapabilities struct {
	AvailableMemoryGB   float64  `json:"available_memory_gb"`
	AvailableStorageGB  float64  `json:"available_storage_gb"`
	HasGPU              bool     `json:"has_gpu"`
	GPUInfo             string   `json:"gpu_info"`
	InstalledFrameworks []string `json:"installed_frameworks"`
}

// DeploymentStatus 部署状态
type DeploymentStatus struct {
	BoxID         uint         `json:"box_id"`
	ModelKey      string       `json:"model_key"`
	Status        DeployStatus `json:"status"`
	Version       string       `json:"version"`
	DeployedAt    *time.Time   `json:"deployed_at"`
	LastCheckedAt time.Time    `json:"last_checked_at"`
	FilePath      string       `json:"file_path"`
	FileSize      int64        `json:"file_size"`
	Checksum      string       `json:"checksum"`
}

// DeployStatus 部署状态
type DeployStatus string

const (
	DeployStatusNotDeployed DeployStatus = "not_deployed" // 未部署
	DeployStatusDeploying   DeployStatus = "deploying"    // 正在部署
	DeployStatusDeployed    DeployStatus = "deployed"     // 已部署
	DeployStatusFailed      DeployStatus = "failed"       // 部署失败
	DeployStatusOutdated    DeployStatus = "outdated"     // 版本过期
)

// DeploymentResult 部署结果
type DeploymentResult struct {
	BoxID            uint         `json:"box_id"`
	ModelKey         string       `json:"model_key"`
	Status           DeployStatus `json:"status"`
	StartedAt        time.Time    `json:"started_at"`
	CompletedAt      *time.Time   `json:"completed_at"`
	ErrorMessage     string       `json:"error_message"`
	Progress         float64      `json:"progress"`
	TransferredBytes int64        `json:"transferred_bytes"`
	TotalBytes       int64        `json:"total_bytes"`
}

// BoxModel 盒子上的模型信息
type BoxModel struct {
	BoxID      uint         `json:"box_id"`
	ModelKey   string       `json:"model_key"`
	ModelName  string       `json:"model_name"`
	Version    string       `json:"version"`
	Status     DeployStatus `json:"status"`
	DeployedAt time.Time    `json:"deployed_at"`
	FileSize   int64        `json:"file_size"`
	LastUsedAt *time.Time   `json:"last_used_at"`
}

// ModelUsageStats 模型使用统计
type ModelUsageStats struct {
	ModelKey          string             `json:"model_key"`
	TotalBoxes        int                `json:"total_boxes"`
	DeployedBoxes     int                `json:"deployed_boxes"`
	ActiveTasks       int                `json:"active_tasks"`
	TotalTasks        int                `json:"total_tasks"`
	UsageByBox        map[uint]int       `json:"usage_by_box"`
	DeploymentHistory []DeploymentRecord `json:"deployment_history"`
	LastUsedAt        *time.Time         `json:"last_used_at"`
}

// DeploymentRecord 部署记录
type DeploymentRecord struct {
	BoxID      uint         `json:"box_id"`
	BoxName    string       `json:"box_name"`
	DeployedAt time.Time    `json:"deployed_at"`
	Status     DeployStatus `json:"status"`
}

// modelDependencyService 模型依赖检查服务实现
type modelDependencyService struct {
	taskRepo           repository.TaskRepository
	boxRepo            repository.BoxRepository
	convertedModelRepo repository.ConvertedModelRepository
}

// NewModelDependencyService 创建模型依赖检查服务实例
func NewModelDependencyService(
	taskRepo repository.TaskRepository,
	boxRepo repository.BoxRepository,
	convertedModelRepo repository.ConvertedModelRepository,
) ModelDependencyService {
	return &modelDependencyService{
		taskRepo:           taskRepo,
		boxRepo:            boxRepo,
		convertedModelRepo: convertedModelRepo,
	}
}

// CheckTaskModelDependency 检查任务的模型依赖
func (s *modelDependencyService) CheckTaskModelDependency(ctx context.Context, taskID uint) (*DependencyCheckResult, error) {
	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	result := &DependencyCheckResult{
		TaskID:        taskID,
		TaskName:      task.Name,
		RequiredModel: "动态生成", // ModelKey现在在运行时动态生成
		CheckedAt:     time.Now(),
		Issues:        []DependencyIssue{},
		Suggestions:   []string{},
	}

	// 检查任务是否有推理任务配置
	if len(task.InferenceTasks) == 0 {
		result.Status = DependencyStatusSatisfied
		result.IsAvailable = true
		result.Suggestions = append(result.Suggestions, "任务未配置推理任务，无需模型依赖检查")
		return result, nil
	}

	// 如果任务还没有分配盒子，检查原始模型是否存在
	if task.BoxID == nil {
		firstInferenceTask := task.InferenceTasks[0]
		if firstInferenceTask.OriginalModelID == "" {
			result.Status = DependencyStatusMissing
			result.IsAvailable = false
			result.Issues = append(result.Issues, DependencyIssue{
				Type:        IssueTypeModelNotFound,
				Severity:    SeverityCritical,
				Description: "任务未指定原始模型ID",
				Solution:    "请为任务配置原始模型ID",
			})
			return result, nil
		}

		result.Status = DependencyStatusSatisfied
		result.IsAvailable = true
		result.Suggestions = append(result.Suggestions, "原始模型已配置，等待任务调度到具体盒子后进行转换模型检查")
		return result, nil
	}

	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, *task.BoxID)
	if err != nil {
		return nil, fmt.Errorf("获取盒子失败: %w", err)
	}

	result.BoxID = box.ID
	result.BoxName = box.Name

	// 由于ModelKey现在是动态生成的，这里暂时跳过具体的部署状态检查
	// TODO: 需要在运行时动态检查转换后模型的可用性
	firstInferenceTask := task.InferenceTasks[0]
	result.RequiredModel = fmt.Sprintf("OriginalModelID: %s", firstInferenceTask.OriginalModelID)

	// 由于ModelKey是动态生成的，这里暂时返回满足状态
	result.Status = DependencyStatusSatisfied
	result.IsAvailable = true
	result.Suggestions = append(result.Suggestions, "任务已分配到盒子，将在部署时动态检查模型依赖")

	return result, nil
}

// CheckBoxModelCompatibility 检查盒子与模型的兼容性
func (s *modelDependencyService) CheckBoxModelCompatibility(ctx context.Context, boxID uint, modelKey string) (*CompatibilityResult, error) {
	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("获取盒子失败: %w", err)
	}

	result := &CompatibilityResult{
		BoxID:     boxID,
		ModelKey:  modelKey,
		CheckedAt: time.Now(),
	}

	// 检查是否为转换后模型
	if strings.HasPrefix(modelKey, "converted-") {
		return s.checkConvertedModelCompatibility(ctx, box, modelKey, result)
	}

	// 获取原始模型信息（保持向后兼容）
	model, err := s.getModelByKey(ctx, modelKey)
	if err != nil {
		return nil, fmt.Errorf("获取模型失败: %w", err)
	}

	// 模拟模型要求（实际应该从模型metadata获取）
	requirements := ModelRequirements{
		MinMemoryGB:         s.getModelMemoryRequirement(model),
		MinStorageGB:        s.getModelStorageRequirement(model),
		RequiredGPU:         s.isGPURequired(model),
		SupportedFrameworks: s.getModelSupportedFrameworks(model),
	}

	// 获取盒子能力
	capabilities := BoxCapabilities{
		AvailableMemoryGB:   s.getBoxAvailableMemory(box),
		AvailableStorageGB:  s.getBoxAvailableStorage(box),
		HasGPU:              s.hasGPU(box),
		GPUInfo:             "Unknown", // GPU信息需要从其他来源获取
		InstalledFrameworks: s.getBoxFrameworks(box),
	}

	// 检查各项兼容性
	compatibility := CompatibilityDetails{}
	score := 0

	// 检查内存
	if capabilities.AvailableMemoryGB >= requirements.MinMemoryGB {
		compatibility.Memory = true
		score += 20
	}

	// 检查存储
	if capabilities.AvailableStorageGB >= requirements.MinStorageGB {
		compatibility.Storage = true
		score += 20
	}

	// 检查GPU
	if !requirements.RequiredGPU || (requirements.RequiredGPU && capabilities.HasGPU) {
		compatibility.Hardware = true
		score += 30
	}

	// 检查框架兼容性
	frameworkCompatible := false
	if len(requirements.SupportedFrameworks) == 0 {
		frameworkCompatible = true // 如果没有特定框架要求
	} else {
		for _, required := range requirements.SupportedFrameworks {
			for _, installed := range capabilities.InstalledFrameworks {
				if strings.EqualFold(required, installed) {
					frameworkCompatible = true
					break
				}
			}
			if frameworkCompatible {
				break
			}
		}
	}

	if frameworkCompatible {
		compatibility.Framework = true
		score += 20
	}

	// 版本兼容性（暂时总是兼容）
	compatibility.Version = true
	score += 10

	compatibility.OverallScore = score
	result.IsCompatible = score >= 80 // 80分以上认为兼容

	result.Compatibility = compatibility
	result.Requirements = requirements
	result.BoxCapabilities = capabilities

	return result, nil
}

// GetModelDeploymentStatus 获取模型在指定盒子上的部署状态
func (s *modelDependencyService) GetModelDeploymentStatus(ctx context.Context, boxID uint, modelKey string) (*DeploymentStatus, error) {
	// TODO: 实现与盒子通信，查询模型部署状态
	// 这里暂时返回模拟数据

	status := &DeploymentStatus{
		BoxID:         boxID,
		ModelKey:      modelKey,
		Status:        DeployStatusNotDeployed,
		LastCheckedAt: time.Now(),
	}

	// 模拟检查逻辑（实际应该通过API调用盒子）
	// 假设部分模型已经部署
	if strings.Contains(modelKey, "yolo") {
		status.Status = DeployStatusDeployed
		deployedTime := time.Now().Add(-24 * time.Hour)
		status.DeployedAt = &deployedTime
		status.Version = "v1.0.0"
		status.FilePath = "/models/" + modelKey
		status.FileSize = 100 * 1024 * 1024 // 100MB
	}

	return status, nil
}

// DeployModelToBox 将模型部署到指定盒子
func (s *modelDependencyService) DeployModelToBox(ctx context.Context, boxID uint, modelKey string) (*DeploymentResult, error) {
	// TODO: 实现模型部署逻辑
	// 这应该包括：
	// 1. 检查模型文件
	// 2. 传输模型到盒子
	// 3. 在盒子上安装/加载模型
	// 4. 验证部署结果

	result := &DeploymentResult{
		BoxID:     boxID,
		ModelKey:  modelKey,
		Status:    DeployStatusDeploying,
		StartedAt: time.Now(),
		Progress:  0,
	}

	log.Printf("开始部署模型 %s 到盒子 %d", modelKey, boxID)

	// 模拟部署过程（实际实现中应该是异步的）
	go s.simulateDeployment(result)

	return result, nil
}

// EnsureModelAvailability 确保模型在盒子上可用
func (s *modelDependencyService) EnsureModelAvailability(ctx context.Context, boxID uint, modelKey string) error {
	// 检查部署状态
	status, err := s.GetModelDeploymentStatus(ctx, boxID, modelKey)
	if err != nil {
		return fmt.Errorf("检查部署状态失败: %w", err)
	}

	// 如果已经部署，直接返回
	if status.Status == DeployStatusDeployed {
		return nil
	}

	// 检查兼容性
	compatibility, err := s.CheckBoxModelCompatibility(ctx, boxID, modelKey)
	if err != nil {
		return fmt.Errorf("检查兼容性失败: %w", err)
	}

	if !compatibility.IsCompatible {
		return fmt.Errorf("盒子与模型不兼容，无法部署")
	}

	// 开始部署
	_, err = s.DeployModelToBox(ctx, boxID, modelKey)
	if err != nil {
		return fmt.Errorf("部署模型失败: %w", err)
	}

	return nil
}

// GetBoxModels 获取盒子上所有已部署的模型
func (s *modelDependencyService) GetBoxModels(ctx context.Context, boxID uint) ([]*BoxModel, error) {
	// TODO: 实现从盒子获取模型列表
	// 这里返回模拟数据

	var models []*BoxModel

	// 模拟一些已部署的模型
	models = append(models, &BoxModel{
		BoxID:      boxID,
		ModelKey:   "yolo-v5",
		ModelName:  "YOLO v5 目标检测",
		Version:    "v1.0.0",
		Status:     DeployStatusDeployed,
		DeployedAt: time.Now().Add(-24 * time.Hour),
		FileSize:   100 * 1024 * 1024,
	})

	return models, nil
}

// GetModelUsage 获取模型的使用情况统计
func (s *modelDependencyService) GetModelUsage(ctx context.Context, modelKey string) (*ModelUsageStats, error) {
	// 统计使用该模型的任务数量
	tasks, err := s.taskRepo.FindTasksByModel(ctx, modelKey)
	if err != nil {
		return nil, fmt.Errorf("查询模型任务失败: %w", err)
	}

	stats := &ModelUsageStats{
		ModelKey:   modelKey,
		TotalTasks: len(tasks),
		UsageByBox: make(map[uint]int),
	}

	// 统计各个盒子的使用情况
	for _, task := range tasks {
		if task.BoxID != nil {
			stats.UsageByBox[*task.BoxID]++
			if task.Status == models.TaskStatusRunning {
				stats.ActiveTasks++
			}
		}
	}

	stats.DeployedBoxes = len(stats.UsageByBox)

	return stats, nil
}

// 辅助方法

func (s *modelDependencyService) checkModelExists(ctx context.Context, modelKey string) (bool, error) {
	// 检查是否为转换后模型的ModelKey格式 (converted-{chip}-{precision}-{name})
	if strings.HasPrefix(modelKey, "converted-") {
		// 查找转换后模型
		models, err := s.convertedModelRepo.GetActiveModels(ctx)
		if err != nil {
			return false, fmt.Errorf("查询转换后模型失败: %w", err)
		}

		for _, model := range models {
			if model.ModelKey == modelKey {
				return true, nil
			}
		}
		return false, nil
	}

	// 原始模型的检查逻辑（保持向后兼容）
	// 这里可以根据modelKey格式解析出模型信息进行查找
	// 暂时返回true保持向后兼容
	return true, nil
}

// checkConvertedModelCompatibility 检查转换后模型的兼容性
func (s *modelDependencyService) checkConvertedModelCompatibility(ctx context.Context, box *models.Box, modelKey string, result *CompatibilityResult) (*CompatibilityResult, error) {
	// 从modelKey中解析芯片类型 (converted-{chip}-{precision}-{name})
	parts := strings.Split(modelKey, "-")
	if len(parts) < 4 {
		result.IsCompatible = false
		result.Compatibility = CompatibilityDetails{
			Hardware:     false,
			Memory:       true,
			Storage:      true,
			Framework:    true,
			Version:      true,
			OverallScore: 20,
		}
		return result, nil
	}

	modelChip := parts[1]
	modelPrecision := parts[2]

	// 检查盒子芯片类型是否匹配
	// 假设盒子的Hardware字段包含芯片信息
	hardwareInfo := fmt.Sprintf("%s %s %s", box.Hardware.Platform, box.Hardware.CPUModelName, box.Hardware.KernelArch)
	hardwareMatch := strings.Contains(hardwareInfo, modelChip) || strings.Contains(hardwareInfo, strings.ToLower(modelChip))

	// 检查资源是否足够（这里简化处理）
	memoryMatch := true
	storageMatch := true

	// 简化的内存检查（假设盒子有足够内存）
	if modelPrecision == "fp32" {
		// FP32模型通常需要更多内存，这里简化检查
		memoryMatch = box.Hardware.CPUCores >= 2 // 使用CPU核心数作为简化指标
	}

	result.IsCompatible = hardwareMatch && memoryMatch && storageMatch

	score := 0
	if hardwareMatch {
		score += 40
	}
	if memoryMatch {
		score += 30
	}
	if storageMatch {
		score += 30
	}

	result.Compatibility = CompatibilityDetails{
		Hardware:     hardwareMatch,
		Memory:       memoryMatch,
		Storage:      storageMatch,
		Framework:    true, // 软件兼容性通过转换保证
		Version:      true,
		OverallScore: score,
	}

	return result, nil
}

func (s *modelDependencyService) getModelByKey(ctx context.Context, modelKey string) (*models.OriginalModel, error) {
	// TODO: 通过modelKey获取模型信息
	// 这里返回模拟数据
	return &models.OriginalModel{
		Name:      modelKey,
		ModelType: "pt",
		Framework: "pytorch",
		Version:   "1.0.0",
	}, nil
}

func (s *modelDependencyService) getModelMemoryRequirement(model *models.OriginalModel) float64 {
	// 根据模型类型估算内存需求
	switch model.ModelType {
	case "pt":
		return 2.0 // 2GB
	case "onnx":
		return 4.0 // 4GB
	default:
		return 2.0
	}
}

func (s *modelDependencyService) getModelStorageRequirement(model *models.OriginalModel) float64 {
	// 根据模型类型估算存储需求
	switch model.ModelType {
	case "pt":
		return 0.5 // 500MB
	case "onnx":
		return 1.0 // 1GB
	default:
		return 0.5
	}
}

func (s *modelDependencyService) isGPURequired(model *models.OriginalModel) bool {
	// 假设所有模型都可以使用GPU加速，但不是必须的
	return false
}

func (s *modelDependencyService) getModelSupportedFrameworks(model *models.OriginalModel) []string {
	return []string{model.Framework}
}

func (s *modelDependencyService) getBoxAvailableMemory(box *models.Box) float64 {
	available := box.Resources.MemoryAvailable
	return float64(available) / (1024 * 1024 * 1024) // 转换为GB
}

func (s *modelDependencyService) getBoxAvailableStorage(box *models.Box) float64 {
	// 计算主要磁盘的可用空间（取第一个磁盘或根目录）
	if len(box.Resources.DiskData) > 0 {
		for _, disk := range box.Resources.DiskData {
			if disk.Path == "/" || disk.Path == "/data" {
				return float64(disk.Free) / (1024 * 1024 * 1024) // 转换为GB
			}
		}
		// 如果没找到根目录，返回第一个磁盘的可用空间
		return float64(box.Resources.DiskData[0].Free) / (1024 * 1024 * 1024)
	}
	return 0
}

func (s *modelDependencyService) hasGPU(box *models.Box) bool {
	// 基于TPU使用情况判断是否有TPU（算能盒子主要是TPU）
	return box.Resources.TPUUsed >= 0
}

func (s *modelDependencyService) getBoxFrameworks(box *models.Box) []string {
	// TODO: 从盒子获取已安装的框架列表
	// 这里返回模拟数据
	return []string{"pytorch", "onnx", "tensorflow"}
}

func (s *modelDependencyService) simulateDeployment(result *DeploymentResult) {
	// 模拟部署过程
	for i := 0; i <= 100; i += 10 {
		time.Sleep(100 * time.Millisecond)
		result.Progress = float64(i)
		log.Printf("部署进度: %d%%", i)
	}

	result.Status = DeployStatusDeployed
	completedTime := time.Now()
	result.CompletedAt = &completedTime
	log.Printf("模型 %s 部署到盒子 %d 完成", result.ModelKey, result.BoxID)
}
