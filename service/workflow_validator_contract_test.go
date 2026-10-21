package service

import (
	"encoding/json"
	"testing"

	"box-manage-service/models"
)

// validatorWithDefaultedTemplates 构造带「模板默认值」的校验器，
// 用于验证 required 检查会尊重模板兜底值。
func validatorWithDefaultedTemplates() *WorkflowValidator {
	return NewWorkflowValidator([]*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 9}, TypeKey: "python_script", TypeName: "Python脚本",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 17}, TypeKey: "wechat_work", TypeName: "企业微信推送",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "key", Type: "string", Direction: models.VariableDirectionInput},
				// msgtype 模板默认 "text"，box-app 里也有 safe_str(inputs,"msgtype","text")
				{KeyName: "msgtype", Type: "string", Direction: models.VariableDirectionInput,
					DefaultValue: models.JSONValue{Data: "text"}},
				// body 模板默认为空，仍需编排给出
				{KeyName: "body", Type: "json", Direction: models.VariableDirectionInput,
					DefaultValue: models.JSONValue{Data: ""}},
			}},
	})
}

// TestRequiredInputSkipsTemplateDefault 回归误报：
// wechat_work.msgtype 在模板里有默认值 "text"，运行时也有兜底，
// 不应报「必填输入既没有引用上游也没有默认值」。线上 264 份编排里有 217 次这类误报。
func TestRequiredInputSkipsTemplateDefault(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "wechat_work-002", "type": "wechat_work", "properties": {
	      "key_name": "wechat_work-002", "config": {},
	      "inputs": [
	        {"key_name": "key", "type": "string", "required": true, "default_value": "k"},
	        {"key_name": "msgtype", "type": "string", "required": true, "default_value": ""}
	      ], "outputs": []}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "wechat_work-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "wechat_work-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorWithDefaultedTemplates().Validate(document, simplifyGraph(document))

	for _, issue := range report.Errors {
		if issue.Code == "required_input_missing" && issue.NodeID == "wechat_work-002" {
			t.Fatalf("msgtype 模板有默认值，不应报必填缺失: %s", issue.Message)
		}
	}
}

// TestRequiredInputStillReportsWhenNoTemplateDefault 确认放宽没有过头：
// 模板没有默认值的必填参数缺失时仍然报错。
func TestRequiredInputStillReportsWhenNoTemplateDefault(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "wechat_work-002", "type": "wechat_work", "properties": {
	      "key_name": "wechat_work-002", "config": {},
	      "inputs": [
	        {"key_name": "key", "type": "string", "required": true, "default_value": ""}
	      ], "outputs": []}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "wechat_work-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "wechat_work-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorWithDefaultedTemplates().Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Errors, "required_input_missing")
}

// TestPythonReservedOutputPortIgnored 回归误报：
// "output" 是平台预留端口，box-app 生成输出捕获代码时显式跳过
// （node_executor.cpp:2239），因此不能要求脚本为它赋值。
// 线上 264 份编排里有 165 次这类误报。
func TestPythonReservedOutputPortIgnored(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "python_script-002", "type": "python_script", "properties": {
	      "key_name": "python_script-002",
	      "config": {"script_content": "alarm_lock = True\nstatus = 'ok'\n"},
	      "inputs": [],
	      "outputs": [
	        {"key_name": "output", "type": "string"},
	        {"key_name": "status", "type": "string"}
	      ]}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "python_script-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorWithDefaultedTemplates().Validate(document, simplifyGraph(document))

	for _, issue := range append(append([]ValidationIssue{}, report.Errors...), report.Warnings...) {
		if issue.Code == "python_output_unassigned" && issue.NodeID == "python_script-002" {
			t.Fatalf(`预留端口 "output" 不应被要求赋值: %s`, issue.Message)
		}
	}
}

// TestNonUpstreamReferenceIsWarningNotError 锁定降级语义：
// box-app 的 resolveReference 按名字查全局 node_outputs_（workflow_context.cpp），
// 运行时不校验上下游，因此「引用非上游节点」能取到值，只算 warning。
func TestNonUpstreamReferenceIsWarningNotError(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "python_script-002", "type": "python_script", "properties": {
	      "key_name": "python_script-002", "config": {"script_content": "a = 1\n"},
	      "inputs": [], "outputs": [{"key_name": "a", "type": "number"}]}},
	    {"id": "python_script-003", "type": "python_script", "properties": {
	      "key_name": "python_script-003", "config": {"script_content": "b = a\n"},
	      "inputs": [{"key_name": "a", "type": "number", "ref_key_name": "python_script-002.a"}],
	      "outputs": [{"key_name": "b", "type": "number"}]}},
	    {"id": "end-004", "type": "end", "properties": {"key_name": "end-004", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-003", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "python_script-003", "targetNodeId": "end-004", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorWithDefaultedTemplates().Validate(document, simplifyGraph(document))

	assertIssueCode(t, report.Warnings, "reference_not_upstream")
	for _, issue := range report.Errors {
		if issue.Code == "reference_not_upstream" {
			t.Fatalf("reference_not_upstream 不应是 error: %s", issue.Message)
		}
	}
}
