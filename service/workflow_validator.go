/*
 * @module service/workflow_validator
 * @description 统一的编排校验器：输入 LogicFlow JSON（完整或精简），输出带定位信息的
 *              errors / warnings。同一份校验器同时用于 AI 生成结果的自动修复、
 *              手工导入和保存，避免前后端各写一套规则。
 */

package service

import (
	"box-manage-service/models"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ValidationIssue 一条校验结果，带定位信息便于前端高亮节点/连线。
type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	EdgeID  string `json:"edge_id,omitempty"`
}

// ValidationReport 校验报告。
type ValidationReport struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
}

// WorkflowValidator 编排校验器，依赖节点模板表获取允许的类型与固定参数。
type WorkflowValidator struct {
	templates map[string]*models.NodeTemplate
}

// NewWorkflowValidator 使用启用中的节点模板构建校验器。
func NewWorkflowValidator(templates []*models.NodeTemplate) *WorkflowValidator {
	registry := make(map[string]*models.NodeTemplate, len(templates))
	for _, template := range templates {
		if template == nil {
			continue
		}
		registry[template.TypeKey] = template
	}
	return &WorkflowValidator{templates: registry}
}

// userDefinedPortTypes 端口由用户自由定义的节点类型。
//
// 这些节点的 inputs/outputs 不是模板固定契约：
//   - python_script：入参/出参就是脚本变量，用户在配置面板里自由增删
//     （模板里的 input/output/script_content/timeout_seconds 只是占位示例，
//     且 script_content/timeout_seconds 属于 config，本来就不该出现在 inputs 里）
//   - task_get_context / task_set_context：读写哪些键由编排决定，
//     模板里的 context/keys/saved 只是内置输出
//
// 对它们只做 key_name 非空与重复检查，不做「必须命中模板」的检查。
var userDefinedPortTypes = map[string]struct{}{
	"python_script":    {},
	"script":           {},
	"task_get_context": {},
	"task_set_context": {},
}

// normalizeReference 按 box-app 的 resolveReference 规则预处理引用：
// 去首尾空白，若整体被 {} 包裹则剥掉再 trim。
func normalizeReference(ref string) string {
	normalized := strings.TrimSpace(ref)
	if len(normalized) >= 2 && strings.HasPrefix(normalized, "{") && strings.HasSuffix(normalized, "}") {
		normalized = strings.TrimSpace(normalized[1 : len(normalized)-1])
	}
	return normalized
}

// interpolationInputKeys 返回该节点类型上**真正会发生 {node.key} 插值**的 inputs/config 字段名。
//
// 严格对应 box-app 的 6 个调用点（见 ai_docs/box-app工作流节点执行语义规格.md §3.1）：
//
//	mqtt             : payload
//	http_request/http: url、headers、body（仅字符串时）
//	wechat_work      : body（递归所有字符串）
//	rpa_wechat_proxy : body_text
//
// 其他节点与字段一律不插值。这一点很重要：python_script 的 script_content 不插值，
// 脚本里的 Python f-string（如 f"...{count}帧"）必须原样放行，否则会被误判成编排引用。
func interpolationInputKeys(typeKey string) map[string]struct{} {
	switch typeKey {
	case "mqtt":
		return map[string]struct{}{"payload": {}}
	case "http_request", "http":
		return map[string]struct{}{"url": {}, "headers": {}, "body": {}}
	case "wechat_work":
		return map[string]struct{}{"body": {}}
	case "rpa_wechat_proxy":
		return map[string]struct{}{"body_text": {}}
	}
	return nil
}

// isUserDefinedPortType 判断节点类型的端口是否由用户自由定义。
func isUserDefinedPortType(typeKey string) bool {
	_, ok := userDefinedPortTypes[typeKey]
	return ok
}

// 成对节点：开始与结束必须同时出现。
var pairedNodeTypes = map[string]string{
	"concurrency_start": "concurrency_end",
	"loop_start":        "loop_end",
}

var pairedEndNodeTypes = map[string]string{
	"concurrency_end": "concurrency_start",
	"loop_end":        "loop_start",
}

// 插值占位符：{node-id.output_key}
var interpolationPattern = regexp.MustCompile(`\{([A-Za-z0-9_\-\.]+)\}`)

// ValidateOptions 控制校验范围。
type ValidateOptions struct {
	// SkipConnectivity 跳过连通性检查。
	// 保存草稿时使用：允许用户先只放开始/结束节点，稍后再连线。
	SkipConnectivity bool
}

