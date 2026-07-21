package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"box-manage-service/client"
	"box-manage-service/config"
	"box-manage-service/models"

	"gorm.io/gorm"
)

// SmartVisionService 处理 SmartVision 内登、用户同步和模型同步。
type SmartVisionService struct {
	db                   *gorm.DB
	client               *client.SmartVisionClient
	cfg                  config.SmartVisionConfig
	modelStorageBasePath string
}

// SmartVisionSyncResult 同步结果。
type SmartVisionSyncResult struct {
	Total    int `json:"total"`
	Synced   int `json:"synced"`
	Skipped  int `json:"skipped"`
	Failed   int `json:"failed"`
	Duration int `json:"duration_ms"`
}

// SmartVisionInnerLoginResult 内登结果。
type SmartVisionInnerLoginResult struct {
	LocalToken      map[string]interface{}     `json:"local_token"`
	SmartVisionUser *client.SmartVisionUser    `json:"smartvision_user"`
	UserInfo        map[string]interface{}     `json:"user_info,omitempty"`
	Raw             map[string]json.RawMessage `json:"-"`
}

type smartVisionLocalFile struct {
	Path   string
	Size   int64
	MD5    string
	SHA256 string
}

var errLocalUserUnavailable = errors.New("local user unavailable")

// NewSmartVisionService 创建 SmartVision 服务。
func NewSmartVisionService(db *gorm.DB, smartClient *client.SmartVisionClient, cfg config.SmartVisionConfig, modelStorageBasePath string) *SmartVisionService {
	return &SmartVisionService{db: db, client: smartClient, cfg: cfg, modelStorageBasePath: modelStorageBasePath}
}

// InnerLogin 使用 SmartVision token 换取本地 token。
func (s *SmartVisionService) InnerLogin(ctx context.Context, smartVisionToken string) (*SmartVisionInnerLoginResult, error) {
	smartVisionToken = strings.TrimSpace(smartVisionToken)
	if smartVisionToken == "" {
		return nil, fmt.Errorf("SmartVision token 不能为空")
	}

	log.Printf("[SmartVision][Service] 开始内登: tokenLen=%d", len(smartVisionToken))
	user, err := s.client.ValidateToken(ctx, smartVisionToken)
	if err != nil {
		log.Printf("[SmartVision][Service] 内登 token 校验失败: err=%v", err)
		return nil, err
	}
	log.Printf("[SmartVision][Service] 内登 token 校验成功: username=%s smartvisionUserID=%s", user.Username, user.ID)

	plainPassword, userInfo, err := s.findLocalPlainPassword(ctx, user.Username)
	if errors.Is(err, errLocalUserUnavailable) {
		log.Printf("[SmartVision][Service] 本地用户不存在，尝试同步用户: username=%s", user.Username)
		if _, syncErr := s.SyncUsers(ctx); syncErr != nil {
			log.Printf("[SmartVision][Service] 内登触发用户同步失败: username=%s err=%v", user.Username, syncErr)
			return nil, fmt.Errorf("本地用户不存在，尝试同步 SmartVision 用户失败: %w", syncErr)
		}
		plainPassword, userInfo, err = s.findLocalPlainPassword(ctx, user.Username)
	}
	if err != nil {
		log.Printf("[SmartVision][Service] 查询本地用户失败: username=%s err=%v", user.Username, err)
		return nil, err
	}
	if plainPassword == "" {
		log.Printf("[SmartVision][Service] 本地用户缺少明文密码，内登终止: username=%s", user.Username)
		return nil, fmt.Errorf("用户 %s 未配置本地明文密码，无法完成内登", user.Username)
	}

	tokenData, err := s.getLocalToken(ctx, user.Username, plainPassword)
	if err != nil {
		log.Printf("[SmartVision][Service] 获取本地 token 失败: username=%s err=%v", user.Username, err)
		return nil, err
	}
	log.Printf("[SmartVision][Service] 内登成功: username=%s", user.Username)

	return &SmartVisionInnerLoginResult{
		LocalToken:      tokenData,
		SmartVisionUser: user,
		UserInfo:        userInfo,
	}, nil
}

