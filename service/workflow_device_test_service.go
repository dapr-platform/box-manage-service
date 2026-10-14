/*
 * @module service/workflow_device_test_service
 * @description 编排的真机测试：把编排临时下发到测试盒子，用同一组用例逐帧真实执行，
 *              收集每个节点的输入输出，跑完自动删除。
 *
 * 与前端仿真的关系：两者产出**完全相同的帧结果结构**（见 models.WorkflowTestFrameResult），
 * 断言只在前端实现一份。真机结果最可信，发布前以它为准。
 *
 * 关键设计：
 * - 测试部署使用 `wf_test_` 前缀的独立 ID，**不写 workflow_deployments 表**，
 *   因此不受「必须已发布」的限制，也不污染正式部署记录。
 * - 每个用例使用 `wf_test_...` 前缀的独立 task_id，测试结束清理，避免互相影响。
 * - 逐帧**串行**执行：跨帧状态依赖上一帧的写入；盒子有并发保护，并发触发会被丢弃。
 * - 同一盒子同一时间只跑一个测试（按 box_id 加锁）。
 * - 结果只放内存，保留一段时间后清除。
 */

package service

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// 测试部署与测试 task_id 的前缀，便于识别与兜底清理
const (
	testDeploymentPrefix = "wf_test_"
	testTaskIDPrefix     = "wf_test_"
	// 单帧超时：盒子执行一帧超过该时间判定该帧失败
	defaultFrameTimeout = 30 * time.Second
	// 运行结果在内存中的保留时间
	runResultTTL = 30 * time.Minute
	// 同时保留的运行记录上限
	maxRetainedRuns = 20
)

// WorkflowDeviceTestRunRequest 真机测试请求
type WorkflowDeviceTestRunRequest struct {
	WorkflowID uint   `json:"workflow_id"`
	BoxID      uint   `json:"box_id"`
	CaseIDs    []uint `json:"case_ids"`
	// DryRun 为 true 时盒子上的企业微信/MQTT/HTTP/RPA 节点只记录不发送
	DryRun *bool `json:"dry_run,omitempty"`
}

// WorkflowDeviceTestCaseProgress 单个用例的进度
type WorkflowDeviceTestCaseProgress struct {
	CaseID uint                              `json:"case_id"`
	Name   string                            `json:"name"`
	Frames []*models.WorkflowTestFrameResult `json:"frames"`
	Error  string                            `json:"error,omitempty"`
}

// WorkflowDeviceTestRun 一次真机测试运行
type WorkflowDeviceTestRun struct {
	RunID      string                            `json:"run_id"`
	WorkflowID uint                              `json:"workflow_id"`
	BoxID      uint                              `json:"box_id"`
	Status     string                            `json:"status"` // running/finished/stopped/error
	DryRun     bool                              `json:"dry_run"`
	Progress   WorkflowDeviceTestProgress        `json:"progress"`
	Cases      []*WorkflowDeviceTestCaseProgress `json:"cases"`
	Error      string                            `json:"error,omitempty"`
	StartedAt  time.Time                         `json:"started_at"`
	FinishedAt *time.Time                        `json:"finished_at,omitempty"`
	cancel     context.CancelFunc
}

// WorkflowDeviceTestProgress 运行进度
type WorkflowDeviceTestProgress struct {
	CaseIndex   int `json:"case_index"`
	FrameIndex  int `json:"frame_index"`
	TotalFrames int `json:"total_frames"`
}

// WorkflowDeviceTestService 真机测试服务接口
type WorkflowDeviceTestService interface {
	Start(ctx context.Context, req *WorkflowDeviceTestRunRequest) (*WorkflowDeviceTestRun, error)
	Get(runID string) (*WorkflowDeviceTestRun, bool)
	Stop(runID string) error
}

type workflowDeviceTestService struct {
	workflowRepo repository.WorkflowRepository
	nodeRepo     repository.NodeDefinitionRepository
	lineRepo     repository.LineDefinitionRepository
	varRepo      repository.VariableDefinitionRepository
	boxRepo      repository.BoxRepository
	testCaseRepo repository.WorkflowTestCaseRepository

	mu      sync.Mutex
	runs    map[string]*WorkflowDeviceTestRun
	boxLock map[uint]bool
}