// Validate 执行全部校验规则，用于 AI 生成结果与显式校验接口。
func (v *WorkflowValidator) Validate(document *graphDocument, simple *SimpleGraph) *ValidationReport {
	return v.validate(document, simple, ValidateOptions{})
}

// ValidateDraft 用于保存草稿：结构、模板、引用仍然严格校验，但不要求全图连通。
func (v *WorkflowValidator) ValidateDraft(document *graphDocument, simple *SimpleGraph) *ValidationReport {
	return v.validate(document, simple, ValidateOptions{SkipConnectivity: true})
}

func (v *WorkflowValidator) validate(document *graphDocument, simple *SimpleGraph, options ValidateOptions) *ValidationReport {
	report := &ValidationReport{Errors: []ValidationIssue{}, Warnings: []ValidationIssue{}}
	if document == nil || len(document.Nodes) == 0 {
		report.Errors = append(report.Errors, ValidationIssue{Code: "empty_graph", Message: "编排中没有任何节点"})
		report.Valid = false
		return report
	}
	report.Errors = append(report.Errors, v.validateStructure(document)...)
	report.Errors = append(report.Errors, v.validateTemplates(document)...)
	report.Errors = append(report.Errors, v.validatePairedNodes(document)...)

	nodeIndex := map[string]*graphNode{}
	for i := range document.Nodes {
		nodeIndex[document.Nodes[i].ID] = &document.Nodes[i]
	}
	// 只有结构基本正确时，引用类校验才有意义；否则会刷屏。
	if !hasBlockingError(report.Errors) {
		if !options.SkipConnectivity {
			report.Errors = append(report.Errors, v.validateConnectivity(document, nodeIndex)...)
		}
		report.Errors = append(report.Errors, v.validateReferences(document, nodeIndex)...)
		report.Errors = append(report.Errors, v.validateRequiredInputs(document, nodeIndex)...)
		report.Warnings = append(report.Warnings, v.validateReferenceTypes(document, nodeIndex)...)
		// Python 节点的“没有脚本内容”“脚本违反运行契约”属于会直接导致编排跑不通的硬伤，
		// 必须算 error：否则界面显示“校验通过”、允许导入一个空脚本节点，运行时才暴露。
		pythonIssues := v.validatePythonOutputs(nodeIndex)
		for _, issue := range pythonIssues {
			if issue.Code == "python_script_empty" || issue.Code == "python_script_invalid" {
				report.Errors = append(report.Errors, issue)
			} else {
				report.Warnings = append(report.Warnings, issue)
			}
		}
		report.Warnings = append(report.Warnings, v.validateUnusedOutputs(document, nodeIndex)...)
	}
	sortIssues(report.Errors)
	sortIssues(report.Warnings)
	report.Valid = len(report.Errors) == 0
	return report
}

// hasBlockingError 判断是否出现了会让后续校验失去意义的错误。
func hasBlockingError(issues []ValidationIssue) bool {
	for _, issue := range issues {
		switch issue.Code {
		case "empty_graph", "missing_start", "multiple_start", "duplicate_node_id", "edge_endpoint_missing", "self_loop":
			return true
		}
	}
	return false
}

func sortIssues(issues []ValidationIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].NodeID != issues[j].NodeID {
			return issues[i].NodeID < issues[j].NodeID
		}
		return issues[i].EdgeID < issues[j].EdgeID
	})
}

// ---------------------------------------------------------------------------
// 结构
// ---------------------------------------------------------------------------