// SyncUsers 同步 SmartVision 用户到 postgrest.users。
func (s *SmartVisionService) SyncUsers(ctx context.Context) (*SmartVisionSyncResult, error) {
	started := time.Now()
	log.Printf("[SmartVision][Service] 开始同步用户")
	users, err := s.client.ListAllUsers(ctx)
	if err != nil {
		log.Printf("[SmartVision][Service] 拉取用户列表失败: err=%v", err)
		return nil, err
	}
	if len(users) == 0 {
		log.Printf("[SmartVision][Service] SmartVision 用户列表为空，跳过同步")
		return nil, fmt.Errorf("SmartVision 用户列表为空，已跳过同步以避免误删本地同步账号")
	}

	result := &SmartVisionSyncResult{Total: len(users)}
	plainPassword := strings.TrimSpace(s.cfg.DefaultSyncedPassword)
	if plainPassword == "" {
		plainPassword = "tk@2026"
	}
	defaultRole := strings.TrimSpace(s.cfg.DefaultSyncedRole)
	if defaultRole == "" {
		defaultRole = "user"
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		log.Printf("[SmartVision][Service] 开始写入用户影子表: count=%d defaultRole=%s", len(users), defaultRole)
		if err := tx.Exec("TRUNCATE TABLE postgrest.users_shadow").Error; err != nil {
			log.Printf("[SmartVision][Service] 清空用户影子表失败: err=%v", err)
			return err
		}

		for _, user := range users {
			username := strings.TrimSpace(user.Username)
			if username == "" {
				result.Skipped++
				log.Printf("[SmartVision][Service] 跳过 SmartVision 用户: 原始记录缺少 username userID=%s", user.ID)
				continue
			}

			fullName := firstNonEmpty(user.RealName, username)
			displayName := fullName
			raw := rawJSON(user.Raw)
			active := user.Status == 0 || user.Status == 1

			if err := tx.Exec(`
INSERT INTO postgrest.users_shadow (
  username, password_hash, email, full_name, display_name, is_active,
  sync_time, is_synced_account, company, department, position, employee_no,
  phone, avatar, plain_password, smartvision_user_id, smartvision_raw,
  created_at, updated_at
) VALUES (
  ?, crypt(?, extensions.gen_salt('bf')), ?, ?, ?, ?, now(), true, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, now(), now()
) ON CONFLICT (username) DO UPDATE SET
  password_hash = EXCLUDED.password_hash,
  email = EXCLUDED.email,
  full_name = EXCLUDED.full_name,
  display_name = EXCLUDED.display_name,
  is_active = EXCLUDED.is_active,
  sync_time = EXCLUDED.sync_time,
  is_synced_account = true,
  company = EXCLUDED.company,
  department = EXCLUDED.department,
  position = EXCLUDED.position,
  employee_no = EXCLUDED.employee_no,
  phone = EXCLUDED.phone,
  avatar = EXCLUDED.avatar,
  plain_password = EXCLUDED.plain_password,
  smartvision_user_id = EXCLUDED.smartvision_user_id,
  smartvision_raw = EXCLUDED.smartvision_raw,
  updated_at = now()
`, username, plainPassword, user.Email, fullName, displayName, active,
				user.CompanyName, firstNonEmpty(user.OrgCodeText, user.DepartIDs, user.OrgCode), firstNonEmpty(user.PostText, user.Post), user.WorkNo,
				firstNonEmpty(user.Phone, user.Telephone), user.Avatar, plainPassword, user.ID, raw).Error; err != nil {
				result.Failed++
				log.Printf("[SmartVision][Service] 写入用户影子表失败: username=%s userID=%s err=%v", username, user.ID, err)
				return fmt.Errorf("写入 SmartVision 用户影子表失败(%s): %w", username, err)
			}
			result.Synced++
		}
		log.Printf("[SmartVision][Service] 用户影子表写入完成: synced=%d skipped=%d failed=%d", result.Synced, result.Skipped, result.Failed)

		if err := tx.Exec(`
DELETE FROM postgrest.users u
WHERE u.is_synced_account = true
  AND NOT EXISTS (SELECT 1 FROM postgrest.users_shadow s WHERE s.username = u.username)
`).Error; err != nil {
			log.Printf("[SmartVision][Service] 删除 SmartVision 已移除同步用户失败: err=%v", err)
			return err
		}
		log.Printf("[SmartVision][Service] 已删除 SmartVision 缺失的同步用户，准备 upsert users")

		if err := tx.Exec(`
INSERT INTO postgrest.users (
  username, password_hash, email, full_name, display_name, is_active,
  sync_time, is_synced_account, company, department, position, employee_no,
  phone, avatar, plain_password, smartvision_user_id, smartvision_raw,
  created_at, updated_at
)
SELECT
  username, password_hash, email, full_name, display_name, is_active,
  sync_time, true, company, department, position, employee_no,
  phone, avatar, plain_password, smartvision_user_id, smartvision_raw,
  created_at, updated_at
FROM postgrest.users_shadow
ON CONFLICT (username) DO UPDATE SET
  password_hash = EXCLUDED.password_hash,
  email = EXCLUDED.email,
  full_name = EXCLUDED.full_name,
  display_name = EXCLUDED.display_name,
  is_active = EXCLUDED.is_active,
  sync_time = EXCLUDED.sync_time,
  is_synced_account = true,
  company = EXCLUDED.company,
  department = EXCLUDED.department,
  position = EXCLUDED.position,
  employee_no = EXCLUDED.employee_no,
  phone = EXCLUDED.phone,
  avatar = EXCLUDED.avatar,
  plain_password = EXCLUDED.plain_password,
  smartvision_user_id = EXCLUDED.smartvision_user_id,
  smartvision_raw = EXCLUDED.smartvision_raw,
	  updated_at = now()
`).Error; err != nil {
			log.Printf("[SmartVision][Service] 影子表 upsert 到 users 失败: err=%v", err)
			return err
		}
		log.Printf("[SmartVision][Service] 影子表 upsert 到 users 完成")

		if err := tx.Exec(`
INSERT INTO postgrest.user_roles (username, role_name, assigned_by, is_active)
SELECT s.username, ?, 'smartvision-sync', true
FROM postgrest.users_shadow s
WHERE NOT EXISTS (
  SELECT 1 FROM postgrest.user_roles ur WHERE ur.username = s.username AND ur.is_active = true
)
ON CONFLICT DO NOTHING
`, defaultRole).Error; err != nil {
			log.Printf("[SmartVision][Service] 初始化同步用户角色失败: role=%s err=%v", defaultRole, err)
			return err
		}
		log.Printf("[SmartVision][Service] 同步用户角色初始化完成: role=%s", defaultRole)

		return nil
	})
	if err != nil {
		log.Printf("[SmartVision][Service] 用户同步失败: err=%v duration=%s", err, time.Since(started))
		return nil, err
	}

	result.Duration = int(time.Since(started).Milliseconds())
	log.Printf("[SmartVision][Service] 用户同步完成: total=%d synced=%d skipped=%d failed=%d duration=%dms", result.Total, result.Synced, result.Skipped, result.Failed, result.Duration)
	return result, nil
}

