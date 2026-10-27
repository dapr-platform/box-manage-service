package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"testing"
)

// stubAISessionRepo 内存版会话仓库，供单测使用。
type stubAISessionRepo struct {
	repository.WorkflowAISessionRepository
	sessions []*models.WorkflowAISession
	nextID   uint
	updates  int
	creates  int
}

func (s *stubAISessionRepo) Create(_ context.Context, entity *models.WorkflowAISession) error {
	s.nextID++
	entity.ID = s.nextID
	s.sessions = append(s.sessions, entity)
	s.creates++
	return nil
}

func (s *stubAISessionRepo) Update(_ context.Context, entity *models.WorkflowAISession) error {
	for i, item := range s.sessions {
		if item.ID == entity.ID {
			s.sessions[i] = entity
			break
		}
	}
	s.updates++
	return nil
}

func (s *stubAISessionRepo) Delete(_ context.Context, id uint) error {
	for i, item := range s.sessions {
		if item.ID == id {
			s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
			break
		}
	}
	return nil
}

func (s *stubAISessionRepo) FindByWorkflowKeyName(_ context.Context, keyName string) ([]*models.WorkflowAISession, error) {
	result := []*models.WorkflowAISession{}
	for _, item := range s.sessions {
		if item.WorkflowKeyName == keyName {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *stubAISessionRepo) FindLatestByWorkflowKeyName(_ context.Context, keyName string) (*models.WorkflowAISession, error) {
	var latest *models.WorkflowAISession
	for _, item := range s.sessions {
		if item.WorkflowKeyName == keyName {
			latest = item
		}
	}
	return latest, nil
}

// stubWorkflowRepoForSession 只实现 GetByID，用于验证按 workflow_id 反查 key_name。
type stubWorkflowRepoForSession struct {
	repository.WorkflowRepository
	workflows map[uint]*models.Workflow
}

func (s *stubWorkflowRepoForSession) GetByID(_ context.Context, id uint) (*models.Workflow, error) {
	return s.workflows[id], nil
}

func newSessionService(repo *stubAISessionRepo, workflows map[uint]*models.Workflow) WorkflowAISessionService {
	service := &workflowAISessionService{sessionRepo: repo}
	if workflows != nil {
		service.workflowRepo = &stubWorkflowRepoForSession{workflows: workflows}
	}
	return service
}

// TestSaveSessionCreatesOnce 首次保存创建会话。
func TestSaveSessionCreatesOnce(t *testing.T) {
	repo := &stubAISessionRepo{}
	svc := newSessionService(repo, nil)

	saved, err := svc.Save(context.Background(), &models.WorkflowAISession{
		WorkflowKeyName: "2669",
		Description:     "检测到车辆在禁停区停留超过 30 帧就推送企业微信",
		Messages: models.JSONArr{
			map[string]interface{}{"role": "user", "content": "需求", "round": 1},
			map[string]interface{}{"role": "assistant", "content": "已生成", "round": 1},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("应分配 ID")
	}
	if repo.creates != 1 || repo.updates != 0 {
		t.Fatalf("首次保存应只创建，实际 creates=%d updates=%d", repo.creates, repo.updates)
	}
	// 标题自动取需求前 40 字
	if saved.Title == "" {
		t.Fatal("应自动生成标题")
	}
	if saved.Status != AISessionStatusOpen {
		t.Fatalf("默认状态应为 open，实际 %q", saved.Status)
	}
	// 轮次按消息里的最大 round 推导
	if saved.Round != 1 {
		t.Fatalf("轮次应为 1，实际 %d", saved.Round)
	}
}

// TestSaveSessionReusesExisting 同一编排的连续对话复用同一条会话，不重复建行。
func TestSaveSessionReusesExisting(t *testing.T) {
	repo := &stubAISessionRepo{}
	svc := newSessionService(repo, nil)

	first, _ := svc.Save(context.Background(), &models.WorkflowAISession{
		WorkflowKeyName: "2669",
		Description:     "第一轮需求",
		Messages:        models.JSONArr{map[string]interface{}{"role": "user", "content": "需求", "round": 1}},
	})
	second, err := svc.Save(context.Background(), &models.WorkflowAISession{
		WorkflowKeyName: "2669",
		Description:     "第一轮需求",
		Messages: models.JSONArr{
			map[string]interface{}{"role": "user", "content": "需求", "round": 1},
			map[string]interface{}{"role": "user", "content": "脚本补上", "round": 2},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("应复用同一条会话，first=%d second=%d", first.ID, second.ID)
	}
	if repo.creates != 1 {
		t.Fatalf("不应重复创建，实际 creates=%d", repo.creates)
	}
	if second.Round != 2 {
		t.Fatalf("轮次应更新为 2，实际 %d", second.Round)
	}
}

// TestSaveSessionResolvesKeyNameFromWorkflowID 未传 key_name 时按 workflow_id 反查。
func TestSaveSessionResolvesKeyNameFromWorkflowID(t *testing.T) {
	repo := &stubAISessionRepo{}
	svc := newSessionService(repo, map[uint]*models.Workflow{
		233: {BaseModel: models.BaseModel{ID: 233}, KeyName: "2669", Name: "人员入侵累计检测编排"},
	})

	saved, err := svc.Save(context.Background(), &models.WorkflowAISession{
		WorkflowID:  233,
		Description: "需求",
		Messages:    models.JSONArr{map[string]interface{}{"role": "user", "content": "需求", "round": 1}},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if saved.WorkflowKeyName != "2669" {
		t.Fatalf("应从 workflow_id 反查到 key_name=2669，实际 %q", saved.WorkflowKeyName)
	}
}

// TestSaveSessionRequiresKeyOrID 两者都没传时报错。
func TestSaveSessionRequiresKeyOrID(t *testing.T) {
	repo := &stubAISessionRepo{}
	svc := newSessionService(repo, nil)

	_, err := svc.Save(context.Background(), &models.WorkflowAISession{Description: "需求"})
	if err == nil {
		t.Fatal("缺少 key_name 与 workflow_id 时应报错")
	}
}

// TestLatestSessionReturnsNilWhenAbsent 没有会话时返回 nil（前端据此判断「首次使用」）。
func TestLatestSessionReturnsNilWhenAbsent(t *testing.T) {
	repo := &stubAISessionRepo{}
	svc := newSessionService(repo, nil)

	session, err := svc.Latest(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if session != nil {
		t.Fatalf("不存在的编排应返回 nil，实际 %+v", session)
	}
}

// TestLatestSessionReturnsSaved 保存后能取回最近一次会话（含对话与图）。
func TestLatestSessionReturnsSaved(t *testing.T) {
	repo := &stubAISessionRepo{}
	svc := newSessionService(repo, nil)

	graph := models.JSONMap{"nodes": []interface{}{map[string]interface{}{"id": "start-001", "type": "start"}}}
	_, err := svc.Save(context.Background(), &models.WorkflowAISession{
		WorkflowKeyName: "2669",
		Description:     "需求",
		Messages:        models.JSONArr{map[string]interface{}{"role": "user", "content": "需求", "round": 1}},
		LatestGraph:     graph,
		Status:          AISessionStatusImported,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	session, err := svc.Latest(context.Background(), "2669")
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if session == nil {
		t.Fatal("应取回会话")
	}
	if session.Status != AISessionStatusImported {
		t.Fatalf("状态应保留 imported，实际 %q", session.Status)
	}
	if session.LatestGraph == nil {
		t.Fatal("最近一次的编排图应被保存")
	}
	if _, ok := session.LatestGraph["nodes"]; !ok {
		t.Fatalf("编排图应含 nodes，实际 %+v", session.LatestGraph)
	}
}

// TestDecodeAISessionMessages 对话内容能从 JSON 数组还原成结构化消息。
func TestDecodeAISessionMessages(t *testing.T) {
	session := &models.WorkflowAISession{
		Messages: models.JSONArr{
			map[string]interface{}{"role": "user", "content": "把脚本补上", "round": 2},
			map[string]interface{}{"role": "assistant", "content": "已补全", "round": 2},
		},
	}
	messages, err := DecodeAISessionMessages(session)
	if err != nil {
		t.Fatalf("DecodeAISessionMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("应还原 2 条消息，实际 %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Content != "把脚本补上" || messages[0].Round != 2 {
		t.Fatalf("消息内容不正确: %+v", messages[0])
	}
}

// TestTruncateRunesKeepsChineseIntact 标题按字符截断，不产生乱码。
func TestTruncateRunesKeepsChineseIntact(t *testing.T) {
	text := "检测到车辆在禁停区停留超过三十帧就推送企业微信文字和图片"
	got := truncateRunes(text, 10)
	if len([]rune(got)) != 11 { // 10 字符 + 省略号
		t.Fatalf("应截成 10 字符加省略号，实际 %q (%d 字符)", got, len([]rune(got)))
	}
	if got == "" {
		t.Fatal("截断结果不应为空")
	}
}
