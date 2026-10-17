/*
 * @module service/workflow_ai_prompt
 * @description AI 快速编排的提示词组装：编排规则（固定文本）+ 基础节点说明（由节点模板
 *              动态生成）+ 参考模板编排（精简后）+ 业务需求 + 用户回答 + 上一轮错误。
 */

package service

import (
	"box-manage-service/models"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// workflowAISystemPrompt 固定角色与输出要求。
const workflowAISystemPrompt = `你是 AI 盒子业务编排（DAG）生成助手。用户会给出一个参考编排模板、一项业务需求，以及可用的节点类型说明。
你的任务是输出一张**可直接导入编排画布**的新编排，只输出 JSON，不要输出解释文字，不要输出 markdown 代码围栏。

严格遵守：
1. 只输出一个 JSON 对象，顶层字段为 status。status 取 "generated" 或 "need_clarification"。
2. 不要输出坐标（x/y）、宽高、锚点、折线点、样式等视图字段，这些由系统自动补全。
3. 不要输出 node_template_id，系统会按 type 自动补全。
4. 节点 type 只能使用「基础节点说明」中列出的类型。
5. 参考模板只是写法示例。必须按业务需求调整节点组合、阈值与逻辑，不要原样照抄。
6. 只输出 JSON。`

// 编排规则：变量、引用、条件、上下文、Python 脚本。
const workflowAIRuleText = `【编排规则】
0. 忠于需求（最高优先级）
   - 只实现【业务需求】明确说到的内容。需求里没提到的阈值、状态、跳过、延迟、去重、限流等机制，**一律不要自行增加**。
   - 需求里的数值（如「超过 30 帧」「离开 20 帧」）直接作为判断阈值，不要额外叠加进入态、冷却期之类的中间阈值来变相修改它。
   - 需求的措辞要精确落地：「超过 30 帧」表示第 31 帧才触发（用 > 30），不是第 30 帧（>= 30）。
   - 确实必须假设时，只在 assumptions 里说明，不要偷偷体现在逻辑里。
   - **声明的每个输出都必须被真正使用**：要么被其他节点的 ref_key_name / 占位符引用，要么被某条连线的 condition_expression 引用。不要声明了却从不使用的输出（例如只置位不发送的通知标志）。
   - 需要「通知类」动作时，统一用一个布尔输出（如 send_alarm）驱动连线，不要同时维护 need_notify、should_notify 等多个含义重叠的标志。
1. 结构
   - 必须有且只有一个 type 为 start 的节点，至少一个 type 为 end 的节点。
   - 节点 id 与 properties.key_name 保持一致，格式为 <type>-<序号>，例如 python_script-005、detection_filter-004。
   - 连线必须有 id，格式为 line-<序号>，例如 line-001；sourceNodeId / targetNodeId 必须是已定义节点的 id。
2. 参数结构
   - 参数放在节点的 properties.inputs 与 properties.outputs 数组中，每个参数字段为：
     { "name": "中文名", "key_name": "英文标识", "type": "string|number|boolean|json|array", "required": true|false, "default_value": 任意值, "description": "说明", "ref_key_name": "上游节点id.输出key_name" }
   - 默认值字段名必须是 default_value，不要写 default。
   - key_name 在同一个节点内不能重复。
3. 变量引用
   - 输入参数引用上游输出时填写 ref_key_name，格式为 "<节点id>.<输出key_name>"，被引用节点必须是当前节点的上游（能沿连线走到当前节点）。
   - default_value 与 ref_key_name 二选一：常量写 default_value 并让 ref_key_name 为空；引用上游则填 ref_key_name。
   - 字符串里拼接变量使用 {<节点id>.<key_name>} 占位符，例如 "{\"content\": \"{python_script-005.alarm_message}\"}"。
   - 开始节点（start）的 inputs 用来接收推理上下文，系统会注入，例如 detections、image_base64、frame_index；其他节点通过 ref_key_name 引用它们。
   - **类别 / 枚举类参数（如 detection_filter 的 class_id）默认必须留空（default_value 填 null）**，表示不按该条件过滤。除非【业务需求】或用户的回答里明确给出了具体数值，否则绝对不要自己填一个数字——填错会让整个编排永远不触发，而校验器看不出来。
   - 若实现需求确实依赖某个枚举值（例如「只检测车辆」必须知道车辆的类别 ID），而需求与澄清回答里都没有给出，应返回 need_clarification 反问，不要猜。
4. 条件连线
   - 分支条件线在 edge.properties 中写 "condition_type": "expression"、"condition_expression": "{<节点id>.<布尔输出>}"、"condition_expression_view": "可读的中文说明"、"description": "简短说明"。
   - 无条件连线写 "condition_type": "none"，不要写 condition_expression。
   - 一条无条件连线从上游进入判断分支，各条件线从判断节点连出，注意补上默认分支。
5. 跨帧状态
   - 需要跨帧累计的状态用 task_get_context 读取 → 业务节点处理 → task_set_context 写回；两处参数的 key_name 必须一致。
   - **task_set_context 的 inputs 由你自己按业务定义，没有固定清单**；每个 input 的 type 必须与它引用的上游输出类型完全一致（布尔写 boolean、计数写 number、文本写 string），不要一律写 string，否则会产生类型不匹配警告。
   - task_get_context 的 outputs 同理：除内置的 context 外，你自己声明的每个状态键，type 也要与该状态的真实类型一致。
6. 图片推送
   - 需要推送「带检测框的图片」时，先接 image_annotator 节点（inputs 引用上游 detections 与原图），再引用它的输出。
   - **image_annotator 的输出字段是 image（标注后的 base64）、image_url、detections；没有 annotated_image 这个字段**，写错会导致引用无效。
   - 若只需要把原图透传给下游，可用 detection_filter 的 image 输出，但要在 assumptions 里说明「推送的是原图，不含检测框」。
7. Python 节点
   - 脚本写在 properties.config.script_content 中。
   - 必须是顶层代码片段：禁止 def run / def main / ctx / 顶层 return，不要用函数包裹业务逻辑。
   - 直接使用 inputs 的 key_name 作为变量名，直接给 outputs 的 key_name 赋值；box-app 会自动注入和捕获。
   - 所有用到的库在代码开头显式 import（如 import json），只允许标准库；禁止文件读写、网络、子进程、eval/exec/compile、动态导入。
   - 每个声明的输出都必须在脚本中被赋值。
8. 输出格式（二选一）
   生成编排：
   {
     "status": "generated",
     "name": "编排名称（中文，简洁）",
     "summary": "业务逻辑说明，分点描述主要流程与阈值",
     "assumptions": ["需求未明确的假设，例如未说明阈值，按 10 帧处理"],
     "nodes": [
       { "id": "start-001", "type": "start", "properties": { "key_name": "start-001", "name": "开始", "config": {}, "inputs": [], "outputs": [] } },
       { "id": "detection_filter-004", "type": "detection_filter", "properties": { "key_name": "detection_filter-004", "name": "检测结果过滤", "config": {}, "inputs": [ ... ], "outputs": [ ... ] } }
     ],
     "edges": [
       { "id": "line-001", "sourceNodeId": "start-001", "targetNodeId": "detection_filter-004", "properties": { "condition_type": "none" } }
     ]
   }
   反问用户：
   {
     "status": "need_clarification",
     "questions": [
       { "id": "q1", "question": "车辆对应的检测类别 ID 是多少？（留空表示不按类别过滤）", "options": ["留空（所有类别）", "自定义具体ID"], "multiple": false, "default": "留空（所有类别）" }
     ]
   }
9. 何时反问
   - 只有缺少会直接影响编排结构或关键业务参数、且无法合理假设的信息时才提问（例如检测目标类别、报警渠道、状态切换条件）。
   - 阈值、文案、超时时间等可以合理假设的内容直接取默认值，并写入 assumptions，不要提问。
   - 例外：**类别/枚举型参数缺失时必须反问**（如检测目标类别 ID），因为它无法合理假设，猜错会让编排静默失效。
   - 已经回答过的问题不要再问。
   - 每轮最多 5 个问题，每个问题都要给出 default 建议值。`

// workflowAIQuestion 反问问题。
type workflowAIQuestion struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Multiple bool     `json:"multiple,omitempty"`
	Default  string   `json:"default,omitempty"`
}

