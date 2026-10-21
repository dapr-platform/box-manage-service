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
			"message": map[string]string{"content": "```python\nimport json\nrows = [r for r in source_rows if r.get(\"status\") == status]\n```"},
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
	if draft.Code != "import json\nrows = [r for r in source_rows if r.get(\"status\") == status]" {
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
	got := extractPythonCode("```python\nimport json\nresult = {}\n```")
	if got != "import json\nresult = {}" {
		t.Fatalf("extractPythonCode() = %q", got)
	}
}

func TestValidatePythonScriptRequiresTopLevelSnippet(t *testing.T) {
	if err := validatePythonScript("import json\ncontext_value = {}\nresult = context_value", 0); err != nil {
		t.Fatalf("top-level snippet should pass validation: %v", err)
	}
	for _, code := range []string{
		"def run(ctx):\n    return {}",
		"result = ctx.inputs",
		"return {}",
	} {
		if err := validatePythonScript(code, 0); err == nil {
			t.Fatalf("expected validation failure for %q", code)
		}
	}
}

func TestPythonScriptGeneratorRepairsLegacyWrapper(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := "def run(ctx):\n    return {}"
		if calls > 1 {
			content = "import json\nresult = {}"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{map[string]interface{}{
			"message": map[string]string{"content": content},
		}}})
	}))
	defer server.Close()

	svc := NewPythonScriptGeneratorService(config.LLMConfig{
		BaseURL: server.URL, APIKey: "test-key", Model: "test", RequestTimeout: time.Second,
	})
	draft, err := svc.Generate(context.Background(), &PythonScriptGenerateRequest{Description: "生成结果"})
	if err != nil {
		t.Fatalf("Generate() should repair legacy wrapper: %v", err)
	}
	if draft.Code != "import json\nresult = {}" || calls != 2 {
		t.Fatalf("unexpected repair result: code=%q calls=%d", draft.Code, calls)
	}
}

// TestValidatePythonScriptAllowsHelperFunctions 回归线上误报：
//
// 一份完全合法的脚本（只有 parse_int / parse_bool 两个辅助函数，
// 函数内部有 return，没有任何 ctx 引用）被误判为「不符合运行契约」，
// 导致编排无法发布。
//
// 根因：旧的 return 正则 `(?m)^\s*return\b` 把缩进的函数内 return 也当成顶层 return。
// box-app 是把脚本写成文件后用 python3 执行，只有**模块级** return 才是 SyntaxError。
func TestValidatePythonScriptAllowsHelperFunctions(t *testing.T) {
	code := `import json

def parse_int(value, default=0):
    if value is None or value == '':
        return default
    return int(value)

def parse_bool(value, default=False):
    if value is None:
        return default
    return bool(value)

count = parse_int(count)
enabled = parse_bool(enabled)
`
	if err := validatePythonScript(code, 0); err != nil {
		t.Fatalf("辅助函数内的 return 是合法写法，不应被拒: %v", err)
	}
}

// TestValidatePythonScriptAllowsCtxAsIdentifier 确认 ctx 作为普通变量名/字典键不被误伤。
func TestValidatePythonScriptAllowsCtxAsIdentifier(t *testing.T) {
	for _, code := range []string{
		"data = {'ctx': 1}",
		"ctx = 1\nresult = ctx + 1",
		"# 这里的 ctx_related 说明\nx = 1",
		"context = 1\nvalue = context",
	} {
		if err := validatePythonScript(code, 0); err != nil {
			t.Fatalf("普通标识符不应被拒: %q -> %v", code, err)
		}
	}
}

// TestValidatePythonScriptRejectsRealViolations 确认放宽之后，
// 真正会运行失败的写法仍然被拦住。
func TestValidatePythonScriptRejectsRealViolations(t *testing.T) {
	cases := map[string]string{
		"把 ctx 当对象用(属性)": "x = ctx.get('a')",
		"把 ctx 当对象用(下标)": "v = ctx['a']",
		"模块级 return":     "x = 1\nreturn x",
		"def run 包装":     "def run(ctx):\n    return {}",
		"eval 调用":        "x = eval('1')",
		"import os":      "import os\nx = 1",
	}
	for name, code := range cases {
		if err := validatePythonScript(code, 0); err == nil {
			t.Fatalf("%s 应被拒绝: %q", name, code)
		}
	}
}
