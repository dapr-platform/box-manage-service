/*
 * @module service/model_deployment_service
 * @description 模型部署服务，提供转换后模型到盒子的部署功能
 * @architecture 服务层
 * @documentReference REQ-004: 模型部署功能
 * @stateFlow Controller -> Service -> Repository -> Database / BoxClient
 * @rules 实现模型部署业务逻辑，包括模型检查、上传、部署跟踪
 * @dependencies box-manage-service/repository, box-manage-service/client
 * @refs DESIGN-007.md
 */

package service

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ModelDeploymentService 模型部署服务接口
type ModelDeploymentService interface {
	// 创建部署任务
	CreateDeploymentTask(ctx context.Context, req *CreateDeploymentRequest) (*models.ModelDeploymentTask, error)

	// 获取部署任务
	GetDeploymentTask(ctx context.Context, taskID string) (*models.ModelDeploymentTask, error)
	GetDeploymentTasks(ctx context.Context, userID uint) ([]*models.ModelDeploymentTask, error)

	// 执行部署
	StartDeployment(ctx context.Context, taskID string) error
	CancelDeployment(ctx context.Context, taskID string) error

	// 删除部署任务
	DeleteDeploymentTask(ctx context.Context, taskID string) error

	// 获取部署项
	GetDeploymentItems(ctx context.Context, taskID string) ([]*models.ModelDeploymentItem, error)
}

// CreateDeploymentRequest 创建部署请求
type CreateDeploymentRequest struct {
	Name              string `json:"name"`                // 任务名称
	ConvertedModelIDs []uint `json:"converted_model_ids"` // 转换后模型ID列表
	BoxIDs            []uint `json:"box_ids"`             // 目标盒子ID列表
	UserID            uint   `json:"user_id"`             // 用户ID
}

// modelDeploymentService 模型部署服务实现
type modelDeploymentService struct {
	deploymentRepo         repository.ModelDeploymentRepository
	convertedModelRepo     repository.ConvertedModelRepository
	boxRepo                repository.BoxRepository
	modelBoxDeploymentRepo repository.ModelBoxDeploymentRepository
}

// NewModelDeploymentService 创建模型部署服务
func NewModelDeploymentService(
	deploymentRepo repository.ModelDeploymentRepository,
	convertedModelRepo repository.ConvertedModelRepository,
	boxRepo repository.BoxRepository,
	modelBoxDeploymentRepo repository.ModelBoxDeploymentRepository,
) ModelDeploymentService {
	return &modelDeploymentService{
		deploymentRepo:         deploymentRepo,
		convertedModelRepo:     convertedModelRepo,
		boxRepo:                boxRepo,
		modelBoxDeploymentRepo: modelBoxDeploymentRepo,
	}
}

// CreateDeploymentTask 创建部署任务
func (s *modelDeploymentService) CreateDeploymentTask(ctx context.Context, req *CreateDeploymentRequest) (*models.ModelDeploymentTask, error) {
	log.Printf("[ModelDeploymentService] CreateDeploymentTask started - Name: %s, Models: %v, Boxes: %v, UserID: %d",
		req.Name, req.ConvertedModelIDs, req.BoxIDs, req.UserID)

	// 验证输入
	if len(req.ConvertedModelIDs) == 0 {
		return nil, fmt.Errorf("至少需要选择一个转换后模型")
	}
	if len(req.BoxIDs) == 0 {
		return nil, fmt.Errorf("至少需要选择一个目标盒子")
	}

	// 验证模型存在性和状态
	for _, modelID := range req.ConvertedModelIDs {
		model, err := s.convertedModelRepo.GetByID(ctx, modelID)
		if err != nil {
			return nil, fmt.Errorf("获取转换后模型失败 (ID: %d): %w", modelID, err)
		}
		if model == nil {
			return nil, fmt.Errorf("转换后模型不存在 (ID: %d)", modelID)
		}
		if !model.IsValidForDeployment() {
			return nil, fmt.Errorf("模型 %s 状态不允许部署 (状态: %s)", model.Name, model.Status)
		}
	}

	// 验证盒子存在性和状态
	for _, boxID := range req.BoxIDs {
		box, err := s.boxRepo.GetByID(ctx, boxID)
		if err != nil {
			return nil, fmt.Errorf("获取盒子失败 (ID: %d): %w", boxID, err)
		}
		if box == nil {
			return nil, fmt.Errorf("盒子不存在 (ID: %d)", boxID)
		}
		if !box.IsOnline() {
			return nil, fmt.Errorf("盒子 %s 不在线，无法部署", box.Name)
		}
	}

	// 创建部署任务
	taskID := fmt.Sprintf("deploy_%s", uuid.New().String()[:8])
	task := &models.ModelDeploymentTask{
		TaskID:     taskID,
		Name:       req.Name,
		UserID:     req.UserID,
		Status:     models.ModelDeploymentStatusPending,
		TotalItems: len(req.ConvertedModelIDs) * len(req.BoxIDs),
		Progress:   0,
	}

	// 设置ID列表
	if err := task.SetConvertedModelIDs(req.ConvertedModelIDs); err != nil {
		return nil, fmt.Errorf("设置转换后模型ID列表失败: %w", err)
	}
	if err := task.SetBoxIDs(req.BoxIDs); err != nil {
		return nil, fmt.Errorf("设置盒子ID列表失败: %w", err)
	}

	task.AddLog("部署任务创建成功")

	// 保存任务
	if err := s.deploymentRepo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("保存部署任务失败: %w", err)
	}

	// 创建部署项
	items, err := s.createDeploymentItems(ctx, task, req.ConvertedModelIDs, req.BoxIDs)
	if err != nil {
		return nil, fmt.Errorf("创建部署项失败: %w", err)
	}

	// 保存部署项
	if err := s.deploymentRepo.CreateItems(ctx, items); err != nil {
		return nil, fmt.Errorf("保存部署项失败: %w", err)
	}

	log.Printf("[ModelDeploymentService] CreateDeploymentTask completed - TaskID: %s, Items: %d", taskID, len(items))
	return task, nil
}

