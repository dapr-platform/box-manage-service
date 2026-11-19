package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"testing"
)

// stubConvertedModelRepoForType 只实现 FindByModelKey，其余方法用嵌入接口占位。
type stubConvertedModelRepoForType struct {
	repository.ConvertedModelRepository
	models2 map[string]*models.ConvertedModel
}

func (s *stubConvertedModelRepoForType) FindByModelKey(_ context.Context, key string) (*models.ConvertedModel, error) {
	if m, ok := s.models2[key]; ok {
		return m, nil
	}
	return nil, nil
}

// TestNormalizeInferenceTaskTypeKeepsExplicit 显式传入的合法类型应原样保留。
func TestNormalizeInferenceTaskTypeKeepsExplicit(t *testing.T) {
	cases := map[string]string{
		"detection":     "detection",
		"segmentation":  "segmentation",
		"obb":           "obb",
		"segment":       "segmentation",
		"seg":           "segmentation",
		"detection_obb": "obb",
		" DETECTION ":   "detection",
	}
	for input, want := range cases {
		if got := normalizeInferenceTaskType(input, "", nil, context.Background()); got != want {
			t.Fatalf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestNormalizeInferenceTaskTypeFromModelKey 类型为空时从 model_key 前缀推断。
//
// 这条对应线上故障：AI 平台下发的任务 type 为空，盒子端默认按 detection 创建，
// 用分割模型报 "has wrong type: segmentation (expected: detection)"。
func TestNormalizeInferenceTaskTypeFromModelKey(t *testing.T) {
	cases := map[string]string{
		"segmentation-yolov8-BM1684X-seg_yolo8n_gf_BM1684X_F16": "segmentation",
		"SEGMENTATION-yolov8-x":                                 "segmentation",
		"obb-yolov8-BM1684X-xxx":                                "obb",
		"detection-yolov8-BM1684X-yolov8n":                      "detection",
		"unknown-key":                                           "detection",
		"":                                                      "detection",
	}
	for key, want := range cases {
		if got := normalizeInferenceTaskType("", key, nil, context.Background()); got != want {
			t.Fatalf("normalize(%q) = %q, want %q", key, got, want)
		}
	}
}

// TestNormalizeInferenceTaskTypePrefersModelTaskType 模型的 task_type 优先于 key 前缀。
func TestNormalizeInferenceTaskTypePrefersModelTaskType(t *testing.T) {
	repo := &stubConvertedModelRepoForType{models2: map[string]*models.ConvertedModel{
		// key 前缀看着像 detection，但模型自身声明是 segmentation
		"detection-yolov8-misleading": {TaskType: models.ModelTaskTypeSegmentation},
	}}
	got := normalizeInferenceTaskType("", "detection-yolov8-misleading", repo, context.Background())
	if got != "segmentation" {
		t.Fatalf("应优先用模型的 task_type=segmentation，实际 %q", got)
	}
}

// TestNormalizeInferenceTaskTypeFallsBackToDetection 完全无法判断时退回 detection，
// 与盒子端的默认行为一致。
func TestNormalizeInferenceTaskTypeFallsBackToDetection(t *testing.T) {
	repo := &stubConvertedModelRepoForType{models2: map[string]*models.ConvertedModel{}}
	if got := normalizeInferenceTaskType("", "weird-model-key", repo, context.Background()); got != "detection" {
		t.Fatalf("无法判断时应退回 detection，实际 %q", got)
	}
}
