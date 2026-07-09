package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"box-manage-service/config"
)

const smartVisionTokenHeader = "X-Access-Token"

var errSmartVisionUnauthorized = errors.New("smartvision unauthorized")

// SmartVisionClient SmartVision 平台 HTTP 客户端。
type SmartVisionClient struct {
	baseURL    string
	username   string
	password   string
	clientName string
	clientID   string
	httpClient *http.Client

	mu    sync.Mutex
	token string
}

// SmartVisionAPIResponse SmartVision 通用响应结构。
type SmartVisionAPIResponse[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Result    T      `json:"result"`
	Success   bool   `json:"success"`
	Timestamp int64  `json:"timestamp"`
}

// SmartVisionPage SmartVision 分页响应结构。
type SmartVisionPage[T any] struct {
	Current int64 `json:"current"`
	Pages   int64 `json:"pages"`
	Records []T   `json:"records"`
	Size    int64 `json:"size"`
	Total   int64 `json:"total"`
}

// SmartVisionUser SmartVision 用户字段。
type SmartVisionUser struct {
	ID          string          `json:"id"`
	Username    string          `json:"username"`
	RealName    string          `json:"realname"`
	Email       string          `json:"email"`
	Avatar      string          `json:"avatar"`
	Phone       string          `json:"phone"`
	Telephone   string          `json:"telephone"`
	CompanyCode string          `json:"companycode"`
	CompanyName string          `json:"companyname"`
	DepartIDs   string          `json:"departIds"`
	OrgCode     string          `json:"orgCode"`
	OrgCodeText string          `json:"orgCodeTxt"`
	Post        string          `json:"post"`
	PostText    string          `json:"postText"`
	WorkNo      string          `json:"workNo"`
	Status      int             `json:"status"`
	Raw         json.RawMessage `json:"-"`
}

// SmartVisionModel SmartVision 成功模型字段。
type SmartVisionModel struct {
	ID              int64           `json:"id"`
	ModelName       string          `json:"modelName"`
	ModelNumber     string          `json:"modelNumber"`
	ModelType       string          `json:"modelType"`
	YoloModelType   string          `json:"yoloModelType"`
	ModelStatus     string          `json:"modelStatus"`
	ProjectName     string          `json:"projectName"`
	ProjectNumber   string          `json:"projectNumber"`
	ProjectPath     string          `json:"projectPath"`
	PTModelPath     string          `json:"ptModelPath"`
	ONNXModelPath   string          `json:"onnxModelPath"`
	BINModelPath    string          `json:"binModelPath"`
	XMLModelPath    string          `json:"xmlModelPath"`
	ImageSize       string          `json:"imageSize"`
	ClassNumber     string          `json:"clsNum"`
	PictureCount    string          `json:"pictureCount"`
	ModelScore      string          `json:"modelScore"`
	ModelFalseRate  string          `json:"modelFalserate"`
	ModelMissedRate string          `json:"modelMissedrate"`
	Message         string          `json:"message"`
	CreateTime      string          `json:"createTime"`
	EndTime         string          `json:"endTime"`
	ReserveField1   string          `json:"reservefield1"`
	ReserveField2   string          `json:"reservefield2"`
	ReserveField3   string          `json:"reservefield3"`
	ReserveField4   string          `json:"reservefield4"`
	ReserveField5   string          `json:"reservefield5"`
	ReserveField6   string          `json:"reservefield6"`
	ReserveField7   string          `json:"reservefield7"`
	ReserveField8   string          `json:"reservefield8"`
	ReserveField9   string          `json:"reservefield9"`
	ReserveField10  string          `json:"reservefield10"`
	Raw             json.RawMessage `json:"-"`
}

