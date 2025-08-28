package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ZLMediaKitClient ZLMediaKit HTTP API客户端
type ZLMediaKitClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

// NewZLMediaKitClient 创建新的ZLMediaKit客户端
func NewZLMediaKitClient(host string, port int, secret string) *ZLMediaKitClient {
	return &ZLMediaKitClient{
		baseURL:    fmt.Sprintf("http://%s:%d", host, port),
		secret:     secret,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ZLMResponse ZLMediaKit通用响应
type ZLMResponse struct {
	Code   int         `json:"code"`
	Msg    string      `json:"msg,omitempty"`
	Data   interface{} `json:"data,omitempty"`
	Result interface{} `json:"result,omitempty"`
}

// StreamInfo 流信息
type StreamInfo struct {
	App              string `json:"app"`
	Stream           string `json:"stream"`
	Schema           string `json:"schema"`
	Vhost            string `json:"vhost"`
	AliveTime        int    `json:"aliveTime"`
	BytesSpeed       int    `json:"bytesSpeed"`
	ReaderCount      int    `json:"readerCount"`
	TotalReaderCount int    `json:"totalReaderCount"`
}

// AddStreamProxyRequest 添加流代理请求
type AddStreamProxyRequest struct {
	Vhost      string `json:"vhost"`       // 虚拟主机，默认__defaultVhost__
	App        string `json:"app"`         // 应用名，推荐为live
	Stream     string `json:"stream"`      // 流id
	URL        string `json:"url"`         // 拉流地址
	RetryCount int    `json:"retry_count"` // 重试次数，默认为-1无限重试
	RtpType    int    `json:"rtp_type"`    // 拉流方式，0:tcp，1:udp，2:组播
	Timeout    int    `json:"timeout_sec"` // 拉流超时时间，单位秒
}

// AddStreamProxyResponse 添加流代理响应
type AddStreamProxyResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
	Data struct {
		Key string `json:"key"` // 流代理的唯一标识key
	} `json:"data,omitempty"`
}

// CloseStreamRequest 关闭流请求
type CloseStreamRequest struct {
	Vhost  string `json:"vhost"`  // 虚拟主机
	App    string `json:"app"`    // 应用名
	Stream string `json:"stream"` // 流id
	Force  int    `json:"force"`  // 是否强制关闭(无视播放者)
}

// StartRecordRequest 开始录制请求
type StartRecordRequest struct {
	Type           int    `json:"type"`                      // 0为hls，1为mp4
	Vhost          string `json:"vhost"`                     // 虚拟主机
	App            string `json:"app"`                       // 应用名
	Stream         string `json:"stream"`                    // 流id
	CustomizedPath string `json:"customized_path,omitempty"` // 录制文件保存根目录
	MaxSecond      int    `json:"max_second,omitempty"`      // mp4录制最大时长，单位秒
}

// StopRecordRequest 停止录制请求
type StopRecordRequest struct {
	Type   int    `json:"type"`   // 0为hls，1为mp4
	Vhost  string `json:"vhost"`  // 虚拟主机
	App    string `json:"app"`    // 应用名
	Stream string `json:"stream"` // 流id
}

// RecordStatus 录制状态
type RecordStatus struct {
	Status bool `json:"status"`
	Type   int  `json:"type"`
}

// GetRecordStatusRequest 获取录制状态请求
type GetRecordStatusRequest struct {
	Type   int    `json:"type"`   // 0为hls，1为mp4
	Vhost  string `json:"vhost"`  // 虚拟主机
	App    string `json:"app"`    // 应用名
	Stream string `json:"stream"` // 流id
}

// MediaList 流媒体列表
type MediaList struct {
	Code int          `json:"code"`
	Data []StreamInfo `json:"data"`
}

// doRequest 执行HTTP请求
func (c *ZLMediaKitClient) doRequest(ctx context.Context, method, endpoint string, params map[string]interface{}) (*ZLMResponse, error) {
	var req *http.Request
	var err error

	// 添加secret参数
	if params == nil {
		params = make(map[string]interface{})
	}
	if c.secret != "" {
		params["secret"] = c.secret
	}

	reqURL := c.baseURL + endpoint

	if method == "GET" {
		// GET请求，参数放在URL中
		values := url.Values{}
		for key, value := range params {
			values.Set(key, fmt.Sprintf("%v", value))
		}
		if len(values) > 0 {
			reqURL += "?" + values.Encode()
		}
		req, err = http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	} else if method == "POST" {
		// POST请求，参数放在body中
		jsonData, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params failed: %w", err)
		}
		req, err = http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonData))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	var zlmResp ZLMResponse
	if err := json.Unmarshal(body, &zlmResp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	return &zlmResp, nil
}