// NewWorkflowDeviceTestService 创建真机测试服务
func NewWorkflowDeviceTestService(repoManager repository.RepositoryManager) WorkflowDeviceTestService {
	service := &workflowDeviceTestService{
		runs:    make(map[string]*WorkflowDeviceTestRun),
		boxLock: make(map[uint]bool),
	}
	if repoManager == nil {
		return service
	}
	service.workflowRepo = repoManager.Workflow()
	service.nodeRepo = repoManager.NodeDefinition()
	service.lineRepo = repoManager.LineDefinition()
	service.varRepo = repoManager.VariableDefinition()
	service.boxRepo = repoManager.Box()
	service.testCaseRepo = repoManager.WorkflowTestCase()
	return service
}

// Start 启动一次真机测试（后台执行，立即返回 run 记录）。
func (s *workflowDeviceTestService) Start(ctx context.Context, req *WorkflowDeviceTestRunRequest) (*WorkflowDeviceTestRun, error) {
	if s.workflowRepo == nil || s.boxRepo == nil || s.testCaseRepo == nil {
		return nil, fmt.Errorf("真机测试服务未正确初始化")
	}
	if req == nil || req.WorkflowID == 0 || req.BoxID == 0 {
		return nil, fmt.Errorf("workflow_id 与 box_id 必填")
	}

	workflow, err := s.workflowRepo.GetByID(ctx, req.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("查询编排失败: %w", err)
	}
	if workflow == nil {
		return nil, fmt.Errorf("编排 %d 不存在", req.WorkflowID)
	}
	box, err := s.boxRepo.GetByID(ctx, req.BoxID)
	if err != nil {
		return nil, fmt.Errorf("查询盒子失败: %w", err)
	}
	if box == nil {
		return nil, fmt.Errorf("盒子 %d 不存在", req.BoxID)
	}
	if box.Status != models.BoxStatusOnline {
		return nil, fmt.Errorf("盒子 %s 不在线，无法进行真机测试", box.Name)
	}

	// 载入用例
	allCases, err := s.testCaseRepo.FindByWorkflowKeyName(ctx, workflow.KeyName)
	if err != nil {
		return nil, fmt.Errorf("查询测试用例失败: %w", err)
	}
	selected := make([]*models.WorkflowTestCase, 0, len(allCases))
	if len(req.CaseIDs) == 0 {
		selected = allCases
	} else {
		wanted := make(map[uint]bool, len(req.CaseIDs))
		for _, id := range req.CaseIDs {
			wanted[id] = true
		}
		for _, item := range allCases {
			if wanted[item.ID] {
				selected = append(selected, item)
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("没有可运行的测试用例")
	}

	// 同一盒子同一时间只允许一个测试
	s.mu.Lock()
	if s.boxLock[req.BoxID] {
		s.mu.Unlock()
		return nil, fmt.Errorf("该盒子上已有测试正在运行，请等待其结束或先停止")
	}
	s.boxLock[req.BoxID] = true

	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	runID := fmt.Sprintf("r_%d_%d", time.Now().UnixNano()/int64(time.Millisecond), req.BoxID)
	runCtx, cancel := context.WithCancel(context.Background())
	run := &WorkflowDeviceTestRun{
		RunID:      runID,
		WorkflowID: req.WorkflowID,
		BoxID:      req.BoxID,
		Status:     "running",
		DryRun:     dryRun,
		Cases:      make([]*WorkflowDeviceTestCaseProgress, 0, len(selected)),
		StartedAt:  time.Now(),
		cancel:     cancel,
	}
	for _, item := range selected {
		run.Cases = append(run.Cases, &WorkflowDeviceTestCaseProgress{CaseID: item.ID, Name: item.Name, Frames: []*models.WorkflowTestFrameResult{}})
	}
	s.retainRun(runID, run)
	s.mu.Unlock()

	go s.execute(runCtx, run, workflow, box, selected, dryRun)
	return run, nil
}

// retainRun 保存运行记录并清理过期项（必须在持有锁时调用）。
func (s *workflowDeviceTestService) retainRun(runID string, run *WorkflowDeviceTestRun) {
	s.runs[runID] = run
	if len(s.runs) <= maxRetainedRuns {
		return
	}
	// 先清理已结束且超过 TTL 的
	cutoff := time.Now().Add(-runResultTTL)
	for id, item := range s.runs {
		if id == runID || item.FinishedAt == nil {
			continue
		}
		if item.FinishedAt.Before(cutoff) {
			delete(s.runs, id)
		}
	}
}

// Get 查询运行记录。
func (s *workflowDeviceTestService) Get(runID string) (*WorkflowDeviceTestRun, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	return run, ok
}

// Stop 停止运行（会触发清理）。
func (s *workflowDeviceTestService) Stop(runID string) error {
	s.mu.Lock()
	run, ok := s.runs[runID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("运行 %s 不存在", runID)
	}
	if run.cancel != nil {
		run.cancel()
	}
	return nil
}

// execute 后台执行：下发 → 逐用例逐帧触发 → 收集 → 清理。
func (s *workflowDeviceTestService) execute(
	ctx context.Context,
	run *WorkflowDeviceTestRun,
	workflow *models.Workflow,
	box *models.Box,
	testCases []*models.WorkflowTestCase,
	dryRun bool,
) {
	deploymentID := fmt.Sprintf("%s%d_%s", testDeploymentPrefix, workflow.ID, run.RunID)
	boxClient := client.NewBoxClient(box)
	var finalErr string

	// 无论成功失败都要清理测试部署与测试上下文
	defer func() {
		s.cleanup(boxClient, deploymentID, run)
		s.mu.Lock()
		finished := time.Now()
		run.FinishedAt = &finished
		if run.Status == "running" {
			if finalErr != "" {
				run.Status = "error"
				run.Error = finalErr
			} else {
				run.Status = "finished"
			}
		}
		s.boxLock[run.BoxID] = false
		s.mu.Unlock()
	}()

	// 1. 下发测试部署（不写 workflow_deployments 表，因此不要求已发布）
	if err := s.deployTestWorkflow(ctx, boxClient, workflow, deploymentID); err != nil {
		finalErr = fmt.Sprintf("下发测试编排失败: %v", err)
		return
	}

	// 2. 逐用例执行
	for caseIndex, testCase := range testCases {
		if ctx.Err() != nil {
			s.markStopped(run)
			return
		}
		var content models.WorkflowTestCaseContent
		if err := json.Unmarshal([]byte(testCase.Content), &content); err != nil {
			run.Cases[caseIndex].Error = fmt.Sprintf("用例内容不是合法 JSON: %v", err)
			continue
		}

		taskID := fmt.Sprintf("%s%s_%d", testTaskIDPrefix, run.RunID, testCase.ID)
		// 3a. 初始化该用例的任务上下文
		if len(content.InitialContext) > 0 {
			if err := boxClient.SetTaskContext(ctx, taskID, content.InitialContext, true); err != nil {
				run.Cases[caseIndex].Error = fmt.Sprintf("初始化任务上下文失败: %v", err)
				continue
			}
		} else {
			// 即使没有初始上下文也重置一次，避免上轮残留影响本次
			if err := boxClient.DeleteTaskContext(ctx, taskID); err != nil {
				log.Printf("[DeviceTest] 重置任务上下文失败(忽略): %v", err)
			}
		}

		// 3b. 展开帧并逐帧串行执行
		expanded := expandTestCaseFrames(&content)
		run.Progress.TotalFrames = len(expanded)
		for frameIndex, item := range expanded {
			if ctx.Err() != nil {
				s.markStopped(run)
				return
			}
			run.Progress.CaseIndex = caseIndex
			run.Progress.FrameIndex = frameIndex

			frameResult := s.runOneFrame(ctx, boxClient, deploymentID, taskID, item, frameIndex, dryRun)
			frameResult.Label = item.Label
			run.Cases[caseIndex].Frames = append(run.Cases[caseIndex].Frames, frameResult)

			if frameResult.Status == "failed" {
				// 单帧失败记录后继续跑该用例的后续帧，便于观察恢复行为
				log.Printf("[DeviceTest] 用例 %d 第 %d 帧失败: %v", testCase.ID, frameIndex, frameResult.Errors)
			}
		}

		// 3c. 清理该用例的任务上下文
		if err := boxClient.DeleteTaskContext(ctx, taskID); err != nil {
			log.Printf("[DeviceTest] 清理用例任务上下文失败(忽略): %v", err)
		}
	}
}

// markStopped 把运行标记为已停止（若仍处于 running）。
func (s *workflowDeviceTestService) markStopped(run *WorkflowDeviceTestRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if run.Status == "running" {
		run.Status = "stopped"
	}
}

// cleanup 删除测试部署与残留的测试上下文。
func (s *workflowDeviceTestService) cleanup(boxClient *client.BoxClient, deploymentID string, run *WorkflowDeviceTestRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := boxClient.DeleteDeployment(ctx, deploymentID); err != nil {
		log.Printf("[DeviceTest] 删除测试部署失败(忽略): deployment=%s, err=%v", deploymentID, err)
	}
	// 兜底清理：即使中途异常退出，也把本次运行产生的上下文清掉
	if affected, err := boxClient.CleanupTaskContexts(ctx, fmt.Sprintf("%s%s", testTaskIDPrefix, run.RunID)); err != nil {
		log.Printf("[DeviceTest] 清理测试上下文失败(忽略): run=%s, err=%v", run.RunID, err)
	} else if affected > 0 {
		log.Printf("[DeviceTest] 已清理 %d 条测试上下文: run=%s", affected, run.RunID)
	}
}

// deployTestWorkflow 组装并下发测试部署。
// 与正式部署使用同一份载荷结构，但不落 workflow_deployments 表，
// 因此草稿也能下发测试（正式 Deploy 要求已发布）。
func (s *workflowDeviceTestService) deployTestWorkflow(
	ctx context.Context,
	boxClient *client.BoxClient,
	workflow *models.Workflow,
	deploymentID string,
) error {
	nodes, err := s.nodeRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("查询节点定义失败: %w", err)
	}
	lines, err := s.lineRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("查询连接线定义失败: %w", err)
	}
	variables, err := s.varRepo.FindByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return fmt.Errorf("查询变量定义失败: %w", err)
	}

	// 确保 structure_json 是最新的（盒子端执行依赖它）
	if err := workflow.BuildStructureJSON(); err != nil {
		return fmt.Errorf("构建 structure_json 失败: %w", err)
	}

	// 测试部署对象：ID 使用 wf_test_ 前缀，不写库
	deployment := map[string]interface{}{
		"id":               deploymentID,
		"key":              deploymentID,
		"name":             fmt.Sprintf("[测试] %s", workflow.Name),
		"description":      "编排真机测试临时部署，测试结束后自动删除",
		"workflow_id":      workflow.ID,
		"workflow_name":    workflow.Name,
		"workflow_version": workflow.Version,
		"status":           "deployed",
		"is_test":          true,
	}

	payload := &client.DeploymentDistributionRequest{
		Deployment: deployment,
		Workflow:   workflow,
		Nodes:      convertToInterfaceSlice(nodes),
		Variables:  convertVariablesToBoxPayload(variables),
		Lines:      convertToInterfaceSlice(lines),
	}
	if err := boxClient.DistributeDeployment(ctx, payload); err != nil {
		return err
	}
	return nil
}

