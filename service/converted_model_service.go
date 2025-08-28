/*
 * @module service/converted_model_service
 * @description 转换后模型管理服务，提供转换后模型的CRUD和查询功能
 * @architecture 服务层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow Controller -> ConvertedModelService -> Repository -> Database
 * @rules 实现转换后模型的业务逻辑，包括文件管理、查询等
 * @dependencies box-manage-service/repository, box-manage-service/models
 * @refs REQ-003.md, DESIGN-005.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

// ConvertedModelConfig 转换后模型服务配置
type ConvertedModelConfig struct {
	StorageBasePath    string `yaml:"storage_base_path"`     // 存储基础路径
	MaxFileSize        int64  `yaml:"max_file_size"`         // 最大文件大小
	DeleteFileOnRemove bool   `yaml:"delete_file_on_remove"` // 删除时是否删除文件
}

// convertedModelService 转换后模型服务实现
type convertedModelService struct {
	convertedModelRepo repository.ConvertedModelRepository
	originalModelRepo  repository.OriginalModelRepository
	config             *ConvertedModelConfig
}

// NewConvertedModelService 创建转换后模型服务实例
func NewConvertedModelService(
	convertedModelRepo repository.ConvertedModelRepository,
	originalModelRepo repository.OriginalModelRepository,
	config *ConvertedModelConfig,
) ConvertedModelService {
	return &convertedModelService{
		convertedModelRepo: convertedModelRepo,
		originalModelRepo:  originalModelRepo,
		config:             config,
	}
}

// CreateConvertedModel 创建转换后模型
func (s *convertedModelService) CreateConvertedModel(ctx context.Context, req *CreateConvertedModelRequest) (*models.ConvertedModel, error) {
	log.Printf("[ConvertedModelService] CreateConvertedModel started - Name: %s, UserID: %d",
		req.Name, req.UserID)

	// 验证原始模型是否存在
	_, err := s.originalModelRepo.GetByID(ctx, req.OriginalModelID)
	if err != nil {
		log.Printf("[ConvertedModelService] Original model not found - ID: %d, Error: %v", req.OriginalModelID, err)
		return nil, fmt.Errorf("原始模型不存在: %w", err)
	}

	// 检查名称是否重复
	exists, err := s.convertedModelRepo.IsNameExists(ctx, req.Name)
	if err != nil {
		log.Printf("[ConvertedModelService] Failed to check name existence - Name: %s, Error: %v", req.Name, err)
		return nil, fmt.Errorf("检查名称重复失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("转换后模型名称已存在: %s", req.Name)
	}

	// 创建转换后模型
	model := &models.ConvertedModel{
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		Version:          req.Version,
		OriginalModelID:  req.OriginalModelID,
		ConversionTaskID: req.ConversionTaskID,
		FileName:         req.FileName,
		FilePath:         req.FilePath,
		ConvertedPath:    req.FilePath, // 设置转换后文件路径，与FilePath相同
		FileSize:         req.FileSize,
		FileMD5:          req.FileMD5,
		FileSHA256:       req.FileSHA256,
		ConvertParams:    req.ConvertParams,
		Quantization:     req.Quantization,
		Status:           models.ConvertedModelStatusCompleted,
		ConvertedAt:      time.Now(),
		UserID:           req.UserID,
	}

	// 设置标签
	if len(req.Tags) > 0 {
		model.SetTagList(req.Tags)
	}

	// 保存到数据库
	if err := s.convertedModelRepo.Create(ctx, model); err != nil {
		log.Printf("[ConvertedModelService] Failed to create converted model - Name: %s, Error: %v", req.Name, err)
		return nil, fmt.Errorf("创建转换后模型失败: %w", err)
	}

	log.Printf("[ConvertedModelService] CreateConvertedModel completed - ID: %d, Name: %s", model.ID, model.Name)
	return model, nil
}

// GetConvertedModel 获取转换后模型
func (s *convertedModelService) GetConvertedModel(ctx context.Context, id uint) (*models.ConvertedModel, error) {
	log.Printf("[ConvertedModelService] GetConvertedModel started - ID: %d", id)

	model, err := s.convertedModelRepo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[ConvertedModelService] Failed to get converted model - ID: %d, Error: %v", id, err)
		return nil, fmt.Errorf("获取转换后模型失败: %w", err)
	}

	// 增加访问次数
	model.IncrementAccessCount()
	if err := s.convertedModelRepo.UpdateAccessTime(ctx, id); err != nil {
		log.Printf("[ConvertedModelService] Failed to update access time - ID: %d, Error: %v", id, err)
		// 不影响主流程
	}

	log.Printf("[ConvertedModelService] GetConvertedModel completed - ID: %d, Name: %s", model.ID, model.Name)
	return model, nil
}

// UpdateConvertedModel 更新转换后模型
func (s *convertedModelService) UpdateConvertedModel(ctx context.Context, id uint, req *UpdateConvertedModelRequest) error {
	model, err := s.convertedModelRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取转换后模型失败: %w", err)
	}

	// 更新字段
	if req.DisplayName != nil {
		model.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		model.Description = *req.Description
	}
	if req.Tags != nil {
		model.SetTagList(*req.Tags)
	}

	return s.convertedModelRepo.Update(ctx, model)
}

// DeleteConvertedModel 删除转换后模型
func (s *convertedModelService) DeleteConvertedModel(ctx context.Context, id uint) error {
	model, err := s.convertedModelRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取转换后模型失败: %w", err)
	}

	// 删除数据库记录
	if err := s.convertedModelRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除转换后模型失败: %w", err)
	}

	// 删除文件（如果配置允许）
	if s.config.DeleteFileOnRemove && model.FilePath != "" {
		if err := os.Remove(model.FilePath); err != nil {
			log.Printf("删除模型文件失败: %s, error: %v", model.FilePath, err)
			// 不影响主流程
		}
	}

	return nil
}

// GetConvertedModelList 获取转换后模型列表
func (s *convertedModelService) GetConvertedModelList(ctx context.Context, req *GetConvertedModelListRequest) (*GetConvertedModelListResponse, error) {
	log.Printf("[ConvertedModelService] GetConvertedModelList started - Page: %d, PageSize: %d",
		req.Pagination.Page, req.Pagination.PageSize)

	// 构建查询请求
	listReq := &repository.GetConvertedModelListRequest{
		Status:          req.Status,
		TargetChip:      req.TargetChip,
		UserID:          req.UserID,
		OriginalModelID: req.OriginalModelID,
		Tags:            req.Tags,
		Pagination:      req.Pagination,
		Sort:            req.Sort,
	}

	repoResp, err := s.convertedModelRepo.GetList(ctx, listReq)
	if err != nil {
		log.Printf("[ConvertedModelService] Failed to get converted model list - Error: %v", err)
		return nil, fmt.Errorf("获取转换后模型列表失败: %w", err)
	}

	response := &GetConvertedModelListResponse{
		Models:   repoResp.Models,
		Total:    repoResp.Total,
		Page:     req.Pagination.Page,
		PageSize: req.Pagination.PageSize,
	}

	log.Printf("[ConvertedModelService] GetConvertedModelList completed - Total: %d", repoResp.Total)
	return response, nil
}

// GetConvertedModelsByOriginal 根据原始模型ID获取转换后模型
func (s *convertedModelService) GetConvertedModelsByOriginal(ctx context.Context, originalModelID uint) ([]*models.ConvertedModel, error) {
	return s.convertedModelRepo.GetByOriginalModelID(ctx, originalModelID)
}

// GetConvertedModelByName 根据名称获取转换后模型
func (s *convertedModelService) GetConvertedModelByName(ctx context.Context, name string) (*models.ConvertedModel, error) {
	return s.convertedModelRepo.GetByName(ctx, name)
}

// GetConvertedModelsByChip 根据芯片类型获取转换后模型
func (s *convertedModelService) GetConvertedModelsByChip(ctx context.Context, chip string) ([]*models.ConvertedModel, error) {
	return s.convertedModelRepo.GetByTargetChip(ctx, chip)
}

// SearchConvertedModels 搜索转换后模型
func (s *convertedModelService) SearchConvertedModels(ctx context.Context, req *SearchConvertedModelRequest) (*SearchConvertedModelResponse, error) {
	// 构建搜索请求
	searchReq := &repository.SearchConvertedModelRequest{
		Keyword:    req.Keyword,
		TargetChip: req.TargetChip,
		Status:     req.Status,
		UserID:     req.UserID,
		Pagination: req.Pagination,
	}

	repoResp, err := s.convertedModelRepo.SearchModels(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("搜索转换后模型失败: %w", err)
	}

	return &SearchConvertedModelResponse{
		Models:   repoResp.Models,
		Total:    repoResp.Total,
		Page:     req.Pagination.Page,
		PageSize: req.Pagination.PageSize,
		Keyword:  req.Keyword,
	}, nil
}

// DownloadConvertedModel 下载转换后模型
func (s *convertedModelService) DownloadConvertedModel(ctx context.Context, id uint) (*DownloadInfo, error) {
	model, err := s.convertedModelRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取转换后模型失败: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(model.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("模型文件不存在: %s", model.FilePath)
	}

	// 增加下载次数
	model.IncrementDownloadCount()
	if err := s.convertedModelRepo.IncrementDownloadCount(ctx, id); err != nil {
		log.Printf("更新下载次数失败: %v", err)
		// 不影响主流程
	}

	return &DownloadInfo{
		FilePath:    model.FilePath,
		FileName:    model.FileName,
		FileSize:    model.FileSize,
		ContentType: "application/octet-stream", // 默认二进制文件类型
	}, nil
}

// GetConvertedModelStatistics 获取转换后模型统计
func (s *convertedModelService) GetConvertedModelStatistics(ctx context.Context, userID *uint) (*ConvertedModelStatistics, error) {
	return s.convertedModelRepo.GetStatistics(ctx, userID)
}

// getContentType 根据格式获取内容类型
func (s *convertedModelService) getContentType(format string) string {
	switch format {
	case "bmodel":
		return "application/octet-stream"
	case "onnx":
		return "application/octet-stream"
	case "pb":
		return "application/octet-stream"
	case "tflite":
		return "application/octet-stream"
	case "engine":
		return "application/octet-stream"
	case "plan":
		return "application/octet-stream"
	case "bin":
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

// 请求和响应结构体

// CreateConvertedModelRequest 创建转换后模型请求
type CreateConvertedModelRequest struct {
	Name             string   `json:"name" binding:"required"`
	DisplayName      string   `json:"display_name"`
	Description      string   `json:"description"`
	Version          string   `json:"version" binding:"required"`
	OriginalModelID  uint     `json:"original_model_id" binding:"required"`
	ConversionTaskID string   `json:"conversion_task_id"`
	FileName         string   `json:"file_name" binding:"required"`
	FilePath         string   `json:"file_path" binding:"required"`
	FileSize         int64    `json:"file_size"`
	FileMD5          string   `json:"file_md5"`
	FileSHA256       string   `json:"file_sha256"`
	ConvertParams    string   `json:"convert_params"`
	Quantization     string   `json:"quantization"`
	UserID           uint     `json:"user_id" binding:"required"`
	Tags             []string `json:"tags"`
}

// UpdateConvertedModelRequest 更新转换后模型请求
type UpdateConvertedModelRequest struct {
	DisplayName *string   `json:"display_name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
}

// GetConvertedModelListRequest 获取转换后模型列表请求
type GetConvertedModelListRequest struct {
	Status          string                        `json:"status,omitempty"`
	TargetChip      string                        `json:"target_chip,omitempty"`
	UserID          *uint                         `json:"user_id,omitempty"`
	OriginalModelID *uint                         `json:"original_model_id,omitempty"`
	Tags            []string                      `json:"tags,omitempty"`
	Pagination      *repository.PaginationOptions `json:"pagination,omitempty"`
	Sort            *repository.SortOptions       `json:"sort,omitempty"`
}

// GetConvertedModelListResponse 获取转换后模型列表响应
type GetConvertedModelListResponse struct {
	Models   []*models.ConvertedModel `json:"models"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

// SearchConvertedModelRequest 搜索转换后模型请求
type SearchConvertedModelRequest struct {
	Keyword    string                        `json:"keyword" binding:"required"`
	TargetChip string                        `json:"target_chip,omitempty"`
	Status     string                        `json:"status,omitempty"`
	UserID     *uint                         `json:"user_id,omitempty"`
	Pagination *repository.PaginationOptions `json:"pagination,omitempty"`
}

// SearchConvertedModelResponse 搜索转换后模型响应
type SearchConvertedModelResponse struct {
	Models   []*models.ConvertedModel `json:"models"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Keyword  string                   `json:"keyword"`
}

// ConvertedModelStatistics 转换后模型统计
type ConvertedModelStatistics = repository.ConvertedModelStatistics