// NewSmartVisionClient 创建 SmartVision 客户端。
func NewSmartVisionClient(cfg config.SmartVisionConfig) *SmartVisionClient {
	return &SmartVisionClient{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		username:   cfg.Username,
		password:   cfg.Password,
		clientName: cfg.ClientName,
		clientID:   cfg.ClientID,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// LoginThird 登录 SmartVision 并缓存 API token。
func (c *SmartVisionClient) LoginThird(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	token, err := c.loginThirdLocked(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	return token, nil
}

// ValidateToken 验证前端传入的 SmartVision token 并返回用户信息。
func (c *SmartVisionClient) ValidateToken(ctx context.Context, token string) (*SmartVisionUser, error) {
	query := url.Values{}
	query.Set("token", token)

	var resp SmartVisionAPIResponse[json.RawMessage]
	status, err := c.do(ctx, http.MethodGet, "/sys/user/getUserSectionInfoByToken", token, query, nil, &resp)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || resp.Code == http.StatusUnauthorized || !resp.Success {
		return nil, fmt.Errorf("SmartVision token 校验失败: %s", resp.Message)
	}

	user, err := decodeSmartVisionUser(resp.Result)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.Username) == "" {
		return nil, fmt.Errorf("SmartVision token 校验成功但未返回 username")
	}
	return user, nil
}

// ListAllUsers 查询 SmartVision 所有用户。
func (c *SmartVisionClient) ListAllUsers(ctx context.Context) ([]SmartVisionUser, error) {
	var resp SmartVisionAPIResponse[SmartVisionPage[json.RawMessage]]
	if err := c.doWithAPITokenRetry(ctx, http.MethodGet, "/sys/user/listAll", nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Success && resp.Code != 0 && resp.Code != http.StatusOK {
		return nil, fmt.Errorf("SmartVision 用户同步失败: %s", resp.Message)
	}

	users := make([]SmartVisionUser, 0, len(resp.Result.Records))
	for _, raw := range resp.Result.Records {
		user, err := decodeSmartVisionUser(raw)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(user.Username) != "" {
			users = append(users, *user)
		}
	}
	return users, nil
}

// SuccessModelList 查询 SmartVision 训练成功的模型。
func (c *SmartVisionClient) SuccessModelList(ctx context.Context) ([]SmartVisionModel, error) {
	var resp SmartVisionAPIResponse[SmartVisionPage[json.RawMessage]]
	if err := c.doWithAPITokenRetry(ctx, http.MethodGet, "/project/modelInfo/successModelList", nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Success && resp.Code != 0 && resp.Code != http.StatusOK {
		return nil, fmt.Errorf("SmartVision 模型同步失败: %s", resp.Message)
	}

	models := make([]SmartVisionModel, 0, len(resp.Result.Records))
	for _, raw := range resp.Result.Records {
		var model SmartVisionModel
		if err := json.Unmarshal(raw, &model); err != nil {
			return nil, fmt.Errorf("解析 SmartVision 模型失败: %w", err)
		}
		model.Raw = append(json.RawMessage(nil), raw...)
		models = append(models, model)
	}
	return models, nil
}

// DownloadFile 按 SmartVision 返回的文件路径下载文件。
func (c *SmartVisionClient) DownloadFile(ctx context.Context, filePath string) (io.ReadCloser, int64, error) {
	token, err := c.apiToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	body, size, err := c.downloadFile(ctx, filePath, token)
	if errors.Is(err, errSmartVisionUnauthorized) {
		token, err = c.LoginThird(ctx)
		if err != nil {
			return nil, 0, err
		}
		body, size, err = c.downloadFile(ctx, filePath, token)
	}
	return body, size, err
}

func (c *SmartVisionClient) doWithAPITokenRetry(ctx context.Context, method, path string, body any, out any) error {
	token, err := c.apiToken(ctx)
	if err != nil {
		return err
	}

	status, err := c.do(ctx, method, path, token, nil, body, out)
	if err != nil {
		return err
	}
	if status != http.StatusUnauthorized {
		return nil
	}

	token, err = c.LoginThird(ctx)
	if err != nil {
		return err
	}
	status, err = c.do(ctx, method, path, token, nil, body, out)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		return fmt.Errorf("SmartVision API token 已失效且重新登录后仍返回 401")
	}
	return nil
}

func (c *SmartVisionClient) apiToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token != "" {
		return token, nil
	}
	return c.LoginThird(ctx)
}

func (c *SmartVisionClient) loginThirdLocked(ctx context.Context) (string, error) {
	payload := map[string]string{
		"captcha":    "",
		"checkKey":   "",
		"clientId":   c.clientID,
		"clientName": c.clientName,
		"password":   c.password,
		"username":   c.username,
	}

	var resp SmartVisionAPIResponse[struct {
		Token string `json:"token"`
	}]
	status, err := c.do(ctx, http.MethodPost, "/sys/thirdLogin/loginThird", "", nil, payload, &resp)
	if err != nil {
		return "", err
	}
	if status == http.StatusUnauthorized || (!resp.Success && resp.Code != http.StatusOK) || resp.Result.Token == "" {
		return "", fmt.Errorf("SmartVision 第三方登录失败: code=%d message=%s", resp.Code, resp.Message)
	}
	return resp.Result.Token, nil
}

func (c *SmartVisionClient) do(ctx context.Context, method, path, token string, query url.Values, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(payload)
	}

	requestURL := c.buildURL(path, query)
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set(smartVisionTokenHeader, token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if len(data) > 0 && out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("解析 SmartVision 响应失败: %w, body=%s", err, string(data))
		}
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
		return resp.StatusCode, fmt.Errorf("SmartVision 请求失败: status=%d body=%s", resp.StatusCode, string(data))
	}
	return resp.StatusCode, nil
}

