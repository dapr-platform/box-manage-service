package service

import (
	"box-manage-service/client"
	"box-manage-service/config"
	"box-manage-service/models"
	"box-manage-service/modules/ffmpeg"
	"box-manage-service/repository"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// VideoSourceService实现
type videoSourceService struct {
	repo          repository.VideoSourceRepository
	videoFileRepo repository.VideoFileRepository
	zlmClient     *client.ZLMediaKitClient
	config        config.VideoConfig
	ffmpegModule  *ffmpeg.FFmpegModule

	// 监控相关
	monitorTicker *time.Ticker
	monitorDone   chan bool
	monitorMux    sync.RWMutex
	isMonitoring  bool
}

func NewVideoSourceService(repo repository.VideoSourceRepository, videoFileRepo repository.VideoFileRepository, zlmClient *client.ZLMediaKitClient, cfg config.VideoConfig, ffmpegModule *ffmpeg.FFmpegModule) VideoSourceService {
	service := &videoSourceService{
		repo:          repo,
		videoFileRepo: videoFileRepo,
		zlmClient:     zlmClient,
		config:        cfg,
		ffmpegModule:  ffmpegModule,
		monitorDone:   make(chan bool),
		isMonitoring:  false,
	}

	// 启动监控服务
	go service.StartMonitoring()

	return service
}

func (s *videoSourceService) CreateVideoSource(ctx context.Context, req *CreateVideoSourceRequest) (*models.VideoSource, error) {
	log.Printf("[VideoSourceService] CreateVideoSource started - Name: %s, Type: %s, UserID: %d", req.Name, req.Type, req.UserID)

	// 验证请求参数
	if req.Type == models.VideoSourceTypeStream && req.URL == "" {
		log.Printf("[VideoSourceService] Validation failed - Stream type requires URL")
		return nil, fmt.Errorf("流类型视频源必须提供URL")
	}
	if req.Type == models.VideoSourceTypeFile && req.VideoFileID == nil {
		log.Printf("[VideoSourceService] Validation failed - File type requires VideoFileID")
		return nil, fmt.Errorf("文件类型视频源必须提供video_file_id")
	}

	log.Printf("[VideoSourceService] Creating video source - Type: %s, URL: %s, VideoFileID: %v, StreamID: %s",
		req.Type, req.URL, req.VideoFileID, req.StreamID)

	vs := &models.VideoSource{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		URL:         req.URL,
		Username:    req.Username,
		Password:    req.Password,
		StreamID:    req.StreamID,
		VideoFileID: req.VideoFileID,
		Status:      models.VideoSourceStatusInactive,
		UserID:      req.UserID,
	}
	if req.Type == models.VideoSourceTypeFile {
		vs.Status = models.VideoSourceStatusActive
	}
	log.Printf("[VideoSourceService] Generating stream info for video source - Name: %s", vs.Name)
	vs.GenerateStreamInfo()
	log.Printf("[VideoSourceService] Generated stream info - StreamID: %s", vs.StreamID)

	if err := s.repo.Create(vs); err != nil {
		log.Printf("[VideoSourceService] Failed to create video source - Name: %s, Error: %v", req.Name, err)
		return nil, fmt.Errorf("创建视频源失败: %w", err)
	}

	log.Printf("[VideoSourceService] Video source created successfully - ID: %d, Name: %s", vs.ID, vs.Name)

	// 生成播放地址
	log.Printf("[VideoSourceService] Generating play URL for video source - ID: %d, Type: %s", vs.ID, vs.Type)
	if err := s.generatePlayURL(ctx, vs); err != nil {
		log.Printf("[VideoSourceService] Failed to generate play URL - ID: %d, Error: %v", vs.ID, err)
		return nil, fmt.Errorf("生成播放地址失败: %w", err)
	}

	log.Printf("[VideoSourceService] Play URL generated - ID: %d, PlayURL: %s", vs.ID, vs.PlayURL)

	// 如果是实时流类型，自动同步到ZLMediaKit
	if vs.Type == models.VideoSourceTypeStream {
		log.Printf("[VideoSourceService] Auto-starting stream video source - VideoSourceID: %d, StreamID: %s", vs.ID, vs.StreamID)
		if err := s.startStreamVideoSource(ctx, vs); err != nil {
			log.Printf("[VideoSourceService] Failed to auto-start stream - VideoSourceID: %d, Error: %v", vs.ID, err)
			// 设置错误状态，但不影响创建
			vs.Status = models.VideoSourceStatusError
			vs.ErrorMsg = fmt.Sprintf("自动启动流失败: %v", err)
		}
	}

	if err := s.repo.Update(vs); err != nil {
		log.Printf("[VideoSourceService] Failed to update video source - ID: %d, Error: %v", vs.ID, err)
		return nil, fmt.Errorf("更新视频源失败: %w", err)
	}

	log.Printf("[VideoSourceService] CreateVideoSource completed successfully - ID: %d, Name: %s, Status: %s, PlayURL: %s",
		vs.ID, vs.Name, vs.Status, vs.PlayURL)
	return vs, nil
}

// generatePlayURL 生成播放地址
func (s *videoSourceService) generatePlayURL(ctx context.Context, vs *models.VideoSource) error {
	log.Printf("[VideoSourceService] generatePlayURL started - VideoSourceID: %d, Type: %s, VideoFileID: %v",
		vs.ID, vs.Type, vs.VideoFileID)

	if vs.Type == models.VideoSourceTypeStream {
		// 流类型使用默认格式
		log.Printf("[VideoSourceService] Generating play URL for stream type - VideoSourceID: %d", vs.ID)
		vs.PlayURL = vs.GetPlayURL(s.config.PlayURL.StreamPrefix, s.config.PlayURL.FilePrefix)
		log.Printf("[VideoSourceService] Stream play URL generated - VideoSourceID: %d, PlayURL: %s", vs.ID, vs.PlayURL)
	} else if vs.Type == models.VideoSourceTypeFile && vs.VideoFileID != nil {
		// 文件类型需要获取转换后的文件信息
		log.Printf("[VideoSourceService] Getting video file info for file type - VideoSourceID: %d, VideoFileID: %d",
			vs.ID, *vs.VideoFileID)
		vf, err := s.videoFileRepo.GetByID(*vs.VideoFileID)
		if err != nil {
			log.Printf("[VideoSourceService] Failed to get video file - VideoSourceID: %d, VideoFileID: %d, Error: %v",
				vs.ID, *vs.VideoFileID, err)
			return fmt.Errorf("获取关联视频文件失败: %w", err)
		}

		log.Printf("[VideoSourceService] Video file retrieved - VideoFileID: %d, Status: %s, PlayURL: %s",
			vf.ID, vf.Status, vf.PlayURL)

		// 如果视频文件已经转换完成，使用转换后的播放地址
		if vf.Status == models.VideoFileStatusReady && vf.PlayURL != "" {
			vs.PlayURL = vf.PlayURL
			log.Printf("[VideoSourceService] Using converted video file play URL - VideoSourceID: %d, PlayURL: %s",
				vs.ID, vs.PlayURL)
		} else {
			// 如果还未转换完成，生成默认播放地址
			vs.PlayURL = vs.GetPlayURL(s.config.PlayURL.StreamPrefix, s.config.PlayURL.FilePrefix)
			log.Printf("[VideoSourceService] Video file not ready, using default play URL - VideoSourceID: %d, PlayURL: %s, FileStatus: %s",
				vs.ID, vs.PlayURL, vf.Status)
		}
	} else {
		// 默认使用模型方法生成
		log.Printf("[VideoSourceService] Using default play URL generation - VideoSourceID: %d", vs.ID)
		vs.PlayURL = vs.GetPlayURL(s.config.PlayURL.StreamPrefix, s.config.PlayURL.FilePrefix)
		log.Printf("[VideoSourceService] Default play URL generated - VideoSourceID: %d, PlayURL: %s", vs.ID, vs.PlayURL)
	}

	log.Printf("[VideoSourceService] generatePlayURL completed - VideoSourceID: %d, Final PlayURL: %s", vs.ID, vs.PlayURL)
	return nil
}

// startStreamVideoSource 启动实时流视频源（内部方法）
func (s *videoSourceService) startStreamVideoSource(ctx context.Context, vs *models.VideoSource) error {
	log.Printf("[VideoSourceService] startStreamVideoSource started - VideoSourceID: %d, StreamID: %s", vs.ID, vs.StreamID)

	if vs.Type != models.VideoSourceTypeStream {
		return fmt.Errorf("只能启动实时流类型的视频源")
	}

	if vs.URL == "" {
		return fmt.Errorf("实时流URL不能为空")
	}

	// 向ZLMediaKit添加流代理
	log.Printf("[VideoSourceService] Adding stream proxy to ZLMediaKit - URL: %s, StreamID: %s", vs.URL, vs.StreamID)
	req := &client.AddStreamProxyRequest{
		Vhost:      "__defaultVhost__",
		App:        "live",
		Stream:     vs.StreamID,
		URL:        vs.URL,
		RetryCount: -1, // 无限重试
		RtpType:    0,  // TCP
		Timeout:    30,
	}

	resp, err := s.zlmClient.AddStreamProxy(ctx, req)
	if err != nil {
		log.Printf("[VideoSourceService] Failed to add stream proxy - VideoSourceID: %d, Error: %v", vs.ID, err)
		vs.Status = models.VideoSourceStatusError
		vs.ErrorMsg = fmt.Sprintf("添加流代理失败: %v", err)
		return fmt.Errorf("添加流代理失败: %w", err)
	}

	// 保存代理key，用于后续删除
	vs.Key = resp.Data.Key
	log.Printf("[VideoSourceService] Stream proxy added successfully - VideoSourceID: %d, ProxyKey: %s", vs.ID, resp.Data.Key)

	// 更新状态
	vs.Status = models.VideoSourceStatusActive
	now := time.Now()
	vs.LastActive = &now
	vs.ErrorMsg = ""

	// 重新生成播放地址
	if err := s.generatePlayURL(ctx, vs); err != nil {
		log.Printf("[VideoSourceService] Failed to regenerate play URL after starting - VideoSourceID: %d, Error: %v", vs.ID, err)
		return fmt.Errorf("生成播放地址失败: %w", err)
	}

	log.Printf("[VideoSourceService] startStreamVideoSource completed successfully - VideoSourceID: %d, Status: %s", vs.ID, vs.Status)
	return nil
}

func (s *videoSourceService) GetVideoSource(ctx context.Context, id uint) (*models.VideoSource, error) {
	return s.repo.GetByID(id)
}

func (s *videoSourceService) GetVideoSources(ctx context.Context, req *GetVideoSourcesRequest) ([]*models.VideoSource, int64, error) {
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}
	return s.repo.List(req.Page, req.PageSize, req.UserID)
}

