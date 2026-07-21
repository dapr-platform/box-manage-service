package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"box-manage-service/config"
)

const smartVisionTokenHeader = "X-Access-Token"

const smartVisionMinDirectModelStreamSize = 1024

var errSmartVisionUnauthorized = errors.New("smartvision unauthorized")

type readerWithCloser struct {
	io.Reader
	io.Closer
}

// SmartVisionClient SmartVision 平台 HTTP 客户端。
type SmartVisionClient struct {
	baseURL    string
	username   string
	password   string
	clientName string
	clientID   string
	httpClient *http.Client

	mu     sync.Mutex
	token  string
	userID string
}

type smartVisionLoginResult struct {
	Token    string          `json:"token"`
	UserID   string          `json:"userId"`
	ID       string          `json:"id"`
	UserInfo json.RawMessage `json:"userInfo"`
	User     json.RawMessage `json:"user"`
}

// SmartVisionAPIResponse SmartVision 通用响应结构。
type SmartVisionAPIResponse[T any] struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Result    T               `json:"result"`
	Success   bool            `json:"success"`
	Timestamp json.RawMessage `json:"timestamp"`
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
	ProjectID       string          `json:"projectId"`
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

// SmartVisionModelDeploymentRequest SmartVision 模型部署/下载请求。
type SmartVisionModelDeploymentRequest struct {
	ClientIP  string `json:"client_ip"`
	ModelNo   string `json:"modelNo"`
	ProjectID string `json:"projectId"`
	UserID    string `json:"userId"`
}

