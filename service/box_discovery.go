/*
 * @module service/box_discovery
 * @description 盒子发现服务，实现网络扫描和盒子自动发现功能
 * @architecture 服务层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow 网络扫描 -> 健康检查 -> 版本验证 -> 自动注册
 * @rules 支持IP范围扫描、手动添加验证、自动发现等功能
 * @dependencies gorm.io/gorm, net/http
 * @refs DESIGN-001.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BoxDiscoveryService 盒子发现服务
type BoxDiscoveryService struct {
	repoManager  repository.RepositoryManager
	httpClient   *http.Client
	scanTasks    map[string]*ScanTask // 扫描任务管理
	scanTasksMux sync.RWMutex         // 扫描任务锁
	logService   SystemLogService     // 系统日志服务
}

// ScanTask 扫描任务
type ScanTask struct {
	ScanID       string    `json:"scan_id"`
	IPRange      string    `json:"ip_range"`
	Port         int       `json:"port"`
	CreatedBy    uint      `json:"created_by"`
	Status       string    `json:"status"` // scanning, completed, failed, cancelled
	TotalIPs     int       `json:"total_ips"`
	ScannedIPs   int       `json:"scanned_ips"`
	FoundBoxes   int       `json:"found_boxes"`
	StartTime    time.Time `json:"start_time"`
	UpdateTime   time.Time `json:"update_time"`
	EndTime      time.Time `json:"end_time"`
	ErrorMessage string    `json:"error_message"`
	Progress     float64   `json:"progress"`
	CurrentIP    string    `json:"current_ip"`
	Cancel       chan bool `json:"-"` // 取消信号
}

// NewBoxDiscoveryService 创建盒子发现服务
func NewBoxDiscoveryService(repoManager repository.RepositoryManager, logService SystemLogService) *BoxDiscoveryService {
	return &BoxDiscoveryService{
		repoManager: repoManager,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		scanTasks:  make(map[string]*ScanTask),
		logService: logService,
	}
}

// GetRepoManager 获取repository manager
func (s *BoxDiscoveryService) GetRepoManager() repository.RepositoryManager {
	return s.repoManager
}

// DiscoveredBox 发现的盒子信息
type DiscoveredBox struct {
	IPAddress string   `json:"ip_address"`
	Port      int      `json:"port"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Status    string   `json:"status"`
	IsNew     bool     `json:"is_new"`   // 是否是新发现的盒子
	Exists    bool     `json:"exists"`   // 是否已存在于数据库
	BoxInfo   *BoxInfo `json:"box_info"` // 详细信息
}

// BoxInfo 盒子详细信息
type BoxInfo struct {
	Service     string `json:"service"`
	Version     string `json:"version"`
	BuildTime   string `json:"build_time"`
	APIVersion  string `json:"api_version"`
	Timestamp   string `json:"timestamp"`
	HealthCheck bool   `json:"health_check"`
}

// ScanNetwork 扫描网络范围内的盒子
func (s *BoxDiscoveryService) ScanNetwork(ipRange string, port int) ([]*DiscoveredBox, error) {
	log.Printf("[BoxDiscoveryService] ScanNetwork started - IPRange: %s, Port: %d", ipRange, port)

	// 记录扫描开始日志
	if s.logService != nil {
		s.logService.Info("box_discovery_service", "网络扫描开始",
			fmt.Sprintf("开始扫描网络范围 %s:%d", ipRange, port),
			WithMetadata(map[string]interface{}{
				"ip_range": ipRange,
				"port":     port,
			}))
	}

	ips, err := s.parseIPRange(ipRange)
	if err != nil {
		log.Printf("[BoxDiscoveryService] Failed to parse IP range - IPRange: %s, Error: %v", ipRange, err)

		// 记录解析失败日志
		if s.logService != nil {
			s.logService.Error("box_discovery_service", "IP范围解析失败",
				fmt.Sprintf("无法解析IP范围 %s", ipRange), err,
				WithMetadata(map[string]interface{}{
					"ip_range": ipRange,
					"port":     port,
				}))
		}

		return nil, fmt.Errorf("解析IP范围失败: %w", err)
	}

	log.Printf("[BoxDiscoveryService] Parsed %d IPs to scan", len(ips))

	// 并发扫描
	var wg sync.WaitGroup
	var mu sync.Mutex
	var discoveredBoxes []*DiscoveredBox

	// 限制并发数
	semaphore := make(chan struct{}, 50)

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if box := s.scanSingleIP(ip, port); box != nil {
				mu.Lock()
				discoveredBoxes = append(discoveredBoxes, box)
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()

	// 检查哪些盒子已存在于数据库
	log.Printf("[BoxDiscoveryService] Checking existing boxes in database - Found: %d", len(discoveredBoxes))
	err = s.checkExistingBoxes(discoveredBoxes)
	if err != nil {
		log.Printf("[BoxDiscoveryService] Failed to check existing boxes - Error: %v", err)

		// 记录检查失败日志
		if s.logService != nil {
			s.logService.Error("box_discovery_service", "检查已存在盒子失败",
				"检查数据库中已存在盒子时发生错误", err,
				WithMetadata(map[string]interface{}{
					"ip_range":    ipRange,
					"port":        port,
					"found_boxes": len(discoveredBoxes),
				}))
		}

		return nil, fmt.Errorf("检查已存在盒子失败: %w", err)
	}

	log.Printf("[BoxDiscoveryService] Network scan completed - Total discovered: %d", len(discoveredBoxes))

	// 记录扫描完成日志
	if s.logService != nil {
		s.logService.Info("box_discovery_service", "网络扫描完成",
			fmt.Sprintf("网络扫描完成，发现 %d 个盒子", len(discoveredBoxes)),
			WithMetadata(map[string]interface{}{
				"ip_range":         ipRange,
				"port":             port,
				"discovered_boxes": len(discoveredBoxes),
				"total_ips":        len(ips),
			}))
	}

	return discoveredBoxes, nil
}

// VerifyBox 验证指定IP和端口的盒子
func (s *BoxDiscoveryService) VerifyBox(ip string, port int) (*BoxInfo, error) {
	log.Printf("[BoxDiscoveryService] VerifyBox started - IP: %s, Port: %d", ip, port)

	// 首先进行健康检查
	log.Printf("[BoxDiscoveryService] Performing health check - IP: %s, Port: %d", ip, port)
	healthOK := s.checkHealth(ip, port)
	if !healthOK {
		log.Printf("[BoxDiscoveryService] Health check failed - IP: %s, Port: %d", ip, port)
		return nil, fmt.Errorf("盒子健康检查失败")
	}

	// 获取版本信息
	log.Printf("[BoxDiscoveryService] Getting box info - IP: %s, Port: %d", ip, port)
	boxInfo, err := s.getBoxInfo(ip, port)
	if err != nil {
		log.Printf("[BoxDiscoveryService] Failed to get box info - IP: %s, Port: %d, Error: %v", ip, port, err)
		return nil, fmt.Errorf("获取盒子信息失败: %w", err)
	}

	boxInfo.HealthCheck = true
	log.Printf("[BoxDiscoveryService] Box verification completed successfully - IP: %s, Version: %s", ip, boxInfo.Version)
	return boxInfo, nil
}

// RegisterBox 注册新发现的盒子
func (s *BoxDiscoveryService) RegisterBox(boxInfo *DiscoveredBox, createdBy uint) (*models.Box, error) {
	log.Printf("[BoxDiscoveryService] RegisterBox started - IP: %s:%d, Name: %s, CreatedBy: %d",
		boxInfo.IPAddress, boxInfo.Port, boxInfo.Name, createdBy)

	// 检查是否已存在
	ctx := context.Background()
	log.Printf("[BoxDiscoveryService] Checking for existing box - IP: %s", boxInfo.IPAddress)
	existingBox, err := s.repoManager.Box().FindByIPAddress(ctx, boxInfo.IPAddress)
	if err != nil {
		log.Printf("[BoxDiscoveryService] Failed to check existing box - IP: %s, Error: %v", boxInfo.IPAddress, err)
		return nil, fmt.Errorf("检查盒子是否存在失败: %w", err)
	}
	if existingBox != nil && existingBox.Port == boxInfo.Port {
		log.Printf("[BoxDiscoveryService] Box already exists - IP: %s:%d, ExistingID: %d",
			boxInfo.IPAddress, boxInfo.Port, existingBox.ID)
		return nil, fmt.Errorf("盒子已存在: %s:%d", boxInfo.IPAddress, boxInfo.Port)
	}

	// 生成盒子名称（如果没有提供）
	if boxInfo.Name == "" {
		boxInfo.Name = fmt.Sprintf("Box-%s-%d", strings.ReplaceAll(boxInfo.IPAddress, ".", "-"), boxInfo.Port)
	}

	// 创建新盒子
	box := &models.Box{
		Name:      boxInfo.Name,
		IPAddress: boxInfo.IPAddress,
		Port:      boxInfo.Port,
		Status:    models.BoxStatusOnline,
		CreatedBy: createdBy,
	}

	// 如果有详细信息，设置硬件信息（使用默认值，后续通过监控服务更新）
	if boxInfo.BoxInfo != nil {
		box.Hardware = models.Hardware{
			CPUCores:          0,
			CPULogicalCores:   0,
			CPUModelName:      "Unknown",
			Hostname:          boxInfo.Name,
			KernelArch:        "Unknown",
			KernelVersion:     "Unknown",
			OS:                "Unknown",
			Platform:          "Unknown",
			PlatformFamily:    "Unknown",
			PlatformVersion:   "Unknown",
			SDKVersion:        "Unknown",
			SoftwareBuildTime: "Unknown",
			SoftwareVersion:   boxInfo.Version,
		}
	}

	// 更新心跳时间
	box.UpdateHeartbeat()

	// 保存到数据库
	log.Printf("[BoxDiscoveryService] Saving box to database - Name: %s", box.Name)
	if err := s.repoManager.Box().Create(ctx, box); err != nil {
		log.Printf("[BoxDiscoveryService] Failed to save box to database - Name: %s, Error: %v", box.Name, err)

		// 记录保存失败日志
		if s.logService != nil {
			s.logService.Error("box_discovery_service", "盒子注册失败",
				fmt.Sprintf("保存盒子到数据库失败: %s", box.Name), err,
				WithMetadata(map[string]interface{}{
					"box_name":   box.Name,
					"ip_address": box.IPAddress,
					"port":       box.Port,
					"created_by": createdBy,
				}))
		}

		return nil, fmt.Errorf("保存盒子到数据库失败: %w", err)
	}

	log.Printf("[BoxDiscoveryService] Box registered successfully - ID: %d, Name: %s, IP: %s:%d",
		box.ID, box.Name, box.IPAddress, box.Port)

	// 记录注册成功日志
	if s.logService != nil {
		s.logService.Info("box_discovery_service", "盒子注册成功",
			fmt.Sprintf("成功注册新盒子: %s (%s:%d)", box.Name, box.IPAddress, box.Port),
			WithMetadata(map[string]interface{}{
				"box_id":     box.ID,
				"box_name":   box.Name,
				"ip_address": box.IPAddress,
				"port":       box.Port,
				"created_by": createdBy,
				"version":    boxInfo.Version,
			}))
	}

	return box, nil
}

// ManualAddBox 手动添加盒子
func (s *BoxDiscoveryService) ManualAddBox(name, ipAddress string, port int, createdBy uint) (*models.Box, error) {
	// 验证盒子连接
	boxInfo, err := s.VerifyBox(ipAddress, port)
	if err != nil {
		return nil, fmt.Errorf("验证盒子连接失败: %w", err)
	}

	// 创建发现的盒子信息
	discoveredBox := &DiscoveredBox{
		IPAddress: ipAddress,
		Port:      port,
		Name:      name,
		Version:   boxInfo.Version,
		Status:    "online",
		BoxInfo:   boxInfo,
	}

	// 注册盒子
	return s.RegisterBox(discoveredBox, createdBy)
}

// parseIPRange 解析IP范围
func (s *BoxDiscoveryService) parseIPRange(ipRange string) ([]string, error) {
	var ips []string

	if strings.Contains(ipRange, "-") {
		// 范围格式: 192.168.1.1-192.168.1.254
		parts := strings.Split(ipRange, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的IP范围格式")
		}

		startIP := net.ParseIP(strings.TrimSpace(parts[0]))
		endIP := net.ParseIP(strings.TrimSpace(parts[1]))
		if startIP == nil || endIP == nil {
			return nil, fmt.Errorf("无效的IP地址")
		}

		// 生成IP列表
		current := startIP.To4()
		end := endIP.To4()

		for {
			ips = append(ips, current.String())
			if current.Equal(end) {
				break
			}
			// 递增IP
			current[3]++
			if current[3] == 0 {
				current[2]++
				if current[2] == 0 {
					current[1]++
					if current[1] == 0 {
						current[0]++
					}
				}
			}
		}
	} else if strings.Contains(ipRange, "/") {
		// CIDR格式: 192.168.1.0/24
		_, ipNet, err := net.ParseCIDR(ipRange)
		if err != nil {
			return nil, fmt.Errorf("无效的CIDR格式: %w", err)
		}

		for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); {
			ips = append(ips, ip.String())
			// 递增IP
			for j := len(ip) - 1; j >= 0; j-- {
				ip[j]++
				if ip[j] > 0 {
					break
				}
			}
		}
	} else {
		// 单个IP
		if net.ParseIP(ipRange) == nil {
			return nil, fmt.Errorf("无效的IP地址")
		}
		ips = append(ips, ipRange)
	}

	return ips, nil
}

// scanSingleIP 扫描单个IP
func (s *BoxDiscoveryService) scanSingleIP(ip string, port int) *DiscoveredBox {
	// 首先检查端口是否开放
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 1*time.Second)
	if err != nil {
		return nil
	}
	conn.Close()

	// 检查是否是AI盒子
	if !s.checkHealth(ip, port) {
		return nil
	}

	// 获取盒子信息
	boxInfo, err := s.getBoxInfo(ip, port)
	if err != nil {
		return nil
	}

	return &DiscoveredBox{
		IPAddress: ip,
		Port:      port,
		Version:   boxInfo.Version,
		Status:    "online",
		BoxInfo:   boxInfo,
	}
}

// checkHealth 检查盒子健康状态
func (s *BoxDiscoveryService) checkHealth(ip string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/api/v1/health", ip, port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// getBoxInfo 获取盒子详细信息
func (s *BoxDiscoveryService) getBoxInfo(ip string, port int) (*BoxInfo, error) {
	url := fmt.Sprintf("http://%s:%d/api/v1/system/version", ip, port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var boxInfo BoxInfo
	if err := json.NewDecoder(resp.Body).Decode(&boxInfo); err != nil {
		return nil, err
	}

	return &boxInfo, nil
}

// checkExistingBoxes 检查发现的盒子是否已存在
func (s *BoxDiscoveryService) checkExistingBoxes(boxes []*DiscoveredBox) error {
	if len(boxes) == 0 {
		return nil
	}

	// 逐一检查每个盒子是否存在
	ctx := context.Background()
	for _, box := range boxes {
		existingBox, err := s.repoManager.Box().FindByIPAddress(ctx, box.IPAddress)
		if err != nil {
			return err
		}

		// 检查IP和端口都匹配的情况
		if existingBox != nil && existingBox.Port == box.Port {
			box.Exists = true
		} else {
			box.IsNew = true
		}
	}

	return nil
}

// ScanNetworkAsync 异步扫描网络范围内的盒子
func (s *BoxDiscoveryService) ScanNetworkAsync(ipRange string, port int, createdBy uint, sseService SSEService) (string, error) {
	log.Printf("[BoxDiscoveryService] ScanNetworkAsync started - IPRange: %s, Port: %d, CreatedBy: %d", ipRange, port, createdBy)

	// 解析IP范围以计算总数
	ips, err := s.parseIPRange(ipRange)
	if err != nil {
		log.Printf("[BoxDiscoveryService] Failed to parse IP range - IPRange: %s, Error: %v", ipRange, err)
		return "", fmt.Errorf("解析IP范围失败: %w", err)
	}

	// 生成扫描任务ID
	scanID := s.generateScanID()

	// 创建扫描任务
	scanTask := &ScanTask{
		ScanID:     scanID,
		IPRange:    ipRange,
		Port:       port,
		CreatedBy:  createdBy,
		Status:     "scanning",
		TotalIPs:   len(ips),
		ScannedIPs: 0,
		FoundBoxes: 0,
		StartTime:  time.Now(),
		UpdateTime: time.Now(),
		Progress:   0.0,
		Cancel:     make(chan bool, 1),
	}

	// 注册扫描任务
	s.scanTasksMux.Lock()
	s.scanTasks[scanID] = scanTask
	s.scanTasksMux.Unlock()

	log.Printf("[BoxDiscoveryService] Scan task created - ScanID: %s, TotalIPs: %d", scanID, len(ips))

	// 发送扫描开始事件
	if sseService != nil {
		progress := &DiscoveryProgress{
			ScanID:        scanID,
			IPRange:       ipRange,
			Port:          port,
			TotalIPs:      len(ips),
			ScannedIPs:    0,
			FoundBoxes:    0,
			Status:        "scanning",
			Progress:      0.0,
			CurrentIP:     "",
			StartTime:     scanTask.StartTime,
			UpdateTime:    scanTask.UpdateTime,
			EstimatedTime: 0,
			ErrorMessage:  "",
		}
		sseService.BroadcastDiscoveryProgress(progress)

		// 发送系统事件
		metadata := map[string]interface{}{
			"scan_id":   scanID,
			"ip_range":  ipRange,
			"port":      port,
			"total_ips": len(ips),
			"source":    "box_discovery_service",
			"source_id": scanID,
			"title":     "网络扫描已启动",
			"message":   fmt.Sprintf("开始扫描IP范围 %s:%d，预计扫描 %d 个IP地址", ipRange, port, len(ips)),
		}
		sseService.BroadcastDiscoveryStarted(scanID, ipRange, port, metadata)
	}

	// 启动异步扫描
	go s.performAsyncScan(scanTask, ips, sseService)

	return scanID, nil
}

// performAsyncScan 执行异步扫描
func (s *BoxDiscoveryService) performAsyncScan(scanTask *ScanTask, ips []string, sseService SSEService) {
	log.Printf("[BoxDiscoveryService] performAsyncScan started - ScanID: %s", scanTask.ScanID)

	var discoveredBoxes []*DiscoveredBox
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 限制并发数
	semaphore := make(chan struct{}, 50)
	progressUpdateTicker := time.NewTicker(2 * time.Second) // 每2秒更新一次进度
	defer progressUpdateTicker.Stop()

	// 启动进度更新协程
	go func() {
		for {
			select {
			case <-scanTask.Cancel:
				return
			case <-progressUpdateTicker.C:
				s.updateScanProgress(scanTask, sseService)
			}
		}
	}()

	startTime := time.Now()

	for i, ip := range ips {
		select {
		case <-scanTask.Cancel:
			log.Printf("[BoxDiscoveryService] Scan cancelled - ScanID: %s", scanTask.ScanID)
			s.finalizeScan(scanTask, discoveredBoxes, "cancelled", "", sseService)
			return
		default:
		}

		wg.Add(1)
		go func(ip string, index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 更新当前扫描的IP
			s.scanTasksMux.Lock()
			scanTask.CurrentIP = ip
			scanTask.ScannedIPs = index + 1
			scanTask.Progress = float64(index+1) / float64(scanTask.TotalIPs) * 100
			scanTask.UpdateTime = time.Now()
			s.scanTasksMux.Unlock()

			// 扫描单个IP
			if box := s.scanSingleIP(ip, scanTask.Port); box != nil {
				mu.Lock()
				discoveredBoxes = append(discoveredBoxes, box)
				s.scanTasksMux.Lock()
				scanTask.FoundBoxes = len(discoveredBoxes)
				s.scanTasksMux.Unlock()
				mu.Unlock()

				log.Printf("[BoxDiscoveryService] Box discovered - ScanID: %s, IP: %s, Version: %s",
					scanTask.ScanID, ip, box.Version)
			}
		}(ip, i)
	}

	wg.Wait()

	// 检查哪些盒子已存在于数据库
	log.Printf("[BoxDiscoveryService] Checking existing boxes - ScanID: %s, Found: %d", scanTask.ScanID, len(discoveredBoxes))
	if err := s.checkExistingBoxes(discoveredBoxes); err != nil {
		log.Printf("[BoxDiscoveryService] Failed to check existing boxes - ScanID: %s, Error: %v", scanTask.ScanID, err)
		s.finalizeScan(scanTask, discoveredBoxes, "failed", fmt.Sprintf("检查已存在盒子失败: %v", err), sseService)
		return
	}

	duration := time.Since(startTime)
	log.Printf("[BoxDiscoveryService] Scan completed successfully - ScanID: %s, Duration: %v, Found: %d",
		scanTask.ScanID, duration, len(discoveredBoxes))

	s.finalizeScan(scanTask, discoveredBoxes, "completed", "", sseService)
}

// finalizeScan 完成扫描
func (s *BoxDiscoveryService) finalizeScan(scanTask *ScanTask, discoveredBoxes []*DiscoveredBox, status, errorMessage string, sseService SSEService) {
	// 更新扫描任务状态
	s.scanTasksMux.Lock()
	scanTask.Status = status
	scanTask.EndTime = time.Now()
	scanTask.ErrorMessage = errorMessage
	if status == "completed" {
		scanTask.Progress = 100.0
	}
	s.scanTasksMux.Unlock()

	if sseService != nil {
		// 发送最终结果
		newBoxes := 0
		existingBoxes := 0
		for _, box := range discoveredBoxes {
			if box.IsNew {
				newBoxes++
			}
			if box.Exists {
				existingBoxes++
			}
		}

		result := &DiscoveryResult{
			ScanID:          scanTask.ScanID,
			IPRange:         scanTask.IPRange,
			Port:            scanTask.Port,
			Status:          status,
			TotalIPs:        scanTask.TotalIPs,
			ScannedIPs:      scanTask.ScannedIPs,
			FoundBoxes:      len(discoveredBoxes),
			NewBoxes:        newBoxes,
			ExistingBoxes:   existingBoxes,
			DiscoveredBoxes: s.convertDiscoveredBoxes(discoveredBoxes),
			StartTime:       scanTask.StartTime,
			EndTime:         scanTask.EndTime,
			Duration:        int(scanTask.EndTime.Sub(scanTask.StartTime).Seconds()),
			ErrorMessage:    errorMessage,
		}
		sseService.BroadcastDiscoveryResult(result)

		// 发送系统事件
		var title, message string

		switch status {
		case "completed":
			title = "网络扫描完成"
			message = fmt.Sprintf("扫描完成：共扫描 %d 个IP，发现 %d 个盒子（新盒子 %d 个，已存在 %d 个）",
				scanTask.TotalIPs, len(discoveredBoxes), newBoxes, existingBoxes)
		case "failed":
			title = "网络扫描失败"
			message = fmt.Sprintf("扫描失败：%s", errorMessage)
		default:
			title = "网络扫描已取消"
			message = "网络扫描被用户取消"
		}

		metadata := map[string]interface{}{
			"scan_id":        scanTask.ScanID,
			"ip_range":       scanTask.IPRange,
			"port":           scanTask.Port,
			"total_ips":      scanTask.TotalIPs,
			"scanned_ips":    scanTask.ScannedIPs,
			"found_boxes":    len(discoveredBoxes),
			"new_boxes":      newBoxes,
			"existing_boxes": existingBoxes,
			"duration":       int(scanTask.EndTime.Sub(scanTask.StartTime).Seconds()),
			"source":         "box_discovery_service",
			"source_id":      scanTask.ScanID,
			"title":          title,
			"message":        message,
		}

		switch status {
		case "completed":
			sseService.BroadcastDiscoveryCompleted(scanTask.ScanID, len(discoveredBoxes), newBoxes, metadata)
		case "failed":
			sseService.BroadcastDiscoveryFailed(scanTask.ScanID, errorMessage, metadata)
		default:
			sseService.BroadcastDiscoveryFailed(scanTask.ScanID, "网络扫描被用户取消", metadata)
		}
	}

	log.Printf("[BoxDiscoveryService] Scan finalized - ScanID: %s, Status: %s", scanTask.ScanID, status)
}

// updateScanProgress 更新扫描进度
func (s *BoxDiscoveryService) updateScanProgress(scanTask *ScanTask, sseService SSEService) {
	if sseService == nil {
		return
	}

	s.scanTasksMux.RLock()
	progress := &DiscoveryProgress{
		ScanID:        scanTask.ScanID,
		IPRange:       scanTask.IPRange,
		Port:          scanTask.Port,
		TotalIPs:      scanTask.TotalIPs,
		ScannedIPs:    scanTask.ScannedIPs,
		FoundBoxes:    scanTask.FoundBoxes,
		Status:        scanTask.Status,
		Progress:      scanTask.Progress,
		CurrentIP:     scanTask.CurrentIP,
		StartTime:     scanTask.StartTime,
		UpdateTime:    scanTask.UpdateTime,
		EstimatedTime: s.calculateEstimatedTime(scanTask),
		ErrorMessage:  scanTask.ErrorMessage,
	}
	s.scanTasksMux.RUnlock()

	sseService.BroadcastDiscoveryProgress(progress)
}

// calculateEstimatedTime 计算预计剩余时间
func (s *BoxDiscoveryService) calculateEstimatedTime(scanTask *ScanTask) int {
	if scanTask.ScannedIPs == 0 {
		return 0
	}

	elapsed := time.Since(scanTask.StartTime).Seconds()
	avgTimePerIP := elapsed / float64(scanTask.ScannedIPs)
	remainingIPs := scanTask.TotalIPs - scanTask.ScannedIPs
	estimatedSeconds := avgTimePerIP * float64(remainingIPs)

	return int(estimatedSeconds)
}

// convertDiscoveredBoxes 转换发现的盒子格式
func (s *BoxDiscoveryService) convertDiscoveredBoxes(boxes []*DiscoveredBox) []interface{} {
	result := make([]interface{}, len(boxes))
	for i, box := range boxes {
		// 构建盒子信息，确保包含所有必要的字段
		boxData := map[string]interface{}{
			"ip_address": box.IPAddress, // 前端添加盒子时需要的IP地址
			"port":       box.Port,      // 前端添加盒子时需要的端口
			"name":       box.Name,      // 盒子名称（可能为空，前端可以生成默认名称）
			"version":    box.Version,   // 盒子版本信息
			"status":     box.Status,    // 盒子状态（online等）
			"is_new":     box.IsNew,     // 是否是新发现的盒子（前端可据此决定是否显示"添加"按钮）
			"exists":     box.Exists,    // 是否已存在于数据库（前端可据此显示不同状态）
		}

		// 如果有详细的盒子信息，添加到结果中
		if box.BoxInfo != nil {
			boxData["box_info"] = map[string]interface{}{
				"service":      box.BoxInfo.Service,     // 服务名称
				"version":      box.BoxInfo.Version,     // 软件版本
				"build_time":   box.BoxInfo.BuildTime,   // 构建时间
				"api_version":  box.BoxInfo.APIVersion,  // API版本
				"timestamp":    box.BoxInfo.Timestamp,   // 时间戳
				"health_check": box.BoxInfo.HealthCheck, // 健康检查状态
			}
		}

		// 添加前端可能需要的额外信息
		boxData["endpoint"] = fmt.Sprintf("%s:%d", box.IPAddress, box.Port) // 完整的访问端点

		// 生成默认名称（如果名称为空）
		if box.Name == "" {
			boxData["suggested_name"] = fmt.Sprintf("Box-%s-%d",
				strings.ReplaceAll(box.IPAddress, ".", "-"), box.Port)
		}

		result[i] = boxData
	}
	return result
}

// GetScanTask 获取扫描任务
func (s *BoxDiscoveryService) GetScanTask(scanID string) (*ScanTask, error) {
	s.scanTasksMux.RLock()
	defer s.scanTasksMux.RUnlock()

	task, exists := s.scanTasks[scanID]
	if !exists {
		return nil, fmt.Errorf("扫描任务不存在: %s", scanID)
	}

	return task, nil
}

// CancelScanTask 取消扫描任务
func (s *BoxDiscoveryService) CancelScanTask(scanID string) error {
	s.scanTasksMux.Lock()
	defer s.scanTasksMux.Unlock()

	task, exists := s.scanTasks[scanID]
	if !exists {
		return fmt.Errorf("扫描任务不存在: %s", scanID)
	}

	if task.Status != "scanning" {
		return fmt.Errorf("扫描任务不能取消，当前状态: %s", task.Status)
	}

	// 发送取消信号
	select {
	case task.Cancel <- true:
		log.Printf("[BoxDiscoveryService] Scan task cancelled - ScanID: %s", scanID)
	default:
		// 信号已发送或通道已关闭
	}

	return nil
}

// CleanupScanTasks 清理旧的扫描任务
func (s *BoxDiscoveryService) CleanupScanTasks(olderThan time.Duration) int {
	s.scanTasksMux.Lock()
	defer s.scanTasksMux.Unlock()

	cutoff := time.Now().Add(-olderThan)
	cleaned := 0

	for scanID, task := range s.scanTasks {
		if task.EndTime.Before(cutoff) && task.Status != "scanning" {
			delete(s.scanTasks, scanID)
			cleaned++
		}
	}

	log.Printf("[BoxDiscoveryService] Cleaned up %d old scan tasks", cleaned)
	return cleaned
}

// generateScanID 生成扫描任务ID
func (s *BoxDiscoveryService) generateScanID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("scan_%s", hex.EncodeToString(bytes))
}