func (s *videoSourceService) UpdateVideoSource(ctx context.Context, id uint, req *UpdateVideoSourceRequest) (*models.VideoSource, error) {
	vs, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("视频源不存在: %w", err)
	}

	if req.Name != nil {
		vs.Name = *req.Name
	}
	if req.Description != nil {
		vs.Description = *req.Description
	}
	if req.URL != nil {
		vs.URL = *req.URL
	}
	if req.Username != nil {
		vs.Username = *req.Username
	}
	if req.Password != nil {
		vs.Password = *req.Password
	}
	if req.StreamID != nil {
		vs.StreamID = *req.StreamID
	}
	if req.VideoFileID != nil {
		vs.VideoFileID = *req.VideoFileID
	}

	// 如果关键字段发生变化，重新生成播放地址
	if req.VideoFileID != nil || req.StreamID != nil {
		if err := s.generatePlayURL(ctx, vs); err != nil {
			return nil, fmt.Errorf("生成播放地址失败: %w", err)
		}
	}

	if err := s.repo.Update(vs); err != nil {
		return nil, fmt.Errorf("更新视频源失败: %w", err)
	}

	return vs, nil
}

func (s *videoSourceService) DeleteVideoSource(ctx context.Context, id uint) error {
	vs, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("获取视频源失败: %w", err)
	}

	err = s.zlmClient.DelStreamProxy(ctx, vs.Key)
	if err != nil {
		fmt.Printf("删除流代理失败: %v", err)
	}
	return s.repo.Delete(id)
}

