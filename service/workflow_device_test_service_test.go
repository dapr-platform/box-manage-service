package service

import (
	"box-manage-service/models"
	"testing"
)

// TestExpandTestCaseFramesMatchesFrontend 确认真机侧的帧展开与前端 expandFrames 语义一致：
// repeat 复制 N 份，缺省/非法值按 1 处理。两边不一致会导致断言错位。
func TestExpandTestCaseFramesMatchesFrontend(t *testing.T) {
	content := &models.WorkflowTestCaseContent{
		Frames: []models.TestCaseFrame{
			{Label: "f1", Repeat: 3, Inputs: map[string]interface{}{"a": 1}},
			{Label: "f2", Inputs: map[string]interface{}{"a": 2}},
			{Label: "f3", Repeat: 0, Inputs: map[string]interface{}{"a": 3}},
		},
	}
	frames := expandTestCaseFrames(content)
	if len(frames) != 5 {
		t.Fatalf("expected 5 frames (3+1+1), got %d", len(frames))
	}
	if frames[0].Label != "f1" || frames[2].Label != "f1" {
		t.Fatalf("repeat should duplicate the frame: %+v", frames)
	}
	if frames[3].Label != "f2" {
		t.Fatalf("unexpected order: %+v", frames)
	}
	if frames[4].Label != "f3" {
		t.Fatalf("repeat=0 should fall back to 1: %+v", frames)
	}
}

func TestExpandTestCaseFramesCarriesInputs(t *testing.T) {
	content := &models.WorkflowTestCaseContent{
		Frames: []models.TestCaseFrame{
			{
				Label:  "有人",
				Repeat: 2,
				Inputs: map[string]interface{}{"detections": []interface{}{map[string]interface{}{"class_id": 0}}},
				Mocks:  map[string]interface{}{"reasoning-001": map[string]interface{}{"x": 1}},
			},
		},
	}
	frames := expandTestCaseFrames(content)
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	for _, frame := range frames {
		if frame.Inputs == nil || frame.Inputs["detections"] == nil {
			t.Fatalf("inputs lost: %+v", frame)
		}
		// mocks 会带过去，真机侧据此提示「将被忽略」
		if frame.Mocks == nil {
			t.Fatalf("mocks lost: %+v", frame)
		}
	}
}

// TestTestPrefixesStayIsolated 确认测试使用独立前缀，不会误伤正式部署与真实任务。
func TestTestPrefixesStayIsolated(t *testing.T) {
	if testDeploymentPrefix != "wf_test_" {
		t.Fatalf("测试部署前缀必须是 wf_test_，当前为 %q", testDeploymentPrefix)
	}
	if testTaskIDPrefix != "wf_test_" {
		t.Fatalf("测试 task_id 前缀必须是 wf_test_，当前为 %q", testTaskIDPrefix)
	}
	// 前缀必须非空，否则按前缀清理会误删全部上下文
	if testTaskIDPrefix == "" || testDeploymentPrefix == "" {
		t.Fatal("前缀不能为空")
	}
}

func TestToMapHandlesStringAndObject(t *testing.T) {
	// 盒子不同版本可能返回对象或 JSON 字符串
	if got := toMap(map[string]interface{}{"a": 1}); got["a"] != 1 {
		t.Fatalf("object form not handled: %+v", got)
	}
	if got := toMap(`{"b":2}`); got["b"] != float64(2) {
		t.Fatalf("string form not handled: %+v", got)
	}
	if got := toMap(""); got != nil {
		t.Fatalf("empty string should give nil, got %+v", got)
	}
	if got := toMap("not json"); got != nil {
		t.Fatalf("invalid json should give nil, got %+v", got)
	}
}

func TestDeviceTestServiceRejectsBadRequest(t *testing.T) {
	service := NewWorkflowDeviceTestService(nil)
	if _, err := service.Start(nil, nil); err == nil {
		t.Fatal("nil request should fail")
	}
}