// AddStreamProxy 添加流代理
func (c *ZLMediaKitClient) AddStreamProxy(ctx context.Context, req *AddStreamProxyRequest) (*AddStreamProxyResponse, error) {
	params := map[string]interface{}{
		"vhost":       req.Vhost,
		"app":         req.App,
		"stream":      req.Stream,
		"url":         req.URL,
		"retry_count": req.RetryCount,
		"rtp_type":    req.RtpType,
		"timeout_sec": req.Timeout,
	}

	resp, err := c.doRequest(ctx, "POST", "/index/api/addStreamProxy", params)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("addStreamProxy failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	var result AddStreamProxyResponse
	result.Code = resp.Code
	result.Msg = resp.Msg

	if resp.Data != nil {
		dataBytes, _ := json.Marshal(resp.Data)
		json.Unmarshal(dataBytes, &result.Data)
	}

	return &result, nil
}

// DelStreamProxy 删除流代理
func (c *ZLMediaKitClient) DelStreamProxy(ctx context.Context, key string) error {
	params := map[string]interface{}{
		"key": key,
	}

	resp, err := c.doRequest(ctx, "POST", "/index/api/delStreamProxy", params)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("delStreamProxy failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// CloseStream 关闭流
func (c *ZLMediaKitClient) CloseStream(ctx context.Context, req *CloseStreamRequest) error {
	params := map[string]interface{}{
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
		"force":  req.Force,
	}

	resp, err := c.doRequest(ctx, "POST", "/index/api/close_stream", params)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("closeStream failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// StartRecord 开始录制
func (c *ZLMediaKitClient) StartRecord(ctx context.Context, req *StartRecordRequest) error {
	params := map[string]interface{}{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}

	if req.CustomizedPath != "" {
		params["customized_path"] = req.CustomizedPath
	}
	if req.MaxSecond > 0 {
		params["max_second"] = req.MaxSecond
	}

	resp, err := c.doRequest(ctx, "POST", "/index/api/startRecord", params)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("startRecord failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// StopRecord 停止录制
func (c *ZLMediaKitClient) StopRecord(ctx context.Context, req *StopRecordRequest) error {
	params := map[string]interface{}{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}

	resp, err := c.doRequest(ctx, "POST", "/index/api/stopRecord", params)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("stopRecord failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// GetRecordStatus 获取录制状态
func (c *ZLMediaKitClient) GetRecordStatus(ctx context.Context, req *GetRecordStatusRequest) (*RecordStatus, error) {
	params := map[string]interface{}{
		"type":   req.Type,
		"vhost":  req.Vhost,
		"app":    req.App,
		"stream": req.Stream,
	}

	resp, err := c.doRequest(ctx, "GET", "/index/api/getRecordStatus", params)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("getRecordStatus failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	var status RecordStatus
	if resp.Data != nil {
		dataBytes, _ := json.Marshal(resp.Data)
		json.Unmarshal(dataBytes, &status)
	}

	return &status, nil
}

// GetMediaList 获取流媒体列表
func (c *ZLMediaKitClient) GetMediaList(ctx context.Context) ([]StreamInfo, error) {
	resp, err := c.doRequest(ctx, "GET", "/index/api/getMediaList", nil)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("getMediaList failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	var streams []StreamInfo
	if resp.Data != nil {
		dataBytes, _ := json.Marshal(resp.Data)
		json.Unmarshal(dataBytes, &streams)
	}

	return streams, nil
}

// IsMediaOnline 检查流是否在线
func (c *ZLMediaKitClient) IsMediaOnline(ctx context.Context, vhost, app, stream string) (bool, error) {
	params := map[string]interface{}{
		"vhost":  vhost,
		"app":    app,
		"stream": stream,
	}

	resp, err := c.doRequest(ctx, "GET", "/index/api/isMediaOnline", params)
	if err != nil {
		return false, err
	}

	// 对于isMediaOnline，code为0表示在线，-1表示离线
	return resp.Code == 0, nil
}

// GetSnap 获取截图
func (c *ZLMediaKitClient) GetSnap(ctx context.Context, vhost, app, stream string, timeout int) ([]byte, error) {
	params := map[string]interface{}{
		"vhost":       vhost,
		"app":         app,
		"stream":      stream,
		"timeout_sec": timeout,
	}

	reqURL := c.baseURL + "/index/api/getSnap"
	values := url.Values{}
	for key, value := range params {
		values.Set(key, fmt.Sprintf("%v", value))
	}
	if c.secret != "" {
		values.Set("secret", c.secret)
	}
	reqURL += "?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getSnap failed: status=%d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
