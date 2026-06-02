/*
 * @module service/executors/face_compare_executor
 * @description 人脸比对节点 — 调用人脸比对接口，返回匹配结果
 * @architecture 策略模式
 * @stateFlow 解码base64 → multipart POST 人脸比对API → 解析响应 → 输出结果
 * @rules 支持多图比对，出参可直接对接 face_result_parser 节点
 * @dependencies models, net/http, mime/multipart
 */

package executors

import (
	"box-manage-service/models"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// FaceCompareExecutor 人脸比对执行器
type FaceCompareExecutor struct {
	*BaseExecutor
	client *http.Client
}

func NewFaceCompareExecutor() *FaceCompareExecutor {
	return &FaceCompareExecutor{
		BaseExecutor: NewBaseExecutor("face_compare"),
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *FaceCompareExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行人脸比对"}

	// 解析配置
	apiURL := e.getStringParam(execCtx, "api_url", "http://10.188.96.7:5004/compareFaceImg")
	apiKey := e.getStringParam(execCtx, "api_key", "zlghcgePiO")
	score := e.getStringParam(execCtx, "score", "0.5")

	// 获取图片
	imageData := e.getParam(execCtx, "image")
	images := e.collectImages(imageData, execCtx.Variables)
	if len(images) == 0 {
		err := fmt.Errorf("未找到待比对图片")
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), err
	}
	logs = append(logs, fmt.Sprintf("待比对图片: %d 张, 阈值: %s", len(images), score))

	// 构建 multipart 请求
	body, contentType, err := e.buildMultipart(images, score)
	if err != nil {
		logs = append(logs, fmt.Sprintf("构建请求失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		logs = append(logs, fmt.Sprintf("创建请求失败: %v", err))
		return CreateFailureResult(err, logs), err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", apiKey)

	startTime := time.Now()
	resp, err := e.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logs = append(logs, fmt.Sprintf("请求失败: %v, 耗时: %v", err, duration))
		return CreateFailureResult(err, logs), err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("人脸比对接口返回 %d: %s", resp.StatusCode, e.truncate(string(respBody), 200))
		logs = append(logs, errMsg)
		return CreateFailureResult(fmt.Errorf(errMsg), logs), fmt.Errorf(errMsg)
	}

	// 解析响应
	faceResults, msg := e.parseResponse(respBody)
	if msg != "" {
		logs = append(logs, msg)
		if faceResults == nil {
			outputs := map[string]interface{}{
				"face_results": []interface{}{},
				"message":      msg,
			}
			return CreateSuccessResult(CreateOutputs(outputs, map[string]interface{}{"duration_ms": duration.Milliseconds()}), logs), nil
		}
	}

	logs = append(logs, fmt.Sprintf("匹配到 %d 条结果", len(faceResults)))
	logs = append(logs, fmt.Sprintf("人脸比对完成, 耗时: %v", duration))

	outputs := map[string]interface{}{
		"face_results": faceResults,
	}
	extras := map[string]interface{}{
		"duration_ms":     duration.Milliseconds(),
		"face_count":      len(faceResults),
		"score_threshold": score,
	}
	return CreateSuccessResult(CreateOutputs(outputs, extras), logs), nil
}

// buildMultipart 构建 multipart/form-data 请求体
func (e *FaceCompareExecutor) buildMultipart(images []imageItem, score string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// score 字段
	_ = w.WriteField("score", score)

	// 图片文件
	for _, img := range images {
		decoded, err := base64.StdEncoding.DecodeString(img.data)
		if err != nil {
			return nil, "", fmt.Errorf("base64解码失败(%s): %w", img.name, err)
		}
		part, err := w.CreateFormFile("imgs", img.name)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(decoded); err != nil {
			return nil, "", err
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return &buf, w.FormDataContentType(), nil
}

// parseResponse 解析人脸比对接口响应
func (e *FaceCompareExecutor) parseResponse(body []byte) ([]interface{}, string) {
	// 先判 error
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return nil, fmt.Sprintf("接口错误: %s", errResp.Error)
	}

	// 再判 no match
	var msgResp struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &msgResp) == nil && msgResp.Message == "no match" {
		return nil, "无匹配结果"
	}

	// 解析二维数组 [[{...}]]
	var results [][]interface{}
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Sprintf("解析响应失败: %v", err)
	}

	// 展平为一位数组
	var flat []interface{}
	for _, group := range results {
		flat = append(flat, group...)
	}

	return flat, ""
}

type imageItem struct {
	name string
	data string // base64 编码的图片数据
}

// collectImages 收集所有待比对图片
func (e *FaceCompareExecutor) collectImages(imageData interface{}, variables map[string]interface{}) []imageItem {
	var images []imageItem

	// 主参数 image
	if s, ok := imageData.(string); ok && s != "" {
		images = append(images, e.extractImageItem(s, "capture.jpg"))
	}

	// 从变量中搜索额外图片
	for k, v := range variables {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		// 匹配 image_xxx 格式的变量
		if strings.HasPrefix(k, "image_") && k != "image" {
			images = append(images, e.extractImageItem(s, k+".jpg"))
		}
	}

	return images
}

func (e *FaceCompareExecutor) extractImageItem(s, name string) imageItem {
	b64 := s
	if idx := strings.Index(s, ";base64,"); idx != -1 {
		b64 = s[idx+8:]
	}
	return imageItem{name: name, data: b64}
}

func (e *FaceCompareExecutor) getStringParam(execCtx *ExecutionContext, key, defaultVal string) string {
	v := e.getParam(execCtx, key)
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return defaultVal
}

func (e *FaceCompareExecutor) getParam(execCtx *ExecutionContext, key string) interface{} {
	if execCtx.Inputs != nil {
		if v, ok := execCtx.Inputs[key]; ok && v != nil {
			return v
		}
	}
	if execCtx.Variables != nil {
		if v, ok := execCtx.Variables[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func (e *FaceCompareExecutor) Validate(nodeInstance *models.NodeInstance) error {
	return e.BaseExecutor.Validate(nodeInstance)
}

func (e *FaceCompareExecutor) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
