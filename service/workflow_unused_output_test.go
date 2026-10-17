package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// unusedOutputGraph 构造一个含「声明但未使用输出」的编排。
// 对应线上真实故障：AI 生成的编排声明了 need_notify 并在脚本里置位，
// 但没有任何连线或引用使用它，导致「恢复通知」永远发不出去。
const unusedOutputGraph = `{
  "nodes": [
    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001",
      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "detections"}], "outputs": []}},
    {"id": "python_script-002", "type": "python_script", "properties": {
      "key_name": "python_script-002",
      "config": {"script_content": "send_alarm = False\nneed_notify = True\nalarm_message = 'x'"},
      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"}],
      "outputs": [
        {"key_name": "send_alarm", "type": "boolean"},
        {"key_name": "need_notify", "type": "boolean"},
        {"key_name": "alarm_message", "type": "string"}
      ]}},
    {"id": "wechat_work-003", "type": "wechat_work", "properties": {
      "key_name": "wechat_work-003", "config": {},
      "inputs": [
        {"key_name": "key", "type": "string", "default_value": "k"},
        {"key_name": "msgtype", "type": "string", "default_value": "text"},
        {"key_name": "body", "type": "json", "default_value": "{\"content\": \"{python_script-002.alarm_message}\"}"}
      ], "outputs": []}},
    {"id": "end-004", "type": "end", "properties": {"key_name": "end-004", "inputs": [], "outputs": []}}
  ],
  "edges": [
    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-002", "properties": {"condition_type": "none"}},
    {"id": "line-002", "sourceNodeId": "python_script-002", "targetNodeId": "wechat_work-003",
      "properties": {"condition_type": "expression", "condition_expression": "{python_script-002.send_alarm}", "condition_expression_view": "报警时"}},
    {"id": "line-003", "sourceNodeId": "wechat_work-003", "targetNodeId": "end-004", "properties": {"condition_type": "none"}},
    {"id": "line-004", "sourceNodeId": "python_script-002", "targetNodeId": "end-004",
      "properties": {"condition_type": "expression", "condition_expression": "not {python_script-002.send_alarm}", "condition_expression_view": "不报警时"}}
  ]
}`

// TestUnusedOutputDetected 确认「声明但从不使用」的输出会被检出。
func TestUnusedOutputDetected(t *testing.T) {
	document, err := parseGraph(json.RawMessage(unusedOutputGraph))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	var found string
	for _, warning := range report.Warnings {
		if warning.Code == "unused_output" && strings.Contains(warning.Message, "need_notify") {
			found = warning.Message
		}
	}
	if found == "" {
		t.Fatalf("应检出 need_notify 未使用，实际警告: %+v", report.Warnings)
	}
	if !strings.Contains(found, "python_script-002") {
		t.Fatalf("警告应指向声明该输出的节点，实际: %s", found)
	}

	// send_alarm 参与了连线条件、alarm_message 被企微 body 引用，都不应被误报
	for _, warning := range report.Warnings {
		if warning.Code == "unused_output" && (strings.Contains(warning.Message, "send_alarm") ||
			strings.Contains(warning.Message, "alarm_message")) {
			t.Fatalf("已使用的输出被误报: %s", warning.Message)
		}
	}
}

// TestUnusedOutputIsWarningNotError 确认该规则只告警、不阻断发布，
// 否则会挡住「先声明输出、之后再接下游」的正常编辑流程。
func TestUnusedOutputIsWarningNotError(t *testing.T) {
	document, err := parseGraph(json.RawMessage(unusedOutputGraph))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	for _, issue := range report.Errors {
		if issue.Code == "unused_output" {
			t.Fatalf("unused_output 不应是 error: %s", issue.Message)
		}
	}
}

// TestUnusedOutputSkipsFixedTemplateNodes 确认固定契约节点的内置输出不会被报未使用。
// detection_filter 声明的 matched_detections 等输出在实际编排里常常用不到，属于正常情况。
func TestUnusedOutputSkipsFixedTemplateNodes(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001",
	      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "detections"}], "outputs": []}},
	    {"id": "detection_filter-002", "type": "detection_filter", "properties": {
	      "key_name": "detection_filter-002", "config": {},
	      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"}],
	      "outputs": [
	        {"key_name": "matched_detections", "type": "array"},
	        {"key_name": "match_count", "type": "number"},
	        {"key_name": "has_match", "type": "boolean"},
	        {"key_name": "image", "type": "string"}
	      ]}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "detection_filter-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "detection_filter-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	for _, warning := range report.Warnings {
		if warning.Code == "unused_output" {
			t.Fatalf("固定契约节点的内置输出不应被报未使用: %s", warning.Message)
		}
	}
}

// TestUnusedOutputSkipsTemplateBuiltinOutputs 确认模板自带的标准输出不会被报未使用。
//
// 线上实测：生成结果里对 task_get_context 的 context、task_set_context 的 saved
// 都报了「未使用」，但这两个是模板定义的内置输出，编排不引用属于正常情况。
// 规则应当只针对**模型自己额外声明**的输出。
func TestUnusedOutputSkipsTemplateBuiltinOutputs(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001",
	      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "detections"}], "outputs": []}},
	    {"id": "task_get_context-002", "type": "task_get_context", "properties": {
	      "key_name": "task_get_context-002", "config": {}, "inputs": [],
	      "outputs": [{"key_name": "context", "type": "json"}]}},
	    {"id": "task_set_context-003", "type": "task_set_context", "properties": {
	      "key_name": "task_set_context-003", "config": {},
	      "inputs": [{"key_name": "count", "type": "string", "default_value": "1"}],
	      "outputs": [{"key_name": "saved", "type": "boolean"}]}},
	    {"id": "end-004", "type": "end", "properties": {"key_name": "end-004", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "task_get_context-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "task_get_context-002", "targetNodeId": "task_set_context-003", "properties": {"condition_type": "none"}},
	    {"id": "line-003", "sourceNodeId": "task_set_context-003", "targetNodeId": "end-004", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	for _, warning := range report.Warnings {
		if warning.Code != "unused_output" {
			continue
		}
		if strings.Contains(warning.Message, `"context"`) || strings.Contains(warning.Message, `"saved"`) {
			t.Fatalf("模板内置输出不应被报未使用: %s", warning.Message)
		}
	}
}

// TestUnusedOutputStillCatchesCustomOnContextNode 确认豁免只针对内置输出，
// 模型在上下文节点上自己声明的额外输出，未使用时仍要报出来。
func TestUnusedOutputStillCatchesCustomOnContextNode(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001",
	      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "detections"}], "outputs": []}},
	    {"id": "task_get_context-002", "type": "task_get_context", "properties": {
	      "key_name": "task_get_context-002", "config": {}, "inputs": [],
	      "outputs": [
	        {"key_name": "context", "type": "json"},
	        {"key_name": "custom_state", "type": "number"}
	      ]}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "task_get_context-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "task_get_context-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	var found bool
	for _, warning := range report.Warnings {
		if warning.Code == "unused_output" && strings.Contains(warning.Message, "custom_state") {
			found = true
		}
		if warning.Code == "unused_output" && strings.Contains(warning.Message, `"context"`) {
			t.Fatalf("内置输出不该被报: %s", warning.Message)
		}
	}
	if !found {
		t.Fatalf("自定义输出 custom_state 未被使用时应当报出，实际警告: %+v", report.Warnings)
	}
}