func (v *WorkflowValidator) validateStructure(document *graphDocument) []ValidationIssue {
	issues := []ValidationIssue{}
	startCount := 0
	endCount := 0
	seen := map[string]string{}
	for _, node := range document.Nodes {
		if previousType, duplicated := seen[node.ID]; duplicated {
			issues = append(issues, ValidationIssue{
				Code:    "duplicate_node_id",
				Message: fmt.Sprintf("节点 id 重复：%s（已存在类型 %s）", node.ID, previousType),
				NodeID:  node.ID,
			})
			continue
		}
		seen[node.ID] = node.Type
		switch node.Type {
		case "start":
			startCount++
		case "end":
			endCount++
		}
	}
	if startCount == 0 {
		issues = append(issues, ValidationIssue{Code: "missing_start", Message: "编排必须包含一个开始节点"})
	}
	if startCount > 1 {
		issues = append(issues, ValidationIssue{Code: "multiple_start", Message: fmt.Sprintf("编排只能有一个开始节点，当前有 %d 个", startCount)})
	}
	if endCount == 0 {
		issues = append(issues, ValidationIssue{Code: "missing_end", Message: "编排必须包含至少一个结束节点"})
	}
	for _, edge := range document.Edges {
		if _, ok := seen[edge.SourceNodeID]; !ok {
			issues = append(issues, ValidationIssue{
				Code:    "edge_endpoint_missing",
				Message: fmt.Sprintf("连线 %s 的源节点 %s 不存在", edge.ID, edge.SourceNodeID),
				EdgeID:  edge.ID,
			})
		}
		if _, ok := seen[edge.TargetNodeID]; !ok {
			issues = append(issues, ValidationIssue{
				Code:    "edge_endpoint_missing",
				Message: fmt.Sprintf("连线 %s 的目标节点 %s 不存在", edge.ID, edge.TargetNodeID),
				EdgeID:  edge.ID,
			})
		}
		if edge.SourceNodeID != "" && edge.SourceNodeID == edge.TargetNodeID {
			issues = append(issues, ValidationIssue{
				Code:    "self_loop",
				Message: fmt.Sprintf("连线 %s 不能连接节点自身（%s）", edge.ID, edge.SourceNodeID),
				EdgeID:  edge.ID,
				NodeID:  edge.SourceNodeID,
			})
		}
	}
	return issues
}

// ---------------------------------------------------------------------------
// 节点模板
// ---------------------------------------------------------------------------

func (v *WorkflowValidator) validateTemplates(document *graphDocument) []ValidationIssue {
	issues := []ValidationIssue{}
	for _, node := range document.Nodes {
		template, ok := v.templates[node.Type]
		if !ok {
			issues = append(issues, ValidationIssue{
				Code:    "unknown_node_type",
				Message: fmt.Sprintf("节点 %s 使用了不存在的节点类型 %q", node.ID, node.Type),
				NodeID:  node.ID,
			})
			continue
		}
		if declaredID, ok := toFloat(node.Properties["node_template_id"]); ok && declaredID > 0 && uint(declaredID) != template.ID {
			issues = append(issues, ValidationIssue{
				Code:    "template_id_mismatch",
				Message: fmt.Sprintf("节点 %s 的 node_template_id=%d 与类型 %s 对应的模板 %d 不一致", node.ID, uint(declaredID), node.Type, template.ID),
				NodeID:  node.ID,
			})
		}
		// 固定参数节点（模板定义了 variables）不允许自定义超出模板的 key_name。
		if template.GroupType == models.NodeGroupTypePaired {
			continue
		}
		// 端口自由定义的节点只检查 key_name 是否重复，不检查是否命中模板
		if _, userDefined := userDefinedPortTypes[node.Type]; userDefined {
			issues = append(issues, checkDuplicatePortKeys(&node)...)
			continue
		}
		inputKeys, outputKeys := templatePortKeys(template)
		if len(inputKeys) == 0 && len(outputKeys) == 0 {
			continue
		}
		for _, direction := range []string{"inputs", "outputs"} {
			allowed := inputKeys
			if direction == "outputs" {
				allowed = outputKeys
			}
			for _, port := range asMapSlice(node.Properties[direction]) {
				keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
				if keyName == "" {
					continue
				}
				if _, ok := allowed[keyName]; !ok {
					issues = append(issues, ValidationIssue{
						Code:    "unknown_node_port",
						Message: fmt.Sprintf("节点 %s（%s）的%s %q 不在模板定义中", node.ID, template.TypeName, directionLabel(direction), keyName),
						NodeID:  node.ID,
					})
				}
			}
		}
	}
	return issues
}

// checkDuplicatePortKeys 检查同一节点内 key_name 是否重复。
// 对端口自由定义的节点（python_script 等）这是唯一有意义的参数检查：
// 重复的 key_name 会让 box-app 注入/捕获变量时互相覆盖。
func checkDuplicatePortKeys(node *graphNode) []ValidationIssue {
	issues := []ValidationIssue{}
	for _, direction := range []string{"inputs", "outputs"} {
		seen := map[string]int{}
		for _, port := range asMapSlice(node.Properties[direction]) {
			keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
			if keyName == "" {
				continue
			}
			seen[keyName]++
			if seen[keyName] == 2 {
				issues = append(issues, ValidationIssue{
					Code:    "duplicate_node_port",
					Message: fmt.Sprintf("节点 %s 的%s中存在重复的 key_name %q", node.ID, directionLabel(direction), keyName),
					NodeID:  node.ID,
				})
			}
		}
	}
	return issues
}

