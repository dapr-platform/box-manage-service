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

// TestDynamicOutputTypesAllowCustomFields 回归线上误报：
// reasoning 节点的输出是运行时动态生成的（image_info、processing_time、model_type、
// 以及推理服务返回的 rois/cameraIp/detectRois 等），模板只声明了 5 个固定字段。
// 校验器按模板限制「未知端口」会导致用户引用真实字段时无法发布。
func TestDynamicOutputTypesAllowCustomFields(t *testing.T) {
	validator := NewWorkflowValidator([]*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 8}, TypeKey: "reasoning", TypeName: "Reasoning推理节点",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "rtsp_url", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "model_key", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "detections", Type: "array", Direction: models.VariableDirectionOutput},
				{KeyName: "detection_count", Type: "number", Direction: models.VariableDirectionOutput},
			}},
	})

	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "reasoning-002", "type": "reasoning", "properties": {
	      "key_name": "reasoning-002", "config": {},
	      "inputs": [
	        {"key_name": "rtsp_url", "type": "string", "default_value": "rtsp://x"},
	        {"key_name": "model_key", "type": "string", "default_value": "m"}
	      ],
	      "outputs": [
	        {"key_name": "detections", "type": "array"},
	        {"key_name": "detection_count", "type": "number"},
	        {"key_name": "rois", "type": "array"},
	        {"key_name": "cameraIp", "type": "string"},
	        {"key_name": "detectRois", "type": "array"},
	        {"key_name": "image_info", "type": "json"}
	      ]}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "reasoning-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "reasoning-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validator.Validate(document, simplifyGraph(document))

	for _, issue := range report.Errors {
		if issue.Code == "unknown_node_port" {
			t.Fatalf("reasoning 的输出字段是运行时动态生成的，不应报未知端口: %s", issue.Message)
		}
	}
}

// TestFixedNodeStillRejectsUnknownOutput 确认放宽只针对动态输出节点：
// image_annotator 这类输出固定的节点，写错输出名仍然要报错。
func TestFixedNodeStillRejectsUnknownOutput(t *testing.T) {
	validator := NewWorkflowValidator([]*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 24}, TypeKey: "image_annotator", TypeName: "图片标注",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "image", Type: "string", Direction: models.VariableDirectionOutput},
				{KeyName: "image_url", Type: "string", Direction: models.VariableDirectionOutput},
			}},
	})
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "image_annotator-002", "type": "image_annotator", "properties": {
	      "key_name": "image_annotator-002", "config": {}, "inputs": [],
	      "outputs": [{"key_name": "annotated_image", "type": "string"}]}},
	    {"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "inputs": [], "outputs": []}}
	  ],
	  "edges": [
	    {"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "image_annotator-002", "properties": {"condition_type": "none"}},
	    {"id": "line-002", "sourceNodeId": "image_annotator-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
	  ]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validator.Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Errors, "unknown_node_port")
}

// TestConfigKeyInInputsAutoFixedWithWarning 回归线上故障：
// python_script 的 script_content 被写进 inputs，它的默认值会顶掉 config 里的真实脚本，
// 导致「脚本明明填了 2818 字符，发布却报没有脚本内容」。
//
// 现在的处理是**自动修正 + 给出警告**：直接报 error 会把用户挡在发布之外，
// 而这类结构性错位用户往往无从下手；自动移除后 config 里的值即可生效。
func TestConfigKeyInInputsAutoFixedWithWarning(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "python_script-002", "type": "python_script", "properties": {
	      "key_name": "python_script-002",
	      "config": {"script_content": "print(1)\n"},
	      "inputs": [
	        {"key_name": "script_content", "type": "string", "default_value": null},
	        {"key_name": "timeout_seconds", "type": "string", "default_value": "10"}
	      ],
	      "outputs": []}},
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

	// 不应报 error（否则用户无法发布）
	for _, issue := range report.Errors {
		if issue.Code == "config_key_in_inputs" {
			t.Fatalf("应自动修正而不是报错: %s", issue.Message)
		}
	}
	// 但要给出警告，让用户知道数据被修正过
	assertIssueCode(t, report.Warnings, "config_key_in_inputs_autofixed")
}

// TestDropConfigKeysFromInputsCleans 确认保存时会静默清理配置项端口，
// 让 config 里的真实值能正常生效（否则用户无从下手）。
func TestDropConfigKeysFromInputsCleans(t *testing.T) {
	properties := map[string]interface{}{
		"config": map[string]interface{}{"script_content": "print(1)\n"},
		"inputs": []interface{}{
			map[string]interface{}{"key_name": "script_content", "default_value": nil},
			map[string]interface{}{"key_name": "timeout_seconds", "default_value": "10"},
			map[string]interface{}{"key_name": "count", "default_value": "0"},
		},
	}
	dropConfigKeysFromInputs(properties)

	inputs := asMapSlice(properties["inputs"])
	if len(inputs) != 1 {
		t.Fatalf("应只保留业务入参 count，实际剩 %d 个", len(inputs))
	}
	if asString(inputs[0]["key_name"]) != "count" {
		t.Fatalf("保留的应是 count，实际 %q", asString(inputs[0]["key_name"]))
	}
}

