package controllers

import (
	"box-manage-service/service"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeAIService 返回固定结果，用于验证 HTTP 层的行为。
type fakeAIService struct {
	result *service.WorkflowAIGenerateResult
	err    error
	got    *service.WorkflowAIGenerateRequest
}

func (f *fakeAIService) Generate(_ context.Context, req *service.WorkflowAIGenerateRequest) (*service.WorkflowAIGenerateResult, error) {
	f.got = req
	return f.result, f.err
}

// fakeGraphService 返回固定报告，用于验证 HTTP 层的行为。
type fakeGraphService struct {
	graph  *service.WorkflowNormalizeResult
	report *service.ValidationReport
	err    error
	got    json.RawMessage
}

func (f *fakeGraphService) Normalize(_ context.Context, raw json.RawMessage) (*service.WorkflowNormalizeResult, error) {
	f.got = raw
	return f.graph, f.err
}

func (f *fakeGraphService) Validate(_ context.Context, raw json.RawMessage) (*service.ValidationReport, error) {
	f.got = raw
	return f.report, f.err
}

func doRequest(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()
	handler(recorder, req)
	return recorder
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, recorder.Body.String())
	}
	return payload
}

func TestGenerateWorkflowReturnsGeneratedGraph(t *testing.T) {
	fake := &fakeAIService{result: &service.WorkflowAIGenerateResult{
		Status: "generated",
		Name:   "人员入侵累计检测",
		Report: &service.ValidationReport{Valid: true, Errors: []service.ValidationIssue{}, Warnings: []service.ValidationIssue{}},
	}}
	controller := NewWorkflowAIController(fake, &fakeGraphService{})

	recorder := doRequest(controller.GenerateWorkflow, `{"template_workflow_id": 12, "description": "连续 30 帧有人就报警"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeResponse(t, recorder)
	if payload["code"] != float64(0) {
		t.Fatalf("unexpected code: %v", payload["code"])
	}
	data, _ := payload["data"].(map[string]interface{})
	if data["status"] != "generated" || data["name"] != "人员入侵累计检测" {
		t.Fatalf("unexpected data: %+v", data)
	}
	if fake.got == nil || fake.got.TemplateWorkflowID != 12 {
		t.Fatalf("request not forwarded: %+v", fake.got)
	}
}

func TestGenerateWorkflowForwardsClarificationsAndPrevious(t *testing.T) {
	fake := &fakeAIService{result: &service.WorkflowAIGenerateResult{Status: "generated"}}
	controller := NewWorkflowAIController(fake, &fakeGraphService{})

	body := `{
		"description": "车辆违停报警",
		"clarifications": [{"question": "类别 ID？", "answer": "2"}],
		"previous": {"raw": "{\"status\":\"generated\"}", "errors": ["连线缺失"]}
	}`
	recorder := doRequest(controller.GenerateWorkflow, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	if fake.got == nil || len(fake.got.Clarifications) != 1 || fake.got.Clarifications[0].Answer != "2" {
		t.Fatalf("clarifications not forwarded: %+v", fake.got)
	}
	if fake.got.Previous == nil || len(fake.got.Previous.Errors) != 1 {
		t.Fatalf("previous not forwarded: %+v", fake.got.Previous)
	}
}

func TestGenerateWorkflowRejectsEmptyDescription(t *testing.T) {
	controller := NewWorkflowAIController(&fakeAIService{}, &fakeGraphService{})
	recorder := doRequest(controller.GenerateWorkflow, `{"description": "   "}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty description, got %d", recorder.Code)
	}
}

func TestGenerateWorkflowHandlesMissingService(t *testing.T) {
	controller := NewWorkflowAIController(nil, nil)
	recorder := doRequest(controller.GenerateWorkflow, `{"description": "x"}`)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

func TestNormalizeWorkflowForwardsGraph(t *testing.T) {
	fake := &fakeGraphService{graph: &service.WorkflowNormalizeResult{
		Report: &service.ValidationReport{Valid: true, Errors: []service.ValidationIssue{}, Warnings: []service.ValidationIssue{}},
	}}
	controller := NewWorkflowAIController(nil, fake)
	recorder := doRequest(controller.NormalizeWorkflow, `{"graph": {"nodes": [{"id": "start-001", "type": "start"}], "edges": []}}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(fake.got) == 0 {
		t.Fatalf("graph not forwarded")
	}
}

func TestNormalizeWorkflowRejectsEmptyGraph(t *testing.T) {
	controller := NewWorkflowAIController(nil, &fakeGraphService{})
	recorder := doRequest(controller.NormalizeWorkflow, `{}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestValidateWorkflowGraphReturnsReport(t *testing.T) {
	fake := &fakeGraphService{report: &service.ValidationReport{
		Valid:    false,
		Errors:   []service.ValidationIssue{{Code: "missing_start", Message: "编排必须包含一个开始节点"}},
		Warnings: []service.ValidationIssue{},
	}}
	controller := NewWorkflowAIController(nil, fake)
	recorder := doRequest(controller.ValidateWorkflowGraph, `{"graph": {"nodes": [{"id": "end-001", "type": "end"}], "edges": []}}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeResponse(t, recorder)
	data, _ := payload["data"].(map[string]interface{})
	if data["valid"] != false {
		t.Fatalf("unexpected report: %+v", data)
	}
	errors, _ := data["errors"].([]interface{})
	if len(errors) != 1 {
		t.Fatalf("unexpected errors: %+v", data["errors"])
	}
	first, _ := errors[0].(map[string]interface{})
	if first["code"] != "missing_start" {
		t.Fatalf("unexpected issue code: %+v", first)
	}
}