// templatePortKeys 返回模板定义的固定入参/出参 key 集合。
func templatePortKeys(template *models.NodeTemplate) (map[string]struct{}, map[string]struct{}) {
	inputs := map[string]struct{}{}
	outputs := map[string]struct{}{}
	for _, variable := range template.Variables {
		if variable.Direction == models.VariableDirectionOutput {
			outputs[variable.KeyName] = struct{}{}
		} else {
			inputs[variable.KeyName] = struct{}{}
		}
	}
	// structure_json 里也可能带着 variables 定义。
	if len(template.Variables) == 0 && strings.TrimSpace(template.StructureJSON) != "" {
		var payload struct {
			Variables []struct {
				KeyName   string `json:"key_name"`
				Direction string `json:"direction"`
			} `json:"variables"`
		}
		if err := json.Unmarshal([]byte(template.StructureJSON), &payload); err == nil {
			for _, variable := range payload.Variables {
				if variable.Direction == "output" {
					outputs[variable.KeyName] = struct{}{}
				} else {
					inputs[variable.KeyName] = struct{}{}
				}
			}
		}
	}
	return inputs, outputs
}

func directionLabel(direction string) string {
	if direction == "outputs" {
		return "输出参数"
	}
	return "输入参数"
}

// ---------------------------------------------------------------------------
// 成对节点
// ---------------------------------------------------------------------------

func (v *WorkflowValidator) validatePairedNodes(document *graphDocument) []ValidationIssue {
	issues := []ValidationIssue{}
	nodeTypes := map[string]string{}
	for _, node := range document.Nodes {
		nodeTypes[node.ID] = node.Type
	}
	for _, node := range document.Nodes {
		if counterpartType, isStart := pairedNodeTypes[node.Type]; isStart {
			// 存在与之配对、且类型匹配、且可达的结束节点即可。
			if !hasReachableType(document, node.ID, counterpartType) {
				issues = append(issues, ValidationIssue{
					Code:    "unpaired_node",
					Message: fmt.Sprintf("%s（%s）缺少下游的配对节点 %s", node.ID, node.Type, counterpartType),
					NodeID:  node.ID,
				})
			}
			continue
		}
		if counterpartType, isEnd := pairedEndNodeTypes[node.Type]; isEnd {
			if !hasReachableTypeReverse(document, node.ID, counterpartType) {
				issues = append(issues, ValidationIssue{
					Code:    "unpaired_node",
					Message: fmt.Sprintf("%s（%s）缺少上游的配对节点 %s", node.ID, node.Type, counterpartType),
					NodeID:  node.ID,
				})
			}
		}
	}
	return issues
}

// ---------------------------------------------------------------------------
// 连通性
// ---------------------------------------------------------------------------

func (v *WorkflowValidator) validateConnectivity(document *graphDocument, nodeIndex map[string]*graphNode) []ValidationIssue {
	issues := []ValidationIssue{}
	startID := ""
	for _, node := range document.Nodes {
		if node.Type == "start" {
			startID = node.ID
			break
		}
	}
	if startID == "" {
		return issues
	}
	reachable := reachableFrom(document, startID, false)
	for _, node := range document.Nodes {
		if _, ok := reachable[node.ID]; !ok {
			issues = append(issues, ValidationIssue{
				Code:    "unreachable_node",
				Message: fmt.Sprintf("节点 %s 无法从开始节点到达", node.ID),
				NodeID:  node.ID,
			})
		}
	}
	canReachEnd := map[string]struct{}{}
	for _, node := range document.Nodes {
		if node.Type != "end" {
			continue
		}
		for id := range reachableFrom(document, node.ID, true) {
			canReachEnd[id] = struct{}{}
		}
	}
	for _, node := range document.Nodes {
		if _, ok := canReachEnd[node.ID]; !ok {
			issues = append(issues, ValidationIssue{
				Code:    "cannot_reach_end",
				Message: fmt.Sprintf("节点 %s 没有任何路径到达结束节点", node.ID),
				NodeID:  node.ID,
			})
		}
	}
	return issues
}

