package executors

import (
	"box-manage-service/models"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RpaWechatProxyExecutor struct {
	*BaseExecutor
}

func NewRpaWechatProxyExecutor() *RpaWechatProxyExecutor {
	return &RpaWechatProxyExecutor{
		BaseExecutor: NewBaseExecutor("rpa_wechat_proxy"),
	}
}

func (e *RpaWechatProxyExecutor) Validate(instance *models.NodeInstance) error {
	return nil
}

func (e *RpaWechatProxyExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{}
	logs = append(logs, "开始执行 RPA代理企业微信推送...")

	webhookURL := getStringInput(execCtx.Inputs, "webhook_url")
	rpaProxyURL := getStringInput(execCtx.Inputs, "rpa_proxy_url")
	if rpaProxyURL == "" {
		rpaProxyURL = "http://10.188.32.17:8080/RPA"
	}
	bodyText := getStringFromBody(execCtx.Inputs, "body_text")
	bodyImage := getStringFromBody(execCtx.Inputs, "body_image")
	mentionedList := getStringInput(execCtx.Inputs, "mentioned_list")
	mentionedMobileList := getStringInput(execCtx.Inputs, "mentioned_mobile_list")

	if webhookURL == "" {
		err := fmt.Errorf("webhook_url 不能为空")
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), nil
	}

	var mentionTags strings.Builder
	if mentionedList != "" {
		for _, uid := range strings.Split(mentionedList, ",") {
			uid = strings.TrimSpace(uid)
			if uid != "" {
				mentionTags.WriteString("<@" + uid + ">")
			}
		}
	}
	if mentionedMobileList != "" {
		for _, mobile := range strings.Split(mentionedMobileList, ",") {
			mobile = strings.TrimSpace(mobile)
			if mobile != "" {
				mentionTags.WriteString("<@" + mobile + ">")
			}
		}
	}

	textSent := false
	errorMsg := ""

	// 发送文本
	textContent := mentionTags.String() + bodyText
	textPayload := map[string]interface{}{"message": textContent, "send_url": webhookURL}
	bodyBytes, _ := json.Marshal(textPayload)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(rpaProxyURL+"/send_wechat_root_markdown", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		errorMsg = "文本发送失败: " + err.Error()
		logs = append(logs, errorMsg)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			textSent = true
			logs = append(logs, fmt.Sprintf("文本发送成功: status=%d", resp.StatusCode))
		} else {
			errorMsg = fmt.Sprintf("文本发送失败: HTTP %d, body=%s", resp.StatusCode, string(respBody))
			logs = append(logs, errorMsg)
		}
	}

	// 发送图片
	imageSent := false
	if bodyImage != "" {
		imgB64 := bodyImage
		if idx := strings.Index(imgB64, ";base64,"); idx != -1 {
			imgB64 = imgB64[idx+8:]
		} else if idx := strings.Index(imgB64, "base64,"); idx != -1 {
			imgB64 = imgB64[idx+7:]
		}
		decoded, err := base64.StdEncoding.DecodeString(imgB64)
		if err != nil {
			errMsg := "图片base64解码失败: " + err.Error()
			logs = append(logs, errMsg)
			if errorMsg != "" {
				errorMsg += "; "
			}
			errorMsg += errMsg
		} else {
			md5Hash := md5.Sum(decoded)
			md5Str := fmt.Sprintf("%x", md5Hash)
			imgPayload := map[string]interface{}{"base64Content": imgB64, "md5Content": md5Str, "send_url": webhookURL}
			bodyBytes, _ := json.Marshal(imgPayload)
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Post(rpaProxyURL+"/send_wechat_root_image", "application/json", bytes.NewReader(bodyBytes))
			if err != nil {
				errMsg := "图片发送失败: " + err.Error()
				logs = append(logs, errMsg)
				if errorMsg != "" {
					errorMsg += "; "
				}
				errorMsg += errMsg
			} else {
				imgRespBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					imageSent = true
					logs = append(logs, fmt.Sprintf("图片发送成功: status=%d", resp.StatusCode))
				} else {
					errMsg := fmt.Sprintf("图片发送失败: HTTP %d, body=%s", resp.StatusCode, string(imgRespBody))
					logs = append(logs, errMsg)
					if errorMsg != "" {
						errorMsg += "; "
					}
					errorMsg += errMsg
				}
			}
		}
	}

	outputs := map[string]interface{}{"text_sent": textSent, "image_sent": imageSent, "error_message": errorMsg}
	logs = append(logs, fmt.Sprintf("推送完成: text=%v, image=%v, err=%s", textSent, imageSent, errorMsg))
	return CreateSuccessResult(outputs, logs), nil
}

func getStringInput(inputs map[string]interface{}, key string) string {
	if v, ok := inputs[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringFromBody(inputs map[string]interface{}, key string) string {
	if v, ok := inputs[key]; ok && v != nil {
		if m, ok := v.(map[string]interface{}); ok {
			if content, ok := m["content"]; ok && content != nil {
				if s, ok := content.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}
