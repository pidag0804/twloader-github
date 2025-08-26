// twloader-tool/game/paths.go
package game

import (
	"fmt"
	"os"
	"path/filepath"
	"twloader-tool/config"
)

var DefaultBaseDir = filepath.Join(os.Getenv("ProgramFiles(x86)"), "TWLoader")

// ResolveBasePath 解析 TWLoader 的基礎路徑
func ResolveBasePath() (string, error) {
	cfg := config.Get()
	basePath := cfg.CustomBasePath

	if basePath == "" {
		basePath = DefaultBaseDir
	}

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return "", fmt.Errorf("基礎路徑不存在: %s。請透過設定指定正確的 TWLoader 主安裝資料夾", basePath)
	}
	return basePath, nil
}

// ResolveTargetPath 解析最終的 edata 目錄路徑
func ResolveTargetPath(mode string, customPathHint string) (string, error) {
	var basePath string
	var err error

	if customPathHint != "" {
		basePath = customPathHint
	} else {
		basePath, err = ResolveBasePath()
		if err != nil {
			return "", err
		}
	}

	var finalPath string
	switch mode {
	case "plus":
		finalPath = filepath.Join(basePath, "Plus", "edata")
	case "plusup":
		finalPath = filepath.Join(basePath, "PlusUP", "edata")
	default:
		return "", fmt.Errorf("無效的模式: '%s'", mode)
	}

	return finalPath, nil
}