func (s *videoSourceService) StartVideoSource(ctx context.Context, id uint) error {
	vs, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("获取视频源失败: %w", err)
	}

	if vs.Status == models.VideoSourceStatusActive {
		return fmt.Errorf("视频源已处于活动状态")
	}

	// 如果是实时流类型，需要添加到ZLMediaKit
	if vs.Type == models.VideoSourceTypeStream {
		req := &client.AddStreamProxyRequest{
			Vhost:      "__defaultVhost__",
			App:        "live",
			Stream:     vs.StreamID,
			URL:        vs.URL,
			RetryCount: -1, // 无限重试
			RtpType:    0,  // TCP
			Timeout:    30,
		}

		resp, err := s.zlmClient.AddStreamProxy(ctx, req)
		if err != nil {
			vs.Status = models.VideoSourceStatusError
			vs.ErrorMsg = fmt.Sprintf("添加流代理失败: %v", err)
			s.repo.Update(vs)
			return fmt.Errorf("添加流代理失败: %w", err)
		}

		// 保存代理key，用于后续删除
		vs.Key = resp.Data.Key
	}

	// 更新状态
	vs.Status = models.VideoSourceStatusActive
	now := time.Now()
	vs.LastActive = &now
	vs.ErrorMsg = ""

	// 生成播放地址 (相对路径)
	if err := s.generatePlayURL(ctx, vs); err != nil {
		return fmt.Errorf("生成播放地址失败: %w", err)
	}

	return s.repo.Update(vs)
}

func (s *videoSourceService) StopVideoSource(ctx context.Context, id uint) error {
	vs, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("获取视频源失败: %w", err)
	}

	// 如果是实时流类型，需要从ZLMediaKit删除
	if vs.Type == models.VideoSourceTypeStream && vs.StreamID != "" {
		// 关闭流
		closeReq := &client.CloseStreamRequest{
			Vhost:  "__defaultVhost__",
			App:    "live",
			Stream: vs.StreamID,
			Force:  1,
		}
		s.zlmClient.CloseStream(ctx, closeReq)

		// 删除流代理
		s.zlmClient.DelStreamProxy(ctx, vs.Key)
	}

	// 更新状态
	vs.Status = models.VideoSourceStatusInactive
	vs.ErrorMsg = ""

	return s.repo.Update(vs)
}

func (s *videoSourceService) SyncToZLMediaKit(ctx context.Context) error {
	log.Printf("[VideoSourceService] SyncToZLMediaKit started - syncing all stream video sources")

	// 获取所有流类型的视频源（不仅仅是活动的）
	sources, _, err := s.GetVideoSources(ctx, &GetVideoSourcesRequest{
		Page:     1,
		PageSize: 1000, // 假设最多1000个源
	})
	if err != nil {
		log.Printf("[VideoSourceService] Failed to get video sources for sync - Error: %v", err)
		return fmt.Errorf("获取视频源列表失败: %w", err)
	}

	// 过滤出流类型的视频源
	var streamSources []*models.VideoSource
	for _, vs := range sources {
		if vs.Type == models.VideoSourceTypeStream {
			streamSources = append(streamSources, vs)
		}
	}

	log.Printf("[VideoSourceService] Found %d stream video sources to sync", len(streamSources))

	var syncedCount, errorCount int

	// 同步每个流视频源
	for _, vs := range streamSources {
		log.Printf("[VideoSourceService] Syncing stream video source - ID: %d, Name: %s, URL: %s", vs.ID, vs.Name, vs.URL)

		// 重新启动流代理
		if err := s.startStreamVideoSource(ctx, vs); err != nil {
			log.Printf("[VideoSourceService] Failed to sync video source - ID: %d, Name: %s, Error: %v", vs.ID, vs.Name, err)
			errorCount++

			// 更新失败状态到数据库
			vs.Status = models.VideoSourceStatusError
			vs.ErrorMsg = fmt.Sprintf("同步失败: %v", err)
			if updateErr := s.repo.Update(vs); updateErr != nil {
				log.Printf("[VideoSourceService] Failed to update error status - ID: %d, Error: %v", vs.ID, updateErr)
			}
		} else {
			log.Printf("[VideoSourceService] Successfully synced video source - ID: %d, Name: %s", vs.ID, vs.Name)
			syncedCount++

			// 保存成功状态到数据库
			if updateErr := s.repo.Update(vs); updateErr != nil {
				log.Printf("[VideoSourceService] Failed to update success status - ID: %d, Error: %v", vs.ID, updateErr)
			}
		}
	}

	log.Printf("[VideoSourceService] SyncToZLMediaKit completed - Total: %d, Synced: %d, Errors: %d",
		len(streamSources), syncedCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("同步完成，但有 %d 个视频源同步失败", errorCount)
	}

	return nil
}

