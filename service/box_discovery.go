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
	repoManager repository.RepositoryManager
	httpClient  *http.Client
}

// NewBoxDiscoveryService 创建盒子发现服务
func NewBoxDiscoveryService(repoManager repository.RepositoryManager) *BoxDiscoveryService {
	return &BoxDiscoveryService{
		repoManager: repoManager,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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

	ips, err := s.parseIPRange(ipRange)
	if err != nil {
		log.Printf("[BoxDiscoveryService] Failed to parse IP range - IPRange: %s, Error: %v", ipRange, err)
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
		return nil, fmt.Errorf("检查已存在盒子失败: %w", err)
	}

	log.Printf("[BoxDiscoveryService] Network scan completed - Total discovered: %d", len(discoveredBoxes))
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
		return nil, fmt.Errorf("保存盒子到数据库失败: %w", err)
	}

	log.Printf("[BoxDiscoveryService] Box registered successfully - ID: %d, Name: %s, IP: %s:%d",
		box.ID, box.Name, box.IPAddress, box.Port)
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
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 2*time.Second)
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
