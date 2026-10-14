package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestStripGraphPayloadClearsHeavyFields 确认列表用的清空方法会清掉两个大字段。
func TestStripGraphPayloadClearsHeavyFields(t *testing.T) {
	workflow := &Workflow{
		Name:              "人员入侵累计检测编排-2669",
		KeyName:           "2669",
		Version:           1,
		Status:            WorkflowStatusDraft,
		StructureJSON:     `{"nodes":[{"id":"start-001"}],"lines":[]}`,
		StructureJSONView: `{"nodes":[{"id":"start-001","x":1,"y":2}]}`,
	}
	workflow.StripGraphPayload()

	if workflow.StructureJSON != "" {
		t.Fatalf("StructureJSON 应被清空，实际: %q", workflow.StructureJSON)
	}
	if workflow.StructureJSONView != "" {
		t.Fatalf("StructureJSONView 应被清空，实际: %q", workflow.StructureJSONView)
	}
	// 元信息必须保留，否则列表页和模板下拉框就没法用了
	if workflow.Name == "" || workflow.KeyName == "" || workflow.Version != 1 {
		t.Fatalf("元信息不应被清空: %+v", workflow)
	}
}

// TestListPayloadOmitsGraphJSON 锁定列表响应不再携带编排图 JSON。
//
// 这是线上故障的直接回归：/workflows/all 返回 100 条、每条带完整 structure_json，
// 响应达 8.6MB / 16s，AI 快速编排的模板下拉框直接空掉（"暂无数据"）。
func TestListPayloadOmitsGraphJSON(t *testing.T) {
	bigGraph := `{"nodes":[` + strings.Repeat(`{"id":"n"},`, 500) + `{"id":"last"}]}`
	workflow := &Workflow{
		Name:              "某编排",
		KeyName:           "k1",
		Version:           3,
		StructureJSON:     bigGraph,
		StructureJSONView: bigGraph,
	}
	workflow.StripGraphPayload()

	encoded, err := json.Marshal(workflow)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	payload := string(encoded)

	if strings.Contains(payload, "structure_json") {
		t.Fatalf("列表载荷不应包含 structure_json / structure_json_view: %s", payload)
	}
	if len(payload) > 1024 {
		t.Fatalf("清空后单条载荷应该很小，实际 %d 字节", len(payload))
	}
	// 前端下拉框依赖 id / name / version，必须存在
	for _, field := range []string{`"id"`, `"name"`, `"version"`} {
		if !strings.Contains(payload, field) {
			t.Fatalf("列表载荷缺少字段 %s: %s", field, payload)
		}
	}
}

// TestDetailPayloadKeepsGraphJSON 确认详情路径不受影响：
// 字段非空时仍会完整序列化，否则编辑器就打不开编排了。
func TestDetailPayloadKeepsGraphJSON(t *testing.T) {
	graph := `{"nodes":[{"id":"start-001"}],"lines":[]}`
	workflow := &Workflow{
		Name:              "某编排",
		KeyName:           "k1",
		Version:           1,
		StructureJSON:     graph,
		StructureJSONView: `{"nodes":[{"id":"start-001","x":120,"y":100}]}`,
	}

	encoded, err := json.Marshal(workflow)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	payload := string(encoded)

	if !strings.Contains(payload, "structure_json") {
		t.Fatalf("详情载荷必须包含 structure_json: %s", payload)
	}
	if !strings.Contains(payload, "structure_json_view") {
		t.Fatalf("详情载荷必须包含 structure_json_view: %s", payload)
	}
	if !strings.Contains(payload, "start-001") {
		t.Fatalf("详情载荷必须包含完整图内容: %s", payload)
	}
}