// createDeploymentItems 创建部署项
func (s *modelDeploymentService) createDeploymentItems(ctx context.Context, task *models.ModelDeploymentTask, modelIDs []uint, boxIDs []uint) ([]*models.ModelDeploymentItem, error) {
	var items []*models.ModelDeploymentItem

	for _, modelID := range modelIDs {
		// 获取模型信息
		model, err := s.convertedModelRepo.GetByID(ctx, modelID)
		if err != nil {
			return nil, err
		}

		for _, boxID := range boxIDs {
			// 获取盒子信息
			box, err := s.boxRepo.GetByID(ctx, boxID)
			if err != nil {
				return nil, err
			}

			// 创建部署项
			item := &models.ModelDeploymentItem{
				TaskID:           task.TaskID,
				ConvertedModelID: modelID,
				BoxID:            boxID,
				Status:           models.ModelDeploymentItemStatusPending,
				ModelKey:         model.ModelKey,
				BoxAddress:       fmt.Sprintf("%s:%d", box.IPAddress, box.Port),
				UploadNeeded:     true,
				UploadSize:       model.FileSize,
			}

			items = append(items, item)
		}
	}

	return items, nil
}

// GetDeploymentTask 获取部署任务
func (s *modelDeploymentService) GetDeploymentTask(ctx context.Context, taskID string) (*models.ModelDeploymentTask, error) {
	return s.deploymentRepo.GetTaskByTaskID(ctx, taskID)
}

// GetDeploymentTasks 获取用户的部署任务
func (s *modelDeploymentService) GetDeploymentTasks(ctx context.Context, userID uint) ([]*models.ModelDeploymentTask, error) {
	return s.deploymentRepo.GetTasksByUserID(ctx, userID)
}

// StartDeployment 启动部署
func (s *modelDeploymentService) StartDeployment(ctx context.Context, taskID string) error {
	log.Printf("[ModelDeploymentService] StartDeployment called - TaskID: %s", taskID)

	// 获取任务
	task, err := s.deploymentRepo.GetTaskByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取部署任务失败: %w", err)
	}

	// 验证任务状态
	if task.Status != models.ModelDeploymentStatusPending && task.Status != models.ModelDeploymentStatusFailed {
		return fmt.Errorf("任务状态不允许启动 (当前状态: %s)", task.Status)
	}

	// 启动任务
	task.Start()
	task.AddLog("开始执行部署任务")

	if err := s.deploymentRepo.UpdateTask(ctx, task); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 异步执行部署
	go s.executeDeployment(context.Background(), taskID)

	return nil
}

