package service

import (
	"encoding/json"
	"testing"
)

// contextMismatchGraph 复现线上问题：task_set_context 的输入全部写成 string，
// 但引用的 python 输出是 boolean / number，产生一堆类型不匹配警告。
const contextMismatchGraph = `{
  "nodes": [
    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001",
      "inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "detections"}], "outputs": []}},
    {"id": "python_script-002", "type": "python_script", "properties": {
      "key_name": "python_script-002",
      "config": {"script_content": "is_alarmed = True\ncount = 5\nsend_alarm = is_alarmed\n"},
      "inputs": [],
      "outputs": [
        {"key_name": "is_alarmed", "type": "boolean"},
        {"key_name": "count", "type": "number"},
        {"key_name": "send_alarm", "type": "boolean"}
      ]}},
    {"id": "task_set_context-003", "type": "task_set_context", "properties": {
      "key_name": "task_set_context-003", "config": {},
      "inputs": [
        {"key_name": "is_alarmed", "type": "string", "default_value": "", "ref_key_name": "python_script-002.is_alarmed"},
        {"key_name": "count", "type": "string", "default_value": "", "ref_key_name": "python_script-002.count"}
      ],
      "outputs": [{"key_name": "saved", "type": "boolean"}]}},
    {"id": "end-004", "type": "end", "properties": {"key_name": "end-004", "inputs": [], "outputs": []}}
  ],
  "edges": [
    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-002", "properties": {"condition_type": "none"}},
    {"id": "line-002", "sourceNodeId": "python_script-002", "targetNodeId": "task_set_context-003", "properties": {"condition_type": "none"}},
    {"id": "line-003", "sourceNodeId": "task_set_context-003", "targetNodeId": "end-004", "properties": {"condition_type": "none"}}
  ]
}`

func countTypeMismatch(report *ValidationReport) int {
	count := 0
	for _, warning := range report.Warnings {
		if warning.Code == "reference_type_mismatch" {
			count++
		}
	}
	return count
}

// TestAlignContextNodePortTypesRemovesMismatch 确认类型对齐能消掉上下文节点的类型不匹配警告。
func TestAlignContextNodePortTypesRemovesMismatch(t *testing.T) {
	document, err := parseGraph(json.RawMessage(contextMismatchGraph))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	validator := validatorForRealWorldGraph()

	before := validator.Validate(document, simplifyGraph(document))
	if countTypeMismatch(before) == 0 {
		t.Fatalf("用例应产生类型不匹配警告，实际: %+v", before.Warnings)
	}

	alignContextNodePortTypes(document)

	after := validator.Validate(document, simplifyGraph(document))
	if got := countTypeMismatch(after); got != 0 {
		t.Fatalf("对齐后不应再有类型不匹配警告，仍有 %d 条: %+v", got, after.Warnings)
	}

	// 类型确实被改成了上游的真实类型
	for _, node := range document.Nodes {
		if node.Type != "task_set_context" {
			continue
		}
		want := map[string]string{"is_alarmed": "boolean", "count": "number"}
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			keyName := asString(port["key_name"])
			if expected, ok := want[keyName]; ok && asString(port["type"]) != expected {
				t.Fatalf("端口 %s 类型应为 %s，实际 %s", keyName, expected, asString(port["type"]))
			}
		}
	}
}

// TestAlignKeepsOtherNodeTypesUntouched 确认对齐只作用于上下文节点，
// 不会顺手改掉 python_script 等节点自己声明的端口类型。
func TestAlignKeepsOtherNodeTypesUntouched(t *testing.T) {
	document, err := parseGraph(json.RawMessage(contextMismatchGraph))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	alignContextNodePortTypes(document)

	for _, node := range document.Nodes {
		if node.Type != "python_script" {
			continue
		}
		for _, port := range asMapSlice(node.Properties["outputs"]) {
			switch asString(port["key_name"]) {
			case "is_alarmed", "send_alarm":
				if asString(port["type"]) != "boolean" {
					t.Fatalf("python 节点输出类型不应被改动: %+v", port)
				}
			case "count":
				if asString(port["type"]) != "number" {
					t.Fatalf("python 节点输出类型不应被改动: %+v", port)
				}
			}
		}
	}
}

// TestAlignIsNoopOnEmptyGraph 边界：空图不应 panic。
func TestAlignIsNoopOnEmptyGraph(t *testing.T) {
	alignContextNodePortTypes(nil)
	alignContextNodePortTypes(&graphDocument{})
}