// SyncModels 同步 SmartVision 训练成功模型到 original_models。
func (s *SmartVisionService) SyncModels(ctx context.Context) (*SmartVisionSyncResult, error) {
	started := time.Now()
	log.Printf("[SmartVision][Service] 开始同步模型")
	remoteModels, err := s.client.SuccessModelList(ctx)
	if err != nil {
		log.Printf("[SmartVision][Service] 拉取成功模型列表失败: err=%v", err)
		return nil, err
	}

	result := &SmartVisionSyncResult{Total: len(remoteModels)}
	for index, remote := range remoteModels {
		if strings.TrimSpace(remote.ModelName) == "" && remote.ID == 0 && strings.TrimSpace(remote.ModelNumber) == "" {
			result.Skipped++
			log.Printf("[SmartVision][Service] 跳过空模型记录: index=%d", index)
			continue
		}
		log.Printf("[SmartVision][Service] 开始处理模型: index=%d id=%d modelNo=%s name=%s projectId=%s projectNo=%s projectName=%s", index, remote.ID, remote.ModelNumber, remote.ModelName, remote.ProjectID, remote.ProjectNumber, remote.ProjectName)

		model, err := s.buildOriginalModel(ctx, remote)
		if err != nil {
			result.Failed++
			log.Printf("[SmartVision][Service] 构建模型失败: index=%d id=%d modelNo=%s name=%s err=%v", index, remote.ID, remote.ModelNumber, remote.ModelName, err)
			return nil, err
		}
		if err := s.upsertOriginalModel(ctx, model); err != nil {
			result.Failed++
			log.Printf("[SmartVision][Service] 模型落库失败: index=%d id=%d modelNo=%s name=%s fileMD5=%s err=%v", index, remote.ID, remote.ModelNumber, remote.ModelName, model.FileMD5, err)
			return nil, err
		}
		result.Synced++
		log.Printf("[SmartVision][Service] 模型处理完成: index=%d id=%d modelNo=%s name=%s localModelID=%d file=%s size=%d md5=%s sha256=%s", index, remote.ID, remote.ModelNumber, remote.ModelName, model.ID, model.FilePath, model.FileSize, model.FileMD5, model.FileSHA256)
	}

	result.Duration = int(time.Since(started).Milliseconds())
	log.Printf("[SmartVision][Service] 模型同步完成: total=%d synced=%d skipped=%d failed=%d duration=%dms", result.Total, result.Synced, result.Skipped, result.Failed, result.Duration)
	return result, nil
}