// TakeScreenshot 视频源截图
func (s *videoSourceService) TakeScreenshot(ctx context.Context, req *ScreenshotRequest) (*ScreenshotResponse, error) {
	log.Printf("[VideoSourceService] TakeScreenshot started - VideoSourceID: %d", req.VideoSourceID)

	// 获取视频源
	vs, err := s.repo.GetByID(req.VideoSourceID)
	if err != nil {
		log.Printf("[VideoSourceService] Failed to get video source %d: %v", req.VideoSourceID, err)
		return nil, fmt.Errorf("视频源不存在: %w", err)
	}
	log.Printf("[VideoSourceService] Video source found - ID: %d, Status: %s, Type: %s, URL: %s, PlayURL: %s",
		vs.ID, vs.Status, vs.Type, vs.URL, vs.PlayURL)

	// 检查视频源状态
	if vs.Status != models.VideoSourceStatusActive {
		log.Printf("[VideoSourceService] Video source %d is not active, current status: %s", vs.ID, vs.Status)
		return nil, fmt.Errorf("视频源未激活，当前状态: %s", vs.Status)
	}

	// 确定输入URL，参考录制服务的URL处理逻辑
	inputURL := vs.PlayURL
	if inputURL == "" {
		inputURL = vs.URL // 如果没有播放地址，使用原始URL
		log.Printf("[VideoSourceService] Using original URL as PlayURL is empty - URL: %s", inputURL)
	} else {
		log.Printf("[VideoSourceService] Using PlayURL - URL: %s", inputURL)
	}

	// 处理相对URL，如果是相对地址，添加ZLMediaKit的host和port前缀
	finalInputURL, err := s.resolveURL(inputURL)
	if err != nil {
		log.Printf("[VideoSourceService] Failed to resolve input URL %s: %v", inputURL, err)
		return nil, fmt.Errorf("解析输入URL失败: %w", err)
	}
	if finalInputURL != inputURL {
		log.Printf("[VideoSourceService] Resolved relative URL - Original: %s, Final: %s", inputURL, finalInputURL)
	}

	// 设置固定参数：全图jpg格式
	format := "jpg"

	// 创建输出目录
	outputDir := filepath.Join("/tmp", "screenshots", fmt.Sprintf("vs_%d", vs.ID))
	log.Printf("[VideoSourceService] Creating output directory: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[VideoSourceService] Failed to create output directory %s: %v", outputDir, err)
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 生成输出文件名
	timestamp := time.Now()
	fileName := fmt.Sprintf("screenshot_%d_%d.%s", vs.ID, timestamp.Unix(), format)
	screenshotPath := filepath.Join(outputDir, fileName)
	log.Printf("[VideoSourceService] Generated screenshot file path: %s", screenshotPath)

	// 执行截图
	log.Printf("[VideoSourceService] Starting FFmpeg screenshot")
	err = s.ffmpegModule.TakeScreenshot(ctx, finalInputURL, screenshotPath)
	if err != nil {
		log.Printf("[VideoSourceService] FFmpeg screenshot failed: %v", err)
		return nil, fmt.Errorf("截图失败: %w", err)
	}

	log.Printf("[VideoSourceService] Screenshot created successfully: %s", screenshotPath)

	// 读取文件内容
	fileData, err := os.ReadFile(screenshotPath)
	if err != nil {
		log.Printf("[VideoSourceService] Failed to read screenshot file %s: %v", screenshotPath, err)
		return nil, fmt.Errorf("读取截图文件失败: %w", err)
	}

	// 转换为base64编码
	base64Data := base64.StdEncoding.EncodeToString(fileData)
	log.Printf("[VideoSourceService] Screenshot converted to base64, original size: %d bytes, base64 length: %d",
		len(fileData), len(base64Data))

	// 获取图片尺寸（使用FFmpeg获取视频信息，由于是单帧，可以获取尺寸）
	videoInfo, err := s.ffmpegModule.GetVideoInfo(ctx, screenshotPath)
	var width, height int
	if err == nil {
		width = videoInfo.Width
		height = videoInfo.Height
		log.Printf("[VideoSourceService] Screenshot dimensions - Width: %d, Height: %d", width, height)
	} else {
		log.Printf("[VideoSourceService] Failed to get screenshot dimensions: %v", err)
	}

	response := &ScreenshotResponse{
		Base64Data:  base64Data,
		ContentType: "image/" + format,
		Width:       width,
		Height:      height,
		FileSize:    int64(len(fileData)),
		Timestamp:   timestamp.Format(time.RFC3339),
	}

	// 删除临时文件
	if err := os.Remove(screenshotPath); err != nil {
		log.Printf("[VideoSourceService] Failed to remove temporary screenshot file %s: %v", screenshotPath, err)
	} else {
		log.Printf("[VideoSourceService] Temporary screenshot file removed: %s", screenshotPath)
	}

	log.Printf("[VideoSourceService] TakeScreenshot completed successfully - VideoSourceID: %d, FileSize: %d, Width: %d, Height: %d, Base64Length: %d",
		req.VideoSourceID, response.FileSize, response.Width, response.Height, len(response.Base64Data))
	return response, nil
}

