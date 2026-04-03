/*
 * @module service/executors/reasoning_executor
 * @description 模型推理节点执行器
 * @architecture 策略模式 - 构建推理节点配置，随 workflow 下发到 box-app 执行
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 解析配置 -> 验证参数 -> 构建节点配置 -> 写入执行上下文（由 box-app 实际执行推理）
 * @rules
 *   - 推理在 box-app 侧执行，manage-service 只负责配置构建和下发
 *   - 图像来源：RTSP 视频流（由 box-app StreamProcessor 采集）
 *   - 支持 ROI 区域配置，对应 box-app ROIConfig 结构
 *   - 推理类型：detection / segmentation / obb
 *   - box-app 节点配置字段：model_key / model_type / conf_threshold / nms_threshold / image_input / output_variable
 */

package executors

import (
	"box-manage-service/models"
	"context"
	"fmt"
	"strings"
)

// ReasoningExecutor 模型推理执行器
type ReasoningExecutor struct {
	*BaseExecutor
}

// NewReasoningExecutor 创建推理执行器
func NewReasoningExecutor() *ReasoningExecutor {
	return &ReasoningExecutor{
		BaseExecutor: NewBaseExecutor("reasoning"),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 配置结构体
// ─────────────────────────────────────────────────────────────────────────────

// ROIArea 多边形顶点（对应 box-app ROIArea）
type ROIArea struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ROIConfig ROI 区域配置（对应 box-app ROIConfig）
type ROIConfig struct {
	ID     int       `json:"id"`
	Name   string    `json:"name"`
	X      int       `json:"x"`
	Y      int       `json:"y"`
	Width  int       `json:"width"`
	Height int       `json:"height"`
	Areas  []ROIArea `json:"areas,omitempty"` // 多边形顶点，用于复杂形状
}

// ReasoningConfig 推理节点配置
type ReasoningConfig struct {
	// 推理类型：detection / segmentation / obb
	// 对应 box-app node_executor.cpp 中的 model_type 字段
	InferenceType string `json:"inference_type"`

	// 模型复合键，格式: {type}-{version}-{hardware}-{name}
	// 例: detection-yolov8-bm1684x-yolov8n
	ModelKey string `json:"model_key"`

	// 图像来源（三选一）：
	//   - rtsp_url：RTSP 视频流，由 box-app StreamProcessor 采集帧（持续推理）
	//   - image_url：图片地址，支持本地路径（/data/img.jpg）或网络地址（http/https）（单次推理）
	// 两者互斥，image_url 优先级高于 rtsp_url
	RTSPUrl  string `json:"rtsp_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`

	// 跳帧数，0 表示不跳帧（每帧都推理），仅 rtsp_url 模式有效
	SkipFrame int `json:"skip_frame"`

	// 置信度阈值，0 表示使用模型默认值
	// 对应 box-app conf_threshold
	ConfidenceThreshold float64 `json:"confidence_threshold"`

	// NMS 阈值，0 表示使用模型默认值
	// 对应 box-app nms_threshold
	NMSThreshold float64 `json:"nms_threshold"`

	// ROI 区域配置列表，为空表示全图推理
	// 对应 box-app ROIConfig 结构
	ROIs []ROIConfig `json:"rois,omitempty"`

	// 是否启用 ROI 推理，有 ROI 配置时自动为 true
	UseROIToInference bool `json:"use_roi_to_inference"`

	// 图像输入引用，对应 box-app image_input 字段
	// 格式: "{node_key_name}.image" 或直接变量名
	// 为空时 box-app 使用视频流当前帧或 image_url
	ImageInput string `json:"image_input,omitempty"`

	// 出参：推理结果写入的变量名
	// 对应 box-app output_variable 字段
	// 为空时默认写入 "reasoning_result"
	OutputVarKey string `json:"output_var_key"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Execute
// ─────────────────────────────────────────────────────────────────────────────

// Execute 构建推理节点配置并写入执行上下文
// 实际推理由 box-app 的 executeReasoningNode 执行
func (e *ReasoningExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"推理节点配置构建开始"}

	cfg, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), nil
	}

	// 日志：图像来源
	if cfg.ImageURL != "" {
		logs = append(logs, fmt.Sprintf("推理类型: %s，模型: %s，图片地址: %s", cfg.InferenceType, cfg.ModelKey, cfg.ImageURL))
	} else {
		logs = append(logs, fmt.Sprintf("推理类型: %s，模型: %s，视频流: %s", cfg.InferenceType, cfg.ModelKey, cfg.RTSPUrl))
	}

	if len(cfg.ROIs) > 0 {
		logs = append(logs, fmt.Sprintf("ROI 区域数量: %d，启用 ROI 推理", len(cfg.ROIs)))
	} else {
		logs = append(logs, "未配置 ROI，使用全图推理")
	}

	// 构建下发给 box-app 的节点配置
	nodeConfig := e.buildNodeConfig(cfg)

	// 将配置写入变量上下文，供 workflow 下发时使用
	outputKey := cfg.OutputVarKey
	if outputKey == "" {
		outputKey = "reasoning_result"
	}

	// 构建标准输出格式：output 为主要输出，其他字段为额外输出
	extras := map[string]interface{}{
		"inference_type": cfg.InferenceType,
		"model_key":      cfg.ModelKey,
		"output_var_key": outputKey,
		"roi_count":      len(cfg.ROIs),
	}
	if cfg.ImageURL != "" {
		extras["image_url"] = cfg.ImageURL
	} else {
		extras["rtsp_url"] = cfg.RTSPUrl
	}

	outputs := CreateOutputs(nodeConfig, extras)

	execCtx.Variables[outputKey] = outputs
	logs = append(logs, fmt.Sprintf("推理节点配置已构建，结果变量: %s", outputKey))
	logs = append(logs, "推理节点配置构建完成，等待 box-app 执行推理")

	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *ReasoningExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	if nodeInstance.Config == nil {
		return fmt.Errorf("节点配置不能为空")
	}
	if _, ok := nodeInstance.Config["model_key"]; !ok {
		return fmt.Errorf("缺少 model_key 配置")
	}
	_, hasRTSP := nodeInstance.Config["rtsp_url"]
	_, hasImageURL := nodeInstance.Config["image_url"]
	if !hasRTSP && !hasImageURL {
		return fmt.Errorf("缺少图像来源配置，需提供 rtsp_url 或 image_url")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 私有方法
// ─────────────────────────────────────────────────────────────────────────────

// parseConfig 解析推理节点配置
func (e *ReasoningExecutor) parseConfig(inputs map[string]interface{}) (*ReasoningConfig, error) {
	cfg := &ReasoningConfig{
		InferenceType: "detection",
	}

	if v, ok := inputs["inference_type"].(string); ok && v != "" {
		cfg.InferenceType = v
	}
	switch cfg.InferenceType {
	case "detection", "segmentation", "obb":
	default:
		return nil, fmt.Errorf("不支持的推理类型: %s，支持: detection / segmentation / obb", cfg.InferenceType)
	}

	if v, ok := inputs["model_key"].(string); ok && v != "" {
		cfg.ModelKey = v
	} else {
		return nil, fmt.Errorf("缺少必需参数: model_key")
	}

	if v, ok := inputs["rtsp_url"].(string); ok && v != "" {
		cfg.RTSPUrl = strings.TrimSpace(v)
	}
	if v, ok := inputs["image_url"].(string); ok && v != "" {
		cfg.ImageURL = strings.TrimSpace(v)
	}
	if cfg.RTSPUrl == "" && cfg.ImageURL == "" {
		return nil, fmt.Errorf("缺少图像来源配置，需提供 rtsp_url 或 image_url")
	}

	if v, ok := inputs["skip_frame"].(float64); ok && v >= 0 {
		cfg.SkipFrame = int(v)
	}
	if v, ok := inputs["confidence_threshold"].(float64); ok {
		cfg.ConfidenceThreshold = v
	}
	if v, ok := inputs["nms_threshold"].(float64); ok {
		cfg.NMSThreshold = v
	}
	if v, ok := inputs["image_input"].(string); ok {
		cfg.ImageInput = v
	}
	if v, ok := inputs["output_var_key"].(string); ok {
		cfg.OutputVarKey = v
	}

	// 解析 ROI 配置
	if roisRaw, ok := inputs["rois"]; ok {
		rois, err := e.parseROIs(roisRaw)
		if err != nil {
			return nil, fmt.Errorf("解析 ROI 配置失败: %w", err)
		}
		cfg.ROIs = rois
	}
	cfg.UseROIToInference = len(cfg.ROIs) > 0

	return cfg, nil
}

// parseROIs 解析 ROI 配置列表
func (e *ReasoningExecutor) parseROIs(raw interface{}) ([]ROIConfig, error) {
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("rois 必须是数组类型")
	}

	rois := make([]ROIConfig, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("rois[%d] 格式无效", i)
		}

		roi := ROIConfig{}
		if v, ok := m["id"].(float64); ok {
			roi.ID = int(v)
		}
		if v, ok := m["name"].(string); ok {
			roi.Name = v
		}
		if v, ok := m["x"].(float64); ok {
			roi.X = int(v)
		}
		if v, ok := m["y"].(float64); ok {
			roi.Y = int(v)
		}
		if v, ok := m["width"].(float64); ok {
			roi.Width = int(v)
		} else {
			return nil, fmt.Errorf("rois[%d] 缺少 width", i)
		}
		if v, ok := m["height"].(float64); ok {
			roi.Height = int(v)
		} else {
			return nil, fmt.Errorf("rois[%d] 缺少 height", i)
		}

		// 解析多边形顶点（可选）
		if areasRaw, ok := m["areas"].([]interface{}); ok {
			for _, areaRaw := range areasRaw {
				if am, ok := areaRaw.(map[string]interface{}); ok {
					area := ROIArea{}
					if v, ok := am["x"].(float64); ok {
						area.X = int(v)
					}
					if v, ok := am["y"].(float64); ok {
						area.Y = int(v)
					}
					roi.Areas = append(roi.Areas, area)
				}
			}
		}

		rois = append(rois, roi)
	}
	return rois, nil
}

// buildNodeConfig 构建下发给 box-app 的节点 config 字段
// 对应 box-app node_executor.cpp executeReasoningNode 读取的字段
func (e *ReasoningExecutor) buildNodeConfig(cfg *ReasoningConfig) map[string]interface{} {
	nodeConfig := map[string]interface{}{
		"model_key":            cfg.ModelKey,
		"model_type":           cfg.InferenceType,
		"use_roi_to_inference": cfg.UseROIToInference,
	}

	// 图像来源：image_url 优先，其次 rtsp_url
	if cfg.ImageURL != "" {
		nodeConfig["image_url"] = cfg.ImageURL
	} else {
		nodeConfig["rtsp_url"] = cfg.RTSPUrl
		nodeConfig["skip_frame"] = cfg.SkipFrame
	}

	if cfg.ConfidenceThreshold > 0 {
		nodeConfig["conf_threshold"] = cfg.ConfidenceThreshold
	}
	if cfg.NMSThreshold > 0 {
		nodeConfig["nms_threshold"] = cfg.NMSThreshold
	}
	if cfg.ImageInput != "" {
		nodeConfig["image_input"] = cfg.ImageInput
	}
	if cfg.OutputVarKey != "" {
		nodeConfig["output_variable"] = cfg.OutputVarKey
	}
	if len(cfg.ROIs) > 0 {
		nodeConfig["rois"] = cfg.ROIs
	}

	return nodeConfig
}