// StartSchedulers 启动 SmartVision 定时同步。
func (s *SmartVisionService) StartSchedulers(ctx context.Context) {
	if s.cfg.SyncOnStart {
		go s.runOnce(ctx, "用户", s.SyncUsers)
		go s.runOnce(ctx, "模型", s.SyncModels)
	}
	if s.cfg.EnableUserSync && s.cfg.UserSyncInterval > 0 {
		go s.runPeriodic(ctx, "用户", s.cfg.UserSyncInterval, s.SyncUsers)
	}
	if s.cfg.EnableModelSync && s.cfg.ModelSyncInterval > 0 {
		go s.runPeriodic(ctx, "模型", s.cfg.ModelSyncInterval, s.SyncModels)
	}
}

func (s *SmartVisionService) findLocalPlainPassword(ctx context.Context, username string) (string, map[string]interface{}, error) {
	var row struct {
		Username      string
		PlainPassword string
		Email         string
		FullName      string
		DisplayName   string
		IsActive      bool
		Company       string
		Department    string
		Position      string
		EmployeeNo    string
		Phone         string
		Avatar        string
	}
	err := s.db.WithContext(ctx).Raw(`
SELECT username, COALESCE(plain_password, '') AS plain_password,
       COALESCE(email, '') AS email, COALESCE(full_name, '') AS full_name,
       COALESCE(display_name, '') AS display_name, is_active,
       COALESCE(company, '') AS company, COALESCE(department, '') AS department,
       COALESCE(position, '') AS position, COALESCE(employee_no, '') AS employee_no,
       COALESCE(phone, '') AS phone, COALESCE(avatar, '') AS avatar
FROM postgrest.users
WHERE username = ? AND is_active = true
`, username).Scan(&row).Error
	if err != nil {
		log.Printf("[SmartVision][Service] 查询本地用户密码失败: username=%s err=%v", username, err)
		return "", nil, err
	}
	if row.Username == "" {
		log.Printf("[SmartVision][Service] 本地用户不存在或未启用: username=%s", username)
		return "", nil, fmt.Errorf("%w: %s", errLocalUserUnavailable, username)
	}
	info := map[string]interface{}{
		"username":     row.Username,
		"email":        row.Email,
		"full_name":    row.FullName,
		"display_name": row.DisplayName,
		"is_active":    row.IsActive,
		"company":      row.Company,
		"department":   row.Department,
		"position":     row.Position,
		"employee_no":  row.EmployeeNo,
		"phone":        row.Phone,
		"avatar":       row.Avatar,
	}
	return row.PlainPassword, info, nil
}

