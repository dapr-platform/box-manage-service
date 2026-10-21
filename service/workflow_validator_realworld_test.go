package service

import (
	"box-manage-service/models"
	"encoding/json"
	"strings"
	"testing"
)

// validatorForRealWorldGraph 构造一套与线上一致的节点模板（ID 与 type_key 取自实际数据）。
func validatorForRealWorldGraph() *WorkflowValidator {
	return NewWorkflowValidator([]*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 9}, TypeKey: "python_script", TypeName: "PythonScript脚本节点",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "input", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "output", Type: "string", Direction: models.VariableDirectionOutput},
			}},
		{BaseModel: models.BaseModel{ID: 20}, TypeKey: "detection_filter", TypeName: "检测结果过滤",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "detections", Type: "array", Direction: models.VariableDirectionInput},
				{KeyName: "class_id", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "score", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "image", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "matched_detections", Type: "array", Direction: models.VariableDirectionOutput},
				{KeyName: "match_count", Type: "number", Direction: models.VariableDirectionOutput},
				{KeyName: "has_match", Type: "boolean", Direction: models.VariableDirectionOutput},
				{KeyName: "image", Type: "string", Direction: models.VariableDirectionOutput},
			}},
		{BaseModel: models.BaseModel{ID: 24}, TypeKey: "image_annotator", TypeName: "图片标注",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "image", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "detections", Type: "array", Direction: models.VariableDirectionInput},
				// 真实输出是 image / image_url / detections，没有 annotated_image
				{KeyName: "image", Type: "string", Direction: models.VariableDirectionOutput},
				{KeyName: "image_url", Type: "string", Direction: models.VariableDirectionOutput},
				{KeyName: "detections", Type: "array", Direction: models.VariableDirectionOutput},
			}},
		{BaseModel: models.BaseModel{ID: 17}, TypeKey: "wechat_work", TypeName: "企业微信推送",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "key", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "msgtype", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "body", Type: "json", Direction: models.VariableDirectionInput},
			}},
		{BaseModel: models.BaseModel{ID: 27}, TypeKey: "task_set_context", TypeName: "任务存上下文",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "saved", Type: "boolean", Direction: models.VariableDirectionOutput},
			}},
		{BaseModel: models.BaseModel{ID: 28}, TypeKey: "task_get_context", TypeName: "任务取上下文",
			GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "context", Type: "json", Direction: models.VariableDirectionOutput},
			}},
	})
}