// workflowAIClarification 用户对某个问题的回答。
type workflowAIClarification struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// workflowAIPrevious 上一轮模型输出与校验错误，用于修复轮次。
type workflowAIPrevious struct {
	Raw    string   `json:"raw"`
	Errors []string `json:"errors"`
}

// promptContext 组装提示词所需的全部素材。
type promptContext struct {
	Templates      []*models.NodeTemplate
	TemplateGraph  *SimpleGraph
	TemplateName   string
	Description    string
	Clarifications []workflowAIClarification
	Previous       *workflowAIPrevious
	// MaxBytes 为提示词字节上限，<=0 表示不限制。
	MaxBytes int64
}

// buildWorkflowAIPrompt 拼装用户提示词。节点说明过长时按“模板用到的类型优先”降级。
func buildWorkflowAIPrompt(ctx promptContext) (string, error) {
	templateJSON := "（未选择参考模板）"
	if ctx.TemplateGraph != nil {
		payload, err := json.MarshalIndent(ctx.TemplateGraph, "", " ")
		if err != nil {
			return "", fmt.Errorf("序列化参考模板失败: %w", err)
		}
		templateJSON = string(payload)
	}

	var builder strings.Builder
	builder.WriteString(workflowAIRuleText)
	builder.WriteString("\n\n")

	nodeSpec := buildNodeSpecification(ctx.Templates, ctx.TemplateGraph)
	builder.WriteString(nodeSpec)
	builder.WriteString("\n\n")

	builder.WriteString("【参考模板编排】")
	if ctx.TemplateName != "" {
		builder.WriteString("（来自已保存的编排：")
		builder.WriteString(ctx.TemplateName)
		builder.WriteString("）")
	}
	builder.WriteString("\n")
	builder.WriteString(templateJSON)
	builder.WriteString("\n\n")

	builder.WriteString("【业务需求】\n")
	builder.WriteString(strings.TrimSpace(ctx.Description))
	builder.WriteString("\n")

	if len(ctx.Clarifications) > 0 {
		builder.WriteString("\n【用户对上一轮问题的回答】\n")
		for _, clarification := range ctx.Clarifications {
			builder.WriteString("- 问题：")
			builder.WriteString(strings.TrimSpace(clarification.Question))
			builder.WriteString("\n  回答：")
			builder.WriteString(strings.TrimSpace(clarification.Answer))
			builder.WriteString("\n")
		}
	}

	if ctx.Previous != nil && strings.TrimSpace(ctx.Previous.Raw) != "" {
		builder.WriteString("\n【上一轮你返回的编排（未通过校验，请修复后重新完整输出）】\n")
		builder.WriteString(strings.TrimSpace(ctx.Previous.Raw))
		builder.WriteString("\n\n【需要修复的问题】\n")
		for index, message := range ctx.Previous.Errors {
			builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, message))
		}
		builder.WriteString("\n请只输出修复后的完整 JSON，保持与上一轮相同的结构，不要输出解释文字。\n")
	} else {
		builder.WriteString("\n请输出 JSON。\n")
	}

	prompt := builder.String()
	if ctx.MaxBytes > 0 && int64(len(prompt)) > ctx.MaxBytes {
		return "", fmt.Errorf("提示词长度 %d 字节，超过上限 %d 字节；请减少参考模板规模或简化需求描述", len(prompt), ctx.MaxBytes)
	}
	return prompt, nil
}