// resolveURL 解析URL，如果是相对地址则添加ZLMediaKit的前缀（从recordTaskService复制）
func (s *videoSourceService) resolveURL(inputURL string) (string, error) {
	if inputURL == "" {
		return "", fmt.Errorf("输入URL不能为空")
	}

	log.Printf("[VideoSourceService] resolveURL called with input: %s", inputURL)

	// 解析URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		log.Printf("[VideoSourceService] Failed to parse URL %s: %v", inputURL, err)
		return "", fmt.Errorf("URL格式错误: %w", err)
	}

	// 如果已经是完整的URL（有scheme），直接返回
	if parsedURL.Scheme != "" {
		log.Printf("[VideoSourceService] URL is already absolute: %s", inputURL)
		return inputURL, nil
	}

	// 如果是相对URL，构建完整的URL
	zlmHost := s.config.ZLMediaKit.Host
	zlmPort := s.config.ZLMediaKit.Port

	// 处理路径，去除路由前缀
	path := parsedURL.Path
	routePrefix := s.config.PlayURL.RoutePrefix

	// 确保路径以/开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 去除路由前缀（如果存在）
	if routePrefix != "" && strings.HasPrefix(path, routePrefix) {
		path = strings.TrimPrefix(path, routePrefix)
		log.Printf("[VideoSourceService] Removed route prefix '%s' from path, new path: %s", routePrefix, path)

		// 确保去除前缀后路径仍以/开头
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	// 保留查询参数和片段
	resolvedURL := fmt.Sprintf("http://%s:%d%s", zlmHost, zlmPort, path)
	if parsedURL.RawQuery != "" {
		resolvedURL += "?" + parsedURL.RawQuery
	}
	if parsedURL.Fragment != "" {
		resolvedURL += "#" + parsedURL.Fragment
	}

	log.Printf("[VideoSourceService] Resolved relative URL - ZLMHost: %s, ZLMPort: %d, RoutePrefix: %s, OriginalPath: %s, FinalPath: %s, Final: %s",
		zlmHost, zlmPort, routePrefix, parsedURL.Path, path, resolvedURL)

	return resolvedURL, nil
}

// VideoFileService实现
type videoFileService struct {
	repo            repository.VideoFileRepository
	videoSourceRepo repository.VideoSourceRepository
	ffmpegModule    *ffmpeg.FFmpegModule
	basePath        string
}

func NewVideoFileService(repo repository.VideoFileRepository, videoSourceRepo repository.VideoSourceRepository, ffmpegModule *ffmpeg.FFmpegModule, basePath string) VideoFileService {
	return &videoFileService{
		repo:            repo,
		videoSourceRepo: videoSourceRepo,
		ffmpegModule:    ffmpegModule,
		basePath:        basePath,
	}
}

func (s *videoFileService) UploadVideoFile(ctx context.Context, req *UploadVideoFileRequest) (*models.VideoFile, error) {
	log.Printf("[VideoFileService] UploadVideoFile started - Name: %s, OriginalFilename: %s, UserID: %d",
		req.Name, req.FileHeader.Filename, req.UserID)

	// 创建上传目录
	uploadDir := filepath.Join(s.basePath, "uploads")
	log.Printf("[VideoFileService] Creating upload directory - Path: %s", uploadDir)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[VideoFileService] Failed to create upload directory - Path: %s, Error: %v", uploadDir, err)
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	// 生成唯一文件名（使用UUID确保唯一性）
	fileExt := filepath.Ext(req.FileHeader.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), fileExt)
	filePath := filepath.Join(uploadDir, filename)
	log.Printf("[VideoFileService] Generated unique filename - Original: %s, Unique: %s, Path: %s",
		req.FileHeader.Filename, filename, filePath)

	// 保存文件
	log.Printf("[VideoFileService] Creating destination file - Path: %s", filePath)
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("[VideoFileService] Failed to create destination file - Path: %s, Error: %v", filePath, err)
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer dst.Close()

	log.Printf("[VideoFileService] Copying file content - Source: %s, Destination: %s", req.FileHeader.Filename, filePath)
	bytesWritten, err := io.Copy(dst, req.File)
	if err != nil {
		log.Printf("[VideoFileService] Failed to copy file content - Error: %v", err)
		os.Remove(filePath) // 清理文件
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}
	log.Printf("[VideoFileService] File content copied successfully - BytesWritten: %d", bytesWritten)

	// 使用FFmpeg获取视频信息
	log.Printf("[VideoFileService] Getting video info using FFmpeg - FilePath: %s", filePath)
	videoInfo, err := s.ffmpegModule.GetVideoInfo(ctx, filePath)
	if err != nil {
		log.Printf("[VideoFileService] Failed to get video info - FilePath: %s, Error: %v", filePath, err)
		os.Remove(filePath) // 清理文件
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}
	log.Printf("[VideoFileService] Video info retrieved - Duration: %.2f, Width: %d, Height: %d, FileSize: %d, Format: %s",
		videoInfo.Duration, videoInfo.Width, videoInfo.Height, videoInfo.FileSize, videoInfo.Format)

	log.Printf("[VideoFileService] Creating video file model - Name: %s, OriginalPath: %s", req.Name, filePath)
	vf := &models.VideoFile{
		Name:             req.Name,
		DisplayName:      req.Name,
		OriginalFileName: req.FileHeader.Filename, // 保存原始文件名
		Description:      req.Description,
		OriginalPath:     filePath,
		FileSize:         videoInfo.FileSize,
		Duration:         videoInfo.Duration,
		Width:            videoInfo.Width,
		Height:           videoInfo.Height,
		FrameRate:        videoInfo.FrameRate,
		Bitrate:          videoInfo.Bitrate,
		Codec:            videoInfo.Codec,
		Format:           videoInfo.Format,
		Status:           models.VideoFileStatusUploaded,
		UserID:           req.UserID,
	}

	log.Printf("[VideoFileService] Saving video file record to database - Name: %s", vf.Name)
	if err := s.repo.Create(vf); err != nil {
		log.Printf("[VideoFileService] Failed to save video file record - Name: %s, Error: %v", vf.Name, err)
		os.Remove(filePath) // 清理文件
		return nil, fmt.Errorf("保存文件记录失败: %w", err)
	}

	log.Printf("[VideoFileService] Video file record saved successfully - ID: %d, Name: %s", vf.ID, vf.Name)

	// 异步转换视频
	log.Printf("[VideoFileService] Starting async video conversion - VideoFileID: %d", vf.ID)
	go func() {
		if err := s.convertVideoFileAsync(context.Background(), vf); err != nil {
			log.Printf("[VideoFileService] Async video conversion failed - VideoFileID: %d, Error: %v", vf.ID, err)
		}
	}()

	log.Printf("[VideoFileService] UploadVideoFile completed successfully - ID: %d, Name: %s, OriginalPath: %s",
		vf.ID, vf.Name, vf.OriginalPath)
	return vf, nil
}

