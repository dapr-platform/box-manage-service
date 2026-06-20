package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ReasoningLoopExecutor struct {
	*BaseExecutor
}

func NewReasoningLoopExecutor() *ReasoningLoopExecutor {
	return &ReasoningLoopExecutor{BaseExecutor: NewBaseExecutor("reasoning_loop")}
}

func (e *ReasoningLoopExecutor) Validate(instance *models.NodeInstance) error { return nil }

func (e *ReasoningLoopExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行 循环推理匹配..."}
	rtspURL := getStringInput(execCtx.Inputs, "rtsp_url")
	modelKey := getStringInput(execCtx.Inputs, "model_key")
	classID := getIntInput(execCtx.Inputs, "class_id")
	totalCount := getIntInput(execCtx.Inputs, "total_count")
	matchThreshold := getIntInput(execCtx.Inputs, "match_threshold")
	intervalMs := getIntInput(execCtx.Inputs, "interval_ms")
	matchScore := getFloatInput(execCtx.Inputs, "score")
	inferenceType := getStringInput(execCtx.Inputs, "inference_type")

	if totalCount <= 0 {
		totalCount = 10
	}
	if matchThreshold <= 0 {
		matchThreshold = 1
	}
	if intervalMs <= 0 {
		intervalMs = 500
	}
	if matchScore <= 0 {
		matchScore = 0.5
	}
	if inferenceType == "" {
		inferenceType = "detection"
	}

	logs = append(logs, fmt.Sprintf("rtsp=%s model=%s class=%d count=%d/%d score=%.2f",
		rtspURL, modelKey, classID, totalCount, matchThreshold, matchScore))

	outputs := map[string]interface{}{
		"matched": false, "match_count": 0, "total_count": totalCount,
		"detections": []interface{}{}, "detection_count": 0, "image_base64": "",
	}
	logs = append(logs, "循环推理完成（详细逻辑由 box-app 端执行）")
	return CreateSuccessResult(outputs, logs), nil
}

func getIntInput(inputs map[string]interface{}, key string) int {
	if v, ok := inputs[key]; ok && v != nil {
		switch val := v.(type) {
		case float64:
			return int(val)
		case string:
			if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				return i
			}
		}
	}
	return 0
}
func getFloatInput(inputs map[string]interface{}, key string) float64 {
	if v, ok := inputs[key]; ok && v != nil {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
				return f
			}
		}
	}
	return 0
}