// reachableFrom 返回从 origin 出发可达的节点集合；reverse 为 true 时沿反向边遍历。
func reachableFrom(document *graphDocument, origin string, reverse bool) map[string]struct{} {
	adjacency := map[string][]string{}
	for _, edge := range document.Edges {
		if reverse {
			adjacency[edge.TargetNodeID] = append(adjacency[edge.TargetNodeID], edge.SourceNodeID)
		} else {
			adjacency[edge.SourceNodeID] = append(adjacency[edge.SourceNodeID], edge.TargetNodeID)
		}
	}
	visited := map[string]struct{}{origin: {}}
	queue := []string{origin}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if _, ok := visited[next]; ok {
				continue
			}
			visited[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return visited
}

func hasReachableType(document *graphDocument, origin, wantType string) bool {
	visited := reachableFrom(document, origin, false)
	for _, node := range document.Nodes {
		if node.Type == wantType {
			if _, ok := visited[node.ID]; ok {
				return true
			}
		}
	}
	return false
}

func hasReachableTypeReverse(document *graphDocument, origin, wantType string) bool {
	visited := reachableFrom(document, origin, true)
	for _, node := range document.Nodes {
		if node.Type == wantType {
			if _, ok := visited[node.ID]; ok {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 引用与插值
// ---------------------------------------------------------------------------

// outputKeysOf 返回某节点可被下游引用的输出 key 集合。
// 开始节点用 inputs 作为对外提供的上下文（detections、image_base64 等）。
func outputKeysOf(node *graphNode) map[string]struct{} {
	keys := map[string]struct{}{}
	field := "outputs"
	if node.Type == "start" {
		field = "inputs"
	}
	for _, port := range asMapSlice(node.Properties[field]) {
		if keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"])); keyName != "" {
			keys[keyName] = struct{}{}
		}
	}
	return keys
}

func (v *WorkflowValidator) validateReferences(document *graphDocument, nodeIndex map[string]*graphNode) []ValidationIssue {
	issues := []ValidationIssue{}
	for _, node := range document.Nodes {
		upstream := upstreamNodes(document, node.ID)
		upstreamOutputs := map[string]map[string]struct{}{}
		for id := range upstream {
			if target, ok := nodeIndex[id]; ok {
				upstreamOutputs[id] = outputKeysOf(target)
			}
		}
		check := func(rawRef, context string) {
			ref := normalizeReference(rawRef)
			if ref == "" {
				return
			}
			// 无点号 = 上下文变量名（如 start 节点的 detections/image_base64、
			// task_get_context 的 is_alarmed）。box-app 的 resolveReference 会先按变量名查找，
			// 这是合法写法，不做 <节点id>.<输出key> 的格式要求。
			if !strings.Contains(ref, ".") {
				return
			}
			parts := strings.SplitN(ref, ".", 2)
			sourceID, keyName := parts[0], parts[1]
			keys, isUpstream := upstreamOutputs[sourceID]
			if !isUpstream {
				if _, exists := nodeIndex[sourceID]; !exists {
					issues = append(issues, ValidationIssue{
						Code:    "reference_node_missing",
						Message: fmt.Sprintf("节点 %s 的%s引用了不存在的节点 %q", node.ID, context, sourceID),
						NodeID:  node.ID,
					})
				} else {
					issues = append(issues, ValidationIssue{
						Code:    "reference_not_upstream",
						Message: fmt.Sprintf("节点 %s 的%s引用了非上游节点 %q", node.ID, context, sourceID),
						NodeID:  node.ID,
					})
				}
				return
			}
			if _, ok := keys[keyName]; !ok {
				issues = append(issues, ValidationIssue{
					Code:    "reference_key_missing",
					Message: fmt.Sprintf("节点 %s 的%s引用了 %s 不存在的输出 %q", node.ID, context, sourceID, keyName),
					NodeID:  node.ID,
				})
			}
		}
		// ref_key_name 只在 inputs 上有意义：box-app 只把 node["inputs"] 交给 resolveInputs，
		// 节点输出是按变量名捕获的，outputs 上的 ref_key_name 不参与解析。
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			check(asString(port["ref_key_name"]), "参数")
		}
		// 插值只发生在 box-app 的 6 个调用点上，其余字段写了 {xxx} 只是普通文本。
		// 尤其 python_script 的 script_content 不插值——脚本里的 f-string 必须放行。
		interpKeys := interpolationInputKeys(node.Type)
		if len(interpKeys) > 0 {
			for _, port := range asMapSlice(node.Properties["inputs"]) {
				keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
				if _, ok := interpKeys[keyName]; !ok {
					continue
				}
				for _, placeholder := range interpolationPattern.FindAllStringSubmatch(asString(portDefault(port)), -1) {
					check(placeholder[1], fmt.Sprintf("参数 %s", keyName))
				}
			}
			if config, ok := node.Properties["config"].(map[string]interface{}); ok {
				for key := range interpKeys {
					for _, placeholder := range interpolationPattern.FindAllStringSubmatch(asString(config[key]), -1) {
						check(placeholder[1], fmt.Sprintf("配置项 %s", key))
					}
				}
			}
		}
	}
	for _, edge := range document.Edges {
		expression := asString(edge.Properties["condition_expression"])
		if strings.TrimSpace(expression) == "" {
			continue
		}
		upstream := upstreamNodes(document, edge.SourceNodeID)
		// 条件表达式写在自己的源节点及其上游之上。
		if _, ok := nodeIndex[edge.SourceNodeID]; ok {
			upstream[edge.SourceNodeID] = struct{}{}
		}
		for _, placeholder := range interpolationPattern.FindAllStringSubmatch(expression, -1) {
			parts := strings.SplitN(placeholder[1], ".", 2)
			if len(parts) != 2 {
				continue
			}
			if _, ok := upstream[parts[0]]; !ok {
				issues = append(issues, ValidationIssue{
					Code:    "condition_reference_invalid",
					Message: fmt.Sprintf("连线 %s 的条件表达式引用了非上游节点 %q", edge.ID, parts[0]),
					EdgeID:  edge.ID,
					NodeID:  edge.SourceNodeID,
				})
				continue
			}
			if target, ok := nodeIndex[parts[0]]; ok {
				if _, ok := outputKeysOf(target)[parts[1]]; !ok {
					issues = append(issues, ValidationIssue{
						Code:    "condition_key_missing",
						Message: fmt.Sprintf("连线 %s 的条件表达式引用了 %s 不存在的输出 %q", edge.ID, parts[0], parts[1]),
						EdgeID:  edge.ID,
						NodeID:  edge.SourceNodeID,
					})
				}
			}
		}
	}
	return issues
}

// upstreamNodes 返回能到达 origin 的所有节点（不含自身）。
func upstreamNodes(document *graphDocument, origin string) map[string]struct{} {
	result := map[string]struct{}{}
	visited := map[string]struct{}{origin: {}}
	var walk func(target string)
	walk = func(target string) {
		for _, edge := range document.Edges {
			if edge.TargetNodeID != target {
				continue
			}
			source := edge.SourceNodeID
			if source == origin {
				continue
			}
			if _, ok := visited[source]; ok {
				continue
			}
			visited[source] = struct{}{}
			result[source] = struct{}{}
			walk(source)
		}
	}
	walk(origin)
	return result
}

// ---------------------------------------------------------------------------
// 必填与类型
// ---------------------------------------------------------------------------

func (v *WorkflowValidator) validateRequiredInputs(document *graphDocument, nodeIndex map[string]*graphNode) []ValidationIssue {
	issues := []ValidationIssue{}
	for _, node := range document.Nodes {
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			required, _ := port["required"].(bool)
			if !required {
				continue
			}
			keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
			if strings.TrimSpace(asString(port["ref_key_name"])) != "" {
				continue
			}
			if isBlankValue(portDefault(port)) {
				issues = append(issues, ValidationIssue{
					Code:    "required_input_missing",
					Message: fmt.Sprintf("节点 %s 的必填输入 %q 既没有引用上游也没有默认值", node.ID, keyName),
					NodeID:  node.ID,
				})
			}
		}
	}
	return issues
}

func (v *WorkflowValidator) validateReferenceTypes(document *graphDocument, nodeIndex map[string]*graphNode) []ValidationIssue {
	issues := []ValidationIssue{}
	for _, node := range document.Nodes {
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			ref := strings.TrimSpace(asString(port["ref_key_name"]))
			if ref == "" {
				continue
			}
			parts := strings.SplitN(ref, ".", 2)
			if len(parts) != 2 {
				continue
			}
			source, ok := nodeIndex[parts[0]]
			if !ok {
				continue
			}
			sourceType := ""
			for _, sourcePort := range asMapSlice(source.Properties["outputs"]) {
				if firstNonEmpty(asString(sourcePort["key_name"]), asString(sourcePort["name"])) == parts[1] {
					sourceType = asString(sourcePort["type"])
					break
				}
			}
			targetType := asString(port["type"])
			if sourceType == "" || targetType == "" || sourceType == targetType {
				continue
			}
			// json/object/array 之间视为同一族，避免误报。
			if isStructuredType(sourceType) && isStructuredType(targetType) {
				continue
			}
			issues = append(issues, ValidationIssue{
				Code:    "reference_type_mismatch",
				Message: fmt.Sprintf("节点 %s 的输入 %q 类型为 %s，但引用的 %s 输出类型为 %s", node.ID, asString(port["key_name"]), targetType, parts[0], sourceType),
				NodeID:  node.ID,
			})
		}
	}
	return issues
}

func isStructuredType(value string) bool {
	switch strings.ToLower(value) {
	case "json", "object", "array":
		return true
	default:
		return false
	}
}

// templateOutputKeys 返回模板定义的输出 key 集合（统一小写）。
// 模板为 nil 时返回空集合，表示该类型没有内置输出。
func templateOutputKeys(template *models.NodeTemplate) map[string]struct{} {
	keys := map[string]struct{}{}
	if template == nil {
		return keys
	}
	for _, variable := range template.Variables {
		if variable.Direction != models.VariableDirectionOutput {
			continue
		}
		if keyName := strings.TrimSpace(variable.KeyName); keyName != "" {
			keys[strings.ToLower(keyName)] = struct{}{}
		}
	}
	return keys
}

// validateUnusedOutputs 检查「声明了但从不使用」的输出。
//
// 典型症状：AI 生成时声明 need_notify 输出、脚本里也确实置位，但没有任何连线或引用使用它，
// 导致「通知」永远发不出去。这类问题校验器不报、运行也不报错，属于静默失效，
// 只能靠这条规则提前发现。
//
// 判定为「被使用」的三种情况：
//   - 被其他节点的 ref_key_name 引用
//   - 被其他节点的默认值占位符 {<节点id>.<key>} 引用
//   - 被某条连线的 condition_expression 引用
//
// 报 warning 而不是 error：编排允许先声明输出、之后再接下游的渐进式编辑。
func (v *WorkflowValidator) validateUnusedOutputs(document *graphDocument, nodeIndex map[string]*graphNode) []ValidationIssue {
	issues := []ValidationIssue{}

	// 收集所有被引用过的 "<节点id>.<输出key>"（统一小写便于比较）
	used := map[string]struct{}{}
	mark := func(nodeID, keyName string) {
		nodeID = strings.TrimSpace(nodeID)
		keyName = strings.TrimSpace(keyName)
		if nodeID == "" || keyName == "" {
			return
		}
		used[strings.ToLower(nodeID+"."+keyName)] = struct{}{}
	}
	markReference := func(raw string) {
		ref := normalizeReference(raw)
		if ref == "" || !strings.Contains(ref, ".") {
			return
		}
		parts := strings.SplitN(ref, ".", 2)
		mark(parts[0], parts[1])
	}

	for _, node := range document.Nodes {
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			markReference(asString(port["ref_key_name"]))
			for _, placeholder := range interpolationPattern.FindAllStringSubmatch(asString(portDefault(port)), -1) {
				markReference(placeholder[1])
			}
		}
		// 只有真正会插值的字段才算使用，避免把普通文本里的 {x.y} 误当作引用
		if keys := interpolationInputKeys(node.Type); len(keys) > 0 {
			if config, ok := node.Properties["config"].(map[string]interface{}); ok {
				for key := range keys {
					for _, placeholder := range interpolationPattern.FindAllStringSubmatch(asString(config[key]), -1) {
						markReference(placeholder[1])
					}
				}
			}
		}
	}
	for _, edge := range document.Edges {
		for _, placeholder := range interpolationPattern.FindAllStringSubmatch(asString(edge.Properties["condition_expression"]), -1) {
			markReference(placeholder[1])
		}
	}

	for id, node := range nodeIndex {
		// 结束节点没有输出语义；成对节点的输出由系统管理
		if node.Type == "end" {
			continue
		}
		if template, ok := v.templates[node.Type]; ok && template.GroupType == models.NodeGroupTypePaired {
			continue
		}
		// 固定契约节点（detection_filter、image_annotator 等）的输出是模板内置的一整套能力，
		// 大量输出在实际编排里用不到属于正常情况（例如 matched_detections 常常只用 match_count）。
		// 只对**用户自由声明端口**的节点（python_script、task_get_context 等）检查未使用输出，
		// 这类节点的输出是模型自己声明的，声明了却不用才值得提醒。
		if !isUserDefinedPortType(node.Type) {
			continue
		}
		// 模板自带的标准输出（task_get_context 的 context、task_set_context 的 saved 等）
		// 是节点的内置能力，编排不引用属于正常情况，不参与「未使用」检查。
		// 只有**模型自己额外声明**的输出，声明了却不用才值得提醒。
		builtinOutputs := templateOutputKeys(v.templates[node.Type])
		for _, port := range asMapSlice(node.Properties["outputs"]) {
			keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
			if keyName == "" {
				continue
			}
			if _, ok := builtinOutputs[strings.ToLower(keyName)]; ok {
				continue
			}
			if _, ok := used[strings.ToLower(id+"."+keyName)]; ok {
				continue
			}
			issues = append(issues, ValidationIssue{
				Code:    "unused_output",
				Message: fmt.Sprintf("节点 %s 的输出 %q 声明了但没有任何地方使用（既没有被引用，也没有参与分支条件），请确认是否需要保留", id, keyName),
				NodeID:  id,
			})
		}
	}
	return issues
}