// realWorldGraphJSON 是线上「车辆禁停区检测报警编排」的实际返回（结构保持原样）。
//
// 这份数据一次性暴露了三类**校验器误报**，已全部修复：
//  1. start 节点用裸 ref_key_name（detections / image_base64）接收推理上下文
//  2. task_get_context 的 outputs 上带裸 ref_key_name（is_alarmed 等上下文键）
//  3. python_script 的 script_content 里有 Python f-string（{consecutive_vehicle_count}），
//     而 script_content 根本不参与编排插值
//
// 同时保留了三个**真实错误**，修复不能把它们一起放过：
//   - image_annotator 用了不存在的输出名 annotated_image（真实是 image）
//   - image_annotator 是死路，到不了结束节点
//   - wechat_work-009 引用了非上游的 image_annotator-007
const realWorldGraphJSON = `{
  "nodes": [
    {"id": "start-001", "type": "start", "properties": {
      "key_name": "start-001", "name": "开始", "config": {},
      "inputs": [
        {"name": "推理结果", "key_name": "detections", "type": "string", "required": false, "default_value": "", "ref_key_name": "detections"},
        {"name": "推理图片", "key_name": "image_base64", "type": "string", "required": false, "default_value": "", "ref_key_name": "image_base64"},
        {"name": "企业微信key", "key_name": "qiyeweixin_key", "type": "string", "required": false, "default_value": "", "ref_key_name": ""}
      ], "outputs": []}},
    {"id": "end-002", "type": "end", "properties": {"key_name": "end-002", "name": "结束", "config": {}, "inputs": [], "outputs": []}},
    {"id": "task_get_context-003", "type": "task_get_context", "properties": {
      "key_name": "task_get_context-003", "name": "任务取上下文", "config": {}, "inputs": [],
      "outputs": [
        {"name": "上下文内容", "key_name": "context", "type": "json", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "是否已发送过报警", "key_name": "is_alarmed", "type": "boolean", "required": false, "default_value": "false", "ref_key_name": "is_alarmed"},
        {"name": "连续检测到车辆的帧数", "key_name": "consecutive_vehicle_count", "type": "number", "required": false, "default_value": "0", "ref_key_name": "consecutive_vehicle_count"},
        {"name": "连续未检测到车辆的帧数", "key_name": "consecutive_no_vehicle_count", "type": "number", "required": false, "default_value": "0", "ref_key_name": "consecutive_no_vehicle_count"}
      ]}},
    {"id": "detection_filter-004", "type": "detection_filter", "properties": {
      "key_name": "detection_filter-004", "name": "检测结果过滤", "config": {},
      "inputs": [
        {"name": "检测结果", "key_name": "detections", "type": "array", "required": true, "default_value": null, "ref_key_name": "start-001.detections"},
        {"name": "目标类别ID", "key_name": "class_id", "type": "string", "required": false, "default_value": "", "ref_key_name": ""},
        {"name": "置信度阈值", "key_name": "score", "type": "string", "required": false, "default_value": "", "ref_key_name": ""},
        {"name": "推理图片", "key_name": "image", "type": "string", "required": false, "default_value": null, "ref_key_name": "start-001.image_base64"}
      ],
      "outputs": [
        {"name": "匹配结果", "key_name": "matched_detections", "type": "array", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "匹配数量", "key_name": "match_count", "type": "number", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "有匹配", "key_name": "has_match", "type": "boolean", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "推理图片", "key_name": "image", "type": "string", "required": false, "default_value": null, "ref_key_name": ""}
      ]}},
    {"id": "python_script-005", "type": "python_script", "properties": {
      "key_name": "python_script-005", "name": "车辆禁停检测逻辑",
      "config": {
        "python_bin": "python3", "input_params": [], "output_params": [],
        "script_content": "import json\nALARM_THRESHOLD = 30\nif consecutive_vehicle_count >= ALARM_THRESHOLD:\n    alarm_message = f\"检测到车辆停留已达{consecutive_vehicle_count}帧\"\n    is_alarmed = True\n"
      },
      "inputs": [
        {"name": "是否已发送过报警", "key_name": "is_alarmed", "type": "boolean", "required": false, "default_value": null, "ref_key_name": "task_get_context-003.is_alarmed"},
        {"name": "连续检测到车辆的帧数", "key_name": "consecutive_vehicle_count", "type": "number", "required": false, "default_value": null, "ref_key_name": "task_get_context-003.consecutive_vehicle_count"},
        {"name": "连续未检测到车辆的帧数", "key_name": "consecutive_no_vehicle_count", "type": "number", "required": false, "default_value": null, "ref_key_name": "task_get_context-003.consecutive_no_vehicle_count"},
        {"name": "当前帧检测到的车辆数量", "key_name": "match_count", "type": "number", "required": false, "default_value": null, "ref_key_name": "detection_filter-004.match_count"}
      ],
      "outputs": [
        {"name": "连续检测到车辆的帧数", "key_name": "consecutive_vehicle_count", "type": "number", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "连续未检测到车辆的帧数", "key_name": "consecutive_no_vehicle_count", "type": "number", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "是否已发送过报警", "key_name": "is_alarmed", "type": "boolean", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "是否需要发送报警", "key_name": "send_alarm", "type": "boolean", "required": false, "default_value": null, "ref_key_name": ""},
        {"name": "报警消息", "key_name": "alarm_message", "type": "string", "required": false, "default_value": null, "ref_key_name": ""}
      ]}},
    {"id": "task_set_context-006", "type": "task_set_context", "properties": {
      "key_name": "task_set_context-006", "name": "任务存上下文", "config": {},
      "inputs": [
        {"name": "是否已发送过报警", "key_name": "is_alarmed", "type": "string", "required": false, "default_value": "", "ref_key_name": "python_script-005.is_alarmed"},
        {"name": "连续检测到车辆的帧数", "key_name": "consecutive_vehicle_count", "type": "string", "required": false, "default_value": "", "ref_key_name": "python_script-005.consecutive_vehicle_count"},
        {"name": "连续未检测到车辆的帧数", "key_name": "consecutive_no_vehicle_count", "type": "string", "required": false, "default_value": "", "ref_key_name": "python_script-005.consecutive_no_vehicle_count"}
      ],
      "outputs": [{"name": "保存成功", "key_name": "saved", "type": "boolean", "required": false, "default_value": null, "ref_key_name": ""}]}},
    {"id": "image_annotator-007", "type": "image_annotator", "properties": {
      "key_name": "image_annotator-007", "name": "图片标注", "config": {},
      "inputs": [
        {"name": "检测结果", "key_name": "detections", "type": "array", "required": false, "default_value": null, "ref_key_name": "detection_filter-004.matched_detections"},
        {"name": "原始图片", "key_name": "image", "type": "string", "required": false, "default_value": null, "ref_key_name": "detection_filter-004.image"}
      ],
      "outputs": [{"name": "标注结果图片", "key_name": "annotated_image", "type": "string", "required": false, "default_value": null, "ref_key_name": ""}]}},
    {"id": "wechat_work-008", "type": "wechat_work", "properties": {
      "key_name": "wechat_work-008", "name": "企业微信推送-文字", "config": {},
      "inputs": [
        {"name": "Webhook Key", "key_name": "key", "type": "string", "required": true, "default_value": null, "ref_key_name": "start-001.qiyeweixin_key"},
        {"name": "消息类型", "key_name": "msgtype", "type": "string", "required": true, "default_value": "text", "ref_key_name": ""},
        {"name": "消息内容体", "key_name": "body", "type": "json", "required": true, "default_value": "{\"content\": \"{python_script-005.alarm_message}\"}", "ref_key_name": ""}
      ], "outputs": []}},
    {"id": "wechat_work-009", "type": "wechat_work", "properties": {
      "key_name": "wechat_work-009", "name": "企业微信推送-图片", "config": {},
      "inputs": [
        {"name": "Webhook Key", "key_name": "key", "type": "string", "required": true, "default_value": null, "ref_key_name": "start-001.qiyeweixin_key"},
        {"name": "消息类型", "key_name": "msgtype", "type": "string", "required": true, "default_value": "image", "ref_key_name": ""},
        {"name": "消息内容体", "key_name": "body", "type": "json", "required": true, "default_value": "{\"image\": {\"base64\": \"{image_annotator-007.annotated_image}\"}}", "ref_key_name": ""}
      ], "outputs": []}}
  ],
  "edges": [
    {"id": "edge-001", "sourceNodeId": "start-001", "targetNodeId": "task_get_context-003", "properties": {"condition_type": "none"}},
    {"id": "edge-002", "sourceNodeId": "task_get_context-003", "targetNodeId": "detection_filter-004", "properties": {"condition_type": "none"}},
    {"id": "edge-003", "sourceNodeId": "detection_filter-004", "targetNodeId": "python_script-005", "properties": {"condition_type": "none"}},
    {"id": "edge-004", "sourceNodeId": "python_script-005", "targetNodeId": "task_set_context-006", "properties": {"condition_type": "none"}},
    {"id": "edge-005", "sourceNodeId": "detection_filter-004", "targetNodeId": "image_annotator-007", "properties": {"condition_type": "none"}},
    {"id": "edge-006", "sourceNodeId": "task_set_context-006", "targetNodeId": "wechat_work-008", "properties": {"condition_type": "expression", "condition_expression": "{python_script-005.send_alarm}", "condition_expression_view": "send_alarm == true", "logic_type": "and"}},
    {"id": "edge-007", "sourceNodeId": "wechat_work-008", "targetNodeId": "wechat_work-009", "properties": {"condition_type": "none"}},
    {"id": "edge-008", "sourceNodeId": "wechat_work-009", "targetNodeId": "end-002", "properties": {"condition_type": "none"}},
    {"id": "edge-009", "sourceNodeId": "task_set_context-006", "targetNodeId": "end-002", "properties": {"condition_type": "expression", "condition_expression": "{python_script-005.send_alarm}", "condition_expression_view": "send_alarm == false", "logic_type": "and"}}
  ]
}`