func (c *SmartVisionClient) buildURL(path string, query url.Values) string {
	if parsed, err := url.Parse(path); err == nil && parsed.IsAbs() {
		if query != nil && len(query) > 0 {
			parsed.RawQuery = query.Encode()
		}
		return parsed.String()
	}
	fullURL := c.baseURL + "/" + strings.TrimLeft(path, "/")
	if query != nil && len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	return fullURL
}

func (c *SmartVisionClient) downloadFile(ctx context.Context, filePath, token string) (io.ReadCloser, int64, error) {
	requestURL := c.buildFileURL(filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}
	if token != "" {
		req.Header.Set(smartVisionTokenHeader, token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, 0, errSmartVisionUnauthorized
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, 0, fmt.Errorf("SmartVision 文件下载失败: status=%d body=%s", resp.StatusCode, string(data))
	}
	return resp.Body, resp.ContentLength, nil
}

func (c *SmartVisionClient) buildFileURL(filePath string) string {
	if parsed, err := url.Parse(filePath); err == nil && parsed.IsAbs() {
		return parsed.String()
	}

	base, err := url.Parse(c.baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return c.buildURL(filePath, nil)
	}
	if strings.HasPrefix(filePath, "/") {
		basePath := strings.TrimRight(base.Path, "/")
		if basePath != "" && strings.HasPrefix(filePath, basePath+"/") {
			base.Path = filePath
			return base.String()
		}
	}
	return c.buildURL(filePath, nil)
}

func decodeSmartVisionUser(raw json.RawMessage) (*SmartVisionUser, error) {
	var user SmartVisionUser
	if err := json.Unmarshal(raw, &user); err != nil {
		return nil, fmt.Errorf("解析 SmartVision 用户失败: %w", err)
	}
	if strings.TrimSpace(user.Username) == "" {
		var wrapped struct {
			UserInfo json.RawMessage `json:"userInfo"`
		}
		if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.UserInfo) > 0 {
			if err := json.Unmarshal(wrapped.UserInfo, &user); err != nil {
				return nil, fmt.Errorf("解析 SmartVision userInfo 失败: %w", err)
			}
		}
	}
	user.Raw = append(json.RawMessage(nil), raw...)
	return &user, nil
}