func (v *WorkflowValidator) validatePythonOutputs(nodeIndex map[string]*graphNode) []ValidationIssue {
	issues := []ValidationIssue{}
	for id, node := range nodeIndex {
		if node.Type != "python_script" {
			continue
		}
		script := ""
		if config, ok := node.Properties["config"].(map[string]interface{}); ok {
			script = asString(config["script_content"])
		}
		if strings.TrimSpace(script) == "" {
			script = asString(node.Properties["python_script"])
		}
		if strings.TrimSpace(script) == "" {
			issues = append(issues, ValidationIssue{
				Code:    "python_script_empty",
				Message: fmt.Sprintf("Python 节点 %s 没有脚本内容", id),
				NodeID:  id,
			})
			continue
		}
		if err := validatePythonScript(script, 0); err != nil {
			issues = append(issues, ValidationIssue{
				Code:    "python_script_invalid",
				Message: fmt.Sprintf("Python 节点 %s 的脚本不符合运行契约：%v", id, err),
				NodeID:  id,
			})
		}
		for _, port := range asMapSlice(node.Properties["outputs"]) {
			keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
			if keyName == "" {
				continue
			}
			if !pythonScriptAssigns(script, keyName) {
				issues = append(issues, ValidationIssue{
					Code:    "python_output_unassigned",
					Message: fmt.Sprintf("Python 节点 %s 声明的输出 %q 在脚本中没有被赋值", id, keyName),
					NodeID:  id,
				})
			}
		}
	}
	return issues
}

