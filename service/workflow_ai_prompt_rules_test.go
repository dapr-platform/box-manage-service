package service

import (
	"strings"
	"testing"
)

// TestPromptForbidsInventingRequirements 锁定「不许自行发明需求」这条最高优先级约束。
//
// 线上实测：需求只说「停留超过 30 帧报警」，模型却加了 ENTER_THRESHOLD=4 的进入态，
// 把实际报警推迟到第 34 帧。
func TestPromptForbidsInventingRequirements(t *testing.T) {
	for _, expected := range []string{
		"忠于需求（最高优先级）",
		"一律不要自行增加",
		"不是第 30 帧（>= 30）",
	} {
		if !strings.Contains(workflowAIRuleText, expected) {
			t.Fatalf("编排规则缺少「忠于需求」约束: %q", expected)
		}
	}
}

// TestPromptRequiresEnumParamsBlank 锁定「枚举参数默认留空、缺失必须反问」。
//
// 线上实测：模型给 detection_filter 的 class_id 填了猜测值 "2"，
// 一旦猜错整个编排永远不触发，而校验器完全看不出来。
func TestPromptRequiresEnumParamsBlank(t *testing.T) {
	for _, expected := range []string{
		"默认必须留空（default_value 填 null）",
		"除非【业务需求】或用户的回答里明确给出了具体数值",
		"类别/枚举型参数缺失时必须反问",
	} {
		if !strings.Contains(workflowAIRuleText, expected) {
			t.Fatalf("编排规则缺少枚举参数约束: %q", expected)
		}
	}
	// 反问示例里的默认值不能预填一个猜测的类别 ID
	if strings.Contains(workflowAIRuleText, `"default": "2"`) {
		t.Fatal("反问示例不应把猜测的类别 ID 作为默认答案")
	}
}

// TestPromptRequiresOutputsToBeUsed 锁定「声明的输出必须被使用」。
//
// 线上实测：模型声明 need_notify 并在脚本里置位，但它不参与任何连线，
// 导致「恢复通知」永远发不出去。
func TestPromptRequiresOutputsToBeUsed(t *testing.T) {
	for _, expected := range []string{
		"声明的每个输出都必须被真正使用",
		"只置位不发送的通知标志",
	} {
		if !strings.Contains(workflowAIRuleText, expected) {
			t.Fatalf("编排规则缺少输出使用约束: %q", expected)
		}
	}
}

// TestPromptStatesImageAnnotatorOutputNames 锁定 image_annotator 的真实输出字段。
//
// 线上实测：模型写成 annotated_image，真实字段是 image，白费一轮修复。
func TestPromptStatesImageAnnotatorOutputNames(t *testing.T) {
	if !strings.Contains(workflowAIRuleText, "没有 annotated_image 这个字段") {
		t.Fatal("编排规则应明确 image_annotator 没有 annotated_image 字段")
	}
	if !strings.Contains(workflowAIRuleText, "image（标注后的 base64）") {
		t.Fatal("编排规则应说明 image_annotator 的正确输出字段")
	}
}

// TestPromptRuleNumbersAreSequential 确认规则编号连续，避免新增规则后编号错乱。
func TestPromptRuleNumbersAreSequential(t *testing.T) {
	expected := []string{"0. 忠于需求", "1. 结构", "2. 参数结构", "3. 变量引用", "4. 条件连线",
		"5. 跨帧状态", "6. 图片推送", "7. Python 节点", "8. 输出格式", "9. 何时反问"}
	for _, marker := range expected {
		if !strings.Contains(workflowAIRuleText, "\n"+marker) {
			t.Fatalf("编排规则缺少或编号错误: %q", marker)
		}
	}
}
