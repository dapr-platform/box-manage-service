package utils

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	oldUpgradeUploadsPrefix = "uploads"
	newUpgradeUploadsPrefix = "data/uploads"
)

// ResolveUpgradePackageFilePath returns an existing file path for upgrade package files.
// It keeps backward compatibility with records saved before upgrade packages were moved
// from uploads/packages to data/uploads/packages.
func ResolveUpgradePackageFilePath(path string) string {
	if path == "" {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}

	cleanPath := filepath.Clean(path)
	oldPrefix := oldUpgradeUploadsPrefix + string(filepath.Separator)
	if strings.HasPrefix(cleanPath, oldPrefix) {
		candidate := filepath.Join(newUpgradeUploadsPrefix, strings.TrimPrefix(cleanPath, oldPrefix))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	dotOldPrefix := "." + string(filepath.Separator) + oldPrefix
	if strings.HasPrefix(cleanPath, dotOldPrefix) {
		candidate := filepath.Join(".", newUpgradeUploadsPrefix, strings.TrimPrefix(cleanPath, dotOldPrefix))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Docker deployments may have used an absolute /app/uploads mount before
	// switching to /app/data. Keep those historical DB records usable as well.
	absoluteOldPrefix := string(filepath.Separator) + filepath.Join("app", oldUpgradeUploadsPrefix) + string(filepath.Separator)
	if strings.HasPrefix(cleanPath, absoluteOldPrefix) {
		candidate := filepath.Join(string(filepath.Separator), "app", newUpgradeUploadsPrefix, strings.TrimPrefix(cleanPath, absoluteOldPrefix))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return path
}
