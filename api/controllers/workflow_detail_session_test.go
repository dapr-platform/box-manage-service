package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// fakeSessionServiceForDetail 只实现 Latest，用于验证详情接口附带会话摘要。
type fakeSessionServiceForDetail struct {
	service.WorkflowAISessionService
	session *models.WorkflowAISession
	err     error
	gotKey  string
}

func (f *fakeSessionServiceForDetail) Latest(_ context.Context, keyName string) (*models.WorkflowAISession, error) {
	f.gotKey = keyName
	return f.session, f.err
}

// fakeWorkflowServiceForDetail 只实现 GetByID。
type fakeWorkflowServiceForDetail struct {
	service.WorkflowService
	workflow *models.Workflow
}

func (f *fakeWorkflowServiceForDetail) GetByID(_ context.Context, id uint) (*models.Workflow, error) {
	return f.workflow, nil
}

func newDetailController(workflow *models.Workflow, session *models.WorkflowAISession, err error) (*WorkflowController, *fakeSessionServiceForDetail) {
	sessionSvc := &fakeSessionServiceForDetail{session: session, err: err}
	controller := NewWorkflowController(&fakeWorkflowServiceForDetail{workflow: workflow}).
		WithAISessionService(sessionSvc)
	return controller, sessionSvc
}

func doGetWorkflow(controller *WorkflowController, id string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Get("/api/v1/workflows/{id}", controller.GetWorkflow)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+id, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeDetail(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("响应缺少 data: %s", recorder.Body.String())
	}
	return data
}

// TestWorkflowDetailIncludesAISession 详情接口应附带 has_ai_session 与会话摘要，
// 让编辑器打开时一次请求就能决定按钮状态。
func TestWorkflowDetailIncludesAISession(t *testing.T) {
	workflow := &models.Workflow{
		BaseModel: models.BaseModel{ID: 366},
		Name:      "多次推理示例",
		KeyName:   "995c",
	}
	session := &models.WorkflowAISession{
		BaseModel: models.BaseModel{
			ID:        7,
			UpdatedAt: models.CustomTime{Time: time.Date(2026, 10, 28, 14, 45, 52, 0, time.UTC)},
		},
		WorkflowKeyName: "995c",
		Title:           "生成一个多次推理的例子",
		Round:           2,
		Status:          service.AISessionStatusOpen,
		Messages:        models.JSONArr{map[string]interface{}{"role": "user", "content": "x"}},
		LatestGraph:     models.JSONMap{"nodes": []interface{}{}},
	}
	controller, sessionSvc := newDetailController(workflow, session, nil)

	recorder := doGetWorkflow(controller, "366")
	if recorder.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", recorder.Code)
	}
	// 会话按编排的 key_name 查询，而不是 ID
	if sessionSvc.gotKey != "995c" {
		t.Fatalf("应按 key_name=995c 查询会话，实际 %q", sessionSvc.gotKey)
	}

	data := decodeDetail(t, recorder)
	if has, _ := data["has_ai_session"].(bool); !has {
		t.Fatalf("has_ai_session 应为 true，实际响应: %v", data)
	}
	summary, ok := data["ai_session"].(map[string]interface{})
	if !ok {
		t.Fatalf("应返回 ai_session 摘要，实际响应: %v", data)
	}
	if summary["title"] != "生成一个多次推理的例子" {
		t.Fatalf("摘要标题不正确: %v", summary)
	}
	if round, _ := summary["round"].(float64); int(round) != 2 {
		t.Fatalf("摘要轮次应为 2，实际 %v", summary["round"])
	}
	// 详情里不应带完整图（避免响应膨胀）
	if _, ok := summary["latest_graph"]; ok {
		t.Fatal("摘要不应包含完整编排图")
	}
}

// TestWorkflowDetailWithoutSession 没有会话时 has_ai_session=false，且不影响详情本身。
func TestWorkflowDetailWithoutSession(t *testing.T) {
	workflow := &models.Workflow{
		BaseModel: models.BaseModel{ID: 100},
		Name:      "普通编排",
		KeyName:   "abc",
	}
	controller, _ := newDetailController(workflow, nil, nil)

	recorder := doGetWorkflow(controller, "100")
	if recorder.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", recorder.Code)
	}
	data := decodeDetail(t, recorder)
	if has, _ := data["has_ai_session"].(bool); has {
		t.Fatalf("没有会话时 has_ai_session 应为 false，实际响应: %v", data)
	}
	// 编排本身的信息仍要完整返回
	if data["name"] != "普通编排" {
		t.Fatalf("详情应包含编排名称，实际响应: %v", data)
	}
}

// TestWorkflowDetailSessionWithoutGraph 会话存在但没有产出编排时视为不可恢复。
func TestWorkflowDetailSessionWithoutGraph(t *testing.T) {
	workflow := &models.Workflow{BaseModel: models.BaseModel{ID: 101}, KeyName: "abc"}
	session := &models.WorkflowAISession{
		BaseModel:       models.BaseModel{ID: 3},
		WorkflowKeyName: "abc",
		LatestGraph:     nil, // 还没产出过编排
	}
	controller, _ := newDetailController(workflow, session, nil)

	data := decodeDetail(t, doGetWorkflow(controller, "101"))
	if has, _ := data["has_ai_session"].(bool); has {
		t.Fatalf("会话没有编排图时不应标记为可恢复: %v", data)
	}
}

// TestWorkflowDetailSessionQueryErrorTolerated 查询会话失败不应让详情接口报错。
func TestWorkflowDetailSessionQueryErrorTolerated(t *testing.T) {
	workflow := &models.Workflow{BaseModel: models.BaseModel{ID: 102}, Name: "编排", KeyName: "abc"}
	controller, _ := newDetailController(workflow, nil, context.DeadlineExceeded)

	recorder := doGetWorkflow(controller, "102")
	if recorder.Code != http.StatusOK {
		t.Fatalf("会话查询失败不应影响详情，实际状态码 %d", recorder.Code)
	}
	data := decodeDetail(t, recorder)
	if data["name"] != "编排" {
		t.Fatalf("详情仍应正常返回，实际: %v", data)
	}
}

// TestWorkflowDetailWithoutSessionService 未注入会话服务时退化为原始返回。
func TestWorkflowDetailWithoutSessionService(t *testing.T) {
	workflow := &models.Workflow{BaseModel: models.BaseModel{ID: 103}, Name: "编排", KeyName: "abc"}
	controller := NewWorkflowController(&fakeWorkflowServiceForDetail{workflow: workflow})

	recorder := doGetWorkflow(controller, "103")
	if recorder.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", recorder.Code)
	}
	data := decodeDetail(t, recorder)
	if data["name"] != "编排" {
		t.Fatalf("应返回编排本身，实际: %v", data)
	}
}