// SmartVisionDeploymentParam SmartVision 部署返回参数。
type SmartVisionDeploymentParam struct {
	Argument string `json:"argument"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Remark   string `json:"remark"`
}

// SmartVisionModelDeploymentResult SmartVision 模型部署结果。
type SmartVisionModelDeploymentResult struct {
	Flag   bool                         `json:"flag"`
	Params []SmartVisionDeploymentParam `json:"paras"`
	URL    string                       `json:"url"`
}

// ParamValue 根据 remark 读取部署参数值。
func (r SmartVisionModelDeploymentResult) ParamValue(remark string) string {
	for _, param := range r.Params {
		if strings.EqualFold(strings.TrimSpace(param.Remark), strings.TrimSpace(remark)) {
			return strings.TrimSpace(param.Value)
		}
	}
	return ""
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

	log.Printf("[SmartVision][Client] 开始第三方登录: baseURL=%s username=%s clientName=%s", c.baseURL, c.username, c.clientName)
	loginResult, err := c.loginThirdLocked(ctx)
	if err != nil {
		log.Printf("[SmartVision][Client] 第三方登录失败: username=%s err=%v", c.username, err)
		return "", err
	}
	c.token = loginResult.Token
	c.userID = loginResult.apiUserID()
	log.Printf("[SmartVision][Client] 第三方登录成功: username=%s tokenLen=%d userId=%s", c.username, len(loginResult.Token), c.userID)
	return loginResult.Token, nil
}

// APIUserID 返回 SmartVision API 调用使用的 userId，来源于第三方登录响应。
func (c *SmartVisionClient) APIUserID(ctx context.Context) (string, error) {
	c.mu.Lock()
	userID := strings.TrimSpace(c.userID)
	c.mu.Unlock()
	if userID != "" {
		return userID, nil
	}

	if _, err := c.LoginThird(ctx); err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	userID = strings.TrimSpace(c.userID)
	if userID == "" {
		return "", fmt.Errorf("SmartVision 第三方登录未返回 userId")
	}
	return userID, nil
}

// ValidateToken 验证前端传入的 SmartVision token 并返回用户信息。
func (c *SmartVisionClient) ValidateToken(ctx context.Context, token string) (*SmartVisionUser, error) {
	query := url.Values{}
	query.Set("token", token)

	log.Printf("[SmartVision][Client] 开始校验前端 token: tokenLen=%d", len(token))
	var resp SmartVisionAPIResponse[json.RawMessage]
	status, err := c.do(ctx, http.MethodGet, "/sys/user/getUserSectionInfoByToken", token, query, nil, &resp)
	if err != nil {
		log.Printf("[SmartVision][Client] 校验前端 token 请求失败: status=%d err=%v", status, err)
		return nil, err
	}
	if status == http.StatusUnauthorized || resp.Code == http.StatusUnauthorized || !resp.Success {
		log.Printf("[SmartVision][Client] 校验前端 token 未通过: status=%d code=%d success=%v message=%s", status, resp.Code, resp.Success, resp.Message)
		return nil, fmt.Errorf("SmartVision token 校验失败: %s", resp.Message)
	}

	user, err := decodeSmartVisionUser(resp.Result)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.Username) == "" {
		return nil, fmt.Errorf("SmartVision token 校验成功但未返回 username")
	}
	log.Printf("[SmartVision][Client] 校验前端 token 成功: username=%s userID=%s", user.Username, user.ID)
	return user, nil
}

// ListAllUsers 查询 SmartVision 所有用户。
func (c *SmartVisionClient) ListAllUsers(ctx context.Context) ([]SmartVisionUser, error) {
	log.Printf("[SmartVision][Client] 开始拉取用户列表")
	var resp SmartVisionAPIResponse[SmartVisionPage[json.RawMessage]]
	if err := c.doWithAPITokenRetry(ctx, http.MethodGet, "/sys/user/listAll", nil, &resp); err != nil {
		log.Printf("[SmartVision][Client] 拉取用户列表失败: err=%v", err)
		return nil, err
	}
	if !resp.Success && resp.Code != 0 && resp.Code != http.StatusOK {
		log.Printf("[SmartVision][Client] 拉取用户列表返回失败: code=%d success=%v message=%s", resp.Code, resp.Success, resp.Message)
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
	log.Printf("[SmartVision][Client] 拉取用户列表成功: total=%d records=%d valid=%d", resp.Result.Total, len(resp.Result.Records), len(users))
	return users, nil
}

// SuccessModelList 查询 SmartVision 训练成功的模型。
func (c *SmartVisionClient) SuccessModelList(ctx context.Context) ([]SmartVisionModel, error) {
	log.Printf("[SmartVision][Client] 开始拉取成功模型列表")
	var resp SmartVisionAPIResponse[SmartVisionPage[json.RawMessage]]
	if err := c.doWithAPITokenRetry(ctx, http.MethodGet, "/project/modelInfo/successModelList", nil, &resp); err != nil {
		log.Printf("[SmartVision][Client] 拉取成功模型列表失败: err=%v", err)
		return nil, err
	}
	if !resp.Success && resp.Code != 0 && resp.Code != http.StatusOK {
		log.Printf("[SmartVision][Client] 拉取成功模型列表返回失败: code=%d success=%v message=%s", resp.Code, resp.Success, resp.Message)
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
	log.Printf("[SmartVision][Client] 拉取成功模型列表成功: total=%d records=%d parsed=%d", resp.Result.Total, len(resp.Result.Records), len(models))
	return models, nil
}

// DeployModel 调用 SmartVision 模型部署接口。下载模型前必须先部署。
func (c *SmartVisionClient) DeployModel(ctx context.Context, payload SmartVisionModelDeploymentRequest) (*SmartVisionModelDeploymentResult, error) {
	log.Printf("[SmartVision][Client] 开始部署模型: modelNo=%s projectId=%s userId=%s clientIP=%s", payload.ModelNo, payload.ProjectID, payload.UserID, payload.ClientIP)
	var resp SmartVisionAPIResponse[SmartVisionModelDeploymentResult]
	if err := c.doWithAPITokenRetry(ctx, http.MethodPost, "/deployment/deploymentInfo/modelDeployment", payload, &resp); err != nil {
		log.Printf("[SmartVision][Client] 部署模型请求失败: modelNo=%s projectId=%s err=%v", payload.ModelNo, payload.ProjectID, err)
		return nil, err
	}
	if !resp.Success && resp.Code != 0 && resp.Code != http.StatusOK {
		log.Printf("[SmartVision][Client] 部署模型返回失败: modelNo=%s projectId=%s code=%d success=%v message=%s flag=%v paramCount=%d detectURL=%s modelPath=%s", payload.ModelNo, payload.ProjectID, resp.Code, resp.Success, resp.Message, resp.Result.Flag, len(resp.Result.Params), resp.Result.URL, limitSmartVisionLogValue(resp.Result.ParamValue("modelPath"), 200))
		return nil, fmt.Errorf("SmartVision 模型部署失败: %s", resp.Message)
	}
	if !resp.Result.Flag {
		log.Printf("[SmartVision][Client] 部署模型 flag=false: modelNo=%s projectId=%s code=%d message=%s", payload.ModelNo, payload.ProjectID, resp.Code, resp.Message)
		return nil, fmt.Errorf("SmartVision 模型部署未成功: %s", resp.Message)
	}
	log.Printf("[SmartVision][Client] 部署模型成功: modelNo=%s projectId=%s paramCount=%d detectURL=%s modelPath=%s", payload.ModelNo, payload.ProjectID, len(resp.Result.Params), resp.Result.URL, limitSmartVisionLogValue(resp.Result.ParamValue("modelPath"), 200))
	return &resp.Result, nil
}

// DownloadFile 按 SmartVision 返回的文件路径下载文件。
func (c *SmartVisionClient) DownloadFile(ctx context.Context, filePath string) (io.ReadCloser, int64, error) {
	token, err := c.apiToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	log.Printf("[SmartVision][Client] 开始下载文件: source=%s", limitSmartVisionLogValue(filePath, 200))
	body, size, err := c.downloadFile(ctx, filePath, token)
	if errors.Is(err, errSmartVisionUnauthorized) {
		log.Printf("[SmartVision][Client] 下载文件返回 401，重新登录后重试: source=%s", limitSmartVisionLogValue(filePath, 200))
		token, err = c.LoginThird(ctx)
		if err != nil {
			return nil, 0, err
		}
		body, size, err = c.downloadFile(ctx, filePath, token)
	}
	if err != nil {
		log.Printf("[SmartVision][Client] 下载文件失败: source=%s err=%v", limitSmartVisionLogValue(filePath, 200), err)
		return nil, 0, err
	}
	log.Printf("[SmartVision][Client] 下载文件响应成功: source=%s contentLength=%d", limitSmartVisionLogValue(filePath, 200), size)
	return body, size, err
}

// DownloadDeployedModel 调用 SmartVision 模型下载接口下载已部署模型。
func (c *SmartVisionClient) DownloadDeployedModel(ctx context.Context, payload SmartVisionModelDeploymentRequest) (io.ReadCloser, int64, string, error) {
	token, err := c.apiToken(ctx)
	if err != nil {
		return nil, 0, "", err
	}

	log.Printf("[SmartVision][Client] 开始调用模型下载接口: modelNo=%s projectId=%s", payload.ModelNo, payload.ProjectID)
	body, size, source, err := c.downloadDeployedModel(ctx, payload, token)
	if errors.Is(err, errSmartVisionUnauthorized) {
		log.Printf("[SmartVision][Client] 模型下载接口返回 401，重新登录后重试: modelNo=%s projectId=%s", payload.ModelNo, payload.ProjectID)
		token, err = c.LoginThird(ctx)
		if err != nil {
			return nil, 0, "", err
		}
		body, size, source, err = c.downloadDeployedModel(ctx, payload, token)
	}
	if err != nil {
		log.Printf("[SmartVision][Client] 模型下载接口失败: modelNo=%s projectId=%s err=%v", payload.ModelNo, payload.ProjectID, err)
		return nil, 0, source, err
	}
	log.Printf("[SmartVision][Client] 模型下载接口成功: modelNo=%s projectId=%s source=%s contentLength=%d", payload.ModelNo, payload.ProjectID, limitSmartVisionLogValue(source, 200), size)
	return body, size, source, err
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

	log.Printf("[SmartVision][Client] API 返回 401，重新登录后重试: method=%s path=%s", method, path)
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

func (c *SmartVisionClient) loginThirdLocked(ctx context.Context) (*smartVisionLoginResult, error) {
	payload := map[string]string{
		"captcha":    "",
		"checkKey":   "",
		"clientId":   c.clientID,
		"clientName": c.clientName,
		"password":   c.password,
		"username":   c.username,
	}

	var resp SmartVisionAPIResponse[smartVisionLoginResult]
	status, err := c.do(ctx, http.MethodPost, "/sys/thirdLogin/loginThird", "", nil, payload, &resp)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || (!resp.Success && resp.Code != http.StatusOK) || resp.Result.Token == "" {
		log.Printf("[SmartVision][Client] 第三方登录响应异常: status=%d code=%d success=%v message=%s tokenLen=%d", status, resp.Code, resp.Success, resp.Message, len(resp.Result.Token))
		return nil, fmt.Errorf("SmartVision 第三方登录失败: code=%d message=%s", resp.Code, resp.Message)
	}
	return &resp.Result, nil
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
	started := time.Now()
	log.Printf("[SmartVision][Client] HTTP 请求开始: method=%s path=%s url=%s hasToken=%v hasBody=%v", method, path, redactSmartVisionURL(requestURL), token != "", body != nil)
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
		log.Printf("[SmartVision][Client] HTTP 请求失败: method=%s path=%s duration=%s err=%v", method, path, time.Since(started), err)
		return 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[SmartVision][Client] HTTP 读取响应失败: method=%s path=%s status=%d duration=%s err=%v", method, path, resp.StatusCode, time.Since(started), err)
		return resp.StatusCode, err
	}
	if len(data) > 0 && out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			log.Printf("[SmartVision][Client] HTTP 解析响应失败: method=%s path=%s status=%d duration=%s bodyLen=%d err=%v", method, path, resp.StatusCode, time.Since(started), len(data), err)
			return resp.StatusCode, fmt.Errorf("解析 SmartVision 响应失败: %w, body=%s", err, string(data))
		}
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
		log.Printf("[SmartVision][Client] HTTP 响应失败: method=%s path=%s status=%d duration=%s body=%s", method, path, resp.StatusCode, time.Since(started), limitSmartVisionLogValue(string(data), 500))
		return resp.StatusCode, fmt.Errorf("SmartVision 请求失败: status=%d body=%s", resp.StatusCode, string(data))
	}
	log.Printf("[SmartVision][Client] HTTP 请求完成: method=%s path=%s status=%d duration=%s bodyLen=%d", method, path, resp.StatusCode, time.Since(started), len(data))
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

func (r smartVisionLoginResult) apiUserID() string {
	if userID := strings.TrimSpace(r.UserID); userID != "" {
		return userID
	}
	if userID := strings.TrimSpace(r.ID); userID != "" {
		return userID
	}
	for _, raw := range []json.RawMessage{r.UserInfo, r.User} {
		if len(raw) == 0 {
			continue
		}
		var user struct {
			ID     string `json:"id"`
			UserID string `json:"userId"`
		}
		if err := json.Unmarshal(raw, &user); err == nil {
			if userID := strings.TrimSpace(user.UserID); userID != "" {
				return userID
			}
			if userID := strings.TrimSpace(user.ID); userID != "" {
				return userID
			}
		}
	}
	return ""
}

func (c *SmartVisionClient) downloadFile(ctx context.Context, filePath, token string) (io.ReadCloser, int64, error) {
	requestURL := c.buildFileURL(filePath)
	started := time.Now()
	log.Printf("[SmartVision][Client] 文件 HTTP 请求开始: url=%s hasToken=%v", limitSmartVisionLogValue(requestURL, 200), token != "")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}
	if token != "" {
		req.Header.Set(smartVisionTokenHeader, token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[SmartVision][Client] 文件 HTTP 请求失败: url=%s duration=%s err=%v", limitSmartVisionLogValue(requestURL, 200), time.Since(started), err)
		return nil, 0, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		log.Printf("[SmartVision][Client] 文件 HTTP 返回 401: url=%s duration=%s", limitSmartVisionLogValue(requestURL, 200), time.Since(started))
		return nil, 0, errSmartVisionUnauthorized
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("[SmartVision][Client] 文件 HTTP 响应失败: url=%s status=%d duration=%s body=%s", limitSmartVisionLogValue(requestURL, 200), resp.StatusCode, time.Since(started), limitSmartVisionLogValue(string(data), 500))
		return nil, 0, fmt.Errorf("SmartVision 文件下载失败: status=%d body=%s", resp.StatusCode, string(data))
	}
	log.Printf("[SmartVision][Client] 文件 HTTP 响应成功: url=%s status=%d duration=%s contentLength=%d", limitSmartVisionLogValue(requestURL, 200), resp.StatusCode, time.Since(started), resp.ContentLength)
	return resp.Body, resp.ContentLength, nil
}

func (c *SmartVisionClient) downloadDeployedModel(ctx context.Context, payload SmartVisionModelDeploymentRequest, token string) (io.ReadCloser, int64, string, error) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, "", err
	}
	requestURL := c.buildURL("/deployment/deploymentInfo/ModelPath", nil)
	started := time.Now()
	log.Printf("[SmartVision][Client] 模型下载 HTTP 请求开始: url=%s modelNo=%s projectId=%s", requestURL, payload.ModelNo, payload.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(smartVisionTokenHeader, token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[SmartVision][Client] 模型下载 HTTP 请求失败: modelNo=%s projectId=%s duration=%s err=%v", payload.ModelNo, payload.ProjectID, time.Since(started), err)
		return nil, 0, "", err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		log.Printf("[SmartVision][Client] 模型下载 HTTP 返回 401: modelNo=%s projectId=%s duration=%s", payload.ModelNo, payload.ProjectID, time.Since(started))
		return nil, 0, "", errSmartVisionUnauthorized
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("[SmartVision][Client] 模型下载 HTTP 响应失败: modelNo=%s projectId=%s status=%d duration=%s body=%s", payload.ModelNo, payload.ProjectID, resp.StatusCode, time.Since(started), limitSmartVisionLogValue(string(data), 500))
		return nil, 0, "", fmt.Errorf("SmartVision 模型下载接口失败: status=%d body=%s", resp.StatusCode, string(data))
	}

	reader := bufio.NewReader(resp.Body)
	peeked, _ := reader.Peek(512)
	trimmed := bytes.TrimSpace(peeked)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[' || trimmed[0] == '"') {
		data, readErr := io.ReadAll(reader)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[SmartVision][Client] 模型下载 JSON 响应读取失败: modelNo=%s projectId=%s duration=%s err=%v", payload.ModelNo, payload.ProjectID, time.Since(started), readErr)
			return nil, 0, "", readErr
		}

		var apiResp SmartVisionAPIResponse[string]
		if err := json.Unmarshal(data, &apiResp); err != nil {
			log.Printf("[SmartVision][Client] 模型下载 JSON 响应解析失败: modelNo=%s projectId=%s duration=%s body=%s err=%v", payload.ModelNo, payload.ProjectID, time.Since(started), limitSmartVisionLogValue(string(data), 500), err)
			return nil, 0, "", fmt.Errorf("解析 SmartVision 模型下载响应失败: %w, body=%s", err, string(data))
		}
		if !apiResp.Success && apiResp.Code != 0 && apiResp.Code != http.StatusOK {
			log.Printf("[SmartVision][Client] 模型下载 JSON 返回失败: modelNo=%s projectId=%s code=%d success=%v message=%s", payload.ModelNo, payload.ProjectID, apiResp.Code, apiResp.Success, apiResp.Message)
			return nil, 0, "", fmt.Errorf("SmartVision 模型下载路径获取失败: %s", apiResp.Message)
		}
		source := strings.TrimSpace(apiResp.Result)
		if source == "" {
			log.Printf("[SmartVision][Client] 模型下载 JSON 未返回路径: modelNo=%s projectId=%s code=%d success=%v", payload.ModelNo, payload.ProjectID, apiResp.Code, apiResp.Success)
			return nil, 0, "", fmt.Errorf("SmartVision 模型下载接口未返回下载路径")
		}
		log.Printf("[SmartVision][Client] 模型下载 JSON 返回路径: modelNo=%s projectId=%s source=%s duration=%s", payload.ModelNo, payload.ProjectID, limitSmartVisionLogValue(source, 200), time.Since(started))
		body, size, err := c.downloadFile(ctx, source, token)
		if err != nil {
			return nil, 0, source, err
		}
		return body, size, source, nil
	}
	if resp.ContentLength >= 0 && resp.ContentLength < smartVisionMinDirectModelStreamSize && looksLikeSmartVisionTextResponse(trimmed, resp.Header.Get("Content-Type")) {
		data, readErr := io.ReadAll(reader)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[SmartVision][Client] 模型下载小响应读取失败: modelNo=%s projectId=%s duration=%s err=%v", payload.ModelNo, payload.ProjectID, time.Since(started), readErr)
			return nil, 0, "", readErr
		}
		log.Printf("[SmartVision][Client] 模型下载疑似错误文本响应: modelNo=%s projectId=%s status=%d duration=%s contentLength=%d contentType=%s body=%s", payload.ModelNo, payload.ProjectID, resp.StatusCode, time.Since(started), resp.ContentLength, resp.Header.Get("Content-Type"), limitSmartVisionLogValue(string(data), 500))
		return nil, 0, "", fmt.Errorf("SmartVision 模型下载返回非模型内容: contentLength=%d body=%s", resp.ContentLength, string(data))
	}

	log.Printf("[SmartVision][Client] 模型下载接口直接返回文件流: modelNo=%s projectId=%s status=%d duration=%s contentLength=%d", payload.ModelNo, payload.ProjectID, resp.StatusCode, time.Since(started), resp.ContentLength)
	return readerWithCloser{Reader: reader, Closer: resp.Body}, resp.ContentLength, "", nil
}

func looksLikeSmartVisionTextResponse(preview []byte, contentType string) bool {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "json") || strings.Contains(contentType, "text") || strings.Contains(contentType, "xml") || strings.Contains(contentType, "html") {
		return true
	}
	if len(preview) == 0 {
		return false
	}
	printable := 0
	for _, b := range preview {
		switch {
		case b == '\n' || b == '\r' || b == '\t':
			printable++
		case b >= 32 && b <= 126:
			printable++
		}
	}
	return printable*100/len(preview) >= 90
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

func limitSmartVisionLogValue(value string, max int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func redactSmartVisionURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return limitSmartVisionLogValue(rawURL, 200)
	}
	query := parsed.Query()
	for _, key := range []string{"token", "password", "access_token"} {
		if query.Has(key) {
			query.Set(key, "***")
		}
	}
	parsed.RawQuery = query.Encode()
	return limitSmartVisionLogValue(parsed.String(), 200)
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