func (s *SmartVisionService) getLocalToken(ctx context.Context, username, password string) (map[string]interface{}, error) {
	var raw string
	log.Printf("[SmartVision][Service] 开始调用本地 get_token: username=%s", username)
	if err := s.db.WithContext(ctx).Raw("SELECT postgrest.get_token(?, ?)::text", username, password).Scan(&raw).Error; err != nil {
		log.Printf("[SmartVision][Service] 调用本地 get_token SQL 失败: username=%s err=%v", username, err)
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		log.Printf("[SmartVision][Service] 本地 get_token 返回空: username=%s", username)
		return nil, fmt.Errorf("postgrest.get_token 未返回数据")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		log.Printf("[SmartVision][Service] 解析本地 get_token 返回失败: username=%s rawLen=%d err=%v", username, len(raw), err)
		return nil, fmt.Errorf("解析本地 token 返回失败: %w", err)
	}
	if success, ok := data["success"].(bool); ok && !success {
		log.Printf("[SmartVision][Service] 本地 get_token 返回失败: username=%s message=%v", username, data["message"])
		return nil, fmt.Errorf("本地登录失败: %v", data["message"])
	}
	log.Printf("[SmartVision][Service] 本地 get_token 成功: username=%s", username)
	return data, nil
}

func (s *SmartVisionService) buildOriginalModel(ctx context.Context, remote client.SmartVisionModel) (*models.OriginalModel, error) {
	now := time.Now()
	deploymentReq, err := s.smartVisionDeploymentRequest(ctx, remote)
	if err != nil {
		return nil, err
	}
	log.Printf("[SmartVision][Service] 模型部署请求已构建: id=%d name=%s modelNo=%s projectId=%s userId=%s clientIP=%s", remote.ID, remote.ModelName, deploymentReq.ModelNo, deploymentReq.ProjectID, deploymentReq.UserID, deploymentReq.ClientIP)
	deployment, err := s.client.DeployModel(ctx, deploymentReq)
	if err != nil {
		return nil, fmt.Errorf("部署 SmartVision 模型失败(id=%d number=%s name=%s): %w", remote.ID, remote.ModelNumber, remote.ModelName, err)
	}

	deployedModelPath := firstNonEmpty(deployment.ParamValue("modelPath"), remote.ONNXModelPath, remote.PTModelPath, remote.BINModelPath, remote.XMLModelPath, remote.ProjectPath)
	log.Printf("[SmartVision][Service] 模型部署完成: id=%d modelNo=%s deployedModelPath=%s detectURL=%s", remote.ID, remote.ModelNumber, truncateLogValue(deployedModelPath, 200), deployment.URL)
	downloader, expectedSize, downloadPath, err := s.client.DownloadDeployedModel(ctx, deploymentReq)
	if err != nil {
		return nil, fmt.Errorf("下载 SmartVision 已部署模型失败(id=%d number=%s name=%s): %w", remote.ID, remote.ModelNumber, remote.ModelName, err)
	}
	defer downloader.Close()
	modelPath := firstNonEmpty(downloadPath, deployedModelPath)
	if modelPath == "" {
		return nil, fmt.Errorf("SmartVision 模型部署后未返回可下载路径: id=%d number=%s name=%s", remote.ID, remote.ModelNumber, remote.ModelName)
	}
	log.Printf("[SmartVision][Service] 模型下载响应已获取: id=%d modelNo=%s downloadPath=%s expectedSize=%d", remote.ID, remote.ModelNumber, truncateLogValue(modelPath, 200), expectedSize)
	fileName := smartVisionFileName(firstNonEmpty(deployedModelPath, modelPath), firstNonEmpty(remote.ModelName, remote.ModelNumber, strconv.FormatInt(remote.ID, 10)))

	modelType := models.OriginalModelTypeONNX
	if remote.ONNXModelPath == "" && strings.TrimSpace(remote.PTModelPath) != "" {
		modelType = models.OriginalModelTypePT
	}

	name := firstNonEmpty(remote.ModelName, remote.ModelNumber, fmt.Sprintf("smartvision-%d", remote.ID))
	version := truncate(firstNonEmpty(remote.EndTime, remote.CreateTime, remote.ModelNumber, strconv.FormatInt(remote.ID, 10), "smartvision"), 50)
	inputWidth, inputHeight := parseImageSize(remote.ImageSize)
	if inputWidth == 0 {
		inputWidth = 640
	}
	if inputHeight == 0 {
		inputHeight = 640
	}
	raw := rawJSON(remote.Raw)
	localFile, err := s.saveModelFile(downloader, expectedSize, fileName, name, version)
	if err != nil {
		return nil, err
	}
	log.Printf("[SmartVision][Service] 模型文件保存完成: id=%d modelNo=%s fileName=%s path=%s size=%d md5=%s sha256=%s", remote.ID, remote.ModelNumber, fileName, localFile.Path, localFile.Size, localFile.MD5, localFile.SHA256)

	return &models.OriginalModel{
		Name:                   name,
		Description:            firstNonEmpty(remote.Message, remote.ProjectName, "SmartVision 同步模型"),
		Version:                version,
		FileName:               fileName,
		FilePath:               localFile.Path,
		FileSize:               localFile.Size,
		FileMD5:                localFile.MD5,
		FileSHA256:             localFile.SHA256,
		ModelType:              modelType,
		Framework:              "smartvision",
		ModelFormat:            string(modelType),
		TaskType:               models.ModelTaskTypeDetection,
		InputWidth:             inputWidth,
		InputHeight:            inputHeight,
		InputChannels:          3,
		Status:                 models.OriginalModelStatusReady,
		UploadProgress:         100,
		IsValidated:            true,
		ValidationMsg:          "SmartVision 同步模型",
		StorageClass:           "hot",
		Tags:                   "smartvision,synced",
		Author:                 "SmartVision",
		ModelURL:               modelPath,
		UserID:                 1,
		IsSynced:               true,
		ModelSyncTime:          &now,
		SmartVisionModelID:     strconv.FormatInt(remote.ID, 10),
		SmartVisionModelNumber: remote.ModelNumber,
		SmartVisionProjectName: remote.ProjectName,
		SmartVisionProjectNo:   remote.ProjectNumber,
		SmartVisionRaw:         raw,
		LastAccessed:           now,
	}, nil
}

