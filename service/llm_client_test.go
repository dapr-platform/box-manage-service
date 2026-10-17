package service

import (
	"box-manage-service/config"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestLLMClient 构造一个指向测试服务器的 llmClient。
func newTestLLMClient(t *testing.T, serverURL string, maxOutputTokens int, thinking bool) *llmClient {
	t.Helper()
	return newLLMClient(config.LLMConfig{
		BaseURL:         serverURL,
		APIKey:          "test-key",
		Model:           "test-model",
		Temperature:     0,
		RequestTimeout:  10 * time.Second,
		TopK:            20,
		EnableThinking:  thinking,
		MaxOutputTokens: maxOutputTokens,
	})
}

// TestLLMRequestSendsMaxTokens 确认请求体显式带上了 max_tokens。
// 不传该参数时服务端用默认值（往往 4~8K），长编排会被截断成残缺 JSON，
// 修复轮次因此反复空转——这是线上实际故障。
func TestLLMRequestSendsMaxTokens(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"status\":\"generated\"}"}}]}`))
	}))
	defer server.Close()

	client := newTestLLMClient(t, server.URL, 16384, true)
	if _, err := client.complete(t.Context(), []chatMessage{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("complete() error = %v", err)
	}

	value, ok := captured["max_tokens"]
	if !ok {
		t.Fatalf("请求体缺少 max_tokens: %+v", captured)
	}
	if int(value.(float64)) != 16384 {
		t.Fatalf("max_tokens 应为 16384，实际 %v", value)
	}
}

// TestLLMDetectsTruncatedOutput 确认 finish_reason=length 会被识别为截断，
// 而不是当成正常内容交给上层去解析成「非法 JSON」。
func TestLLMDetectsTruncatedOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 模拟被截断：JSON 括号不闭合
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"{\"status\":\"generated\",\"nodes\":[{\"id\":\"sta"}}]}`))
	}))
	defer server.Close()

	client := newTestLLMClient(t, server.URL, 1024, true)
	_, err := client.complete(t.Context(), []chatMessage{{Role: "user", Content: "hi"}})

	if err == nil {
		t.Fatal("截断时应返回错误")
	}
	if !IsLLMOutputTruncated(err) {
		t.Fatalf("应被识别为输出截断，实际错误: %v", err)
	}
	// 错误信息要能让运维直接知道怎么改
	if !strings.Contains(err.Error(), "LLM_MAX_OUTPUT_TOKENS") {
		t.Fatalf("错误信息应提示配置项，实际: %v", err)
	}
}

// TestLLMNormalOutputNotFlagged 确认正常结束不会被误判为截断。
func TestLLMNormalOutputNotFlagged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"status\":\"generated\"}"}}]}`))
	}))
	defer server.Close()

	client := newTestLLMClient(t, server.URL, 16384, true)
	content, err := client.complete(t.Context(), []chatMessage{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("complete() error = %v", err)
	}
	if IsLLMOutputTruncated(err) {
		t.Fatal("正常结束不应被判定为截断")
	}
	if content == "" {
		t.Fatal("应返回内容")
	}
}

// TestIsLLMOutputTruncatedOnUnrelatedError 确认类型判断不会误伤其他错误。
func TestIsLLMOutputTruncatedOnUnrelatedError(t *testing.T) {
	if IsLLMOutputTruncated(errors.New("call LLM API: context deadline exceeded")) {
		t.Fatal("超时错误不应被判定为截断")
	}
	if IsLLMOutputTruncated(nil) {
		t.Fatal("nil 不应被判定为截断")
	}
}
