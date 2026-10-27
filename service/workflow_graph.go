/*
 * @module service/workflow_graph
 * @description 编排图的精简（发送给大模型）与补全（补齐模板 ID、坐标、连线样式）。
 *              精简格式即“去掉视图字段的 LogicFlow JSON”，不另外发明语法，
 *              大模型照着模板写即可，返回后再由 completeGraph 还原成可导入的完整 JSON。
 */

package service

import (
	"box-manage-service/models"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// 精简格式（大模型输入 / 输出）
// ---------------------------------------------------------------------------

// SimpleNode 精简节点：只保留业务语义字段。
type SimpleNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// SimpleEdge 精简连线：只保留端点与条件语义。
type SimpleEdge struct {
	ID           string                 `json:"id"`
	SourceNodeID string                 `json:"sourceNodeId"`
	TargetNodeID string                 `json:"targetNodeId"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// SimpleGraph 精简编排图。
type SimpleGraph struct {
	Nodes []SimpleNode `json:"nodes"`
	Edges []SimpleEdge `json:"edges"`
}

// ---------------------------------------------------------------------------
// LogicFlow 完整格式（画布导入用）
// ---------------------------------------------------------------------------

// graphText LogicFlow 的文本字段，既可能是字符串也可能是 {value,x,y}。
type graphText struct {
	Value string  `json:"value"`
	X     float64 `json:"x,omitempty"`
	Y     float64 `json:"y,omitempty"`
}

// UnmarshalJSON 兼容字符串与对象两种写法。
func (t *graphText) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		t.Value = value
		return nil
	}
	type alias graphText
	var payload alias
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*t = graphText(payload)
	return nil
}

// graphNode LogicFlow 节点。
type graphNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	X          *float64               `json:"x,omitempty"`
	Y          *float64               `json:"y,omitempty"`
	Text       *graphText             `json:"text,omitempty"`
	Properties map[string]interface{} `json:"properties"`
}

// graphEdge LogicFlow 连线；同时兼容后端 line 字段命名。
type graphEdge struct {
	ID           string                 `json:"id"`
	Label        string                 `json:"-"`
	Type         string                 `json:"type,omitempty"`
	SourceNodeID string                 `json:"sourceNodeId,omitempty"`
	TargetNodeID string                 `json:"targetNodeId,omitempty"`
	Text         *graphText             `json:"text,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
}

// graphDocument LogicFlow 图文档。
type graphDocument struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
	Lines []graphEdge `json:"lines"`
}

// 视图字段：发送给大模型前一律删除，模型不需要为它们浪费 token。
var nodeViewKeys = map[string]struct{}{
	"x": {}, "y": {}, "width": {}, "height": {}, "node_template_id": {},
}

// ---------------------------------------------------------------------------
// 解析
// ---------------------------------------------------------------------------

// parseGraph 把任意来源的 JSON 解析为 LogicFlow 图文档。
// 支持完整 LogicFlow JSON、精简 JSON，以及后端 lines/source_node_id 命名。
func parseGraph(raw json.RawMessage) (*graphDocument, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("编排内容为空")
	}
	var document graphDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		// 再尝试一次“纯后端结构”解析，便于兼容历史数据。
		fallback, fallbackErr := parseBackendStructure(raw)
		if fallbackErr != nil {
			return nil, fmt.Errorf("解析编排 JSON 失败: %w", err)
		}
		return fallback, nil
	}
	if len(document.Edges) == 0 && len(document.Lines) > 0 {
		document.Edges = document.Lines
	}
	if len(document.Nodes) == 0 {
		return nil, fmt.Errorf("编排 JSON 中没有任何节点")
	}
	// 后端结构（node_id/type_key）也能被上面的 Unmarshal 接受，只是字段全空。
	// 这种情况下改走 parseBackendStructure，否则会得到一堆没有 id 的节点。
	if nodesLookUnparsed(document.Nodes) {
		if fallback, fallbackErr := parseBackendStructure(raw); fallbackErr == nil {
			return fallback, nil
		}
	}
	for i := range document.Edges {
		edge := &document.Edges[i]
		if edge.SourceNodeID == "" {
			edge.SourceNodeID = stringProperty(edge.Properties, "source", "source_node_id")
		}
		if edge.TargetNodeID == "" {
			edge.TargetNodeID = stringProperty(edge.Properties, "target", "target_node_id")
		}
		if edge.Type == "" {
			edge.Type = "polyline"
		}
	}
	for i := range document.Nodes {
		if document.Nodes[i].Properties == nil {
			document.Nodes[i].Properties = map[string]interface{}{}
		}
	}
	return &document, nil
}

