package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUpgradePackageFilePathFallsBackToDataUploads(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer os.Chdir(cwd)

	newPath := filepath.Join("data", "uploads", "packages", "1.0.0", "box-app.soc")
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(newPath, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	oldPath := filepath.Join("uploads", "packages", "1.0.0", "box-app.soc")
	if got := ResolveUpgradePackageFilePath(oldPath); got != newPath {
		t.Fatalf("ResolveUpgradePackageFilePath() = %q, want %q", got, newPath)
	}
}

func TestResolveUpgradePackageFilePathKeepsExistingPath(t *testing.T) {
	tmp := t.TempDir()
	existingPath := filepath.Join(tmp, "uploads", "packages", "1.0.0", "box-app.soc")
	if err := os.MkdirAll(filepath.Dir(existingPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(existingPath, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := ResolveUpgradePackageFilePath(existingPath); got != existingPath {
		t.Fatalf("ResolveUpgradePackageFilePath() = %q, want %q", got, existingPath)
	}
}