// convertVideoFileAsync 异步转换视频文件
func (s *videoFileService) convertVideoFileAsync(ctx context.Context, vf *models.VideoFile) error {
	log.Printf("[VideoFileService] convertVideoFileAsync started - VideoFileID: %d, Name: %s, OriginalPath: %s",
		vf.ID, vf.Name, vf.OriginalPath)

	// 标记为转换中
	log.Printf("[VideoFileService] Marking video file as converting - VideoFileID: %d", vf.ID)
	vf.MarkAsConverting()
	if err := s.repo.Update(vf); err != nil {
		log.Printf("[VideoFileService] Failed to update video file status to converting - VideoFileID: %d, Error: %v",
			vf.ID, err)
	}

	// 创建转换目录
	convertedDir := filepath.Join(s.basePath, "converted")
	log.Printf("[VideoFileService] Creating converted directory - Path: %s", convertedDir)
	if err := os.MkdirAll(convertedDir, 0755); err != nil {
		log.Printf("[VideoFileService] Failed to create converted directory - Path: %s, Error: %v", convertedDir, err)
		vf.MarkAsError(fmt.Sprintf("创建转换目录失败: %v", err))
		s.repo.Update(vf)
		return err
	}

	// 生成转换后的文件名（使用数据库ID确保唯一性）
	convertedFileName := fmt.Sprintf("converted_%d_%s.mp4", vf.ID, uuid.New().String()[:8])
	convertedPath := filepath.Join(convertedDir, convertedFileName)
	log.Printf("[VideoFileService] Generated converted file path - VideoFileID: %d, FileName: %s, Path: %s",
		vf.ID, convertedFileName, convertedPath)

	// 配置转换参数
	log.Printf("[VideoFileService] Configuring FFmpeg conversion parameters - VideoFileID: %d", vf.ID)
	convertReq := &ffmpeg.ConvertVideoRequest{
		InputPath:  vf.OriginalPath,
		OutputPath: convertedPath,
		Format:     "mp4",
		VideoCodec: "h264",
		AudioCodec: "aac",
		Quality:    23,
	}
	log.Printf("[VideoFileService] FFmpeg parameters configured - VideoFileID: %d, InputPath: %s, OutputPath: %s, Format: %s, VideoCodec: %s, AudioCodec: %s, Quality: %d",
		vf.ID, convertReq.InputPath, convertReq.OutputPath, convertReq.Format, convertReq.VideoCodec, convertReq.AudioCodec, convertReq.Quality)

	// 执行转换
	log.Printf("[VideoFileService] Starting FFmpeg conversion - VideoFileID: %d", vf.ID)
	if err := s.ffmpegModule.ConvertVideo(ctx, convertReq); err != nil {
		log.Printf("[VideoFileService] FFmpeg conversion failed - VideoFileID: %d, Error: %v", vf.ID, err)
		vf.MarkAsError(fmt.Sprintf("视频转换失败: %v", err))
		s.repo.Update(vf)
		return err
	}

	log.Printf("[VideoFileService] FFmpeg conversion completed successfully - VideoFileID: %d, OutputPath: %s",
		vf.ID, convertedPath)

	// 验证转换后的文件是否存在
	if fileInfo, err := os.Stat(convertedPath); err != nil {
		log.Printf("[VideoFileService] Converted file does not exist - VideoFileID: %d, Path: %s, Error: %v",
			vf.ID, convertedPath, err)
		vf.MarkAsError("转换后的文件不存在")
		s.repo.Update(vf)
		return fmt.Errorf("转换后的文件不存在: %s", convertedPath)
	} else {
		log.Printf("[VideoFileService] Converted file verified - VideoFileID: %d, Size: %d bytes", vf.ID, fileInfo.Size())
	}

	// 标记为转换完成
	log.Printf("[VideoFileService] Marking video file as ready - VideoFileID: %d, ConvertedPath: %s",
		vf.ID, convertedPath)
	vf.MarkAsReady(convertedPath)

	// 生成播放地址 (相对路径)
	vf.PlayURL = fmt.Sprintf("/stream/record/videos/converted/%s", convertedFileName)
	log.Printf("[VideoFileService] Generated play URL - VideoFileID: %d, PlayURL: %s", vf.ID, vf.PlayURL)

	// 设置stream_id为转换后的文件名（不包括.mp4扩展名）
	vf.StreamID = strings.TrimSuffix(convertedFileName, ".mp4")
	log.Printf("[VideoFileService] Set stream_id for converted video - VideoFileID: %d, StreamID: %s",
		vf.ID, vf.StreamID)

	if err := s.repo.Update(vf); err != nil {
		log.Printf("[VideoFileService] Failed to update video file after conversion - VideoFileID: %d, Error: %v",
			vf.ID, err)
		return err
	}

	log.Printf("[VideoFileService] convertVideoFileAsync completed successfully - VideoFileID: %d, Name: %s, PlayURL: %s",
		vf.ID, vf.Name, vf.PlayURL)
	return nil
}