// runOneFrame 触发一帧并轮询到结束，收集节点结果。
func (s *workflowDeviceTestService) runOneFrame(
	ctx context.Context,
	boxClient *client.BoxClient,
	deploymentID string,
	taskID string,
	item expandedFrame,
	frameIndex int,
	dryRun bool,
) *models.WorkflowTestFrameResult {
	result := &models.WorkflowTestFrameResult{
		Index:         frameIndex,
		Status:        "success",
		ExecutedNodes: []string{},
		NodeOutputs:   map[string]map[string]interface{}{},
		NodeInputs:    map[string]map[string]interface{}{},
		Errors:        []string{},
	}

	triggerData := map[string]interface{}{
		"task_id":         taskID,
		"dry_run":         dryRun,
		"input_variables": item.Inputs,
	}
	if item.Mocks != nil && len(item.Mocks) > 0 {
		// 真机不使用 mocks：这些节点在盒子上真实执行
		// 若用例依赖 mocks，说明该用例只适合仿真，这里不做静默替代
		result.Errors = append(result.Errors, "用例包含 mocks，真机测试会忽略它们（对应节点将由盒子真实执行）")
	}

	instanceID, err := boxClient.RunWorkflowDeployment(ctx, deploymentID, triggerData)
	if err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("触发执行失败: %v", err))
		return result
	}

	// 轮询到实例结束
	instance, err := s.waitForInstance(ctx, boxClient, instanceID)
	if err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	status := asStringValue(instance["status"])
	if status == "failed" {
		result.Status = "failed"
		if message := asStringValue(instance["error_message"]); message != "" {
			result.Errors = append(result.Errors, message)
		}
	}

	// 收集节点输入输出
	nodes, err := boxClient.GetWorkflowInstanceNodes(ctx, instanceID)
	if err != nil {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("查询节点结果失败: %v", err))
		return result
	}
	for _, raw := range nodes {
		node, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		nodeKey := firstNonEmptyValue(asStringValue(node["node_id"]), asStringValue(node["node_key_name"]))
		if nodeKey == "" {
			continue
		}
		result.ExecutedNodes = append(result.ExecutedNodes, nodeKey)
		if outputs := toMap(node["output_data"]); len(outputs) > 0 {
			result.NodeOutputs[nodeKey] = outputs
		}
		if inputs := toMap(node["input_data"]); len(inputs) > 0 {
			result.NodeInputs[nodeKey] = inputs
		}
		if nodeStatus := asStringValue(node["status"]); nodeStatus == "failed" {
			if message := asStringValue(node["error_message"]); message != "" {
				result.Errors = append(result.Errors, fmt.Sprintf("节点 %s 失败: %s", nodeKey, message))
			}
		}
	}
	if len(result.Errors) > 0 {
		result.Status = "failed"
	}
	return result
}

