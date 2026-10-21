package service

import (
	"box-manage-service/config"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// buildTestValidator 构造一个带常见节点模板的校验器。
func buildTestValidator() *WorkflowValidator {
	return NewWorkflowValidator([]*models.NodeTemplate{
		{TypeKey: "start", TypeName: "开始节点", Category: "logic", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{TypeKey: "end", TypeName: "结束节点", Category: "logic", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{TypeKey: "python_script", TypeName: "Python脚本", Category: "business", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{TypeKey: "task_set_context", TypeName: "任务存上下文", Category: "business", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
	})
}

func TestWorkflowValidatorAcceptsValidGraph(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "name": "开始",
				"inputs": [{"key_name": "detections", "type": "array"}], "outputs": []}},
			{"id": "python_script-002", "type": "python_script", "properties": {"key_name": "python_script-002", "name": "计数",
				"config": {"script_content": "import json\ncount = len(detections)\nalarm_message = \"count=%d\" % count"},
				"inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"}],
				"outputs": [{"key_name": "count", "type": "number"}, {"key_name": "alarm_message", "type": "string"}]}},
			{"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "name": "结束",
				"inputs": [{"key_name": "count", "type": "number", "ref_key_name": "python_script-002.count"}], "outputs": []}}
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
	report := buildTestValidator().Validate(document, simplifyGraph(document))
	if !report.Valid {
		t.Fatalf("expected valid graph, got errors: %+v", report.Errors)
	}
}

func TestWorkflowValidatorDetectsDownstreamReference(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [{"key_name": "detections", "type": "array"}], "outputs": []}},
			{"id": "python_script-002", "type": "python_script", "properties": {"key_name": "python_script-002",
				"config": {"script_content": "count = 0"}, "inputs": [], "outputs": [{"key_name": "count", "type": "number"}]}},
			{"id": "end-003", "type": "end", "properties": {"key_name": "end-003",
				"inputs": [{"key_name": "count", "type": "number", "ref_key_name": "python_script-002.count"}], "outputs": []}}
		],
		"edges": [
			{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "end-003", "properties": {"condition_type": "none"}},
			{"id": "line-002", "sourceNodeId": "end-003", "targetNodeId": "python_script-002", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := buildTestValidator().Validate(document, simplifyGraph(document))
	// 引用非上游节点降级为 warning：box-app 的 resolveReference 按名字查全局输出表，
	// 运行时不校验上下游，能取到值（可能是上一帧残留），语义可疑但不必然报错。
	assertIssueCode(t, report.Warnings, "reference_not_upstream")
}

func TestWorkflowValidatorDetectsUnassignedPythonOutput(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "python_script-002", "type": "python_script", "properties": {"key_name": "python_script-002",
				"config": {"script_content": "other = 1"},
				"inputs": [], "outputs": [{"key_name": "count", "type": "number"}]}},
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
	report := buildTestValidator().Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Warnings, "python_output_unassigned")
}

func TestWorkflowValidatorDetectsMissingStartAndUnreachable(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "end-001", "type": "end", "properties": {"key_name": "end-001", "inputs": [], "outputs": []}}
		],
		"edges": []
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := buildTestValidator().Validate(document, simplifyGraph(document))
	if report.Valid {
		t.Fatalf("expected invalid graph")
	}
	// 缺少开始节点时无法计算连通性，只报 missing_start（连通性校验以开始节点为起点）。
	assertIssueCode(t, report.Errors, "missing_start")
}

func TestWorkflowValidatorDetectsUnreachableBehindStart(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "end-002", "type": "end", "properties": {"key_name": "end-002", "inputs": [], "outputs": []}},
			{"id": "python_script-003", "type": "python_script", "properties": {"key_name": "python_script-003",
				"config": {"script_content": "count = 0"}, "inputs": [], "outputs": [{"key_name": "count", "type": "number"}]}}
		],
		"edges": [
			{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "end-002", "properties": {"condition_type": "none"}}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	report := buildTestValidator().Validate(document, simplifyGraph(document))
	assertIssueCode(t, report.Errors, "unreachable_node")
	assertIssueCode(t, report.Errors, "cannot_reach_end")
}