func (s *videoFileService) GetVideoFiles(ctx context.Context, req *GetVideoFilesRequest) ([]*models.VideoFile, int64, error) {
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	var status *models.VideoFileStatus
	if req.Status != nil {
		s := models.VideoFileStatus(*req.Status)
		status = &s
	}

	return s.repo.List(req.Page, req.PageSize, status, req.UserID)
}

func (s *videoFileService) GetVideoFile(ctx context.Context, id uint) (*models.VideoFile, error) {
	return s.repo.GetByID(id)
}

func (s *videoFileService) UpdateVideoFile(ctx context.Context, id uint, req *UpdateVideoFileRequest) (*models.VideoFile, error) {
	vf, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("视频文件不存在: %w", err)
	}

	if req.Name != nil {
		vf.Name = *req.Name
	}
	if req.Description != nil {
		vf.Description = *req.Description
	}
	if req.DisplayName != nil {
		vf.DisplayName = *req.DisplayName
	}

	if err := s.repo.Update(vf); err != nil {
		return nil, fmt.Errorf("更新视频文件失败: %w", err)
	}

	return vf, nil
}

func (s *videoFileService) DeleteVideoFile(ctx context.Context, id uint) error {
	vf, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("视频文件不存在: %w", err)
	}

	// 删除文件
	if vf.OriginalPath != "" {
		os.Remove(vf.OriginalPath)
	}
	if vf.ConvertedPath != "" {
		os.Remove(vf.ConvertedPath)
	}

	return s.repo.Delete(id)
}

func (s *videoFileService) DownloadVideoFile(ctx context.Context, id uint) (*DownloadInfo, error) {
	vf, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("视频文件不存在: %w", err)
	}

	filePath := vf.GetActivePath()
	if filePath == "" {
		return nil, fmt.Errorf("文件路径为空")
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("文件不存在")
	}

	// 使用原始文件名作为下载文件名，如果没有则使用Name + Format
	downloadFileName := vf.OriginalFileName
	if downloadFileName == "" {
		downloadFileName = vf.Name + "." + vf.Format
	}

	return &DownloadInfo{
		FilePath:    filePath,
		FileName:    downloadFileName,
		ContentType: "video/" + vf.Format,
		FileSize:    vf.FileSize,
	}, nil
}

func (s *videoFileService) ConvertVideoFile(ctx context.Context, id uint) error {
	log.Printf("[VideoFileService] ConvertVideoFile called - VideoFileID: %d", id)

	vf, err := s.repo.GetByID(id)
	if err != nil {
		log.Printf("[VideoFileService] Failed to get video file - VideoFileID: %d, Error: %v", id, err)
		return fmt.Errorf("视频文件不存在: %w", err)
	}

	log.Printf("[VideoFileService] Video file found - VideoFileID: %d, Name: %s, Status: %s, OriginalPath: %s",
		vf.ID, vf.Name, vf.Status, vf.OriginalPath)

	// 验证文件状态 - 只允许已上传和失败状态的文件进行转换
	if vf.Status != models.VideoFileStatusUploaded && vf.Status != models.VideoFileStatusFailed {
		log.Printf("[VideoFileService] File status not allowed for conversion - VideoFileID: %d, Status: %s, Allowed: [uploaded, failed]",
			vf.ID, vf.Status)
		return fmt.Errorf("文件状态不允许转换，当前状态: %s，只有uploaded和failed状态的文件可以转换", vf.Status)
	}

	// 检查原始文件是否存在
	if vf.OriginalPath == "" {
		log.Printf("[VideoFileService] Original file path is empty - VideoFileID: %d", vf.ID)
		return fmt.Errorf("原始文件路径为空")
	}

	if _, err := os.Stat(vf.OriginalPath); os.IsNotExist(err) {
		log.Printf("[VideoFileService] Original file does not exist - VideoFileID: %d, Path: %s", vf.ID, vf.OriginalPath)
		return fmt.Errorf("原始文件不存在: %s", vf.OriginalPath)
	}

	log.Printf("[VideoFileService] Starting async video conversion - VideoFileID: %d", vf.ID)

	// 调用异步转换函数 - 使用background context避免HTTP context取消影响
	go func() {
		if err := s.convertVideoFileAsync(context.Background(), vf); err != nil {
			log.Printf("[VideoFileService] Async video conversion failed - VideoFileID: %d, Error: %v", vf.ID, err)
		}
	}()

	log.Printf("[VideoFileService] ConvertVideoFile completed - VideoFileID: %d, async conversion started", vf.ID)
	return nil
}

