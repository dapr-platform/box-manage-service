package service

import (
	"strings"
	"testing"
)

func TestSmartVisionFileNameHandlesWindowsPath(t *testing.T) {
	got := smartVisionFileName(`\\10.188.96.7\ComputerVision\AiProjectFolder\1892843911200546818\Models\1052825070717767680\weights\best.onnx`, "fallback")
	if got != "best.onnx" {
		t.Fatalf("expected best.onnx, got %q", got)
	}
}

func TestSaveModelFileRejectsTinySmartVisionDownload(t *testing.T) {
	svc := &SmartVisionService{modelStorageBasePath: t.TempDir()}
	content := "SmartVision download err"

	_, err := svc.saveModelFile(strings.NewReader(content), int64(len(content)), "best.onnx", "demo", "v1")
	if err == nil {
		t.Fatal("expected tiny SmartVision download to be rejected")
	}
	if !strings.Contains(err.Error(), "不是有效模型文件") {
		t.Fatalf("expected invalid model error, got %v", err)
	}
}
