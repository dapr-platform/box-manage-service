package service

import (
	"box-manage-service/config"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPythonScriptGeneratorServiceGenerate(t *testing.T) {
	var received chatCompletionRequest
	var receivedRaw map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("unexpected authorization header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedRaw); err != nil {
			t.Errorf("decode request: %v", err)
		}
		payload, _ := json.Marshal(receivedRaw)
		_ = json.Unmarshal(payload, &received)
		w.Header().Set("Content-Type", "application/json")
		response, _ := json.Marshal(map[string]interface{}{"choices": []interface{}{map[string]interface{}{
			"message": map[string]string{"content": "```python\ndef run(ctx) -> dict:\n    return {\"rows\": []}\n```"},
		}}})
		_, _ = w.Write(response)
	}))
	defer server.Close()

	svc := NewPythonScriptGeneratorService(config.LLMConfig{
		BaseURL:        server.URL + "/v1",
		APIKey:         "test-key",
		Model:          "deepseek-v4-flash",
		Temperature:    0,
		RequestTimeout: time.Second,
		TopK:           20,
		EnableThinking: true,
	})
	draft, err := svc.Generate(context.Background(), &PythonScriptGenerateRequest{
		Description: "返回空 rows",
		Upstream:    []PythonScriptUpstream{{ID: "source"}},
		NodeInputs: []PythonScriptNodeParameter{{
			KeyName: "source_rows", Type: "array", Direction: "input", Required: true,
		}},
		NodeOutputs: []PythonScriptNodeParameter{{
			KeyName: "rows", Type: "array", Direction: "output", Required: true,
		}},
		NodeConfig: map[string]interface{}{"status": "ok"},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if draft.Code != "def run(ctx) -> dict:\n    return {\"rows\": []}" {
		t.Fatalf("unexpected generated code: %q", draft.Code)
	}
	if len(draft.UsedInputs) != 1 || draft.UsedInputs[0] != "source" {
		t.Fatalf("unexpected used inputs: %#v", draft.UsedInputs)
	}
	if received.Model != "deepseek-v4-flash" || received.Temperature != 0 {
		t.Fatalf("unexpected model parameters: %#v", received)
	}
	if receivedRaw["top_k"] != float64(20) {
		t.Fatalf("top_k not forwarded: %#v", receivedRaw["top_k"])
	}
	messages, ok := receivedRaw["messages"].([]interface{})
	if !ok || len(messages) != 2 {
		t.Fatalf("messages not forwarded: %#v", receivedRaw["messages"])
	}
	userMessage, ok := messages[1].(map[string]interface{})
	if !ok {
		t.Fatalf("user message has unexpected type: %#v", messages[1])
	}
	userContent, _ := userMessage["content"].(string)
	for _, expected := range []string{"node_inputs", "node_outputs", "node_config", "source_rows"} {
		if !strings.Contains(userContent, expected) {
			t.Fatalf("user prompt does not contain %q: %s", expected, userContent)
		}
	}
	kwargs, ok := receivedRaw["chat_template_kwargs"].(map[string]interface{})
	if !ok || kwargs["enable_thinking"] != true {
		t.Fatalf("thinking option not forwarded: %#v", receivedRaw["chat_template_kwargs"])
	}
}

func TestExtractPythonCode(t *testing.T) {
	got := extractPythonCode("```python\ndef run(ctx):\n    return {}\n```")
	if got != "def run(ctx):\n    return {}" {
		t.Fatalf("extractPythonCode() = %q", got)
	}
}
