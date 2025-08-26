// twloader-tool/selfupdate/update.go
package selfupdate

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"twloader-tool/utils"
)

var (
	// 將 appVersion 改為變數，並設定一個開發時的預設值
	appVersion        = "1.0.0-dev"
	appUpdateCheckURL = "http://tlmoo.com/twloader/down/version.json"
)

var selfUpdateLogger = log.New(os.Stdout, "SELFUPDATE | ", log.LstdFlags)

type AppVersionInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Notes   string `json:"notes"`
}

// Check 檢查應用程式是否有新版本
func Check() (map[string]interface{}, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(appUpdateCheckURL)
	if err != nil {
		return nil, fmt.Errorf("無法連線到更新伺服器: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新伺服器回應錯誤: %s", resp.Status)
	}

	var latestVersion AppVersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&latestVersion); err != nil {
		return nil, fmt.Errorf("無法解析版本資訊: %w", err)
	}

	if latestVersion.Version > appVersion {
		return map[string]interface{}{
			"updateAvailable": true,
			"latestVersion":   latestVersion,
			"currentVersion":  appVersion,
		}, nil
	}

	return map[string]interface{}{
		"updateAvailable": false,
		"currentVersion":  appVersion,
	}, nil
}

// Apply 執行應用程式更新
func Apply(versionInfo AppVersionInfo) error {
	selfUpdateLogger.Println("開始下載新版本:", versionInfo.Version)

	newExeBytes, err := utils.DownloadFile(versionInfo.URL)
	if err != nil {
		return fmt.Errorf("下載更新檔失敗: %w", err)
	}

	currentExePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("找不到目前執行檔路徑: %w", err)
	}
	newExePath := filepath.Join(filepath.Dir(currentExePath), "TWLoaderWeb_new.exe")

	if err := os.WriteFile(newExePath, newExeBytes, 0755); err != nil {
		return fmt.Errorf("儲存更新檔失敗: %w", err)
	}
	selfUpdateLogger.Println("新版本已下載至:", newExePath)

	updaterPath := filepath.Join(filepath.Dir(currentExePath), "updater.exe")
	if _, err := os.Stat(updaterPath); os.IsNotExist(err) {
		os.Remove(newExePath) // Clean up downloaded file
		return fmt.Errorf("找不到更新工具 (updater.exe)，請確認程式完整性")
	}

	cmd := exec.Command(updaterPath, currentExePath, newExePath)
	if err := cmd.Start(); err != nil {
		os.Remove(newExePath) // Clean up downloaded file
		return fmt.Errorf("啟動更新程序失敗: %w", err)
	}

	selfUpdateLogger.Println("更新小幫手已啟動，主程式即將關閉...")
	return nil
}
