package service

import (
	"strings"
	"testing"

	"box-manage-service/models"
)

func TestSelectSystemUpdateFiles(t *testing.T) {
	backendFile := models.UpgradeFile{Type: models.FileTypeBackendProgram, Name: "box-app.soc", Path: "/tmp/box-app.soc", Size: 1}
	frontendFile := models.UpgradeFile{Type: models.FileTypeFrontendUI, Name: "dist.zip", Path: "/tmp/dist.zip", Size: 2}
	fontFile := models.UpgradeFile{Type: models.FileTypeFontPackage, Name: "fonts.tgz", Path: "/tmp/fonts.tgz", Size: 3}

	tests := []struct {
		name            string
		pkg             *models.UpgradePackage
		wantUpdateTypes []string
		wantErrContains string
	}{
		{
			name: "full package uploads backend frontend and fonts in order",
			pkg: &models.UpgradePackage{
				Type:  models.PackageTypeFull,
				Files: models.UpgradeFileList{fontFile, frontendFile, backendFile},
			},
			wantUpdateTypes: []string{"program", "web", "fonts"},
		},
		{
			name: "full package can contain only frontend",
			pkg: &models.UpgradePackage{
				Type:  models.PackageTypeFull,
				Files: models.UpgradeFileList{frontendFile},
			},
			wantUpdateTypes: []string{"web"},
		},
		{
			name: "backend package requires backend file",
			pkg: &models.UpgradePackage{
				Type:  models.PackageTypeBackend,
				Files: models.UpgradeFileList{frontendFile},
			},
			wantErrContains: "升级包中不包含后台程序文件",
		},
		{
			name: "invalid package type returns readable error",
			pkg: &models.UpgradePackage{
				Type:  models.UpgradePackageType("invalid"),
				Files: models.UpgradeFileList{backendFile},
			},
			wantErrContains: "无效的升级包类型: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectSystemUpdateFiles(tt.pkg)
			if tt.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("selectSystemUpdateFiles() error = %v, want contains %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectSystemUpdateFiles() unexpected error = %v", err)
			}
			if len(got) != len(tt.wantUpdateTypes) {
				t.Fatalf("selectSystemUpdateFiles() returned %d files, want %d", len(got), len(tt.wantUpdateTypes))
			}
			for i := range got {
				if got[i].UpdateType != tt.wantUpdateTypes[i] {
					t.Fatalf("result[%d].UpdateType = %q, want %q", i, got[i].UpdateType, tt.wantUpdateTypes[i])
				}
				if got[i].File == nil {
					t.Fatalf("result[%d].File is nil", i)
				}
			}
		})
	}
}
