/*
 * @module api/controllers/upgrade_package_controller
 * @description 升级包管理API控制器
 * @architecture API控制器层
 * @documentReference REQ-001: 盒子管理功能 - 升级文件管理
 * @stateFlow HTTP请求 -> 控制器 -> 服务层 -> Repository -> 数据库
 * @rules 提供升级包的上传、查询、下载、管理等功能
 * @dependencies go-chi/chi, go-chi/render
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// UpgradePackageController 升级包管理控制器
type UpgradePackageController struct {
	repoManager repository.RepositoryManager
	uploadDir   string // 文件上传目录
}

// NewUpgradePackageController 创建升级包控制器
func NewUpgradePackageController(repoManager repository.RepositoryManager, uploadDir string) *UpgradePackageController {
	// 确保上传目录存在
	if uploadDir == "" {
		uploadDir = "./uploads/packages"
	}
	os.MkdirAll(uploadDir, 0755)

	return &UpgradePackageController{
		repoManager: repoManager,
		uploadDir:   uploadDir,
	}
}

// UpgradePackageRequest 升级包创建请求
type UpgradePackageRequest struct {
	Name          string                    `json:"name" validate:"required"`
	Version       string                    `json:"version" validate:"required"`
	Description   string                    `json:"description"`
	Type          models.UpgradePackageType `json:"type" validate:"required"`
	MinBoxVersion string                    `json:"min_box_version"`
	MaxBoxVersion string                    `json:"max_box_version"`
	ReleaseNotes  string                    `json:"release_notes"`
	IsPrerelease  bool                      `json:"is_prerelease"`
}

// UpdateStatusRequest 状态更新请求
type UpdateStatusRequest struct {
	Status models.PackageStatus `json:"status" validate:"required"`
}

// UpgradePackageResponse 升级包响应
type UpgradePackageResponse struct {
	ID            uint                      `json:"id"`
	Name          string                    `json:"name"`
	Version       string                    `json:"version"`
	Description   string                    `json:"description"`
	Type          models.UpgradePackageType `json:"type"`
	Status        models.PackageStatus      `json:"status"`
	Files         []UpgradeFileResponse     `json:"files"`
	TotalSize     int64                     `json:"total_size"`
	Checksum      string                    `json:"checksum"`
	MinBoxVersion string                    `json:"min_box_version"`
	MaxBoxVersion string                    `json:"max_box_version"`
	ReleaseNotes  string                    `json:"release_notes"`
	ReleasedAt    *time.Time                `json:"released_at"`
	ReleasedBy    uint                      `json:"released_by"`
	DownloadCount int                       `json:"download_count"`
	UpgradeCount  int                       `json:"upgrade_count"`
	IsPrerelease  bool                      `json:"is_prerelease"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

// UpgradeFileResponse 升级文件响应
type UpgradeFileResponse struct {
	Type        models.FileType `json:"type"`
	Name        string          `json:"name"`
	Size        int64           `json:"size"`
	Checksum    string          `json:"checksum"`
	UploadedAt  time.Time       `json:"uploaded_at"`
	ContentType string          `json:"content_type"`
	DownloadURL string          `json:"download_url"`
}

// CreatePackage 创建升级包
// @Summary 创建升级包
// @Description 创建新的升级包
// @Tags 升级包管理
// @Accept json
// @Produce json
// @Param packageData body UpgradePackageRequest true "升级包信息"
// @Success 200 {object} APIResponse{data=UpgradePackageResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages [post]
func (c *UpgradePackageController) CreatePackage(w http.ResponseWriter, r *http.Request) {
	var req UpgradePackageRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, BadRequestResponse("无效的请求参数", err))
		return
	}

	ctx := r.Context()

	// 检查版本是否已存在
	exists, err := c.repoManager.UpgradePackage().VersionExists(ctx, req.Version)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("检查版本是否存在失败", err))
		return
	}
	if exists {
		render.Render(w, r, BadRequestResponse("版本已存在", nil))
		return
	}

	// 创建升级包
	pkg := &models.UpgradePackage{
		Name:          req.Name,
		Version:       req.Version,
		Description:   req.Description,
		Type:          req.Type,
		Status:        models.PackageStatusPending,
		MinBoxVersion: req.MinBoxVersion,
		MaxBoxVersion: req.MaxBoxVersion,
		ReleaseNotes:  req.ReleaseNotes,
		IsPrerelease:  req.IsPrerelease,
		Files:         make(models.UpgradeFileList, 0),
	}

	err = c.repoManager.UpgradePackage().Create(ctx, pkg)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("创建升级包失败", err))
		return
	}

	response := convertPackageToResponse(pkg)
	render.Render(w, r, SuccessResponse("创建升级包成功", response))
}

// UploadFile 上传文件到升级包
// @Summary 上传文件到升级包
// @Description 上传后台程序或前台界面文件到指定升级包
// @Tags 升级包管理
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "升级包ID"
// @Param file_type formData string true "文件类型" Enums(backend_program,frontend_ui)
// @Param file formData file true "文件"
// @Success 200 {object} APIResponse{data=UpgradePackageResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages/{id}/upload [post]
func (c *UpgradePackageController) UploadFile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的升级包ID", err))
		return
	}

	ctx := r.Context()

	// 获取升级包
	pkg, err := c.repoManager.UpgradePackage().GetByID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("升级包不存在", err))
		return
	}

	// 解析表单
	err = r.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		render.Render(w, r, BadRequestResponse("解析表单失败", err))
		return
	}

	// 获取文件类型
	fileType := models.FileType(r.FormValue("file_type"))
	if fileType != models.FileTypeBackendProgram && fileType != models.FileTypeFrontendUI {
		render.Render(w, r, BadRequestResponse("无效的文件类型", nil))
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		render.Render(w, r, BadRequestResponse("获取上传文件失败", err))
		return
	}
	defer file.Close()

	// 验证文件类型
	if err := c.validateFileType(header.Filename, fileType); err != nil {
		render.Render(w, r, BadRequestResponse("文件类型不匹配", err))
		return
	}

	// 保存文件
	savedPath, checksum, err := c.saveUploadedFile(file, pkg.Version, fileType, header.Filename)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("保存文件失败", err))
		return
	}

	// 创建文件记录
	upgradeFile := models.UpgradeFile{
		Type:        fileType,
		Name:        header.Filename,
		Path:        savedPath,
		Size:        header.Size,
		Checksum:    checksum,
		UploadedAt:  time.Now(),
		ContentType: header.Header.Get("Content-Type"),
	}

	// 添加文件到升级包
	pkg.AddFile(upgradeFile)

	// 更新升级包
	err = c.repoManager.UpgradePackage().Update(ctx, pkg)
	if err != nil {
		// 删除保存的文件
		os.Remove(savedPath)
		render.Render(w, r, InternalErrorResponse("更新升级包失败", err))
		return
	}

	response := convertPackageToResponse(pkg)
	render.Render(w, r, SuccessResponse("文件上传成功", response))
}

// GetPackages 获取升级包列表
// @Summary 获取升级包列表
// @Description 获取升级包列表，支持分页和筛选
// @Tags 升级包管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页大小" default(10)
// @Param type query string false "包类型"
// @Param status query string false "状态"
// @Success 200 {object} APIResponse{data=[]UpgradePackageResponse}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages [get]
func (c *UpgradePackageController) GetPackages(w http.ResponseWriter, r *http.Request) {
	// 解析分页参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 10
	}

	ctx := r.Context()
	conditions := make(map[string]interface{})

	// 解析筛选条件
	if packageType := r.URL.Query().Get("type"); packageType != "" {
		conditions["type"] = packageType
	}
	if status := r.URL.Query().Get("status"); status != "" {
		conditions["status"] = status
	}

	// 查询升级包
	packages, total, err := c.repoManager.UpgradePackage().FindWithPagination(ctx, conditions, page, size)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("查询升级包列表失败", err))
		return
	}

	// 转换响应
	var responses []UpgradePackageResponse
	for _, pkg := range packages {
		responses = append(responses, *convertPackageToResponse(pkg))
	}

	render.Render(w, r, SuccessResponse("获取升级包列表成功", map[string]interface{}{
		"data":  responses,
		"total": total,
		"page":  page,
		"size":  size,
	}))
}

// GetPackage 获取升级包详情
// @Summary 获取升级包详情
// @Description 根据ID获取升级包详情
// @Tags 升级包管理
// @Produce json
// @Param id path int true "升级包ID"
// @Success 200 {object} APIResponse{data=UpgradePackageResponse}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages/{id} [get]
func (c *UpgradePackageController) GetPackage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的升级包ID", err))
		return
	}

	ctx := r.Context()
	pkg, err := c.repoManager.UpgradePackage().GetByID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("升级包不存在", err))
		return
	}

	response := convertPackageToResponse(pkg)
	render.Render(w, r, SuccessResponse("获取升级包详情成功", response))
}

// DownloadFile 下载升级包文件
// @Summary 下载升级包文件
// @Description 下载指定升级包的指定类型文件
// @Tags 升级包管理
// @Produce application/octet-stream
// @Param id path int true "升级包ID"
// @Param file_type path string true "文件类型" Enums(backend_program,frontend_ui)
// @Success 200 {file} file
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages/{id}/download/{file_type} [get]
func (c *UpgradePackageController) DownloadFile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的升级包ID", err))
		return
	}

	fileType := models.FileType(chi.URLParam(r, "file_type"))

	ctx := r.Context()
	pkg, err := c.repoManager.UpgradePackage().GetByID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("升级包不存在", err))
		return
	}

	// 检查包是否可下载
	if !pkg.CanDownload() {
		render.Render(w, r, BadRequestResponse("升级包不可下载", nil))
		return
	}

	// 获取指定类型的文件
	file := pkg.GetFileByType(fileType)
	if file == nil {
		render.Render(w, r, NotFoundResponse("文件不存在", nil))
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(file.Path); os.IsNotExist(err) {
		render.Render(w, r, NotFoundResponse("文件不存在", nil))
		return
	}

	// 更新下载计数
	pkg.MarkAsDownloaded()
	c.repoManager.UpgradePackage().Update(ctx, pkg)

	// 设置响应头
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.Name))
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))

	// 发送文件
	http.ServeFile(w, r, file.Path)
}

// UpdatePackageStatus 更新升级包状态
// @Summary 更新升级包状态
// @Description 更新升级包状态（如设置为就绪、失败等）
// @Tags 升级包管理
// @Accept json
// @Produce json
// @Param id path int true "升级包ID"
// @Param status body object{status=string} true "状态更新"
// @Success 200 {object} APIResponse{data=UpgradePackageResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages/{id}/status [put]
func (c *UpgradePackageController) UpdatePackageStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的升级包ID", err))
		return
	}

	var req UpdateStatusRequest
	if err := render.Bind(r, &req); err != nil {
		render.Render(w, r, BadRequestResponse("无效的请求参数", err))
		return
	}

	ctx := r.Context()
	pkg, err := c.repoManager.UpgradePackage().GetByID(ctx, uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("升级包不存在", err))
		return
	}

	// 更新状态
	pkg.Status = req.Status
	if req.Status == models.PackageStatusReady {
		pkg.SetReady()
		// 验证包类型和文件的一致性
		if err := pkg.ValidatePackageType(); err != nil {
			render.Render(w, r, BadRequestResponse("包验证失败", err))
			return
		}
	} else if req.Status == models.PackageStatusFailed {
		pkg.SetFailed()
	}

	err = c.repoManager.UpgradePackage().Update(ctx, pkg)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("更新升级包状态失败", err))
		return
	}

	response := convertPackageToResponse(pkg)
	render.Render(w, r, SuccessResponse("更新升级包状态成功", response))
}

// GetStatistics 获取升级包统计信息
// @Summary 获取升级包统计信息
// @Description 获取升级包的统计信息
// @Tags 升级包管理
// @Produce json
// @Success 200 {object} APIResponse{data=object}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/upgrade-packages/statistics [get]
func (c *UpgradePackageController) GetStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := c.repoManager.UpgradePackage().GetStatistics(ctx)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取统计信息失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取统计信息成功", stats))
}

// 辅助方法

// validateFileType 验证文件类型
func (c *UpgradePackageController) validateFileType(filename string, fileType models.FileType) error {
	switch fileType {
	case models.FileTypeBackendProgram:
		if !strings.HasSuffix(strings.ToLower(filename), ".soc") {
			return fmt.Errorf("后台程序文件必须是.soc格式")
		}
	case models.FileTypeFrontendUI:
		if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
			return fmt.Errorf("前台界面文件必须是.zip格式")
		}
	default:
		return fmt.Errorf("不支持的文件类型")
	}
	return nil
}

// saveUploadedFile 保存上传的文件
func (c *UpgradePackageController) saveUploadedFile(src io.Reader, version string, fileType models.FileType, originalName string) (string, string, error) {
	// 创建版本目录
	versionDir := filepath.Join(c.uploadDir, version)
	err := os.MkdirAll(versionDir, 0755)
	if err != nil {
		return "", "", err
	}

	// 生成文件名
	var filename string
	switch fileType {
	case models.FileTypeBackendProgram:
		filename = "box-app.soc"
	case models.FileTypeFrontendUI:
		filename = "dist.zip"
	default:
		filename = originalName
	}

	filePath := filepath.Join(versionDir, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		return "", "", err
	}
	defer dst.Close()

	// 复制文件内容并计算MD5
	hash := md5.New()
	_, err = io.Copy(io.MultiWriter(dst, hash), src)
	if err != nil {
		os.Remove(filePath)
		return "", "", err
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	return filePath, checksum, nil
}

// convertPackageToResponse 转换升级包为响应格式
func convertPackageToResponse(pkg *models.UpgradePackage) *UpgradePackageResponse {
	response := &UpgradePackageResponse{
		ID:            pkg.ID,
		Name:          pkg.Name,
		Version:       pkg.Version,
		Description:   pkg.Description,
		Type:          pkg.Type,
		Status:        pkg.Status,
		TotalSize:     pkg.TotalSize,
		Checksum:      pkg.Checksum,
		MinBoxVersion: pkg.MinBoxVersion,
		MaxBoxVersion: pkg.MaxBoxVersion,
		ReleaseNotes:  pkg.ReleaseNotes,
		ReleasedAt:    pkg.ReleasedAt,
		ReleasedBy:    pkg.ReleasedBy,
		DownloadCount: pkg.DownloadCount,
		UpgradeCount:  pkg.UpgradeCount,
		IsPrerelease:  pkg.IsPrerelease,
		CreatedAt:     pkg.CreatedAt,
		UpdatedAt:     pkg.UpdatedAt,
	}

	// 转换文件列表
	for _, file := range pkg.Files {
		fileResponse := UpgradeFileResponse{
			Type:        file.Type,
			Name:        file.Name,
			Size:        file.Size,
			Checksum:    file.Checksum,
			UploadedAt:  file.UploadedAt,
			ContentType: file.ContentType,
			DownloadURL: fmt.Sprintf("/api/v1/upgrade-packages/%d/download/%s", pkg.ID, file.Type),
		}
		response.Files = append(response.Files, fileResponse)
	}

	return response
}

// Bind 实现render.Binder接口
func (r *UpgradePackageRequest) Bind(req *http.Request) error {
	return nil
}

// Bind 实现render.Binder接口
func (r *UpdateStatusRequest) Bind(req *http.Request) error {
	return nil
}