// TestRealWorldGraphNoFalsePositives 回归线上实际暴露的三类校验器误报。
func TestRealWorldGraphNoFalsePositives(t *testing.T) {
	document, err := parseGraph(json.RawMessage(realWorldGraphJSON))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	for _, issue := range report.Errors {
		switch issue.Code {
		case "invalid_reference_format":
			t.Fatalf("裸 ref_key_name（上下文变量）不应报格式错误: %s", issue.Message)
		case "unknown_node_port":
			// python_script 是自由端口节点；image_annotator 仍是固定契约节点
			if strings.Contains(issue.Message, "python_script-005") {
				t.Fatalf("python_script 的自定义端口不应被模板约束: %s", issue.Message)
			}
		}
	}
	// 这条特别检查：script_content 里的 f-string 不能被当成编排引用
	for _, issue := range report.Errors {
		if strings.Contains(issue.Message, "script_content") {
			t.Fatalf("script_content 不参与插值，不应被扫描: %s", issue.Message)
		}
	}
}

// TestRealWorldGraphKeepsGenuineErrors 确认放宽误报的同时，
// 真实存在的问题仍然被检出——否则就是把校验器改废了。
func TestRealWorldGraphKeepsGenuineErrors(t *testing.T) {
	document, err := parseGraph(json.RawMessage(realWorldGraphJSON))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))

	// 1. image_annotator 用了不存在的输出名 annotated_image（真实输出是 image）
	foundBadPort := false
	for _, issue := range report.Errors {
		if issue.Code == "unknown_node_port" && issue.NodeID == "image_annotator-007" &&
			strings.Contains(issue.Message, "annotated_image") {
			foundBadPort = true
		}
	}
	if !foundBadPort {
		t.Fatalf("应检出 image_annotator 输出了不存在的 annotated_image，实际错误: %+v", report.Errors)
	}

	// 2. image_annotator 是死路
	assertIssueCode(t, report.Errors, "cannot_reach_end")

	// 3. wechat_work-009 引用了非上游的 image_annotator-007
	//    这条降级为 warning（见 validateReferences 的说明），所以查 warnings
	foundNotUpstream := false
	for _, issue := range report.Warnings {
		if issue.Code == "reference_not_upstream" && issue.NodeID == "wechat_work-009" {
			foundNotUpstream = true
		}
	}
	if !foundNotUpstream {
		t.Fatalf("应检出 wechat_work-009 引用了非上游节点，实际警告: %+v", report.Warnings)
	}
}

