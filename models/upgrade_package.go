/*
 * @module models/upgrade_package
 * @description 升级包管理数据模型定义
 * @architecture 数据模型层
 * @documentReference REQ-001: 盒子管理功能 - 升级文件管理
 * @stateFlow 文件上传 -> 验证 -> 存储 -> 可用于升级
 * @rules 支持后台程序(box-app.soc)和前台界面(dist.zip)两种文件类型
 * @dependencies gorm.io/gorm
 */

package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// UpgradePackage 升级包模型
type UpgradePackage struct {
	BaseModel
	Name        string             `json:"name" gorm:"not null;uniqueIndex"`         // 升级包名称
	Version     string             `json:"version" gorm:"not null;index"`            // 版本号
	Description string             `json:"description" gorm:"type:text"`             // 描述
	Type        UpgradePackageType `json:"type" gorm:"not null"`                     // 升级包类型
	Status      PackageStatus      `json:"status" gorm:"not null;default:'pending'"` // 状态

	// 文件信息
	Files     UpgradeFileList `json:"files" gorm:"type:jsonb;not null"` // 文件列表
	TotalSize int64           `json:"total_size" gorm:"not null"`       // 总大小(bytes)
	Checksum  string          `json:"checksum" gorm:"not null"`         // 整包校验和

	// 兼容性
	MinBoxVersion string `json:"min_box_version"` // 最小支持的盒子版本
	MaxBoxVersion string `json:"max_box_version"` // 最大支持的盒子版本

	// 发布信息
	ReleaseNotes string     `json:"release_notes" gorm:"type:text"` // 发布说明
	ReleasedAt   *time.Time `json:"released_at"`                    // 发布时间
	ReleasedBy   uint       `json:"released_by" gorm:"index"`       // 发布者

	// 统计信息
	DownloadCount int `json:"download_count" gorm:"default:0"` // 下载次数
	UpgradeCount  int `json:"upgrade_count" gorm:"default:0"`  // 升级次数

	// 是否为预发布版本
	IsPrerelease bool `json:"is_prerelease" gorm:"default:false"`
}

// UpgradePackageType 升级包类型枚举
type UpgradePackageType string

const (
	PackageTypeBackend  UpgradePackageType = "backend"  // 仅后台程序
	PackageTypeFrontend UpgradePackageType = "frontend" // 仅前台界面
)

// PackageStatus 包状态枚举
type PackageStatus string

const (
	PackageStatusPending   PackageStatus = "pending"   // 待处理
	PackageStatusVerifying PackageStatus = "verifying" // 验证中
	PackageStatusReady     PackageStatus = "ready"     // 就绪
	PackageStatusFailed    PackageStatus = "failed"    // 验证失败
	PackageStatusArchived  PackageStatus = "archived"  // 已归档
)

// UpgradeFile 升级文件信息
type UpgradeFile struct {
	Type        FileType  `json:"type"`                   // 文件类型
	Name        string    `json:"name"`                   // 文件名
	Path        string    `json:"path"`                   // 存储路径
	Size        int64     `json:"size"`                   // 文件大小
	Checksum    string    `json:"checksum"`               // 文件校验和
	UploadedAt  time.Time `json:"uploaded_at"`            // 上传时间
	ContentType string    `json:"content_type,omitempty"` // MIME类型
}

// FileType 文件类型枚举
type FileType string

const (
	FileTypeBackendProgram FileType = "backend_program" // 后台程序文件 (box-app.soc)
	FileTypeFrontendUI     FileType = "frontend_ui"     // 前台界面文件 (dist.zip)
)

// UpgradeFileList 升级文件列表类型
type UpgradeFileList []UpgradeFile

// Value 实现 driver.Valuer 接口
func (u UpgradeFileList) Value() (driver.Value, error) {
	return json.Marshal(u)
}

// Scan 实现 sql.Scanner 接口
func (u *UpgradeFileList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, u)
}

// TableName 设置表名
func (UpgradePackage) TableName() string {
	return "upgrade_packages"
}

// GetFileByType 根据类型获取文件
func (u *UpgradePackage) GetFileByType(fileType FileType) *UpgradeFile {
	for _, file := range u.Files {
		if file.Type == fileType {
			return &file
		}
	}
	return nil
}

// HasBackendFile 检查是否包含后台程序文件
func (u *UpgradePackage) HasBackendFile() bool {
	return u.GetFileByType(FileTypeBackendProgram) != nil
}

// HasFrontendFile 检查是否包含前台界面文件
func (u *UpgradePackage) HasFrontendFile() bool {
	return u.GetFileByType(FileTypeFrontendUI) != nil
}

// IsReady 检查是否就绪可用
func (u *UpgradePackage) IsReady() bool {
	return u.Status == PackageStatusReady
}

// CanDownload 检查是否可以下载
func (u *UpgradePackage) CanDownload() bool {
	return u.Status == PackageStatusReady && !u.IsDeleted()
}

// AddFile 添加文件
func (u *UpgradePackage) AddFile(file UpgradeFile) {
	// 检查是否已存在同类型文件，如果存在则替换
	for i, existingFile := range u.Files {
		if existingFile.Type == file.Type {
			u.Files[i] = file
			u.calculateTotalSize()
			return
		}
	}

	// 不存在则添加
	u.Files = append(u.Files, file)
	u.calculateTotalSize()
}

// RemoveFileByType 根据类型移除文件
func (u *UpgradePackage) RemoveFileByType(fileType FileType) {
	for i, file := range u.Files {
		if file.Type == fileType {
			u.Files = append(u.Files[:i], u.Files[i+1:]...)
			u.calculateTotalSize()
			return
		}
	}
}

// calculateTotalSize 计算总大小
func (u *UpgradePackage) calculateTotalSize() {
	u.TotalSize = 0
	for _, file := range u.Files {
		u.TotalSize += file.Size
	}
}

// SetReady 设置为就绪状态
func (u *UpgradePackage) SetReady() {
	u.Status = PackageStatusReady
	if u.ReleasedAt == nil {
		now := time.Now()
		u.ReleasedAt = &now
	}
}

// SetFailed 设置为失败状态
func (u *UpgradePackage) SetFailed() {
	u.Status = PackageStatusFailed
}

// MarkAsDownloaded 标记已下载
func (u *UpgradePackage) MarkAsDownloaded() {
	u.DownloadCount++
}

// MarkAsUpgraded 标记已用于升级
func (u *UpgradePackage) MarkAsUpgraded() {
	u.UpgradeCount++
}

// GetDisplayName 获取显示名称
func (u *UpgradePackage) GetDisplayName() string {
	if u.Name != "" {
		return u.Name + " v" + u.Version
	}
	return u.Version
}

// ValidatePackageType 验证包类型和文件的一致性
func (u *UpgradePackage) ValidatePackageType() error {
	hasBackend := u.HasBackendFile()
	hasFrontend := u.HasFrontendFile()

	switch u.Type {
	case PackageTypeBackend:
		if !hasBackend {
			return errors.New("后台包必须包含后台程序文件")
		}
		if hasFrontend {
			return errors.New("后台包不应包含前台界面文件")
		}
	case PackageTypeFrontend:
		if !hasFrontend {
			return errors.New("前台包必须包含前台界面文件")
		}
		if hasBackend {
			return errors.New("前台包不应包含后台程序文件")
		}
	default:
		return errors.New("无效的包类型")
	}

	return nil
}