// executeDeployment 执行部署
func (s *modelDeploymentService) executeDeployment(ctx context.Context, taskID string) {
	log.Printf("[ModelDeploymentService] executeDeployment started - TaskID: %s", taskID)

	// 获取任务和部署项
	task, err := s.deploymentRepo.GetTaskByTaskID(ctx, taskID)
	if err != nil {
		log.Printf("[ModelDeploymentService] Failed to get task - TaskID: %s, Error: %v", taskID, err)
		return
	}

	items, err := s.deploymentRepo.GetItemsByTaskID(ctx, taskID)
	if err != nil {
		log.Printf("[ModelDeploymentService] Failed to get deployment items - TaskID: %s, Error: %v", taskID, err)
		task.Fail(fmt.Sprintf("获取部署项失败: %v", err))
		s.deploymentRepo.UpdateTask(ctx, task)
		return
	}

	task.AddLog(fmt.Sprintf("开始处理 %d 个部署项", len(items)))

	// 逐个处理部署项
	for _, item := range items {
		if err := s.processDeploymentItem(ctx, task, item); err != nil {
			log.Printf("[ModelDeploymentService] Failed to process deployment item - ItemID: %d, Error: %v", item.ID, err)
			task.FailedItems++
		}

		// 更新任务进度
		task.UpdateProgress()
		s.deploymentRepo.UpdateTask(ctx, task)
	}

	// 完成任务
	if task.FailedItems == 0 {
		task.Complete()
		task.AddLog("所有模型部署成功")
	} else if task.SuccessItems == 0 {
		task.Fail("所有模型部署失败")
	} else {
		task.Complete()
		task.AddLog(fmt.Sprintf("部署完成: 成功 %d, 失败 %d, 跳过 %d", task.SuccessItems, task.FailedItems, task.SkippedItems))
	}

	s.deploymentRepo.UpdateTask(ctx, task)
	log.Printf("[ModelDeploymentService] executeDeployment completed - TaskID: %s", taskID)
}

// processDeploymentItem 处理单个部署项
func (s *modelDeploymentService) processDeploymentItem(ctx context.Context, task *models.ModelDeploymentTask, item *models.ModelDeploymentItem) error {
	log.Printf("[ModelDeploymentService] Processing deployment item - ModelKey: %s, Box: %s", item.ModelKey, item.BoxAddress)

	// 启动部署项
	item.Start()
	s.deploymentRepo.UpdateItem(ctx, item)

	// 获取模型和盒子信息
	model, err := s.convertedModelRepo.GetByID(ctx, item.ConvertedModelID)
	if err != nil {
		item.Fail(fmt.Sprintf("获取模型信息失败: %v", err))
		s.deploymentRepo.UpdateItem(ctx, item)
		return err
	}

	box, err := s.boxRepo.GetByID(ctx, item.BoxID)
	if err != nil {
		item.Fail(fmt.Sprintf("获取盒子信息失败: %v", err))
		s.deploymentRepo.UpdateItem(ctx, item)
		return err
	}

	// 创建盒子客户端
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	// 检查模型是否已存在
	task.AddLog(fmt.Sprintf("检查盒子 %s 上的模型 %s", box.Name, model.ModelKey))

	modelKeys, err := boxClient.GetInstalledModelKeys(ctx)
	if err != nil {
		item.Fail(fmt.Sprintf("检查盒子模型失败: %v", err))
		s.deploymentRepo.UpdateItem(ctx, item)
		return err
	}

	// 检查模型是否已存在
	modelExists := false
	for _, key := range modelKeys {
		if key == model.ModelKey {
			modelExists = true
			break
		}
	}

	item.ModelExists = modelExists
	item.UploadNeeded = !modelExists

	if modelExists {
		// 模型已存在，跳过上传
		task.AddLog(fmt.Sprintf("模型 %s 已存在于盒子 %s，跳过上传", model.ModelKey, box.Name))
		item.Skip("模型已存在")
		task.SkippedItems++
	} else {
		// 需要上传模型
		task.AddLog(fmt.Sprintf("开始上传模型 %s 到盒子 %s", model.ModelKey, box.Name))
		item.SetUploading()
		s.deploymentRepo.UpdateItem(ctx, item)

		// 执行模型上传
		uploadReq := &client.ModelUploadRequest{
			ModelName:  model.Name,
			Type:       string(model.TaskType),
			Version:    model.TargetYoloVersion,
			Hardware:   model.TargetChip,
			FilePath:   model.FilePath,
			MD5Sum:     model.FileMD5,
			Confidence: 0.5, // 默认置信度
			NMS:        0.4, // 默认NMS阈值
		}

		if err := boxClient.UploadModel(ctx, uploadReq); err != nil {
			task.AddLog(fmt.Sprintf("上传模型 %s 到盒子 %s 失败: %v", model.ModelKey, box.Name, err))
			item.Fail(fmt.Sprintf("上传模型失败: %v", err))
			s.deploymentRepo.UpdateItem(ctx, item)
			return err
		}

		task.AddLog(fmt.Sprintf("模型 %s 上传到盒子 %s 成功", model.ModelKey, box.Name))
		item.Complete()
		task.SuccessItems++

		// 创建部署关联关系
		if err := s.createDeploymentRelation(ctx, model, box, task.TaskID); err != nil {
			log.Printf("[ModelDeploymentService] Failed to create deployment relation - Model: %s, Box: %s, Error: %v",
				model.ModelKey, box.Name, err)
			task.AddLog(fmt.Sprintf("创建部署关联关系失败: %v", err))
		} else {
			task.AddLog(fmt.Sprintf("已记录模型 %s 在盒子 %s 的部署关联", model.ModelKey, box.Name))
		}
	}

	s.deploymentRepo.UpdateItem(ctx, item)
	return nil
}