func (s *SmartVisionService) upsertOriginalModel(ctx context.Context, model *models.OriginalModel) error {
	var existing models.OriginalModel
	log.Printf("[SmartVision][Service] 准备 upsert 原始模型: name=%s fileMD5=%s fileSHA256=%s path=%s", model.Name, model.FileMD5, model.FileSHA256, model.FilePath)
	err := s.db.WithContext(ctx).Where("file_md5 = ?", model.FileMD5).First(&existing).Error
	if err == nil {
		log.Printf("[SmartVision][Service] 发现相同 MD5 原始模型，准备更新: existingID=%d existingPath=%s newPath=%s", existing.ID, existing.FilePath, model.FilePath)
		if existing.FilePath != "" && existing.FilePath != model.FilePath {
			if _, statErr := os.Stat(existing.FilePath); statErr == nil {
				log.Printf("[SmartVision][Service] 复用已有模型文件并删除新下载重复文件: existingID=%d existingPath=%s newPath=%s", existing.ID, existing.FilePath, model.FilePath)
				_ = os.Remove(model.FilePath)
				model.FilePath = existing.FilePath
			} else {
				log.Printf("[SmartVision][Service] 已有模型文件不存在，保留新下载文件: existingID=%d existingPath=%s statErr=%v", existing.ID, existing.FilePath, statErr)
			}
		}
		model.ID = existing.ID
		model.CreatedAt = existing.CreatedAt
		if err := s.db.WithContext(ctx).Save(model).Error; err != nil {
			log.Printf("[SmartVision][Service] 更新原始模型失败: id=%d name=%s err=%v", model.ID, model.Name, err)
			return err
		}
		log.Printf("[SmartVision][Service] 更新原始模型成功: id=%d name=%s", model.ID, model.Name)
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("[SmartVision][Service] 查询原始模型失败: fileMD5=%s err=%v", model.FileMD5, err)
		return err
	}
	if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
		log.Printf("[SmartVision][Service] 新增原始模型失败: name=%s fileMD5=%s err=%v", model.Name, model.FileMD5, err)
		return err
	}
	log.Printf("[SmartVision][Service] 新增原始模型成功: id=%d name=%s", model.ID, model.Name)
	return nil
}