func TestWorkflowValidatorDraftSkipsConnectivity(t *testing.T) {
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "end-002", "type": "end", "properties": {"key_name": "end-002", "inputs": [], "outputs": []}}
		],
		"edges": []
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	validator := buildTestValidator()
	if report := validator.Validate(document, simplifyGraph(document)); report.Valid {
		t.Fatalf("strict validation should report unreachable/cannot-reach-end")
	}
	if report := validator.ValidateDraft(document, simplifyGraph(document)); !report.Valid {
		t.Fatalf("draft validation should pass without edges, got %+v", report.Errors)
	}
}

func assertIssueCode(t *testing.T, issues []ValidationIssue, code string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("expected issue code %q, got %+v", code, issues)
}

// TestCompleteGraphFillsViewFields 校验补全器补齐模板 ID、文本与坐标。
func TestCompleteGraphFillsViewFields(t *testing.T) {
	simple := &SimpleGraph{
		Nodes: []SimpleNode{
			{ID: "start-001", Type: "start", Properties: map[string]interface{}{"key_name": "start-001", "name": "开始"}},
			{ID: "python_script-002", Type: "python_script", Properties: map[string]interface{}{"key_name": "python_script-002", "name": "计数"}},
			{ID: "end-003", Type: "end", Properties: map[string]interface{}{"key_name": "end-003", "name": "结束"}},
		},
		Edges: []SimpleEdge{
			{ID: "line-001", SourceNodeID: "start-001", TargetNodeID: "python_script-002", Properties: map[string]interface{}{"condition_type": "none"}},
			{ID: "line-002", SourceNodeID: "python_script-002", TargetNodeID: "end-003", Properties: map[string]interface{}{"condition_type": "expression", "condition_expression": "{python_script-002.trigger}"}},
		},
	}
	templates := map[string]*models.NodeTemplate{
		"start":         {BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点"},
		"python_script": {BaseModel: models.BaseModel{ID: 9}, TypeKey: "python_script", TypeName: "Python脚本"},
		"end":           {BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点"},
	}
	document, err := completeGraph(simple, completeOptions{Templates: templates})
	if err != nil {
		t.Fatalf("completeGraph() error = %v", err)
	}
	if len(document.Nodes) != 3 || len(document.Edges) != 2 {
		t.Fatalf("unexpected graph size: nodes=%d edges=%d", len(document.Nodes), len(document.Edges))
	}
	for _, node := range document.Nodes {
		if node.X == nil || node.Y == nil {
			t.Fatalf("node %s missing auto layout coordinates", node.ID)
		}
		if node.Text == nil || node.Text.Value != node.Properties["name"] {
			t.Fatalf("node %s text not filled: %+v", node.ID, node.Text)
		}
		templateID, _ := toFloat(node.Properties["node_template_id"])
		if templateID == 0 {
			t.Fatalf("node %s missing node_template_id", node.ID)
		}
	}
	if document.Edges[0].Type != "polyline" {
		t.Fatalf("edge type not filled: %q", document.Edges[0].Type)
	}
	conditionStyle, _ := document.Edges[1].Properties["style"].(map[string]interface{})
	if conditionStyle["stroke"] != "#fa8c16" {
		t.Fatalf("conditional edge style not applied: %+v", document.Edges[1].Properties)
	}
	// 开始节点在第 0 层，Python 在第 1 层，结束在第 2 层。
	if *document.Nodes[0].X >= *document.Nodes[1].X || *document.Nodes[1].X >= *document.Nodes[2].X {
		t.Fatalf("auto layout should place nodes left to right: %v %v %v",
			*document.Nodes[0].X, *document.Nodes[1].X, *document.Nodes[2].X)
	}
}

func TestSimplifyGraphDropsViewFields(t *testing.T) {
	raw := `{
		"nodes": [{"id": "start-001", "type": "start", "x": 200, "y": 120, "text": {"value": "开始", "x": 200, "y": 120},
			"properties": {"key_name": "start-001", "name": "开始", "width": 60, "height": 60, "node_template_id": 1}}],
		"edges": [{"id": "line-001", "type": "polyline", "sourceNodeId": "start-001", "targetNodeId": "end-002",
			"properties": {"condition_type": "none", "style": {"stroke": "#666"}}}]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	simple := simplifyGraph(document)
	serialized, err := json.Marshal(simple)
	if err != nil {
		t.Fatalf("marshal simple graph: %v", err)
	}
	for _, forbidden := range []string{"\"x\"", "\"width\"", "\"height\"", "node_template_id", "\"style\""} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("simplified graph still contains view field %s: %s", forbidden, serialized)
		}
	}
}

func TestExtractJSONObjectHandlesFencesAndTrailingText(t *testing.T) {
	fenced := "```json\n{\"status\": \"generated\", \"nodes\": []}\n```"
	if got := extractJSONObject(fenced); got != "{\"status\": \"generated\", \"nodes\": []}" {
		t.Fatalf("extractJSONObject(fenced) = %q", got)
	}
	noisy := "好的，这是结果：\n{\"status\": \"generated\", \"nodes\": [{\"id\": \"a\"}]}\n希望有帮助。"
	got := extractJSONObject(noisy)
	if !json.Valid([]byte(got)) {
		t.Fatalf("extractJSONObject(noisy) returned invalid JSON: %q", got)
	}
	if extractJSONObject("完全不是 JSON") != "" {
		t.Fatalf("extractJSONObject should return empty for non-JSON text")
	}
}

// TestWorkflowAIServiceInterpretsGeneratedGraph 用假的 LLM 服务端跑通完整一轮生成。
func TestWorkflowAIServiceInterpretsGeneratedGraph(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{"choices": []interface{}{map[string]interface{}{
			"message": map[string]string{"content": `{
				"status": "generated",
				"name": "人员入侵累计检测",
				"summary": "连续检测到人员后报警",
				"assumptions": ["未说明阈值，按 10 帧处理"],
				"nodes": [
					{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "name": "开始", "inputs": [{"key_name": "detections", "type": "array"}], "outputs": []}},
					{"id": "python_script-002", "type": "python_script", "properties": {"key_name": "python_script-002", "name": "累计计数",
						"config": {"script_content": "count = len(detections)"},
						"inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"}],
						"outputs": [{"key_name": "count", "type": "number"}]}},
					{"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "name": "结束", "inputs": [{"key_name": "count", "type": "number", "ref_key_name": "python_script-002.count"}], "outputs": []}}
				],
				"edges": [
					{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-002", "properties": {"condition_type": "none"}},
					{"id": "line-002", "sourceNodeId": "python_script-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
				]
			}`},
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	service := &workflowAIService{llm: newLLMClient(config.LLMConfig{
		BaseURL: server.URL, APIKey: "test-key", Model: "test-model", RequestTimeout: 5 * time.Second,
	})}
	templates := []*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 9}, TypeKey: "python_script", TypeName: "Python脚本", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
	}
	raw := `{
		"status": "generated",
		"name": "人员入侵累计检测",
		"summary": "连续检测到人员后报警",
		"assumptions": ["未说明阈值，按 10 帧处理"],
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "name": "开始", "inputs": [{"key_name": "detections", "type": "array"}], "outputs": []}},
			{"id": "python_script-002", "type": "python_script", "properties": {"key_name": "python_script-002", "name": "累计计数",
				"config": {"script_content": "count = len(detections)"},
				"inputs": [{"key_name": "detections", "type": "array", "ref_key_name": "start-001.detections"}],
				"outputs": [{"key_name": "count", "type": "number"}]}},
			{"id": "end-003", "type": "end", "properties": {"key_name": "end-003", "name": "结束", "inputs": [{"key_name": "count", "type": "number", "ref_key_name": "python_script-002.count"}], "outputs": []}}
		],
		"edges": [
			{"id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "python_script-002", "properties": {"condition_type": "none"}},
			{"id": "line-002", "sourceNodeId": "python_script-002", "targetNodeId": "end-003", "properties": {"condition_type": "none"}}
		]
	}`
	service.validator = NewWorkflowValidator(templates)
	result := &WorkflowAIGenerateResult{}
	got, err := service.interpret(raw, result, templates)
	if err != nil {
		t.Fatalf("interpret() error = %v", err)
	}
	if got.Status != "generated" || got.Graph == nil {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Name != "人员入侵累计检测" || got.Summary == "" || len(got.Assumptions) != 1 {
		t.Fatalf("unexpected metadata: name=%q summary=%q assumptions=%v", got.Name, got.Summary, got.Assumptions)
	}
	if got.Report == nil || !got.Report.Valid {
		t.Fatalf("expected valid report, got %+v", got.Report)
	}
	if len(got.Graph.Nodes) != 3 || got.Graph.Nodes[1].Properties["node_template_id"] != uint(9) {
		t.Fatalf("template id not completed: %+v", got.Graph.Nodes[1].Properties)
	}
}

func TestWorkflowAIServiceInterpretsClarification(t *testing.T) {
	service := &workflowAIService{}
	raw := `{"status": "need_clarification", "questions": [
		{"id": "q1", "question": "车辆对应的检测类别 ID 是多少？", "options": ["2", "自定义"], "default": "2"}
	]}`
	got, err := service.interpret(raw, &WorkflowAIGenerateResult{}, nil)
	if err != nil {
		t.Fatalf("interpret() error = %v", err)
	}
	if got.Status != "need_clarification" || len(got.Questions) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Questions[0].Default != "2" {
		t.Fatalf("question default lost: %+v", got.Questions[0])
	}
}

func TestBuildWorkflowAIPromptContainsAllSections(t *testing.T) {
	prompt, err := buildWorkflowAIPrompt(promptContext{
		Templates: []*models.NodeTemplate{
			{TypeKey: "start", TypeName: "开始节点", Category: "logic", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
			{TypeKey: "python_script", TypeName: "Python脚本", Category: "business", GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
				Variables: []models.VariableDefinition{
					{KeyName: "input", Name: "入参", Type: "string", Direction: models.VariableDirectionInput},
					{KeyName: "output", Name: "出参", Type: "string", Direction: models.VariableDirectionOutput},
				}},
		},
		TemplateGraph: &SimpleGraph{Nodes: []SimpleNode{
			{ID: "start-001", Type: "start"},
			{ID: "python_script-002", Type: "python_script"},
		}},
		TemplateName: "人员入侵累计检测编排-2669",
		Description:  "检测到车辆在禁停区停留超过 30 帧就推送企业微信",
		Previous:     &workflowAIPrevious{Raw: `{"status":"generated"}`, Errors: []string{"节点 python_script-002 声明输出未赋值"}},
	})
	if err != nil {
		t.Fatalf("buildWorkflowAIPrompt() error = %v", err)
	}
	// 注意用的是 python_script：它的端口是用户自由定义的，因此标为「示例输入/输出」
	// 并附带提醒，而不是「固定输入/输出」（见 TestNodeSpecificationMarksUserDefinedPorts）。
	for _, expected := range []string{
		"【编排规则】", "【基础节点说明】", "【参考模板编排】", "【业务需求】",
		"人员入侵累计检测编排-2669", "检测到车辆在禁停区停留超过 30 帧", "python_script",
		"示例输入", "示例输出", "由你自己按业务需要定义",
		"【上一轮你返回的编排", "节点 python_script-002 声明输出未赋值",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
}

func TestBuildWorkflowAIPromptRespectsByteLimit(t *testing.T) {
	_, err := buildWorkflowAIPrompt(promptContext{
		Templates:   []*models.NodeTemplate{{TypeKey: "start", TypeName: "开始节点", IsEnabled: true}},
		Description: strings.Repeat("需求", 100),
		MaxBytes:    100,
	})
	if err == nil || !strings.Contains(err.Error(), "超过上限") {
		t.Fatalf("expected byte limit error, got %v", err)
	}
}

// TestNormalizeRejectsEmptyGraph 确认补全接口对空图给出明确错误。
func TestNormalizeRejectsEmptyGraph(t *testing.T) {
	if _, err := parseGraph(json.RawMessage(`{"nodes": [], "edges": []}`)); err == nil {
		t.Fatalf("expected error for empty graph")
	}
	if _, err := parseGraph(json.RawMessage(``)); err == nil {
		t.Fatalf("expected error for empty payload")
	}
}

// TestGraphFromWorkflowPrefersViewJSON 视图 JSON 优先于定义表还原。
func TestGraphFromWorkflowPrefersViewJSON(t *testing.T) {
	workflow := &models.Workflow{
		Name:              "测试编排",
		StructureJSONView: `{"nodes":[{"id":"start-001","type":"start","properties":{"key_name":"start-001"}}],"edges":[]}`,
		Nodes: []models.NodeDefinition{
			{NodeID: "定义表节点", TypeKey: "end"},
		},
	}
	document, err := graphFromWorkflow(workflow)
	if err != nil {
		t.Fatalf("graphFromWorkflow() error = %v", err)
	}
	if len(document.Nodes) != 1 || document.Nodes[0].ID != "start-001" {
		t.Fatalf("view JSON not preferred: %+v", document.Nodes)
	}
}

// TestGraphFromWorkflowFallsBackToDefinitions 无视图时从定义表还原。
func TestGraphFromWorkflowFallsBackToDefinitions(t *testing.T) {
	workflow := &models.Workflow{
		Name: "旧编排",
		Nodes: []models.NodeDefinition{
			{NodeID: "start-001", TypeKey: "start", TypeName: "开始", NodeKeyName: "start-001", Position: models.JSONMap{"x": float64(100), "y": float64(80)}, Inputs: models.JSONArr{map[string]interface{}{"key_name": "detections", "type": "array"}}},
			{NodeID: "end-002", TypeKey: "end", TypeName: "结束", NodeKeyName: "end-002"},
		},
		Lines: []models.LineDefinition{
			{LineID: "line-001", SourceNodeID: "start-001", TargetNodeID: "end-002", ConditionType: models.ConditionTypeNone},
		},
		Variables: []models.VariableDefinition{
			{NodeID: "end-002", KeyName: "count", Name: "计数", Type: "number", Direction: models.VariableDirectionInput, RefKeyName: "start-001.detections"},
		},
	}
	document, err := graphFromWorkflow(workflow)
	if err != nil {
		t.Fatalf("graphFromWorkflow() error = %v", err)
	}
	if len(document.Nodes) != 2 || len(document.Edges) != 1 {
		t.Fatalf("unexpected restored graph: %+v", document)
	}
	simple := simplifyGraph(document)
	serialized, _ := json.Marshal(simple)
	if !strings.Contains(string(serialized), "detections") || !strings.Contains(string(serialized), "count") {
		t.Fatalf("variables not restored into ports: %s", serialized)
	}
}

// TestWorkflowGraphServiceNeedsDatabase 未注入数据库时给出明确错误。
func TestWorkflowGraphServiceNeedsDatabase(t *testing.T) {
	service := NewWorkflowGraphService(nil)
	if _, err := service.Validate(context.Background(), json.RawMessage(`{"nodes":[{"id":"start-001","type":"start"}]}`)); err == nil {
		t.Fatalf("expected error when database is missing")
	}
}

// TestLoadEnabledTemplatesWithVariablesFillsPorts 回归：repository.FindEnabled 不加载变量，
// 若不在服务层补齐，提示词的「基础节点说明」会缺少固定输入/输出，校验器也无从检查固定参数。
func TestLoadEnabledTemplatesWithVariablesFillsPorts(t *testing.T) {
	// 用固定契约节点（detection_filter）：它的变量确实应作为「固定输入/输出」出现在提示词里。
	// python_script 的端口是用户自由定义的，走的是「示例输入」分支，另有测试覆盖。
	repo := &stubNodeTemplateRepo{templates: []*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 20}, TypeKey: "detection_filter", TypeName: "检测结果过滤", IsEnabled: true},
	}}
	variableRepo := &stubVariableRepo{variables: map[uint][]*models.VariableDefinition{
		20: {
			{KeyName: "detections", Name: "检测结果", Type: "array", Direction: models.VariableDirectionInput, Required: true},
			{KeyName: "has_match", Name: "是否命中", Type: "boolean", Direction: models.VariableDirectionOutput},
		},
	}}

	templates, err := loadEnabledTemplatesWithVariables(context.Background(), repo, variableRepo)
	if err != nil {
		t.Fatalf("loadEnabledTemplatesWithVariables() error = %v", err)
	}
	if len(templates) != 1 || len(templates[0].Variables) != 2 {
		t.Fatalf("template variables not filled: %+v", templates[0].Variables)
	}

	// 变量补齐后，模板定义的固定参数才能被校验器识别
	prompt, err := buildWorkflowAIPrompt(promptContext{
		Templates:     templates,
		TemplateGraph: &SimpleGraph{Nodes: []SimpleNode{{ID: "detection_filter-001", Type: "detection_filter"}}},
		Description:   "测试",
	})
	if err != nil {
		t.Fatalf("buildWorkflowAIPrompt() error = %v", err)
	}
	for _, expected := range []string{"固定输入", "固定输出", "detections", "has_match"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q after filling variables", expected)
		}
	}
}

func TestBuildTemplateIndexSkipsNil(t *testing.T) {
	index := buildTemplateIndex([]*models.NodeTemplate{
		nil,
		{TypeKey: "start", TypeName: "开始"},
	})
	if len(index) != 1 {
		t.Fatalf("unexpected index size: %d", len(index))
	}
	if _, ok := index["start"]; !ok {
		t.Fatalf("start template missing from index")
	}
}

// stubNodeTemplateRepo 只实现 FindEnabled，其余方法不被测试用到。
type stubNodeTemplateRepo struct {
	repository.NodeTemplateRepository
	templates []*models.NodeTemplate
}

func (s *stubNodeTemplateRepo) FindEnabled(context.Context) ([]*models.NodeTemplate, error) {
	return s.templates, nil
}

// stubVariableRepo 只实现 FindByNodeTemplateID。
type stubVariableRepo struct {
	repository.VariableDefinitionRepository
	variables map[uint][]*models.VariableDefinition
}

func (s *stubVariableRepo) FindByNodeTemplateID(_ context.Context, nodeTemplateID uint) ([]*models.VariableDefinition, error) {
	return s.variables[nodeTemplateID], nil
}

// TestValidateTemplatePortsRejectsUnknownKey 模板定义固定参数后，超出模板的 key_name 会被拒绝。
func TestValidateTemplatePortsRejectsUnknownKey(t *testing.T) {
	validator := NewWorkflowValidator([]*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		{BaseModel: models.BaseModel{ID: 9}, TypeKey: "reasoning", TypeName: "推理", GroupType: models.NodeGroupTypeSingle, IsEnabled: true,
			Variables: []models.VariableDefinition{
				{KeyName: "model_key", Type: "string", Direction: models.VariableDirectionInput},
			}},
	})
	raw := `{
		"nodes": [
			{"id": "start-001", "type": "start", "properties": {"key_name": "start-001", "inputs": [], "outputs": []}},
			{"id": "reasoning-002", "type": "reasoning", "properties": {"key_name": "reasoning-002",
				"config": {}, "inputs": [{"key_name": "not_in_template", "type": "string", "default": "x"}], "outputs": []}},
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
	assertIssueCode(t, report.Errors, "unknown_node_port")
}

// TestParseGraphAcceptsBackendStructure 回归：后端 nodes/lines 结构也能被解析，
// 不能静默产出一堆没有 id 的节点。
func TestParseGraphAcceptsBackendStructure(t *testing.T) {
	raw := `{
		"nodes": [
			{"node_id": "start_001", "type_key": "start", "type_name": "开始节点", "node_name": "开始", "node_key_name": "start_001",
				"position": {"x": 100, "y": 200}, "config": {}, "inputs": [], "outputs": []},
			{"node_id": "end_002", "type_key": "end", "type_name": "结束节点", "node_name": "结束", "node_key_name": "end_002",
				"position": {"x": 700, "y": 200}, "config": {}, "inputs": [], "outputs": []}
		],
		"lines": [
			{"line_id": "line_001", "source_node_id": "start_001", "target_node_id": "end_002", "condition_type": "none"}
		]
	}`
	document, err := parseGraph(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseGraph() error = %v", err)
	}
	if len(document.Nodes) != 2 || len(document.Edges) != 1 {
		t.Fatalf("unexpected graph: nodes=%d edges=%d", len(document.Nodes), len(document.Edges))
	}
	for _, node := range document.Nodes {
		if node.ID == "" || node.Type == "" {
			t.Fatalf("node not restored from backend structure: %+v", node)
		}
	}
	if document.Nodes[0].ID != "start_001" || document.Nodes[0].Type != "start" {
		t.Fatalf("unexpected first node: %+v", document.Nodes[0])
	}
	if document.Nodes[0].X == nil || *document.Nodes[0].X != 100 {
		t.Fatalf("position not restored: %+v", document.Nodes[0].X)
	}
	if document.Edges[0].SourceNodeID != "start_001" || document.Edges[0].TargetNodeID != "end_002" {
		t.Fatalf("edge endpoints not restored: %+v", document.Edges[0])
	}
	// 后端结构解析后应当能通过补全与校验
	completed, err := completeGraph(simplifyGraph(document), completeOptions{Templates: buildTestValidator().templates})
	if err != nil {
		t.Fatalf("completeGraph() error = %v", err)
	}
	if len(completed.Nodes) != 2 {
		t.Fatalf("unexpected completed graph: %+v", completed.Nodes)
	}
}

// TestWorkflowValidationErrorCarriesIssues 确认校验错误可被 errors.As 识别，且带出明细。
func TestWorkflowValidationErrorCarriesIssues(t *testing.T) {
	var err error = &WorkflowValidationError{Issues: []ValidationIssue{
		{Code: "missing_start", Message: "编排必须包含一个开始节点"},
		{Code: "unreachable_node", Message: "节点 end-002 无法从开始节点到达", NodeID: "end-002"},
	}}
	var target *WorkflowValidationError
	if !errors.As(err, &target) {
		t.Fatalf("errors.As should match *WorkflowValidationError")
	}
	if len(target.Issues) != 2 {
		t.Fatalf("issues lost: %+v", target.Issues)
	}
	if !strings.Contains(err.Error(), "编排必须包含一个开始节点") {
		t.Fatalf("error message should include issue text: %q", err.Error())
	}
	report := target.Report()
	if report.Valid || len(report.Errors) != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

// TestNormalizePortDefaultsMigratesLegacyField 回归：legacy default 必须迁移到规范字段 default_value。
// 这正是之前导致默认值丢失的字段不一致问题。
func TestNormalizePortDefaultsMigratesLegacyField(t *testing.T) {
	properties := map[string]interface{}{
		"inputs": []interface{}{
			map[string]interface{}{"key_name": "a", "default": "legacy"},
			map[string]interface{}{"key_name": "b", "default_value": "canonical"},
			map[string]interface{}{"key_name": "c", "default": false},
			map[string]interface{}{"key_name": "d", "default": 0},
			map[string]interface{}{"key_name": "e"},
		},
		"outputs": []interface{}{
			map[string]interface{}{"key_name": "x", "default": "legacy-output"},
		},
	}
	normalizePortDefaults(properties)

	inputs := properties["inputs"].([]interface{})
	// legacy -> default_value，且 default 被删除
	first := inputs[0].(map[string]interface{})
	if first["default_value"] != "legacy" {
		t.Fatalf("legacy default not migrated: %+v", first)
	}
	if _, stillThere := first["default"]; stillThere {
		t.Fatalf("legacy default field should be removed: %+v", first)
	}
	// 已有的 default_value 不被覆盖
	second := inputs[1].(map[string]interface{})
	if second["default_value"] != "canonical" {
		t.Fatalf("canonical value overwritten: %+v", second)
	}
	// false / 0 是合法默认值，必须保留
	if v, ok := inputs[2].(map[string]interface{})["default_value"]; !ok || v != false {
		t.Fatalf("boolean false default lost: %+v", inputs[2])
	}
	if v, ok := inputs[3].(map[string]interface{})["default_value"]; !ok || v != 0 {
		t.Fatalf("zero default lost: %+v", inputs[3])
	}
	// 没有默认值的端口保持没有该字段
	fifth := inputs[4].(map[string]interface{})
	if _, exists := fifth["default_value"]; exists {
		t.Fatalf("absent default should stay absent: %+v", fifth)
	}
	// outputs 同样处理
	outputs := properties["outputs"].([]interface{})
	if outputs[0].(map[string]interface{})["default_value"] != "legacy-output" {
		t.Fatalf("output default not migrated: %+v", outputs[0])
	}
}

// TestPortDefaultPrefersCanonical 确认读取优先 default_value，并兼容 legacy default。
func TestPortDefaultPrefersCanonical(t *testing.T) {
	if got := portDefault(map[string]interface{}{"default_value": "canonical", "default": "legacy"}); got != "canonical" {
		t.Fatalf("default_value should win, got %v", got)
	}
	if got := portDefault(map[string]interface{}{"default": "legacy"}); got != "legacy" {
		t.Fatalf("legacy default should be the fallback, got %v", got)
	}
	// false 是合法默认值，不能被当成「没有值」而回退到 legacy
	if got := portDefault(map[string]interface{}{"default_value": false, "default": "legacy"}); got != false {
		t.Fatalf("false default_value should be respected, got %v", got)
	}
}

// TestCompleteGraphNormalizesLegacyDefaultField 确认补全后的图只使用 default_value。
func TestCompleteGraphNormalizesLegacyDefaultField(t *testing.T) {
	simple := &SimpleGraph{
		Nodes: []SimpleNode{
			{ID: "start-001", Type: "start", Properties: map[string]interface{}{"key_name": "start-001",
				"inputs": []interface{}{map[string]interface{}{"key_name": "detections", "default": "legacy-value"}}}},
			{ID: "end-002", Type: "end", Properties: map[string]interface{}{"key_name": "end-002"}},
		},
		Edges: []SimpleEdge{{ID: "line-001", SourceNodeID: "start-001", TargetNodeID: "end-002"}},
	}
	templates := map[string]*models.NodeTemplate{
		"start": {BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点"},
		"end":   {BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点"},
	}
	document, err := completeGraph(simple, completeOptions{Templates: templates})
	if err != nil {
		t.Fatalf("completeGraph() error = %v", err)
	}
	serialized, _ := json.Marshal(document)
	if strings.Contains(string(serialized), `"default"`) {
		t.Fatalf("legacy default field leaked into completed graph: %s", serialized)
	}
	if !strings.Contains(string(serialized), `"default_value":"legacy-value"`) {
		t.Fatalf("default value not migrated: %s", serialized)
	}
}

// TestNodeSpecificationMarksUserDefinedPorts 确认提示词不会把 python_script 的占位变量
// 说成「固定输入/输出」，否则模型会以为必须照抄模板里的 input/output。
func TestNodeSpecificationMarksUserDefinedPorts(t *testing.T) {
	templates := []*models.NodeTemplate{
		{BaseModel: models.BaseModel{ID: 20}, TypeKey: "detection_filter", TypeName: "检测结果过滤",
			IsEnabled: true, GroupType: models.NodeGroupTypeSingle,
			Variables: []models.VariableDefinition{
				{KeyName: "detections", Type: "array", Direction: models.VariableDirectionInput},
			}},
		{BaseModel: models.BaseModel{ID: 9}, TypeKey: "python_script", TypeName: "PythonScript脚本节点",
			IsEnabled: true, GroupType: models.NodeGroupTypeSingle,
			Variables: []models.VariableDefinition{
				{KeyName: "input", Type: "string", Direction: models.VariableDirectionInput},
				{KeyName: "output", Type: "string", Direction: models.VariableDirectionOutput},
			}},
	}
	// 两个类型都要出现在「参考模板」里，才会输出完整说明
	graph := &SimpleGraph{Nodes: []SimpleNode{
		{ID: "detection_filter-001", Type: "detection_filter"},
		{ID: "python_script-002", Type: "python_script"},
	}}
	spec := buildNodeSpecification(templates, graph)

	// 固定契约节点用「固定输入」
	if !strings.Contains(spec, "固定输入") {
		t.Fatalf("detection_filter 应标为固定输入:\n%s", spec)
	}
	// 自由端口节点用「示例输入」并给出提醒
	if !strings.Contains(spec, "示例输入") || !strings.Contains(spec, "由你自己按业务需要定义") {
		t.Fatalf("python_script 应标为示例端口并提醒:\n%s", spec)
	}
}

// TestPromptExplainsContextNodePortTyping 确认提示词讲清了 task_set_context 的
// 输入是自定义的、且 type 必须与上游输出一致。
//
// 线上实测：模型把 task_set_context 的所有输入都写成 string，
// 而实际引用的是 boolean/number，每次生成都产生 5 条类型不匹配警告。
func TestPromptExplainsContextNodePortTyping(t *testing.T) {
	_, err := buildWorkflowAIPrompt(promptContext{
		Templates: []*models.NodeTemplate{
			{BaseModel: models.BaseModel{ID: 1}, TypeKey: "start", TypeName: "开始节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
			{BaseModel: models.BaseModel{ID: 2}, TypeKey: "end", TypeName: "结束节点", GroupType: models.NodeGroupTypeSingle, IsEnabled: true},
		},
		Description: "测试",
	})
	if err != nil {
		t.Fatalf("buildWorkflowAIPrompt() error = %v", err)
	}
	text := workflowAIRuleText
	for _, expected := range []string{"task_set_context 的 inputs 由你自己按业务定义", "不要一律写 string"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("编排规则缺少对上下文节点类型的说明: 期望包含 %q", expected)
		}
	}
}