// CancelDeployment 取消部署
func (s *modelDeploymentService) CancelDeployment(ctx context.Context, taskID string) error {
	task, err := s.deploymentRepo.GetTaskByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取部署任务失败: %w", err)
	}

	if task.Status != models.ModelDeploymentStatusRunning {
		return fmt.Errorf("只有运行中的任务可以取消")
	}

	task.Cancel()
	task.AddLog("部署任务已被用户取消")

	return s.deploymentRepo.UpdateTask(ctx, task)
}

// GetDeploymentItems 获取部署项
func (s *modelDeploymentService) GetDeploymentItems(ctx context.Context, taskID string) ([]*models.ModelDeploymentItem, error) {
	return s.deploymentRepo.GetItemsByTaskID(ctx, taskID)
}

// DeleteDeploymentTask 删除部署任务
func (s *modelDeploymentService) DeleteDeploymentTask(ctx context.Context, taskID string) error {
	log.Printf("[ModelDeploymentService] DeleteDeploymentTask called - TaskID: %s", taskID)

	// 获取任务以检查状态
	task, err := s.deploymentRepo.GetTaskByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取部署任务失败: %w", err)
	}

	// 检查任务状态，运行中的任务不能删除
	if task.Status == models.ModelDeploymentStatusRunning {
		return fmt.Errorf("运行中的任务不能删除，请先取消任务")
	}

	// 删除与该任务相关的部署关联关系
	deployments, err := s.modelBoxDeploymentRepo.GetDeploymentsByTaskID(ctx, taskID)
	if err != nil {
		log.Printf("[ModelDeploymentService] Failed to get deployments for task %s: %v", taskID, err)
	} else {
		for _, deployment := range deployments {
			// 将部署关联标记为已移除，而不是直接删除
			deployment.MarkAsRemoved()
			if err := s.modelBoxDeploymentRepo.Update(ctx, deployment); err != nil {
				log.Printf("[ModelDeploymentService] Failed to mark deployment as removed: %v", err)
			}
		}
	}

	// 删除任务（这会级联删除相关的部署项）
	if err := s.deploymentRepo.DeleteTask(ctx, task.ID); err != nil {
		return fmt.Errorf("删除部署任务失败: %w", err)
	}

	log.Printf("[ModelDeploymentService] DeleteDeploymentTask completed - TaskID: %s", taskID)
	return nil
}

// createDeploymentRelation 创建部署关联关系
func (s *modelDeploymentService) createDeploymentRelation(ctx context.Context, model *models.ConvertedModel, box *models.Box, taskID string) error {
	// 检查是否已存在部署关联
	existing, err := s.modelBoxDeploymentRepo.GetByModelAndBox(ctx, model.ID, box.ID)
	if err == nil && existing != nil {
		// 更新现有关联为活跃状态
		existing.MarkAsActive()
		existing.TaskID = taskID
		existing.DeployedAt = time.Now()
		existing.Version = model.Version
		return s.modelBoxDeploymentRepo.Update(ctx, existing)
	}

	// 创建新的部署关联
	deployment := &models.ModelBoxDeployment{
		ConvertedModelID: model.ID,
		BoxID:            box.ID,
		ModelKey:         model.ModelKey,
		TaskID:           taskID,
		DeployedAt:       time.Now(),
		Version:          model.Version,
		Status:           models.ModelBoxDeploymentStatusActive,
		IsVerified:       true,
	}

	return s.modelBoxDeploymentRepo.Create(ctx, deployment)
}