func (s *SmartVisionService) smartVisionDeploymentRequest(ctx context.Context, remote client.SmartVisionModel) (client.SmartVisionModelDeploymentRequest, error) {
	modelNo := firstNonEmpty(remote.ModelNumber, strconv.FormatInt(remote.ID, 10))
	projectID := smartVisionProjectID(remote)
	if modelNo == "" || modelNo == "0" {
		return client.SmartVisionModelDeploymentRequest{}, fmt.Errorf("SmartVision 模型缺少 modelNo: id=%d name=%s", remote.ID, remote.ModelName)
	}
	if projectID == "" {
		return client.SmartVisionModelDeploymentRequest{}, fmt.Errorf("SmartVision 模型缺少 projectId/projectNumber: id=%d number=%s name=%s", remote.ID, remote.ModelNumber, remote.ModelName)
	}
	if strings.TrimSpace(remote.ProjectID) == "" && strings.TrimSpace(remote.ProjectNumber) != "" {
		log.Printf("[SmartVision][Service] 模型列表未返回 projectId，使用 projectNumber 作为部署 projectId: id=%d modelNo=%s projectNumber=%s name=%s", remote.ID, remote.ModelNumber, remote.ProjectNumber, remote.ModelName)
	}
	userID, err := s.client.APIUserID(ctx)
	if err != nil {
		return client.SmartVisionModelDeploymentRequest{}, fmt.Errorf("获取 SmartVision 登录 userId 失败: %w", err)
	}
	if strings.TrimSpace(userID) == "" {
		return client.SmartVisionModelDeploymentRequest{}, fmt.Errorf("SmartVision 第三方登录未返回 userId")
	}
	return client.SmartVisionModelDeploymentRequest{
		ClientIP:  strings.TrimSpace(s.cfg.DeployClientIP),
		ModelNo:   modelNo,
		ProjectID: projectID,
		UserID:    strings.TrimSpace(userID),
	}, nil
}

func (s *SmartVisionService) saveModelFile(body io.Reader, expectedSize int64, fileName, modelName, version string) (*smartVisionLocalFile, error) {
	storagePath := s.smartVisionStoragePath(fileName, modelName, version)
	started := time.Now()
	log.Printf("[SmartVision][Service] 开始保存模型文件: fileName=%s modelName=%s version=%s expectedSize=%d path=%s", fileName, modelName, version, expectedSize, storagePath)
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		log.Printf("[SmartVision][Service] 创建模型存储目录失败: path=%s err=%v", filepath.Dir(storagePath), err)
		return nil, fmt.Errorf("创建 SmartVision 模型存储目录失败: %w", err)
	}

	destFile, err := os.Create(storagePath)
	if err != nil {
		log.Printf("[SmartVision][Service] 创建模型文件失败: path=%s err=%v", storagePath, err)
		return nil, fmt.Errorf("创建 SmartVision 模型文件失败: %w", err)
	}

	md5Hash := md5.New()
	sha256Hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(destFile, md5Hash, sha256Hash), body)
	closeErr := destFile.Close()
	if copyErr != nil {
		_ = os.Remove(storagePath)
		log.Printf("[SmartVision][Service] 保存模型文件失败，已删除临时文件: path=%s written=%d duration=%s err=%v", storagePath, written, time.Since(started), copyErr)
		return nil, fmt.Errorf("保存 SmartVision 模型文件失败: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(storagePath)
		log.Printf("[SmartVision][Service] 关闭模型文件失败，已删除临时文件: path=%s written=%d duration=%s err=%v", storagePath, written, time.Since(started), closeErr)
		return nil, fmt.Errorf("关闭 SmartVision 模型文件失败: %w", closeErr)
	}
	if expectedSize >= 0 && expectedSize != written {
		_ = os.Remove(storagePath)
		log.Printf("[SmartVision][Service] 模型文件大小不一致，已删除文件: path=%s expected=%d actual=%d duration=%s", storagePath, expectedSize, written, time.Since(started))
		return nil, fmt.Errorf("SmartVision 模型文件大小不一致: expected=%d actual=%d", expectedSize, written)
	}

	localFile := &smartVisionLocalFile{
		Path:   storagePath,
		Size:   written,
		MD5:    hex.EncodeToString(md5Hash.Sum(nil)),
		SHA256: hex.EncodeToString(sha256Hash.Sum(nil)),
	}
	log.Printf("[SmartVision][Service] 模型文件保存成功: path=%s size=%d md5=%s sha256=%s duration=%s", localFile.Path, localFile.Size, localFile.MD5, localFile.SHA256, time.Since(started))
	return localFile, nil
}

