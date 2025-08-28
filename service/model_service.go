/*
 * @module service/model_service
 * @description 原始模型管理服务，提供模型上传、验证、存储等业务逻辑
 * @architecture 服务层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow Controller -> ModelService -> Repository -> 数据库
 * @rules 实现模型文件验证、上传管理、版本控制等核心业务逻辑
 * @dependencies box-manage-service/repository, box-manage-service/models
 * @refs REQ-002.md, DESIGN-003.md, DESIGN-004.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// modelService 模型管理服务实现
type modelService struct {
	modelRepo  repository.OriginalModelRepository
	uploadRepo repository.UploadSessionRepository
	tagRepo    repository.ModelTagRepository
	config     *ModelServiceConfig
}

// ModelServiceConfig 模型服务配置
type ModelServiceConfig struct {
	StorageBasePath   string        `yaml:"storage_base_path"`  // 存储基础路径
	TempPath          string        `yaml:"temp_path"`          // 临时文件路径
	MaxFileSize       int64         `yaml:"max_file_size"`      // 最大文件大小
	ChunkSize         int64         `yaml:"chunk_size"`         // 分片大小
	SessionTimeout    time.Duration `yaml:"session_timeout"`    // 会话超时时间
	AllowedTypes      []string      `yaml:"allowed_types"`      // 允许的文件类型
	EnableChecksum    bool          `yaml:"enable_checksum"`    // 启用校验和
	EnableCompression bool          `yaml:"enable_compression"` // 启用压缩
}

// NewModelService 创建模型管理服务实例
func NewModelService(
	modelRepo repository.OriginalModelRepository,
	uploadRepo repository.UploadSessionRepository,
	tagRepo repository.ModelTagRepository,
	config *ModelServiceConfig,
) ModelService {
	return &modelService{
		modelRepo:  modelRepo,
		uploadRepo: uploadRepo,
		tagRepo:    tagRepo,
		config:     config,
	}
}

// 请求和响应结构体

// DirectUploadRequest 直接上传请求
type DirectUploadRequest struct {
	File          io.Reader                `json:"-"`
	FileName      string                   `json:"file_name" binding:"required"`
	FileSize      int64                    `json:"file_size" binding:"required,min=1"`
	Name          string                   `json:"name" binding:"required"`
	Version       string                   `json:"version" binding:"required"`
	ModelType     models.OriginalModelType `json:"model_type" binding:"required"`
	Framework     string                   `json:"framework" binding:"required"`
	Description   string                   `json:"description"`
	Tags          []string                 `json:"tags"`
	TaskType      models.ModelTaskType     `json:"task_type"`
	InputWidth    int                      `json:"input_width"`
	InputHeight   int                      `json:"input_height"`
	InputChannels int                      `json:"input_channels"`
	UserID        uint                     `json:"user_id" binding:"required"`
}

// InitUploadRequest 初始化上传请求
type InitUploadRequest struct {
	FileName    string            `json:"file_name" binding:"required"`
	FileSize    int64             `json:"file_size" binding:"required,min=1"`
	FileMD5     string            `json:"file_md5"`
	UserID      uint              `json:"user_id" binding:"required"`
	ModelName   string            `json:"model_name" binding:"required"`
	Description string            `json:"description"`
	Version     string            `json:"version" binding:"required"`
	ModelType   string            `json:"model_type" binding:"required,oneof=pt onnx"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
}

// InitUploadResponse 初始化上传响应
type InitUploadResponse struct {
	SessionID   string `json:"session_id"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	ExpiresAt   string `json:"expires_at"`
}

// UploadChunkRequest 上传分片请求
type UploadChunkRequest struct {
	SessionID  string `json:"session_id" binding:"required"`
	ChunkIndex int    `json:"chunk_index" binding:"required,min=0"`
	ChunkData  []byte `json:"chunk_data" binding:"required"`
	ChunkMD5   string `json:"chunk_md5"`
}

// GetModelListRequest 获取模型列表请求
type GetModelListRequest struct {
	UserID     uint                          `json:"user_id"`
	ModelType  models.OriginalModelType      `json:"model_type"`
	Status     models.OriginalModelStatus    `json:"status"`
	Tags       []string                      `json:"tags"`
	Pagination *repository.PaginationOptions `json:"pagination"`
	Sort       *repository.SortOptions       `json:"sort"`
}

// GetModelListResponse 获取模型列表响应
type GetModelListResponse struct {
	Models     []*models.OriginalModel `json:"models"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalPages int                     `json:"total_pages"`
}

// UpdateModelRequest 更新模型请求
type UpdateModelRequest struct {
	Name          string                    `json:"name"`
	Description   string                    `json:"description"`
	Tags          []string                  `json:"tags"`
	Author        string                    `json:"author"`
	ModelURL      string                    `json:"model_url"`
	ModelType     *models.OriginalModelType `json:"model_type,omitempty"`
	Framework     string                    `json:"framework"`
	TaskType      *models.ModelTaskType     `json:"task_type,omitempty"`
	InputWidth    *int                      `json:"input_width,omitempty"`
	InputHeight   *int                      `json:"input_height,omitempty"`
	InputChannels *int                      `json:"input_channels,omitempty"`
}

// SearchModelRequest 搜索模型请求
type SearchModelRequest struct {
	Keyword    string                        `json:"keyword" binding:"required"`
	UserID     uint                          `json:"user_id"`
	ModelType  models.OriginalModelType      `json:"model_type"`
	Tags       []string                      `json:"tags"`
	Pagination *repository.PaginationOptions `json:"pagination"`
}

// SearchModelResponse 搜索模型响应
type SearchModelResponse struct {
	Models     []*models.OriginalModel `json:"models"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalPages int                     `json:"total_pages"`
}

// DownloadModelResponse 下载模型响应
type DownloadModelResponse struct {
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// ModelStatisticsResponse 模型统计响应
type ModelStatisticsResponse struct {
	Total          int64            `json:"total"`
	ByStatus       map[string]int64 `json:"by_status"`
	ByType         map[string]int64 `json:"by_type"`
	Public         int64            `json:"public"`
	TotalDownloads int64            `json:"total_downloads"`
}

// StorageStatisticsResponse 存储统计响应
type StorageStatisticsResponse struct {
	TotalSize      int64                       `json:"total_size"`
	ByStorageClass map[string]map[string]int64 `json:"by_storage_class"`
	ByType         map[string]map[string]int64 `json:"by_type"`
}

// 服务实现

// InitializeUpload 初始化上传
func (s *modelService) InitializeUpload(ctx context.Context, req *InitUploadRequest) (*InitUploadResponse, error) {
	// 验证请求参数
	if err := s.validateInitRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 检查文件是否已存在（去重）
	if req.FileMD5 != "" {
		if existingModel, err := s.modelRepo.FindByMD5(ctx, req.FileMD5); err != nil {
			return nil, fmt.Errorf("failed to check duplicate: %w", err)
		} else if existingModel != nil {
			return nil, errors.New("file already exists with same MD5")
		}
	}

	// 检查模型名称和版本是否已存在
	if existingModel, err := s.modelRepo.FindByNameAndVersion(ctx, req.ModelName, req.Version); err != nil {
		return nil, fmt.Errorf("failed to check model version: %w", err)
	} else if existingModel != nil {
		return nil, errors.New("model with same name and version already exists")
	}

	// 创建上传会话
	sessionID := s.generateSessionID()
	chunkSize := s.config.ChunkSize
	totalChunks := int((req.FileSize + chunkSize - 1) / chunkSize)
	expiresAt := time.Now().Add(s.config.SessionTimeout)

	session := &models.UploadSession{
		SessionID:      sessionID,
		UserID:         req.UserID,
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		FileMD5:        req.FileMD5,
		ChunkSize:      chunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: "",
		Status:         models.UploadStatusInitialized,
		Progress:       0,
		ExpiresAt:      expiresAt,
		TempDir:        filepath.Join(s.config.TempPath, sessionID),
		Metadata:       s.encodeMetadata(req),
	}

	// 创建临时目录
	if err := os.MkdirAll(session.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// 保存会话到数据库
	if err := s.uploadRepo.Create(ctx, session); err != nil {
		// 清理临时目录
		os.RemoveAll(session.TempDir)
		return nil, fmt.Errorf("failed to save upload session: %w", err)
	}

	return &InitUploadResponse{
		SessionID:   sessionID,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}, nil
}

// UploadChunk 上传分片
func (s *modelService) UploadChunk(ctx context.Context, req *UploadChunkRequest) error {
	// 获取上传会话
	session, err := s.uploadRepo.FindBySessionID(ctx, req.SessionID)
	if err != nil {
		return fmt.Errorf("failed to find upload session: %w", err)
	}
	if session == nil {
		return errors.New("upload session not found")
	}

	// 检查会话状态
	if session.IsExpired() {
		return errors.New("upload session expired")
	}
	if session.Status != models.UploadStatusInitialized && session.Status != models.UploadStatusUploading {
		return errors.New("invalid upload session status")
	}

	// 验证分片索引
	if req.ChunkIndex < 0 || req.ChunkIndex >= session.TotalChunks {
		return errors.New("invalid chunk index")
	}

	// 检查分片是否已上传
	uploadedChunks := session.GetUploadedChunkList()
	for _, uploaded := range uploadedChunks {
		if uploaded == req.ChunkIndex {
			return nil // 分片已存在，跳过
		}
	}

	// 验证分片大小
	expectedSize := session.ChunkSize
	if req.ChunkIndex == session.TotalChunks-1 {
		// 最后一个分片可能小于标准分片大小
		expectedSize = session.FileSize - int64(req.ChunkIndex)*session.ChunkSize
	}
	if int64(len(req.ChunkData)) != expectedSize {
		return errors.New("invalid chunk size")
	}

	// 验证分片MD5（如果提供）
	if req.ChunkMD5 != "" && s.config.EnableChecksum {
		hash := md5.Sum(req.ChunkData)
		actualMD5 := hex.EncodeToString(hash[:])
		if actualMD5 != req.ChunkMD5 {
			return errors.New("chunk MD5 mismatch")
		}
	}

	// 保存分片文件
	chunkPath := filepath.Join(session.TempDir, fmt.Sprintf("chunk_%d", req.ChunkIndex))
	if err := os.WriteFile(chunkPath, req.ChunkData, 0644); err != nil {
		return fmt.Errorf("failed to save chunk: %w", err)
	}

	// 更新会话状态
	if err := s.uploadRepo.UpdateStatus(ctx, req.SessionID, models.UploadStatusUploading); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	// 添加已上传分片
	if err := s.uploadRepo.AddUploadedChunk(ctx, req.SessionID, req.ChunkIndex); err != nil {
		return fmt.Errorf("failed to update uploaded chunks: %w", err)
	}

	return nil
}

// CompleteUpload 完成上传
func (s *modelService) CompleteUpload(ctx context.Context, sessionID string) (*models.OriginalModel, error) {
	// 获取上传会话
	session, err := s.uploadRepo.FindBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find upload session: %w", err)
	}
	if session == nil {
		return nil, errors.New("upload session not found")
	}

	// 检查上传是否完成
	if !session.IsCompleted() {
		return nil, errors.New("upload not completed")
	}

	// 合并分片文件
	finalPath, err := s.assembleFile(session)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble file: %w", err)
	}

	// 验证文件完整性
	if err := s.validateFile(finalPath, session); err != nil {
		os.Remove(finalPath)
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// 解析元数据
	metadata := s.decodeMetadata(session.Metadata)

	// 创建模型记录
	model := &models.OriginalModel{
		Name:         metadata["model_name"],
		Description:  metadata["description"],
		Version:      metadata["version"],
		FileName:     session.FileName,
		FilePath:     finalPath,
		FileSize:     session.FileSize,
		FileMD5:      session.FileMD5,
		ModelType:    models.OriginalModelType(metadata["model_type"]),
		Framework:    s.detectFramework(models.OriginalModelType(metadata["model_type"])),
		ModelFormat:  metadata["model_type"],
		Status:       models.OriginalModelStatusReady,
		UserID:       session.UserID,
		StorageClass: s.determineStorageClass(session.FileSize),
		LastAccessed: time.Now(),
	}

	// 计算SHA256
	if s.config.EnableChecksum {
		sha256Hash, err := s.calculateSHA256(finalPath)
		if err != nil {
			os.Remove(finalPath)
			return nil, fmt.Errorf("failed to calculate SHA256: %w", err)
		}
		model.FileSHA256 = sha256Hash
	}

	// 保存模型到数据库
	if err := s.modelRepo.Create(ctx, model); err != nil {
		os.Remove(finalPath)
		return nil, fmt.Errorf("failed to save model: %w", err)
	}

	// 处理标签
	if err := s.processModelTags(ctx, model, parseTagList(metadata["tags"])); err != nil {
		// 标签处理失败不阻断主流程，记录日志
		fmt.Printf("Warning: failed to process tags: %v", err)
	}

	// 更新上传会话状态
	if err := s.uploadRepo.MarkAsCompleted(ctx, sessionID, finalPath, model.ID); err != nil {
		fmt.Printf("Warning: failed to update session status: %v", err)
	}

	// 清理临时文件
	go func() {
		time.Sleep(5 * time.Minute) // 延迟清理，确保前端有时间处理
		os.RemoveAll(session.TempDir)
	}()

	return model, nil
}

// DirectUpload 直接上传模型文件
func (s *modelService) DirectUpload(ctx context.Context, req *DirectUploadRequest) (*models.OriginalModel, error) {
	log.Printf("[ModelService] DirectUpload started - Name: %s, FileName: %s, FileSize: %d, UserID: %d",
		req.Name, req.FileName, req.FileSize, req.UserID)

	// 验证请求
	if err := s.validateDirectUploadRequest(req); err != nil {
		log.Printf("[ModelService] Request validation failed - Name: %s, Error: %v", req.Name, err)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 生成唯一的文件名
	fileExt := filepath.Ext(req.FileName)
	uniqueFileName := fmt.Sprintf("%s_%s_%s%s",
		req.Name, req.Version, uuid.New().String()[:8], fileExt)

	// 计算存储路径
	storagePath := filepath.Join(s.config.StorageBasePath, "models", uniqueFileName)
	log.Printf("[ModelService] Generated storage path - Path: %s", storagePath)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		log.Printf("[ModelService] Failed to create storage directory - Path: %s, Error: %v", filepath.Dir(storagePath), err)
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// 创建文件并保存
	log.Printf("[ModelService] Creating destination file - Path: %s", storagePath)
	destFile, err := os.Create(storagePath)
	if err != nil {
		log.Printf("[ModelService] Failed to create destination file - Path: %s, Error: %v", storagePath, err)
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer destFile.Close()

	// 复制文件内容并计算MD5
	log.Printf("[ModelService] Starting file copy with MD5 calculation - ExpectedSize: %d bytes", req.FileSize)
	hasher := md5.New()
	multiWriter := io.MultiWriter(destFile, hasher)

	writtenBytes, err := io.Copy(multiWriter, req.File)
	if err != nil {
		log.Printf("[ModelService] Failed to copy file content - Path: %s, WrittenBytes: %d, Error: %v",
			storagePath, writtenBytes, err)
		os.Remove(storagePath)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("[ModelService] File content copied successfully - Path: %s, WrittenBytes: %d", storagePath, writtenBytes)

	// 验证文件大小
	if writtenBytes != req.FileSize {
		log.Printf("[ModelService] File size mismatch - Path: %s, Expected: %d, Actual: %d",
			storagePath, req.FileSize, writtenBytes)
		os.Remove(storagePath)
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", req.FileSize, writtenBytes)
	}

	// 计算文件校验和
	fileMD5 := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("[ModelService] File MD5 calculated - Path: %s, MD5: %s", storagePath, fileMD5)

	// 计算SHA256
	var fileSHA256 string
	if s.config.EnableChecksum {
		log.Printf("[ModelService] Calculating SHA256 checksum - Path: %s", storagePath)
		sha256Hash, err := s.calculateSHA256(storagePath)
		if err != nil {
			log.Printf("[ModelService] Failed to calculate SHA256 - Path: %s, Error: %v", storagePath, err)
			os.Remove(storagePath)
			return nil, fmt.Errorf("failed to calculate SHA256: %w", err)
		}
		fileSHA256 = sha256Hash
		log.Printf("[ModelService] SHA256 calculated - Path: %s, SHA256: %s", storagePath, fileSHA256)
	}

	// 创建模型记录
	model := &models.OriginalModel{
		Name:          req.Name,
		Version:       req.Version,
		ModelType:     req.ModelType,
		Framework:     req.Framework,
		Description:   req.Description,
		FileName:      req.FileName,
		FilePath:      storagePath,
		FileSize:      req.FileSize,
		FileMD5:       fileMD5,
		FileSHA256:    fileSHA256,
		Status:        models.OriginalModelStatusReady,
		UserID:        req.UserID,
		TaskType:      req.TaskType,
		InputWidth:    req.InputWidth,
		InputHeight:   req.InputHeight,
		InputChannels: req.InputChannels,
		StorageClass:  s.determineStorageClass(req.FileSize),
		LastAccessed:  time.Now(),
	}

	// 保存模型到数据库
	if err := s.modelRepo.Create(ctx, model); err != nil {
		os.Remove(storagePath)
		return nil, fmt.Errorf("failed to save model: %w", err)
	}

	// 处理标签
	if len(req.Tags) > 0 {
		if err := s.processModelTags(ctx, model, req.Tags); err != nil {
			// 标签处理失败不阻断主流程，记录日志
			fmt.Printf("Warning: failed to process tags: %v", err)
		}
	}

	return model, nil
}

// validateDirectUploadRequest 验证直接上传请求
func (s *modelService) validateDirectUploadRequest(req *DirectUploadRequest) error {
	if req.File == nil {
		return errors.New("file is required")
	}

	if req.FileSize <= 0 {
		return errors.New("file size must be greater than 0")
	}

	if req.FileSize > s.config.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", req.FileSize, s.config.MaxFileSize)
	}

	// 验证文件类型
	fileExt := strings.ToLower(filepath.Ext(req.FileName))
	isAllowed := false
	for _, allowedType := range s.config.AllowedTypes {
		if fileExt == allowedType {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return fmt.Errorf("file type %s is not allowed", fileExt)
	}

	return nil
}

// CancelUpload 取消上传
func (s *modelService) CancelUpload(ctx context.Context, sessionID string) error {
	// 获取上传会话
	session, err := s.uploadRepo.FindBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to find upload session: %w", err)
	}
	if session == nil {
		return errors.New("upload session not found")
	}

	// 更新会话状态
	if err := s.uploadRepo.UpdateStatus(ctx, sessionID, models.UploadStatusCancelled); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	// 清理临时文件
	go func() {
		os.RemoveAll(session.TempDir)
	}()

	return nil
}

// GetModelList 获取模型列表
func (s *modelService) GetModelList(ctx context.Context, req *GetModelListRequest) (*GetModelListResponse, error) {
	// 构建查询条件
	conditions := make(map[string]interface{})

	if req.UserID != 0 {
		conditions["user_id"] = req.UserID
	}
	if req.ModelType != "" {
		conditions["model_type"] = req.ModelType
	}
	if req.Status != "" {
		conditions["status"] = req.Status
	}

	// 查询模型列表
	var models []*models.OriginalModel
	var total int64
	var err error

	if len(req.Tags) > 0 {
		// 按标签查询
		models, err = s.modelRepo.FindByTags(ctx, req.Tags)
		total = int64(len(models))
	} else {
		// 普通查询
		models, total, err = s.modelRepo.FindWithPagination(ctx, conditions,
			req.Pagination.Page, req.Pagination.PageSize)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}

	// 预加载关联的转换后模型
	for _, model := range models {
		if err := s.modelRepo.LoadConvertedModels(ctx, model); err != nil {
			// 如果加载转换后模型失败，不影响主流程，只记录日志
			fmt.Printf("Warning: failed to load converted models for original model %d: %v\n", model.ID, err)
		}
	}

	// 计算分页信息
	totalPages := int(total / int64(req.Pagination.PageSize))
	if total%int64(req.Pagination.PageSize) > 0 {
		totalPages++
	}

	return &GetModelListResponse{
		Models:     models,
		Total:      total,
		Page:       req.Pagination.Page,
		PageSize:   req.Pagination.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetModelDetail 获取模型详情
func (s *modelService) GetModelDetail(ctx context.Context, modelID uint) (*models.OriginalModel, error) {
	model, err := s.modelRepo.GetByID(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	if model == nil {
		return nil, errors.New("model not found")
	}

	// 预加载关联的转换后模型
	if err := s.modelRepo.LoadConvertedModels(ctx, model); err != nil {
		// 如果加载转换后模型失败，不影响主流程，只记录日志
		fmt.Printf("Warning: failed to load converted models for original model %d: %v\n", model.ID, err)
	}

	// 更新最后访问时间
	s.modelRepo.UpdateLastAccessed(ctx, modelID)

	return model, nil
}

// UpdateModel 更新模型
func (s *modelService) UpdateModel(ctx context.Context, modelID uint, req *UpdateModelRequest) error {
	// 获取现有模型
	model, err := s.modelRepo.GetByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}
	if model == nil {
		return errors.New("model not found")
	}

	// 更新字段
	if req.Name != "" {
		model.Name = req.Name
	}
	if req.Description != "" {
		model.Description = req.Description
	}
	if req.Author != "" {
		model.Author = req.Author
	}

	if req.ModelURL != "" {
		model.ModelURL = req.ModelURL
	}
	if req.Framework != "" {
		model.Framework = req.Framework
	}
	// 更新AI任务相关字段
	if req.ModelType != nil {
		model.ModelType = *req.ModelType
	}
	if req.TaskType != nil {
		model.TaskType = *req.TaskType
	}
	if req.InputWidth != nil && *req.InputWidth > 0 {
		model.InputWidth = *req.InputWidth
	}
	if req.InputHeight != nil && *req.InputHeight > 0 {
		model.InputHeight = *req.InputHeight
	}
	if req.InputChannels != nil && *req.InputChannels > 0 {
		model.InputChannels = *req.InputChannels
	}

	// 更新标签
	if req.Tags != nil {
		model.SetTagList(req.Tags)
		// 更新标签使用统计
		s.processModelTags(ctx, model, req.Tags)
	}

	// 保存更新
	if err := s.modelRepo.Update(ctx, model); err != nil {
		return fmt.Errorf("failed to update model: %w", err)
	}

	return nil
}

// DeleteModel 删除模型
func (s *modelService) DeleteModel(ctx context.Context, modelID uint) error {
	// 获取模型信息
	model, err := s.modelRepo.GetByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}
	if model == nil {
		return errors.New("model not found")
	}

	// 软删除模型记录
	if err := s.modelRepo.SoftDelete(ctx, modelID); err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	// 异步删除文件
	go func() {
		if model.FilePath != "" {
			os.Remove(model.FilePath)
		}
	}()

	return nil
}

// DownloadModel 下载模型
func (s *modelService) DownloadModel(ctx context.Context, modelID uint) (*DownloadModelResponse, error) {
	// 获取模型信息
	model, err := s.modelRepo.GetByID(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	if model == nil {
		return nil, errors.New("model not found")
	}

	// 验证模型状态
	if model.Status != models.OriginalModelStatusReady {
		return nil, fmt.Errorf("model is not ready for download, current status: %s", model.Status)
	}

	// 检查文件路径是否为空
	if model.FilePath == "" {
		return nil, errors.New("model file path is empty")
	}

	// 检查文件是否存在并获取实际文件信息
	fileInfo, err := os.Stat(model.FilePath)
	if os.IsNotExist(err) {
		return nil, errors.New("model file not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to access model file: %w", err)
	}

	// 验证文件大小一致性
	if fileInfo.Size() != model.FileSize {
		log.Printf("[ModelService] File size mismatch for model %d - DB: %d bytes, File: %d bytes",
			modelID, model.FileSize, fileInfo.Size())
		// 更新数据库中的文件大小为实际大小
		model.FileSize = fileInfo.Size()
		if updateErr := s.modelRepo.Update(ctx, model); updateErr != nil {
			log.Printf("[ModelService] Failed to update file size for model %d: %v", modelID, updateErr)
		}
	}

	// 异步更新下载次数和最后访问时间（避免阻塞下载）
	go func() {
		bgCtx := context.Background()
		if err := s.modelRepo.IncrementDownloadCount(bgCtx, modelID); err != nil {
			log.Printf("[ModelService] Failed to increment download count for model %d: %v", modelID, err)
		}
		if err := s.modelRepo.UpdateLastAccessed(bgCtx, modelID); err != nil {
			log.Printf("[ModelService] Failed to update last accessed time for model %d: %v", modelID, err)
		}
	}()

	// 确定Content-Type
	contentType := mime.TypeByExtension(filepath.Ext(model.FileName))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 记录下载日志
	log.Printf("[ModelService] Model download initiated - ModelID: %d, FileName: %s, FileSize: %d bytes",
		modelID, model.FileName, model.FileSize)

	return &DownloadModelResponse{
		FilePath:    model.FilePath,
		FileName:    model.FileName,
		FileSize:    model.FileSize,
		ContentType: contentType,
	}, nil
}

// SearchModels 搜索模型
func (s *modelService) SearchModels(ctx context.Context, req *SearchModelRequest) (*SearchModelResponse, error) {
	options := &repository.QueryOptions{
		Pagination: req.Pagination,
	}

	existModels, err := s.modelRepo.SearchModels(ctx, req.Keyword, options)
	if err != nil {
		return nil, fmt.Errorf("failed to search models: %w", err)
	}

	// 应用额外筛选条件
	filteredModels := []*models.OriginalModel{}
	for _, model := range existModels {
		if s.matchesSearchCriteria(model, req) {
			filteredModels = append(filteredModels, model)
		}
	}

	total := int64(len(filteredModels))
	totalPages := int(total / int64(req.Pagination.PageSize))
	if total%int64(req.Pagination.PageSize) > 0 {
		totalPages++
	}

	return &SearchModelResponse{
		Models:     filteredModels,
		Total:      total,
		Page:       req.Pagination.Page,
		PageSize:   req.Pagination.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetModelVersions 获取模型版本
func (s *modelService) GetModelVersions(ctx context.Context, modelName string) ([]*models.OriginalModel, error) {
	return s.modelRepo.GetModelVersions(ctx, modelName)
}

// ValidateModel 验证模型
func (s *modelService) ValidateModel(ctx context.Context, modelID uint) error {
	model, err := s.modelRepo.GetByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}
	if model == nil {
		return errors.New("model not found")
	}

	// 验证文件存在
	if _, err := os.Stat(model.FilePath); os.IsNotExist(err) {
		s.modelRepo.MarkAsInvalid(ctx, modelID, "模型文件不存在")
		return errors.New("model file not found")
	}

	// 验证文件大小
	if fileInfo, err := os.Stat(model.FilePath); err == nil {
		if fileInfo.Size() != model.FileSize {
			s.modelRepo.MarkAsInvalid(ctx, modelID, "文件大小不匹配")
			return errors.New("file size mismatch")
		}
	}

	// 验证文件格式
	if !s.isValidModelFormat(model.FilePath, model.ModelType) {
		s.modelRepo.MarkAsInvalid(ctx, modelID, "文件格式无效")
		return errors.New("invalid model format")
	}

	// 标记为已验证
	s.modelRepo.MarkAsValidated(ctx, modelID, "模型验证通过")
	s.modelRepo.UpdateStatus(ctx, modelID, models.OriginalModelStatusReady)

	return nil
}

// GetModelStatistics 获取模型统计
func (s *modelService) GetModelStatistics(ctx context.Context) (*ModelStatisticsResponse, error) {
	stats, err := s.modelRepo.GetModelStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get model statistics: %w", err)
	}

	return &ModelStatisticsResponse{
		Total:          stats["total"].(int64),
		ByStatus:       stats["by_status"].(map[string]int64),
		ByType:         stats["by_type"].(map[string]int64),
		Public:         stats["public"].(int64),
		TotalDownloads: stats["total_downloads"].(int64),
	}, nil
}

// GetStorageStatistics 获取存储统计
func (s *modelService) GetStorageStatistics(ctx context.Context) (*StorageStatisticsResponse, error) {
	stats, err := s.modelRepo.GetStorageStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage statistics: %w", err)
	}

	return &StorageStatisticsResponse{
		TotalSize:      stats["total_size"].(int64),
		ByStorageClass: stats["by_storage_class"].(map[string]map[string]int64),
		ByType:         stats["by_type"].(map[string]map[string]int64),
	}, nil
}

// CleanupExpiredSessions 清理过期会话
func (s *modelService) CleanupExpiredSessions(ctx context.Context) error {
	return s.uploadRepo.CleanupExpiredSessions(ctx)
}

// ArchiveOldModels 归档旧模型
func (s *modelService) ArchiveOldModels(ctx context.Context, olderThanDays int) error {
	olderThan := time.Now().AddDate(0, 0, -olderThanDays)
	return s.modelRepo.ArchiveOldModels(ctx, olderThan)
}

// 私有辅助方法

// validateInitRequest 验证初始化请求
func (s *modelService) validateInitRequest(req *InitUploadRequest) error {
	if req.FileSize > s.config.MaxFileSize {
		return errors.New("file size exceeds limit")
	}

	// 验证文件类型
	ext := strings.ToLower(filepath.Ext(req.FileName))
	if !s.isAllowedFileType(ext) {
		return errors.New("file type not allowed")
	}

	// 验证模型类型
	if !isValidModelType(req.ModelType) {
		return errors.New("invalid model type")
	}

	return nil
}

// generateSessionID 生成会话ID
func (s *modelService) generateSessionID() string {
	return "upload_" + uuid.New().String()
}

// assembleFile 合并分片文件
func (s *modelService) assembleFile(session *models.UploadSession) (string, error) {
	// 生成最终文件路径
	finalPath := s.generateFinalPath(session)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return "", err
	}

	// 创建最终文件
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return "", err
	}
	defer finalFile.Close()

	// 按顺序合并分片
	for i := 0; i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(session.TempDir, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			os.Remove(finalPath)
			return "", err
		}

		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			os.Remove(finalPath)
			return "", err
		}
	}

	return finalPath, nil
}

// validateFile 验证文件完整性
func (s *modelService) validateFile(filePath string, session *models.UploadSession) error {
	// 验证文件大小
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if fileInfo.Size() != session.FileSize {
		return errors.New("file size mismatch")
	}

	// 验证MD5（如果提供）
	if session.FileMD5 != "" && s.config.EnableChecksum {
		actualMD5, err := s.calculateMD5(filePath)
		if err != nil {
			return err
		}
		if actualMD5 != session.FileMD5 {
			return errors.New("MD5 mismatch")
		}
	}

	return nil
}

// generateFinalPath 生成最终文件路径
func (s *modelService) generateFinalPath(session *models.UploadSession) string {
	now := time.Now()
	dir := filepath.Join(s.config.StorageBasePath, "original",
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()))

	// 生成文件名：原名-MD5前8位.扩展名
	ext := filepath.Ext(session.FileName)
	baseName := strings.TrimSuffix(session.FileName, ext)
	md5Prefix := ""
	if len(session.FileMD5) >= 8 {
		md5Prefix = "-" + session.FileMD5[:8]
	}
	fileName := fmt.Sprintf("%s%s%s", baseName, md5Prefix, ext)

	return filepath.Join(dir, fileName)
}

// calculateMD5 计算文件MD5
func (s *modelService) calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// calculateSHA256 计算文件SHA256
func (s *modelService) calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// isAllowedFileType 检查文件类型是否允许
func (s *modelService) isAllowedFileType(ext string) bool {
	for _, allowedType := range s.config.AllowedTypes {
		if ext == allowedType {
			return true
		}
	}
	return false
}

// isValidModelType 检查模型类型是否有效
func isValidModelType(modelType string) bool {
	switch models.OriginalModelType(modelType) {
	case models.OriginalModelTypePT, models.OriginalModelTypeONNX:
		return true
	default:
		return false
	}
}

// detectFramework 检测模型框架
func (s *modelService) detectFramework(modelType models.OriginalModelType) string {
	switch modelType {
	case models.OriginalModelTypePT:
		return "pytorch"
	case models.OriginalModelTypeONNX:
		return "onnx"
	default:
		return "unknown"
	}
}

// determineStorageClass 确定存储类别
func (s *modelService) determineStorageClass(fileSize int64) string {
	if fileSize > 1024*1024*1024 { // > 1GB
		return "warm"
	} else if fileSize > 100*1024*1024 { // > 100MB
		return "hot"
	}
	return "hot"
}

// isValidModelFormat 检查模型格式是否有效
func (s *modelService) isValidModelFormat(filePath string, modelType models.OriginalModelType) bool {
	// 简单的文件格式验证，可以扩展为更复杂的验证逻辑
	ext := strings.ToLower(filepath.Ext(filePath))
	switch modelType {
	case models.OriginalModelTypePT:
		return ext == ".pt"
	case models.OriginalModelTypeONNX:
		return ext == ".onnx"
	default:
		return false
	}
}

// processModelTags 处理模型标签
func (s *modelService) processModelTags(ctx context.Context, model *models.OriginalModel, tags []string) error {
	for _, tagName := range tags {
		if tagName == "" {
			continue
		}

		// 获取或创建标签
		tag, err := s.tagRepo.GetOrCreateTag(ctx, tagName)
		if err != nil {
			return err
		}

		// 增加使用次数
		s.tagRepo.IncrementUsage(ctx, tag.ID)
	}
	return nil
}

// matchesSearchCriteria 检查模型是否匹配搜索条件
func (s *modelService) matchesSearchCriteria(model *models.OriginalModel, req *SearchModelRequest) bool {
	if req.UserID != 0 && model.UserID != req.UserID {
		return false
	}
	if req.ModelType != "" && model.ModelType != req.ModelType {
		return false
	}
	// 可以添加更多筛选条件
	return true
}

// encodeMetadata 编码元数据
func (s *modelService) encodeMetadata(req *InitUploadRequest) string {
	metadata := map[string]string{
		"model_name":  req.ModelName,
		"description": req.Description,
		"version":     req.Version,
		"model_type":  req.ModelType,
		"tags":        strings.Join(req.Tags, ","),
	}

	// 添加额外的元数据
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	// 简单的编码，实际项目中可以使用JSON
	var parts []string
	for k, v := range metadata {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "&")
}

// decodeMetadata 解码元数据
func (s *modelService) decodeMetadata(encoded string) map[string]string {
	metadata := make(map[string]string)
	if encoded == "" {
		return metadata
	}

	parts := strings.Split(encoded, "&")
	for _, part := range parts {
		if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
			metadata[kv[0]] = kv[1]
		}
	}

	return metadata
}

// parseTagList 解析标签列表
func parseTagList(tags string) []string {
	if tags == "" {
		return []string{}
	}

	var result []string
	for _, tag := range strings.Split(tags, ",") {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