// TestValidationNormalizesConfigKeysBeforeCheck 锁定关键行为：
// 校验前会先把误放进 inputs 的配置项清理掉，因此这类老数据不会因为
// 「配置项写进输入参数」而无法发布，config 里的真实值也能正常生效。
//
// 这条对应线上故障：python_script-7ebb 的 config.script_content 有 2818 字符脚本，
// 但 inputs 里也有个空的 script_content，导致发布被拦。
func TestValidationNormalizesConfigKeysBeforeCheck(t *testing.T) {
	raw := `{
	  "nodes": [
	    {"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
	    {"id": "python_script-002", "type": "python_script", "properties": {
	      "key_name": "python_script-002",
	      "config": {"script_content": "print(1)\n"},
	      "inputs": [
	        {"key_name": "script_content", "type": "string", "default_value": null},
	        {"key_name": "timeout_seconds", "type": "string", "default_value": "10"},
	        {"key_name": "count", "type": "number", "default_value": "0"}
	      ],
	      "outputs": []}},
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

	// 清理后不应再报「配置项写进输入参数」
	for _, issue := range report.Errors {
		if issue.Code == "config_key_in_inputs" {
			t.Fatalf("校验前应已自动清理，不该再报该错误: %s", issue.Message)
		}
		if issue.Code == "python_script_empty" {
			t.Fatalf("config 里有脚本，不该报空脚本: %s", issue.Message)
		}
	}
	// 文档对象本身也应被修正（返回给前端的图是干净的）
	for _, node := range document.Nodes {
		if node.Type != "python_script" {
			continue
		}
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			keyName := asString(port["key_name"])
			if keyName == "script_content" || keyName == "timeout_seconds" {
				t.Fatalf("节点 inputs 里的配置项 %q 应被清理", keyName)
			}
		}
		config, _ := node.Properties["config"].(map[string]interface{})
		if asString(config["script_content"]) == "" {
			t.Fatal("config 里的脚本内容不应被破坏")
		}
	}
}

// TestDropConfigKeysMigratesScriptToConfig 锁定「迁移而非丢弃」行为：
// 前端「数据类型=脚本」的输入参数把脚本内容存在端口的 default_value 里，
// 而 python_script 只从 config.script_content 读脚本。清理端口时必须把脚本搬进 config，
// 否则用户明明填了脚本，校验仍会报「没有脚本内容」（线上 python_script-7ebb）。
func TestDropConfigKeysMigratesScriptToConfig(t *testing.T) {
	script := "import json\ndef main():\n    print('hi')\n"
	properties := map[string]interface{}{
		"config": map[string]interface{}{},
		"inputs": []interface{}{
			map[string]interface{}{
				"key_name":      "script_content",
				"name":          "脚本内容",
				"type":          "script",
				"default":       script,
				"default_value": nil,
			},
		},
		"outputs": []interface{}{},
	}

	removed := dropConfigKeysFromInputs(properties)
	if len(removed) != 1 || removed[0] != "script_content" {
		t.Fatalf("应移除 script_content，实际 %v", removed)
	}
	if len(asMapSlice(properties["inputs"])) != 0 {
		t.Fatalf("inputs 应被清空")
	}
	config, _ := properties["config"].(map[string]interface{})
	if asString(config["script_content"]) != script {
		t.Fatalf("脚本内容应迁入 config，实际 %q", asString(config["script_content"]))
	}

	// 迁移后校验不应再报「没有脚本内容」
	node := &graphNode{ID: "python_script-7ebb", Type: "python_script", Properties: properties}
	validator := &WorkflowValidator{}
	for _, issue := range validator.validatePythonOutputs(map[string]*graphNode{node.ID: node}) {
		if issue.Code == "python_script_empty" {
			t.Fatalf("迁移后不应报空脚本: %s", issue.Message)
		}
	}
}

// TestDropConfigKeysKeepsExistingConfig 锁定「不覆盖已有配置」行为：
// config 里已有非空真实值时，端口的旧默认值不能把它顶掉。
func TestDropConfigKeysKeepsExistingConfig(t *testing.T) {
	properties := map[string]interface{}{
		"config": map[string]interface{}{"script_content": "real script"},
		"inputs": []interface{}{
			map[string]interface{}{"key_name": "script_content", "default_value": "stale"},
		},
	}
	dropConfigKeysFromInputs(properties)
	config, _ := properties["config"].(map[string]interface{})
	if asString(config["script_content"]) != "real script" {
		t.Fatalf("config 里的真实值被覆盖了: %q", asString(config["script_content"]))
	}
}

// TestDropConfigKeysSkipsBlankPortValue 锁定「空值不迁移」：
// 端口默认值为 null/空串时不写 config，避免把内置参数配置写成空值。
func TestDropConfigKeysSkipsBlankPortValue(t *testing.T) {
	properties := map[string]interface{}{
		"config": map[string]interface{}{},
		"inputs": []interface{}{
			map[string]interface{}{"key_name": "timeout_seconds", "default_value": nil},
			map[string]interface{}{"key_name": "python_bin", "default_value": "   "},
		},
	}
	dropConfigKeysFromInputs(properties)
	config, _ := properties["config"].(map[string]interface{})
	for _, key := range []string{"timeout_seconds", "python_bin"} {
		if _, exists := config[key]; exists {
			t.Fatalf("空值 %q 不应迁入 config", key)
		}
	}
}