// TestWechatBodyInterpolationStillChecked 确认企微 body 的插值检查没有因为放宽而失效：
// body 确实是 box-app 会插值的字段之一。
func TestWechatBodyInterpolationStillChecked(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "end-002", "type": "end", "properties": {"key_name": "end-002", "inputs": [], "outputs": []}},
			{"id": "wechat_work-003", "type": "wechat_work", "properties": {
				"key_name": "wechat_work-003", "config": {},
				"inputs": [
					{"key_name": "key", "type": "string", "default_value": "k", "required": true},
					{"key_name": "body", "type": "json", "default_value": "{\"content\": \"{start-001.nonexistent_key}\"}", "required": false}
				], "outputs": []}}
		],
		"edges": [
			{"id": "e1", "sourceNodeId": "start-001", "targetNodeId": "wechat_work-003", "properties": {"condition_type": "none"}},
			{"id": "e2", "sourceNodeId": "wechat_work-003", "targetNodeId": "end-002", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Errors, "reference_key_missing")
}

// TestScriptContentFStringNotScanned 单独锁定 script_content 的 f-string 场景。
func TestScriptContentFStringNotScanned(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "end-002", "type": "end", "properties": {"key_name": "end-002", "inputs": [], "outputs": []}},
			{"id": "python_script-003", "type": "python_script", "properties": {
				"key_name": "python_script-003", "config": {
					"script_content": "msg = f\"已达{count}帧\"\nd = {\"a\": 1}\ns = {1, 2}\ncount = 1\nmsg = msg"
				},
				"inputs": [], "outputs": [{"key_name": "count", "type": "number"}, {"key_name": "msg", "type": "string"}]}}
		],
		"edges": [
			{"id": "e1", "sourceNodeId": "start-001", "targetNodeId": "python_script-003", "properties": {"condition_type": "none"}},
			{"id": "e2", "sourceNodeId": "python_script-003", "targetNodeId": "end-002", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := validatorForRealWorldGraph().Validate(document, simplifyGraph(document))
	for _, issue := range report.Errors {
		if issue.Code == "invalid_reference_format" || issue.Code == "reference_node_missing" {
			t.Fatalf("script_content 里的 Python f-string 被误判为编排引用: %s", issue.Message)
		}
	}
}