// StartMonitoring 启动视频源监控服务
func (s *videoSourceService) StartMonitoring() {
	s.monitorMux.Lock()
	if s.isMonitoring {
		s.monitorMux.Unlock()
		log.Printf("[VideoSourceService] Monitoring already started")
		return
	}
	s.isMonitoring = true
	s.monitorMux.Unlock()

	log.Printf("[VideoSourceService] Starting video source monitoring service")

	// 启动时立即执行一次检查
	log.Printf("[VideoSourceService] Performing initial video source check")
	if err := s.checkAndRecoverVideoSources(context.Background()); err != nil {
		log.Printf("[VideoSourceService] Initial video source check failed: %v", err)
	}

	// 设置定期检查，默认每5分钟检查一次
	monitorInterval := 5 * time.Minute
	s.monitorTicker = time.NewTicker(monitorInterval)

	log.Printf("[VideoSourceService] Video source monitoring started with interval: %v", monitorInterval)

	for {
		select {
		case <-s.monitorTicker.C:
			log.Printf("[VideoSourceService] Performing periodic video source check")
			if err := s.checkAndRecoverVideoSources(context.Background()); err != nil {
				log.Printf("[VideoSourceService] Periodic video source check failed: %v", err)
			}
		case <-s.monitorDone:
			log.Printf("[VideoSourceService] Monitoring service stopped")
			s.monitorTicker.Stop()
			s.monitorMux.Lock()
			s.isMonitoring = false
			s.monitorMux.Unlock()
			return
		}
	}
}

// StopMonitoring 停止视频源监控服务
func (s *videoSourceService) StopMonitoring() {
	s.monitorMux.RLock()
	if !s.isMonitoring {
		s.monitorMux.RUnlock()
		log.Printf("[VideoSourceService] Monitoring is not running")
		return
	}
	s.monitorMux.RUnlock()

	log.Printf("[VideoSourceService] Stopping video source monitoring service")
	s.monitorDone <- true
}

// checkAndRecoverVideoSources 检查并恢复视频源
func (s *videoSourceService) checkAndRecoverVideoSources(ctx context.Context) error {
	log.Printf("[VideoSourceService] checkAndRecoverVideoSources started")

	// 获取所有流类型的视频源
	sources, _, err := s.GetVideoSources(ctx, &GetVideoSourcesRequest{
		Page:     1,
		PageSize: 1000, // 假设最多1000个源
	})
	if err != nil {
		log.Printf("[VideoSourceService] Failed to get video sources for monitoring: %v", err)
		return fmt.Errorf("获取视频源列表失败: %w", err)
	}

	// 过滤出活动状态的流类型视频源
	var activeStreamSources []*models.VideoSource
	for _, vs := range sources {
		if vs.Type == models.VideoSourceTypeStream && vs.Status == models.VideoSourceStatusActive {
			activeStreamSources = append(activeStreamSources, vs)
		}
	}

	log.Printf("[VideoSourceService] Found %d active stream video sources to check", len(activeStreamSources))

	if len(activeStreamSources) == 0 {
		log.Printf("[VideoSourceService] No active stream video sources to monitor")
		return nil
	}

	var checkedCount, recoveredCount, errorCount int

	// 检查每个活动的流视频源
	for _, vs := range activeStreamSources {
		log.Printf("[VideoSourceService] Checking stream video source - ID: %d, Name: %s, StreamID: %s",
			vs.ID, vs.Name, vs.StreamID)
		checkedCount++

		// 检查流是否在ZLMediaKit中在线
		isOnline, err := s.zlmClient.IsMediaOnline(ctx, "__defaultVhost__", "live", vs.StreamID)
		if err != nil {
			log.Printf("[VideoSourceService] Failed to check media online status - VideoSourceID: %d, StreamID: %s, Error: %v",
				vs.ID, vs.StreamID, err)
			errorCount++
			continue
		}

		log.Printf("[VideoSourceService] Media online status - VideoSourceID: %d, StreamID: %s, IsOnline: %t",
			vs.ID, vs.StreamID, isOnline)

		if !isOnline {
			log.Printf("[VideoSourceService] Stream is offline, attempting to recover - VideoSourceID: %d, StreamID: %s",
				vs.ID, vs.StreamID)

			// 流不在线，尝试重新添加
			if err := s.startStreamVideoSource(ctx, vs); err != nil {
				log.Printf("[VideoSourceService] Failed to recover video source - ID: %d, Name: %s, Error: %v",
					vs.ID, vs.Name, err)
				errorCount++

				// 更新错误状态到数据库
				vs.Status = models.VideoSourceStatusError
				vs.ErrorMsg = fmt.Sprintf("监控检查失败，无法恢复: %v", err)
				if updateErr := s.repo.Update(vs); updateErr != nil {
					log.Printf("[VideoSourceService] Failed to update error status - ID: %d, Error: %v",
						vs.ID, updateErr)
				}
			} else {
				log.Printf("[VideoSourceService] Successfully recovered video source - ID: %d, Name: %s",
					vs.ID, vs.Name)
				recoveredCount++

				// 保存恢复状态到数据库
				if updateErr := s.repo.Update(vs); updateErr != nil {
					log.Printf("[VideoSourceService] Failed to update recovered status - ID: %d, Error: %v",
						vs.ID, updateErr)
				}
			}
		} else {
			log.Printf("[VideoSourceService] Stream is online - VideoSourceID: %d, StreamID: %s",
				vs.ID, vs.StreamID)
		}
	}

	log.Printf("[VideoSourceService] checkAndRecoverVideoSources completed - Checked: %d, Recovered: %d, Errors: %d",
		checkedCount, recoveredCount, errorCount)

	if errorCount > 0 {
		return fmt.Errorf("监控检查完成，但有 %d 个视频源检查失败", errorCount)
	}

	return nil
}

// GetMonitoringStatus 获取监控状态
func (s *videoSourceService) GetMonitoringStatus() map[string]interface{} {
	s.monitorMux.RLock()
	defer s.monitorMux.RUnlock()

	status := map[string]interface{}{
		"is_monitoring":    s.isMonitoring,
		"monitor_interval": "5m",
	}

	if s.monitorTicker != nil {
		status["next_check"] = "within 5 minutes"
	}

	return status
}