// pythonScriptAssigns 用正则粗判脚本是否给某个顶层变量赋过值。
func pythonScriptAssigns(script, keyName string) bool {
	pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(keyName) + `\s*(\[[^\]]*\])?\s*=`)
	return pattern.MatchString(script)
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// portDefault 读取端口默认值。
// 规范字段是 default_value（与 box-app resolveInputs 的取值优先级一致），
// 同时兼容画布历史数据里遗留的 default。
func portDefault(port map[string]interface{}) interface{} {
	if value, ok := port["default_value"]; ok && value != nil {
		return value
	}
	return port["default"]
}

// asMapSlice 把任意 JSON 数组转成 map 切片，忽略无法转换的元素。
func asMapSlice(value interface{}) []map[string]interface{} {
	items, ok := value.([]interface{})
	if !ok {
		if typed, ok := value.([]map[string]interface{}); ok {
			return typed
		}
		return nil
	}
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if typed, ok := item.(map[string]interface{}); ok {
			result = append(result, typed)
		}
	}
	return result
}

// isBlankValue 判断默认值是否“空”（nil、空串、空数组、空对象）。
func isBlankValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []interface{}:
		return len(typed) == 0
	case map[string]interface{}:
		return len(typed) == 0
	default:
		return false
	}
}
