package service

import (
	"box-manage-service/models"
	"encoding/json"
	"testing"
)

// buildValidatorWithPythonTemplate 构造带 python_script 模板（含占位变量）的校验器。
// 模板里的 input/output/script_content/timeout_seconds 只是占位示例，不是契约。
func buildValidatorWithPythonTemplate() *WorkflowValidator {
	return NewWorkflowValidator([]*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 9}, TypeKey: "python_script", TypeName: "PythonScript脚本节点",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "input", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "output", Type: "string", Direction: models.VariableDirectionOutput},
				{KeyName: "script_content", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "timeout_seconds", Type: "string", Direction: models.VariableDirectionInput},
			}},
		{BaseModel: models.BaseModel{ID: 20}, TypeKey: "detection_filter", TypeName: "检测结果过滤",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "detections", Type: "array", Direction: models.VariableDirectionInput},
				{KeyName: "class_id", Type: "number", Direction: models.VariableDirectionInput},
				{KeyName: "score", Type: "number", Direction: models.VariableDirectionInput},
				{KeyName: "matched_detections", Type: "array", Direction: models.VariableDirectionOutput},
				{KeyName: "match_count", Type: "number", Direction: models.VariableDirectionOutput},
				{KeyName: "has_match", Type: "boolean", Direction: models.VariableDirectionOutput},
			}},
	})
}

// TestPythonScriptPortsAreUserDefined 回归：Python 节点的入参/出参是用户自由定义的，
// 不能因为「不在模板定义中」报错——模板里的 input/output 只是占位示例。
//
// 这条对应真实故障：AI 生成的编排里 python_script 用了
// consecutive_person_count / is_intrusion_detected 等自定义变量，
// 被误报 32 个「不在模板定义中」错误，导致生成结果永远无法通过校验。
func TestPythonScriptPortsAreUserDefined(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [{"key_name": "detections", "type": "array"}], "outputs": []}},
			{"id": "python_script-005", "type": "python_script", "properties": {
				"key_name": "python_script-005", "name": "状态机",
				"config": {"script_content": "count = 1"},
				"inputs": [
					{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"},
					{"key_name": "match_count", "type": "number", "default_value": 0}
				],
				"outputs": [
					{"key_name": "is_intrusion_detected", "type": "boolean"},
					{"key_name": "is_alarmed", "type": "boolean"},
					{"key_name": "consecutive_person_count", "type": "number"},
					{"key_name": "alarm_message", "type": "string"}
				]}},
			{"id": "end-006", "type": "end", "properties": {"key_name": "end-006", "inputs": [], "outputs": []}}
		],
		"edges": [
			{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-005", "properties": {"condition_type": "none"}},
			{"id": "line-002", "sourceNodeId": "python_script-005", "targetNodeId": "end-006", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := buildValidatorWithPythonTemplate().Validate(document, simplifyGraph(document))

	for _, issue := range report.Errors {
		if issue.Code == "unknown_node_port" {
			t.Fatalf("python_script 的端口不应被模板约束，却报错: %s", issue.Message)
		}
	}
	if !report.Valid {
		t.Fatalf("自定义端口的 Python 编排应通过校验，实际错误: %+v", report.Errors)
	}
}

// TestFixedPortNodeStillEnforced 确认豁免只针对自由端口节点，
// 固定契约节点（如 detection_filter）仍然严格检查。
func TestFixedPortNodeStillEnforced(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [{"key_name": "detections", "type": "array"}], "outputs": []}},
			{"id": "detection_filter-004", "type": "detection_filter", "properties": {
				"key_name": "detection_filter-004",
				"config": {},
				"inputs": [
					{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"},
					{"key_name": "not_in_template", "type": "string", "default_value": "x"}
				],
				"outputs": []}},
			{"id": "end-006", "type": "end", "properties": {"key_name": "end-006", "inputs": [], "outputs": []}}
		],
		"edges": [
			{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "detection_filter-004", "properties": {"condition_type": "none"}},
			{"id": "line-002", "sourceNodeId": "detection_filter-004", "targetNodeId": "end-006", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := buildValidatorWithPythonTemplate().Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Errors, "unknown_node_port")
}

// TestDuplicatePortKeyStillDetected 自由端口节点仍要检查 key_name 重复：
// 重复会让 box-app 注入/捕获变量时互相覆盖。
func TestDuplicatePortKeyStillDetected(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "python_script-005", "type": "python_script", "properties": {
				"key_name": "python_script-005",
				"config": {"script_content": "count = 1"},
				"inputs": [
					{"key_name": "count", "type": "number", "default_value": 0},
					{"key_name": "count", "type": "number", "default_value": 1}
				],
				"outputs": [{"key_name": "count", "type": "number"}]}},
			{"id": "end-006", "type": "end", "properties": {"key_name": "end-006", "inputs": [], "outputs": []}}
		],
		"edges": [
			{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-005", "properties": {"condition_type": "none"}},
			{"id": "line-002", "sourceNodeId": "python_script-005", "targetNodeId": "end-006", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := buildValidatorWithPythonTemplate().Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Errors, "duplicate_node_port")
}