// waitForInstance 轮询实例直到结束或超时。
func (s *workflowDeviceTestService) waitForInstance(ctx context.Context, boxClient *client.BoxClient, instanceID string) (map[string]interface{}, error) {
	deadline := time.Now().Add(defaultFrameTimeout)
	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("测试已停止")
		}
		instance, err := boxClient.GetWorkflowInstance(ctx, instanceID)
		if err != nil {
			return nil, fmt.Errorf("查询实例状态失败: %w", err)
		}
		status := asStringValue(instance["status"])
		switch status {
		case "completed", "failed", "cancelled":
			return instance, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("单帧执行超时（超过 %s），实例状态仍为 %s", defaultFrameTimeout, status)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("测试已停止")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// expandedFrame 展开后的一帧
type expandedFrame struct {
	Label  string
	Inputs map[string]interface{}
	Mocks  map[string]interface{}
}

// expandTestCaseFrames 把用例的 frames 展开成逐帧序列（repeat 复制 N 份）。
// 与前端 workflow-test-assert 的 expandFrames 保持一致的语义。
func expandTestCaseFrames(content *models.WorkflowTestCaseContent) []expandedFrame {
	result := make([]expandedFrame, 0)
	for _, frame := range content.Frames {
		repeat := frame.Repeat
		if repeat < 1 {
			repeat = 1
		}
		for i := 0; i < repeat; i++ {
			result = append(result, expandedFrame{
				Label:  frame.Label,
				Inputs: frame.Inputs,
				Mocks:  frame.Mocks,
			})
		}
	}
	return result
}

// ---- 小工具 ----

func asStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// toMap 把接口值转成 map；盒子返回的 output_data 可能是 JSON 字符串或对象。
func toMap(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed
	case string:
		if typed == "" {
			return nil
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(typed), &parsed); err == nil {
			return parsed
		}
	}
	return nil
}
