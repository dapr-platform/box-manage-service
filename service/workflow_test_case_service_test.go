package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// stubTestCaseRepo 只实现测试需要的方法；嵌入接口以满足完整契约。
type stubTestCaseRepo struct {
	repository.WorkflowTestCaseRepository
	cases  []*models.WorkflowTestCase
	nextID uint
}

func (s *stubTestCaseRepo) Create(_ context.Context, entity *models.WorkflowTestCase) error {
	s.nextID++
	entity.ID = s.nextID
	s.cases = append(s.cases, entity)
	return nil
}
func (s *stubTestCaseRepo) GetByID(_ context.Context, id uint) (*models.WorkflowTestCase, error) {
	for _, item := range s.cases {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}
func (s *stubTestCaseRepo) Update(_ context.Context, entity *models.WorkflowTestCase) error {
	return nil
}
func (s *stubTestCaseRepo) Delete(_ context.Context, id uint) error { return nil }
func (s *stubTestCaseRepo) FindByWorkflowKeyName(_ context.Context, keyName string) ([]*models.WorkflowTestCase, error) {
	result := make([]*models.WorkflowTestCase, 0)
	for _, item := range s.cases {
		if item.WorkflowKeyName == keyName {
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *stubTestCaseRepo) FindByWorkflowKeyNames(_ context.Context, keyNames []string) ([]*models.WorkflowTestCase, error) {
	return s.cases, nil
}

// stubWorkflowRepoForCase 只为 resolveKeyName 提供数据。
type stubWorkflowRepoForCase struct {
	repository.WorkflowRepository
	workflow *models.Workflow
}

func (s *stubWorkflowRepoForCase) GetByID(_ context.Context, id uint) (*models.Workflow, error) {
	if s.workflow != nil && s.workflow.ID == id {
		return s.workflow, nil
	}
	return nil, nil
}

func TestNormalizeTestCaseValidatesContent(t *testing.T) {
	valid := `{"frames":[{"label":"f1","inputs":{},"expect":[]}]}`
	cases := []struct {
		name    string
		input   models.WorkflowTestCase
		wantErr string
	}{
		{"名称为空", models.WorkflowTestCase{Content: valid}, "用例名称不能为空"},
		{"内容为空", models.WorkflowTestCase{Name: "x"}, "用例内容不能为空"},
		{"内容非法JSON", models.WorkflowTestCase{Name: "x", Content: "not json"}, "不是合法 JSON"},
		{"没有帧", models.WorkflowTestCase{Name: "x", Content: `{"frames":[]}`}, "至少需要一帧"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			testCase := item.input
			err := normalizeTestCase(&testCase)
			if err == nil || !strings.Contains(err.Error(), item.wantErr) {
				t.Fatalf("expected error containing %q, got %v", item.wantErr, err)
			}
		})
	}
}

func TestNormalizeTestCaseAlignsNameIntoContent(t *testing.T) {
	testCase := models.WorkflowTestCase{
		Name:        "持续有人触发报警",
		Description: "覆盖进入、报警、恢复",
		Content:     `{"description":"","frames":[{"label":"f1"}]}`,
	}
	if err := normalizeTestCase(&testCase); err != nil {
		t.Fatalf("normalizeTestCase() error = %v", err)
	}
	var parsed models.WorkflowTestCaseContent
	if err := json.Unmarshal([]byte(testCase.Content), &parsed); err != nil {
		t.Fatalf("content should stay valid JSON: %v", err)
	}
	if parsed.Name != "持续有人触发报警" {
		t.Fatalf("name not aligned into content: %q", parsed.Name)
	}
	if parsed.Description != "覆盖进入、报警、恢复" {
		t.Fatalf("description not aligned into content: %q", parsed.Description)
	}
}

func TestBuildNodeOutlineOnlyExposesNamesNotScripts(t *testing.T) {
	// 用例必须从需求推导：提示词只给节点与输出名称，不能泄露脚本内容，
	// 否则模型会「照着实现写期望值」，测试退化成同义反复。
	service := &workflowTestCaseAIService{
		workflowRepo: &stubWorkflowRepoForCase{workflow: &models.Workflow{BaseModel: models.BaseModel{ID: 7}, KeyName: "wf"}},
		nodeDefRepo: &stubNodeDefRepo{nodes: []*models.NodeDefinition{
			{NodeID: "start-001", TypeKey: "start", NodeName: "开始"},
			{NodeID: "python_script-005", TypeKey: "python_script", NodeName: "累计计数",
				Config: models.JSONMap{"script_content": "SECRET_SCRIPT_BODY = 1"}},
		}},
		varDefRepo: &stubVariableDefRepo{variables: []*models.VariableDefinition{
			{NodeID: "python_script-005", KeyName: "send_alarm", Name: "是否报警", Direction: models.VariableDirectionOutput},
		}},
	}
	outline, err := service.buildNodeOutline(context.Background(), 7)
	if err != nil {
		t.Fatalf("buildNodeOutline() error = %v", err)
	}
	for _, expected := range []string{"python_script-005", "累计计数", "send_alarm"} {
		if !strings.Contains(outline, expected) {
			t.Fatalf("outline missing %q:\n%s", expected, outline)
		}
	}
	if strings.Contains(outline, "SECRET_SCRIPT_BODY") {
		t.Fatalf("outline must not leak script content:\n%s", outline)
	}
}

// stubNodeDefRepo 只实现 FindByWorkflowID。
type stubNodeDefRepo struct {
	repository.NodeDefinitionRepository
	nodes []*models.NodeDefinition
}

func (s *stubNodeDefRepo) FindByWorkflowID(context.Context, uint) ([]*models.NodeDefinition, error) {
	return s.nodes, nil
}

// stubVariableDefRepo 只实现 FindByWorkflowID。
type stubVariableDefRepo struct {
	repository.VariableDefinitionRepository
	variables []*models.VariableDefinition
}

func (s *stubVariableDefRepo) FindByWorkflowID(context.Context, uint) ([]*models.VariableDefinition, error) {
	return s.variables, nil
}