// buildNodeSpecification 生成「基础节点说明」。
// 模板编排中用到的类型给出完整参数说明，其余类型只给 type + 名称 + 描述，控制提示词长度。
func buildNodeSpecification(templates []*models.NodeTemplate, templateGraph *SimpleGraph) string {
	usedTypes := map[string]struct{}{}
	if templateGraph != nil {
		for _, node := range templateGraph.Nodes {
			usedTypes[node.Type] = struct{}{}
		}
	}
	sorted := make([]*models.NodeTemplate, 0, len(templates))
	for _, template := range templates {
		if template == nil || !template.IsEnabled {
			continue
		}
		sorted = append(sorted, template)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Category != sorted[j].Category {
			return sorted[i].Category < sorted[j].Category
		}
		return sorted[i].SortOrder < sorted[j].SortOrder
	})

	var builder strings.Builder
	builder.WriteString("【基础节点说明】\n")
	builder.WriteString("字段含义：type=节点类型（写入节点的 type 字段）；group_type=single 单节点 / paired 成对节点；参数里的 ref_key_name 表示该参数应引用上游输出。\n")
	for _, template := range sorted {
		_, isUsed := usedTypes[template.TypeKey]
		if isUsed {
			builder.WriteString("\n- type: ")
			builder.WriteString(template.TypeKey)
			builder.WriteString("\n  名称: ")
			builder.WriteString(template.TypeName)
			builder.WriteString("\n  分组: ")
			builder.WriteString(string(template.GroupType))
			if template.Description != "" {
				builder.WriteString("\n  说明: ")
				builder.WriteString(singleLine(template.Description))
			}
			writeTemplatePorts(&builder, template)
			if configSchema := describeConfigSchema(template.ConfigSchema); configSchema != "" {
				builder.WriteString("\n  配置项: ")
				builder.WriteString(configSchema)
			}
			continue
		}
		builder.WriteString("\n- type: ")
		builder.WriteString(template.TypeKey)
		builder.WriteString("（")
		builder.WriteString(template.TypeName)
		builder.WriteString("，")
		builder.WriteString(string(template.GroupType))
		builder.WriteString("）")
		if template.Description != "" {
			builder.WriteString(" — ")
			builder.WriteString(singleLine(template.Description))
		}
	}
	return builder.String()
}