func (s *SmartVisionService) smartVisionStoragePath(fileName, modelName, version string) string {
	basePath := strings.TrimSpace(s.modelStorageBasePath)
	if basePath == "" {
		basePath = "./data/models"
	}
	ext := filepath.Ext(fileName)
	baseName := strings.TrimSuffix(fileName, ext)
	if baseName == "" {
		baseName = firstNonEmpty(modelName, "smartvision-model")
	}
	uniqueFileName := fmt.Sprintf("%s_%s_%s%s",
		sanitizeSmartVisionFileName(firstNonEmpty(modelName, baseName)),
		sanitizeSmartVisionFileName(firstNonEmpty(version, "smartvision")),
		truncate(fmt.Sprintf("%x", md5.Sum([]byte(fileName+time.Now().String()))), 8),
		ext,
	)
	return filepath.Join(basePath, "models", uniqueFileName)
}

func (s *SmartVisionService) runPeriodic(ctx context.Context, name string, interval time.Duration, fn func(context.Context) (*SmartVisionSyncResult, error)) {
	log.Printf("[SmartVision] 启动%s定时同步，间隔: %s", name, interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[SmartVision] 停止%s定时同步", name)
			return
		case <-ticker.C:
			s.runOnce(ctx, name, fn)
		}
	}
}

func (s *SmartVisionService) runOnce(ctx context.Context, name string, fn func(context.Context) (*SmartVisionSyncResult, error)) {
	result, err := fn(ctx)
	if err != nil {
		log.Printf("[SmartVision] %s同步失败: %v", name, err)
		return
	}
	log.Printf("[SmartVision] %s同步完成: total=%d synced=%d skipped=%d failed=%d duration=%dms", name, result.Total, result.Synced, result.Skipped, result.Failed, result.Duration)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func rawJSON(raw json.RawMessage) string {
	if len(raw) == 0 || !json.Valid(raw) {
		return "{}"
	}
	return string(raw)
}

func smartVisionProjectID(remote client.SmartVisionModel) string {
	if projectID := strings.TrimSpace(remote.ProjectID); projectID != "" {
		return projectID
	}

	var raw map[string]json.RawMessage
	if len(remote.Raw) > 0 && json.Unmarshal(remote.Raw, &raw) == nil {
		for _, key := range []string{"projectId", "projectID", "project_id"} {
			if projectID := jsonStringValue(raw[key]); projectID != "" {
				return projectID
			}
		}
	}

	return strings.TrimSpace(remote.ProjectNumber)
}

func jsonStringValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return strings.TrimSpace(number.String())
	}
	return ""
}

func parseImageSize(imageSize string) (int, int) {
	imageSize = strings.TrimSpace(strings.ToLower(imageSize))
	imageSize = strings.ReplaceAll(imageSize, "*", "x")
	parts := strings.Split(imageSize, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	width, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return width, height
}

func sanitizeSmartVisionFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "smartvision-model"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return replacer.Replace(name)
}

func smartVisionFileName(modelPath, fallbackName string) string {
	parsed, err := url.Parse(modelPath)
	pathValue := modelPath
	if err == nil && parsed.Path != "" {
		pathValue = parsed.Path
	}
	fileName := filepath.Base(pathValue)
	if fileName == "." || fileName == string(filepath.Separator) || fileName == "" {
		fileName = sanitizeSmartVisionFileName(fallbackName) + ".onnx"
	}
	return fileName
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func truncateLogValue(value string, max int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
