/*
 * @module service/executors/detection_filter_executor
 * @description 检测结果过滤节点 — 按类别和置信度过滤推理检测结果
 * @architecture 策略模式
 * @stateFlow 解析参数 → 过滤 detections → 输出匹配结果 + 透传图片
 * @rules class_id 留空 = 匹配所有类别，score 过滤置信度
 * @dependencies models
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// DetectionFilterExecutor 检测结果过滤执行器
type DetectionFilterExecutor struct {
	*BaseExecutor
}

func NewDetectionFilterExecutor() *DetectionFilterExecutor {
	return &DetectionFilterExecutor{
		BaseExecutor: NewBaseExecutor("detection_filter"),
	}
}

func (e *DetectionFilterExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行检测结果过滤"}

	// 1. 获取参数
	detections := e.getArrayParam(execCtx, "detections")
	targetClass := e.getIntParam(execCtx, "class_id") // -1 = 匹配所有
	minScore := e.getFloatParam(execCtx, "score", 0.0)
	imageData := e.getParam(execCtx, "image")

	logs = append(logs, fmt.Sprintf("输入 %d 条检测结果, class_id=%d, minScore=%.2f",
		len(detections), targetClass, minScore))

	// 2. 过滤
	matched := e.filterDetections(detections, targetClass, minScore)
	logs = append(logs, fmt.Sprintf("过滤后匹配 %d 条", len(matched)))

	// 3. 构建输出
	outputs := map[string]interface{}{
		"matched_detections": matched,
		"match_count":        len(matched),
		"has_match":          len(matched) > 0,
	}
	if imageData != nil {
		outputs["image"] = imageData
	}

	logs = append(logs, "检测结果过滤完成")
	return CreateSuccessResult(CreateOutputs(outputs, map[string]interface{}{
		"match_count": len(matched),
		"has_match":   len(matched) > 0,
	}), logs), nil
}

// filterDetections 按 class_id + score 过滤
func (e *DetectionFilterExecutor) filterDetections(detections []interface{}, targetClass int, minScore float64) []interface{} {
	var matched []interface{}
	for _, d := range detections {
		m, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		// class_id 过滤（-1 = 不限制）
		if targetClass >= 0 {
			detClass := getClassID(m)
			if detClass != targetClass {
				continue
			}
		}

		// score 过滤
		detScore := getScore(m)
		if detScore < minScore {
			continue
		}

		matched = append(matched, m)
	}
	return matched
}

// getClassID 从 detection map 中提取 class_id
func getClassID(m map[string]interface{}) int {
	v, ok := m["class_id"]
	if !ok {
		return -1
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return -1
}

// getScore 从 detection map 中提取置信度（优先 confidence，其次 score）
func getScore(m map[string]interface{}) float64 {
	// 优先 confidence
	if v, ok := m["confidence"]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	// 其次 score
	if v, ok := m["score"]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	return 0.0
}

func (e *DetectionFilterExecutor) getArrayParam(execCtx *ExecutionContext, key string) []interface{} {
	v := e.getParam(execCtx, key)
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		return val
	case string:
		// 可能是 JSON string
		var arr []interface{}
		if err := json.Unmarshal([]byte(val), &arr); err == nil {
			return arr
		}
	}
	return nil
}

func (e *DetectionFilterExecutor) getIntParam(execCtx *ExecutionContext, key string) int {
	v := e.getParam(execCtx, key)
	if v == nil {
		return -1
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case string:
		if val == "" {
			return -1
		}
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return -1
}

func (e *DetectionFilterExecutor) getFloatParam(execCtx *ExecutionContext, key string, defaultVal float64) float64 {
	v := e.getParam(execCtx, key)
	if v == nil {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func (e *DetectionFilterExecutor) getParam(execCtx *ExecutionContext, key string) interface{} {
	if execCtx.Inputs != nil {
		if v, ok := execCtx.Inputs[key]; ok && v != nil {
			return v
		}
	}
	if execCtx.Variables != nil {
		if v, ok := execCtx.Variables[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func (e *DetectionFilterExecutor) Validate(nodeInstance *models.NodeInstance) error {
	return e.BaseExecutor.Validate(nodeInstance)
}