// writeTemplatePorts 输出模板的固定入参/出参。
func writeTemplatePorts(builder *strings.Builder, template *models.NodeTemplate) {
	inputs := make([]string, 0)
	outputs := make([]string, 0)
	for _, variable := range template.Variables {
		parts := []string{variable.KeyName}
		if variable.Name != "" {
			parts = append(parts, variable.Name)
		}
		if variable.Type != "" {
			parts = append(parts, variable.Type)
		}
		if variable.Required {
			parts = append(parts, "必填")
		}
		if variable.Description != "" {
			parts = append(parts, singleLine(variable.Description))
		}
		entry := strings.Join(parts, " | ")
		if variable.Direction == models.VariableDirectionOutput {
			outputs = append(outputs, entry)
		} else {
			inputs = append(inputs, entry)
		}
	}
	// 端口自由定义的节点（如 python_script）这些只是占位示例，说清楚避免模型照抄
	label := "固定"
	if isUserDefinedPortType(template.TypeKey) {
		label = "示例"
	}
	if len(inputs) > 0 {
		builder.WriteString("\n  " + label + "输入: ")
		for index, entry := range inputs {
			if index > 0 {
				builder.WriteString("；")
			}
			builder.WriteString(entry)
		}
	}
	if len(outputs) > 0 {
		builder.WriteString("\n  " + label + "输出: ")
		for index, entry := range outputs {
			if index > 0 {
				builder.WriteString("；")
			}
			builder.WriteString(entry)
		}
	}
	if isUserDefinedPortType(template.TypeKey) {
		builder.WriteString("\n  ⚠️ 该节点的输入/输出由你自己按业务需要定义，上面的示例仅供参考，不要照抄。")
	}
	if len(inputs) == 0 && len(outputs) == 0 && template.ScriptTemplate != "" {
		builder.WriteString("\n  脚本模板提示: ")
		builder.WriteString(singleLine(template.ScriptTemplate))
	}
}

// describeConfigSchema 把 config_schema 压成一行可读说明。
func describeConfigSchema(schema models.JSONSchema) string {
	if len(schema) == 0 {
		return ""
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return ""
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if typed, ok := properties[key].(map[string]interface{}); ok {
			if description := asString(typed["description"]); description != "" {
				parts = append(parts, fmt.Sprintf("%s(%s)", key, singleLine(description)))
				continue
			}
		}
		parts = append(parts, key)
	}
	return strings.Join(parts, "、")
}

func singleLine(text string) string {
	replacer := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	return strings.TrimSpace(replacer.Replace(text))
}