// nodesLookUnparsed 判断节点是否缺少 LogicFlow 必需的 id/type（说明这份 JSON 不是画布格式）。
func nodesLookUnparsed(nodes []graphNode) bool {
	missing := 0
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) == "" || strings.TrimSpace(node.Type) == "" {
			missing++
		}
	}
	// 允许个别节点靠 properties.type 兜底，但多数节点缺字段就判定为格式不符。
	return missing > 0 && missing*2 >= len(nodes)
}

// parseBackendStructure 兼容后端的 nodes/lines/variables 结构。
func parseBackendStructure(raw json.RawMessage) (*graphDocument, error) {
	var payload struct {
		Nodes []struct {
			NodeID         string                 `json:"node_id"`
			NodeIDAlt      string                 `json:"id"`
			TypeKey        string                 `json:"type_key"`
			TypeKeyAlt     string                 `json:"type"`
			NodeName       string                 `json:"node_name"`
			NodeKeyName    string                 `json:"node_key_name"`
			GroupType      string                 `json:"group_type"`
			NodeTemplateID uint                   `json:"node_template_id"`
			Config         map[string]interface{} `json:"config"`
			Inputs         []interface{}          `json:"inputs"`
			Outputs        []interface{}          `json:"outputs"`
			Position       map[string]interface{} `json:"position"`
		} `json:"nodes"`
		Lines []struct {
			LineID                  string `json:"line_id"`
			ID                      string `json:"id"`
			SourceNodeID            string `json:"source_node_id"`
			TargetNodeID            string `json:"target_node_id"`
			ConditionType           string `json:"condition_type"`
			LogicType               string `json:"logic_type"`
			ConditionExpression     string `json:"condition_expression"`
			ConditionExpressionView string `json:"condition_expression_view"`
			Description             string `json:"description"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if len(payload.Nodes) == 0 {
		return nil, fmt.Errorf("编排 JSON 中没有任何节点")
	}
	document := &graphDocument{}
	for i, node := range payload.Nodes {
		id := firstNonEmpty(node.NodeID, node.NodeIDAlt)
		if id == "" {
			return nil, fmt.Errorf("第 %d 个节点缺少 node_id", i+1)
		}
		properties := map[string]interface{}{
			"key_name": firstNonEmpty(node.NodeKeyName, id),
			"name":     firstNonEmpty(node.NodeName, node.TypeKey, node.TypeKeyAlt),
			"type":     firstNonEmpty(node.TypeKey, node.TypeKeyAlt),
			"config":   node.Config,
			"inputs":   node.Inputs,
			"outputs":  node.Outputs,
		}
		if node.NodeTemplateID > 0 {
			properties["node_template_id"] = node.NodeTemplateID
		}
		if node.GroupType != "" {
			properties["group_type"] = node.GroupType
		}
		entry := graphNode{ID: id, Type: firstNonEmpty(node.TypeKey, node.TypeKeyAlt), Properties: properties}
		if node.Position != nil {
			if x, ok := toFloat(node.Position["x"]); ok {
				entry.X = &x
			}
			if y, ok := toFloat(node.Position["y"]); ok {
				entry.Y = &y
			}
		}
		document.Nodes = append(document.Nodes, entry)
	}
	for _, line := range payload.Lines {
		properties := map[string]interface{}{"condition_type": firstNonEmpty(line.ConditionType, "none")}
		if line.ConditionType != "" && line.ConditionType != "none" {
			properties["logic_type"] = firstNonEmpty(line.LogicType, "and")
			properties["condition_expression"] = line.ConditionExpression
			properties["condition_expression_view"] = line.ConditionExpressionView
			properties["description"] = line.Description
		}
		document.Edges = append(document.Edges, graphEdge{
			ID:           firstNonEmpty(line.LineID, line.ID),
			Type:         "polyline",
			SourceNodeID: line.SourceNodeID,
			TargetNodeID: line.TargetNodeID,
			Properties:   properties,
		})
	}
	return document, nil
}

// ---------------------------------------------------------------------------
// 精简：完整图 -> 发送给大模型的精简图
// ---------------------------------------------------------------------------

// simplifyGraph 删除纯视图字段，保留业务语义字段。
func simplifyGraph(document *graphDocument) *SimpleGraph {
	simple := &SimpleGraph{Nodes: make([]SimpleNode, 0, len(document.Nodes)), Edges: make([]SimpleEdge, 0, len(document.Edges))}
	for _, node := range document.Nodes {
		properties := map[string]interface{}{}
		for key, value := range node.Properties {
			if _, isViewField := nodeViewKeys[key]; isViewField {
				continue
			}
			properties[key] = value
		}
		if _, ok := properties["key_name"]; !ok {
			properties["key_name"] = node.ID
		}
		if _, ok := properties["name"]; !ok {
			if node.Text != nil && node.Text.Value != "" {
				properties["name"] = node.Text.Value
			}
		}
		simple.Nodes = append(simple.Nodes, SimpleNode{ID: node.ID, Type: node.Type, Properties: properties})
	}
	for _, edge := range document.Edges {
		properties := map[string]interface{}{}
		for key, value := range edge.Properties {
			if key == "style" {
				continue
			}
			properties[key] = value
		}
		if len(properties) == 0 {
			properties = nil
		}
		simple.Edges = append(simple.Edges, SimpleEdge{
			ID:           edge.ID,
			SourceNodeID: edge.SourceNodeID,
			TargetNodeID: edge.TargetNodeID,
			Properties:   properties,
		})
	}
	return simple
}

// ---------------------------------------------------------------------------
// 补全：精简图 -> 可导入的完整 LogicFlow 图
// ---------------------------------------------------------------------------

// completeOptions 补全时需要的模板信息。
type completeOptions struct {
	// Templates 以 type_key 为键的启用节点模板。
	Templates map[string]*models.NodeTemplate
}

// completeGraph 补齐 node_template_id、节点文本、坐标（自动布局）与连线样式。
func completeGraph(simple *SimpleGraph, options completeOptions) (*graphDocument, error) {
	if simple == nil || len(simple.Nodes) == 0 {
		return nil, fmt.Errorf("编排中没有任何节点")
	}
	document := &graphDocument{
		Nodes: make([]graphNode, 0, len(simple.Nodes)),
		Edges: make([]graphEdge, 0, len(simple.Edges)),
	}
	seen := map[string]struct{}{}
	for i, node := range simple.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			return nil, fmt.Errorf("第 %d 个节点缺少 id", i+1)
		}
		if _, duplicated := seen[id]; duplicated {
			return nil, fmt.Errorf("节点 id 重复: %s", id)
		}
		seen[id] = struct{}{}
		nodeType := strings.TrimSpace(node.Type)
		if nodeType == "" {
			nodeType = stringProperty(node.Properties, "type", "type_key")
		}
		properties := map[string]interface{}{}
		for key, value := range node.Properties {
			if _, isViewField := nodeViewKeys[key]; isViewField {
				continue
			}
			properties[key] = value
		}
		properties["type"] = nodeType
		if _, ok := properties["key_name"]; !ok || strings.TrimSpace(asString(properties["key_name"])) == "" {
			properties["key_name"] = id
		}
		name := strings.TrimSpace(asString(properties["name"]))
		if name == "" {
			if template, ok := options.Templates[nodeType]; ok {
				name = template.TypeName
			} else {
				name = nodeType
			}
		}
		properties["name"] = name
		// 端口默认值统一成 default_value（模型偶尔仍会按旧写法返回 default）
		normalizePortDefaults(properties)
		// 配置项被误放进 inputs 时静默移除，避免它顶掉 config 里的真实值
		_ = dropConfigKeysFromInputs(properties)
		if template, ok := options.Templates[nodeType]; ok && template != nil {
			properties["node_template_id"] = template.ID
			if _, ok := properties["group_type"]; !ok && template.GroupType != "" {
				properties["group_type"] = string(template.GroupType)
			}
		} else if _, ok := properties["node_template_id"]; !ok {
			properties["node_template_id"] = uint(0)
		}
		if _, ok := properties["config"]; !ok || properties["config"] == nil {
			properties["config"] = map[string]interface{}{}
		}
		if _, ok := properties["inputs"]; !ok || properties["inputs"] == nil {
			properties["inputs"] = []interface{}{}
		}
		if _, ok := properties["outputs"]; !ok || properties["outputs"] == nil {
			properties["outputs"] = []interface{}{}
		}
		document.Nodes = append(document.Nodes, graphNode{
			ID:         id,
			Type:       nodeType,
			Text:       &graphText{Value: name},
			Properties: properties,
		})
	}
	for i, edge := range simple.Edges {
		properties := map[string]interface{}{}
		for key, value := range edge.Properties {
			if key == "style" {
				continue
			}
			properties[key] = value
		}
		conditionType := asString(properties["condition_type"])
		if conditionType == "" {
			conditionType = "none"
			properties["condition_type"] = conditionType
		}
		if conditionType != "none" {
			properties["logic_type"] = firstNonEmpty(asString(properties["logic_type"]), "and")
			properties["style"] = map[string]interface{}{"stroke": "#fa8c16", "strokeWidth": 2}
		} else {
			properties["style"] = map[string]interface{}{"stroke": "#666", "strokeWidth": 1}
		}
		id := strings.TrimSpace(edge.ID)
		if id == "" {
			id = fmt.Sprintf("line-%03d", i+1)
		}
		document.Edges = append(document.Edges, graphEdge{
			ID:           id,
			Type:         "polyline",
			SourceNodeID: strings.TrimSpace(edge.SourceNodeID),
			TargetNodeID: strings.TrimSpace(edge.TargetNodeID),
			Properties:   properties,
		})
	}
	alignContextNodePortTypes(document)
	layoutGraph(document)
	return document, nil
}

// alignContextNodePortTypes 把「上下文读写节点」的端口类型对齐到上游实际类型。
//
// 背景：task_get_context / task_set_context 的输入输出由编排自己声明，
// 模型经常把所有端口一律写成 string（线上实测每次生成都产生 5 条类型不匹配警告）。
// 这类端口只用于表达与校验，box-app 运行时按 JSON 原值处理、不看声明类型，
// 因此可以直接按上游真实类型纠正，既消掉噪音又不会改变运行行为。
func alignContextNodePortTypes(document *graphDocument) {
	if document == nil {
		return
	}
	outputTypes := make(map[string]map[string]string, len(document.Nodes))
	for i := range document.Nodes {
		portTypes := map[string]string{}
		for _, port := range asMapSlice(document.Nodes[i].Properties["outputs"]) {
			keyName := firstNonEmpty(asString(port["key_name"]), asString(port["name"]))
			if keyName == "" {
				continue
			}
			if portType := asString(port["type"]); portType != "" {
				portTypes[keyName] = portType
			}
		}
		outputTypes[document.Nodes[i].ID] = portTypes
	}

	for i := range document.Nodes {
		node := &document.Nodes[i]
		if node.Type != "task_set_context" && node.Type != "task_get_context" {
			continue
		}
		for _, port := range asMapSlice(node.Properties["inputs"]) {
			reference := normalizeReference(asString(port["ref_key_name"]))
			if !strings.Contains(reference, ".") {
				continue
			}
			parts := strings.SplitN(reference, ".", 2)
			upstreamTypes, ok := outputTypes[parts[0]]
			if !ok {
				continue
			}
			actualType, ok := upstreamTypes[parts[1]]
			if !ok || actualType == "" {
				continue
			}
			if asString(port["type"]) != actualType {
				port["type"] = actualType
			}
		}
	}
}

// layoutGraph 按拓扑分层自动布局：x = 120 + 层号 × 200，同层 y 间隔 120。
func layoutGraph(document *graphDocument) {
	if document == nil || len(document.Nodes) == 0 {
		return
	}
	order := make([]string, 0, len(document.Nodes))
	indexOf := map[string]int{}
	for i, node := range document.Nodes {
		order = append(order, node.ID)
		indexOf[node.ID] = i
	}
	adjacency := map[string][]string{}
	indegree := map[string]int{}
	for _, id := range order {
		indegree[id] = 0
	}
	for _, edge := range document.Edges {
		if _, ok := indegree[edge.SourceNodeID]; !ok {
			continue
		}
		if _, ok := indegree[edge.TargetNodeID]; !ok {
			continue
		}
		adjacency[edge.SourceNodeID] = append(adjacency[edge.SourceNodeID], edge.TargetNodeID)
		indegree[edge.TargetNodeID]++
	}

	// 从开始节点出发做最长路径分层；存在环时用松弛次数上限兜底。
	startID := ""
	for _, node := range document.Nodes {
		if node.Type == "start" {
			startID = node.ID
			break
		}
	}
	layer := map[string]int{}
	if startID == "" && len(order) > 0 {
		startID = order[0]
	}
	queue := []string{startID}
	layer[startID] = 0
	maxRelaxation := len(order)*len(order) + len(order) + 1
	relaxations := 0
	for len(queue) > 0 && relaxations < maxRelaxation {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			relaxations++
			if relaxations >= maxRelaxation {
				break
			}
			if currentLayer, ok := layer[next]; !ok || currentLayer < layer[current]+1 {
				layer[next] = layer[current] + 1
				queue = append(queue, next)
			}
		}
	}
	// 不可达节点统一放在最后一层之后，交由校验器提示。
	maxLayer := 0
	for _, value := range layer {
		if value > maxLayer {
			maxLayer = value
		}
	}
	unreachableLayer := maxLayer + 1
	for _, id := range order {
		if _, ok := layer[id]; !ok {
			layer[id] = unreachableLayer
		}
	}

	// 同层内按原有顺序排列，保证结果稳定。
	layers := map[int][]string{}
	for _, id := range order {
		layers[layer[id]] = append(layers[layer[id]], id)
	}
	layerNumbers := make([]int, 0, len(layers))
	for number := range layers {
		layerNumbers = append(layerNumbers, number)
	}
	sort.Ints(layerNumbers)
	for _, number := range layerNumbers {
		ids := layers[number]
		for position, id := range ids {
			index := indexOf[id]
			x := float64(120 + number*200)
			y := float64(100 + position*120)
			document.Nodes[index].X = &x
			document.Nodes[index].Y = &y
			if document.Nodes[index].Text != nil {
				document.Nodes[index].Text.X = x
				document.Nodes[index].Text.Y = y
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

// graphFromWorkflow 从已保存的编排还原 LogicFlow 图，供模板预览与精简使用。
func graphFromWorkflow(workflow *models.Workflow) (*graphDocument, error) {
	if workflow == nil {
		return nil, fmt.Errorf("编排不存在")
	}
	if view := strings.TrimSpace(workflow.StructureJSONView); view != "" {
		if document, err := parseGraph(json.RawMessage(view)); err == nil {
			return document, nil
		}
	}
	if len(workflow.Nodes) == 0 {
		return nil, fmt.Errorf("编排 %q 没有可用的结构数据", workflow.Name)
	}
	// 没有画布视图时，用定义表还原一份等价结构。
	portsByNode := map[string]map[string][]interface{}{}
	for _, variable := range workflow.Variables {
		if portsByNode[variable.NodeID] == nil {
			portsByNode[variable.NodeID] = map[string][]interface{}{"input": {}, "output": {}}
		}
		port := map[string]interface{}{
			"name":     variable.Name,
			"key_name": variable.KeyName,
			"type":     variable.Type,
			"required": variable.Required,
			// 规范字段 default_value，与 box-app resolveInputs 及 variable_definitions 列一致。
			"default_value": variable.DefaultValue.Data,
			"description":   variable.Description,
		}
		if variable.RefKeyName != "" {
			port["ref_key_name"] = variable.RefKeyName
		}
		direction := string(variable.Direction)
		if direction != "output" {
			direction = "input"
		}
		portsByNode[variable.NodeID][direction] = append(portsByNode[variable.NodeID][direction], port)
	}
	document := &graphDocument{}
	for _, node := range workflow.Nodes {
		inputs := portsByNode[node.NodeID]["input"]
		outputs := portsByNode[node.NodeID]["output"]
		if len(inputs) == 0 {
			inputs = normalizePorts(node.Inputs)
		}
		if len(outputs) == 0 {
			outputs = normalizePorts(node.Outputs)
		}
		properties := map[string]interface{}{
			"key_name":         firstNonEmpty(node.NodeKeyName, node.NodeID),
			"name":             firstNonEmpty(node.NodeName, node.TypeName),
			"type":             node.TypeKey,
			"node_template_id": node.NodeTemplateID,
			"config":           map[string]interface{}(node.Config),
			"inputs":           inputs,
			"outputs":          outputs,
		}
		if node.GroupType != "" {
			properties["group_type"] = string(node.GroupType)
		}
		if node.PythonScript != "" {
			properties["python_script"] = node.PythonScript
			if config, ok := properties["config"].(map[string]interface{}); ok {
				if _, exists := config["script_content"]; !exists {
					config["script_content"] = node.PythonScript
				}
			}
		}
		entry := graphNode{ID: node.NodeID, Type: node.TypeKey, Properties: properties, Text: &graphText{Value: node.NodeName}}
		if node.Position != nil {
			if x, ok := toFloat(node.Position["x"]); ok {
				entry.X = &x
			}
			if y, ok := toFloat(node.Position["y"]); ok {
				entry.Y = &y
			}
		}
		document.Nodes = append(document.Nodes, entry)
	}
	for _, line := range workflow.Lines {
		properties := map[string]interface{}{"condition_type": firstNonEmpty(string(line.ConditionType), "none")}
		if line.ConditionType != "" && line.ConditionType != models.ConditionTypeNone {
			properties["logic_type"] = firstNonEmpty(string(line.LogicType), "and")
			properties["condition_expression"] = line.ConditionExpression
			properties["condition_expression_view"] = line.ConditionExpressionView
			properties["description"] = line.Description
		}
		document.Edges = append(document.Edges, graphEdge{
			ID:           line.LineID,
			Type:         "polyline",
			SourceNodeID: line.SourceNodeID,
			TargetNodeID: line.TargetNodeID,
			Properties:   properties,
		})
	}
	return document, nil
}

// normalizePorts 把节点上的入参/出参定义补成画布使用的字段名。
func normalizePorts(ports models.JSONArr) []interface{} {
	result := make([]interface{}, 0, len(ports))
	for _, raw := range ports {
		payload, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		var port map[string]interface{}
		if err := json.Unmarshal(payload, &port); err != nil {
			continue
		}
		if asString(port["key_name"]) == "" {
			port["key_name"] = asString(port["name"])
		}
		if asString(port["name"]) == "" {
			port["name"] = asString(port["key_name"])
		}
		result = append(result, port)
	}
	return result
}

func asString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func toFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// stringProperty 依次尝试从属性表里取出第一个非空字符串字段。
// normalizePortDefaults 把 inputs/outputs 里的 default 迁移到规范字段 default_value。
// box-app 的 resolveInputs 两者都认，但统一写法可以避免「同一份数据两种字段名」引发的丢失。
func normalizePortDefaults(properties map[string]interface{}) {
	for _, direction := range []string{"inputs", "outputs"} {
		ports, ok := properties[direction].([]interface{})
		if !ok {
			continue
		}
		for _, raw := range ports {
			port, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if _, hasCanonical := port["default_value"]; !hasCanonical {
				if legacy, hasLegacy := port["default"]; hasLegacy {
					port["default_value"] = legacy
				}
			}
			delete(port, "default")
		}
	}
}

// dropConfigKeysFromInputs 把被误放进 inputs 的配置项移除。
//
// 配置项（script_content / timeout_seconds / python_bin 等）属于 properties.config，
// 一旦作为输入端口出现，它的默认值会进入 box-app 的 get_cfg 优先级链，
// 把 config 里的真实值顶掉——线上出现过「脚本明明填了 2818 字符，却报没有脚本内容」。
//
// 这里做静默修正而不是只报警：这类数据多由历史画布或 AI 生成产生，
// 直接阻断保存会让用户无从下手；移除后 config 里的值就能正常生效。
//
// 迁移而非丢弃：前端「数据类型=脚本」的输入参数把脚本内容存在端口的
// default_value 里，而 python_script 的脚本只从 config.script_content 读取。
// 若这里直接把端口删掉，用户明明填了脚本却会被判「没有脚本内容」（线上
// python_script-7ebb 即此原因）。因此移除前先把端口默认值搬到 config，
// 但**只在 config 里该键为空时**才搬，避免把内置参数配置里的真实值顶掉。
//
// 返回被移除的 key 名，供调用方决定是否提示用户。
func dropConfigKeysFromInputs(properties map[string]interface{}) []string {
	ports, ok := properties["inputs"].([]interface{})
	if !ok {
		return nil
	}
	config, _ := properties["config"].(map[string]interface{})
	removed := make([]string, 0)
	kept := make([]interface{}, 0, len(ports))
	for _, raw := range ports {
		port, ok := raw.(map[string]interface{})
		if !ok {
			kept = append(kept, raw)
			continue
		}
		keyName := strings.ToLower(firstNonEmpty(asString(port["key_name"]), asString(port["name"])))
		if _, isConfig := configOnlyInputKeys[keyName]; isConfig {
			migratePortValueToConfig(properties, &config, port, keyName)
			removed = append(removed, keyName)
			continue
		}
		kept = append(kept, raw)
	}
	if len(kept) != len(ports) {
		properties["inputs"] = kept
	}
	return removed
}

// migratePortValueToConfig 把被误当作输入端口的配置项默认值迁回 properties.config。
//
// 取值口径与 box-app 的 resolveInputs 一致：default_value 优先，其次 default。
// 只在 config 中该键缺失或为空时迁移——config 是「内置参数配置」的权威来源，
// 里面已有非空值时不能被端口的旧默认值覆盖。
func migratePortValueToConfig(properties map[string]interface{},
	config *map[string]interface{}, port map[string]interface{}, keyName string) {
	value := port["default_value"]
	if value == nil {
		value = port["default"]
	}
	if isBlankValue(value) {
		return
	}
	if *config == nil {
		*config = map[string]interface{}{}
		properties["config"] = *config
	}
	// config 的键按原样匹配：前端写入的是 script_content 这类小写蛇形名，
	// 但历史数据里可能大小写不一致，这里做一次不区分大小写的存在性判断。
	for existingKey, existingValue := range *config {
		if !strings.EqualFold(existingKey, keyName) {
			continue
		}
		if !isBlankValue(existingValue) {
			return
		}
		(*config)[existingKey] = value
		return
	}
	(*config)[keyName] = value
}

func stringProperty(properties map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := asString(properties[key]); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
