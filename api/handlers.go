//go:build windows

// twloader-tool/api/handlers.go
package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"twloader-tool/config"
	"twloader-tool/game"
	"twloader-tool/optimizer"
	"twloader-tool/selfupdate"
	"twloader-tool/ui"
	"twloader-tool/utils"
)

var (
	installMutex  = &sync.Mutex{}
	handlerLogger = log.New(os.Stdout, "API_HANDLER | ", log.LstdFlags)
)

// --- 結構體定義 (保持不變) ---
type BaseRequest struct {
	Mode       string `json:"mode"`
	CustomPath string `json:"customPath,omitempty"`
}
type InstallRequest struct {
	BaseRequest
	Slug     string `json:"slug"`
	Category string `json:"category"`
}
type StatusRequest struct {
	BaseRequest
	Files []string `json:"files"`
}
type StatusResponse struct {
	Exists map[string]bool `json:"exists"`
}
type InitialStateResponse struct {
	PlusExists        bool   `json:"plusExists"`
	PlusUpExists      bool   `json:"plusUpExists"`
	CustomPath        string `json:"customPath"`
	DefaultPathExists bool   `json:"defaultPathExists"`
}
type SelectPathResponse struct {
	Path string `json:"path"`
}
type ResolutionConfig struct {
	WinMode int `json:"winMode"`
	Width   int `json:"width"`
	Height  int `json:"height"`
}

// --- 所有 Handle... 函式 (此處不包含 ServeIndex, ServeCSS, ServeJS) ---
func HandleGetInitialState(w http.ResponseWriter, r *http.Request) {
	basePath, _ := game.ResolveBasePath()
	_, defaultPathErr := os.Stat(game.DefaultBaseDir)

	var plusErr, plusUpErr error = os.ErrNotExist, os.ErrNotExist
	if basePath != "" {
		plusExePath := filepath.Join(basePath, "Plus", "TWLoader.exe")
		_, plusErr = os.Stat(plusExePath)

		plusUpExePath := filepath.Join(basePath, "PlusUP", "TWLoader.exe")
		_, plusUpErr = os.Stat(plusUpExePath)
	}

	response := InitialStateResponse{
		PlusExists:        plusErr == nil,
		PlusUpExists:      plusUpErr == nil,
		CustomPath:        config.Get().CustomBasePath,
		DefaultPathExists: defaultPathErr == nil,
	}
	utils.WriteJSON(w, http.StatusOK, response)
}

func HandleLaunch(w http.ResponseWriter, r *http.Request) {
	mode := r.PathValue("mode")
	if err := game.Launch(mode); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{OK: true})
}

func HandleSelectPath(w http.ResponseWriter, r *http.Request) {
	path, err := ui.SelectDirectory()
	if err != nil {
		if strings.Contains(err.Error(), "執行中") {
			utils.WriteJSONError(w, http.StatusConflict, "一個選擇視窗已在執行中，請先完成前一個操作。")
		} else {
			utils.WriteJSONError(w, http.StatusInternalServerError, "無法開啟資料夾選擇視窗: %v", err)
		}
		return
	}

	if path == "" { // User cancelled
		utils.WriteJSON(w, http.StatusOK, SelectPathResponse{Path: ""})
		return
	}

	cfg := config.Get()
	cfg.CustomBasePath = path
	if err := config.Save(cfg); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "儲存設定檔失敗: %v", err)
		return
	}

	handlerLogger.Printf("使用者選擇了新的基礎路徑: %s", path)
	utils.WriteJSON(w, http.StatusOK, SelectPathResponse{Path: path})
}

func HandleResetPath(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	cfg.CustomBasePath = ""
	if err := config.Save(cfg); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "儲存設定檔失敗: %v", err)
		return
	}
	HandleGetInitialState(w, r)
}

func HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	var req BaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的請求: %v", err)
		return
	}

	itemsToUpdate, err := optimizer.CheckForUpdates(req.Mode, req.CustomPath)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, optimizer.UpdateCheckResponse{OK: false, Error: fmt.Sprintf("處理更新列表失敗: %v", err)})
		return
	}

	updateNeeded := len(itemsToUpdate) > 0
	utils.WriteJSON(w, http.StatusOK, optimizer.UpdateCheckResponse{
		OK:           true,
		UpdateNeeded: updateNeeded,
		Items:        itemsToUpdate,
	})
}

func HandleApplyUpdates(w http.ResponseWriter, r *http.Request) {
	var req optimizer.ApplyUpdatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的請求: %v", err)
		return
	}

	if len(req.Items) == 0 {
		utils.WriteJSON(w, http.StatusOK, optimizer.ApplyUpdatesResponse{OK: true, Message: "目前已是最新版本"})
		return
	}

	updatedFiles, failedUpdates, permissionError := optimizer.ApplyUpdates(r.Context(), req.Items)

	if permissionError {
		utils.WriteJSON(w, http.StatusForbidden, optimizer.ApplyUpdatesResponse{
			OK:        false,
			NeedAdmin: true,
			Error:     "權限不足，無法寫入檔案。請以系統管理員身分重啟程式。",
			Updated:   updatedFiles,
			Failed:    failedUpdates,
		})
		return
	}

	message := fmt.Sprintf("更新完成。成功 %d 個檔案，失敗 %d 個。", len(updatedFiles), len(failedUpdates))
	if len(failedUpdates) == 0 {
		message = "已自動更新到最新版本"
	}

	utils.WriteJSON(w, http.StatusOK, optimizer.ApplyUpdatesResponse{
		OK:      len(failedUpdates) == 0,
		Updated: updatedFiles,
		Failed:  failedUpdates,
		Message: message,
	})
}

func HandleGetItems(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	items, ok := optimizer.GetItemsByCategory(category)
	if !ok {
		utils.WriteJSONError(w, http.StatusNotFound, "找不到類別: %s", category)
		return
	}

	sanitizedItems := make([]optimizer.OptimizationItem, len(items))
	for i, item := range items {
		sanitizedItems[i] = item
		sanitizedItems[i].FileURL = "" // Hide FileURL from client
	}
	utils.WriteJSON(w, http.StatusOK, sanitizedItems)
}

func HandleInstall(w http.ResponseWriter, r *http.Request) {
	installMutex.Lock()
	defer installMutex.Unlock()

	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的請求內容: %v", err)
		return
	}

	item, found := optimizer.FindItemBySlugAndCategory(req.Category, req.Slug)
	if !found {
		utils.WriteJSONError(w, http.StatusNotFound, "在類別 '%s' 中找不到 slug 為 '%s' 的項目。", req.Category, req.Slug)
		return
	}

	targetDir, err := game.ResolveTargetPath(req.Mode, req.CustomPath)
	if err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	bytesWritten, err := optimizer.InstallItem(r.Context(), item, targetDir)
	if err != nil {
		if os.IsPermission(err) {
			utils.WriteJSON(w, http.StatusForbidden, utils.APIResponse{
				OK:        false,
				NeedAdmin: true,
				Error:     fmt.Sprintf("權限不足，無法寫入 %s。請以系統管理員身分執行此程式。", targetDir),
			})
			return
		}
		utils.WriteJSONError(w, http.StatusInternalServerError, "安裝 '%s' 失敗: %v", item.Name, err)
		return
	}

	handlerLogger.Printf("成功安裝 '%s' (%d bytes)", item.Name, bytesWritten)
	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{
		OK:    true,
		Bytes: bytesWritten,
	})
}

func HandleUninstall(w http.ResponseWriter, r *http.Request) {
	installMutex.Lock()
	defer installMutex.Unlock()

	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的請求內容: %v", err)
		return
	}

	item, found := optimizer.FindItemBySlugAndCategory(req.Category, req.Slug)
	if !found {
		utils.WriteJSONError(w, http.StatusNotFound, "在類別 '%s' 中找不到 slug 為 '%s' 的項目。", req.Category, req.Slug)
		return
	}

	targetDir, err := game.ResolveTargetPath(req.Mode, req.CustomPath)
	if err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	err = optimizer.UninstallItem(item, targetDir)
	if err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "移除檔案失敗: %v", err)
		return
	}

	handlerLogger.Printf("成功移除檔案: %s", item.TargetFile)
	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{OK: true})
}

func HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	var req StatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的狀態請求: %v", err)
		return
	}

	targetDir, err := game.ResolveTargetPath(req.Mode, req.CustomPath)
	if err != nil {
		statusMap := make(map[string]bool)
		for _, file := range req.Files {
			statusMap[file] = false
		}
		utils.WriteJSON(w, http.StatusOK, StatusResponse{Exists: statusMap})
		return
	}

	statusMap := make(map[string]bool)
	for _, file := range req.Files {
		filePath := filepath.Join(targetDir, file)
		if _, err := os.Stat(filePath); err == nil {
			statusMap[file] = true
		} else {
			statusMap[file] = false
		}
	}
	utils.WriteJSON(w, http.StatusOK, StatusResponse{Exists: statusMap})
}

func HandleRelaunchAdmin(w http.ResponseWriter, r *http.Request) {
	executable, err := os.Executable()
	if err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "找不到執行檔路徑: %v", err)
		return
	}
	cmd := exec.Command("powershell", "-Command", "Start-Process", "-FilePath", `"`+executable+`"`, "-Verb", "RunAs")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	handlerLogger.Println("正在嘗試以系統管理員身分重啟...")
	if err := cmd.Start(); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, "執行提權命令失敗: %v", err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{OK: true})

	go func() {
		time.Sleep(1 * time.Second)
		TriggerShutdown() // 改為大寫
	}()
}

func HandleGetGameUpdateStatus(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, game.GetUpdateState())
}

func HandleRunGamePatcher(w http.ResponseWriter, r *http.Request) {
	if err := game.RunPatcher(); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{OK: true})
}

func HandleGetResolutionConfig(w http.ResponseWriter, r *http.Request) {
	installPath, err := game.GetInstallPath()
	if err != nil {
		utils.WriteJSONError(w, http.StatusNotFound, "找不到遊戲安裝路徑，請確認勁舞團是否已安裝: %v", err)
		return
	}

	configPath := filepath.Join(installPath, "Config.ini")
	config := ResolutionConfig{WinMode: 1, Width: 1024, Height: 768} // 預設值

	data, err := os.ReadFile(configPath)
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key, value := strings.ToUpper(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
			switch key {
			case "WINMODE":
				config.WinMode, _ = strconv.Atoi(value)
			case "WIDTH":
				config.Width, _ = strconv.Atoi(value)
			case "HEIGHT":
				config.Height, _ = strconv.Atoi(value)
			}
		}
	} else if !os.IsNotExist(err) {
		utils.WriteJSONError(w, http.StatusInternalServerError, "讀取 Config.ini 失敗: %v", err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, config)
}

func HandleSetResolutionConfig(w http.ResponseWriter, r *http.Request) {
	var req ResolutionConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的請求內容: %v", err)
		return
	}

	installPath, err := game.GetInstallPath()
	if err != nil {
		utils.WriteJSONError(w, http.StatusNotFound, "找不到遊戲安裝路徑，無法儲存設定: %v", err)
		return
	}

	configPath := filepath.Join(installPath, "Config.ini")
	content := fmt.Sprintf("[CONFIG]\r\nWINMODE=%d\r\nWIDTH=%d\r\nHEIGHT=%d\r\n", req.WinMode, req.Width, req.Height)

	err = os.WriteFile(configPath, []byte(content), 0666)
	if err != nil {
		if os.IsPermission(err) {
			utils.WriteJSON(w, http.StatusForbidden, utils.APIResponse{
				OK:        false,
				NeedAdmin: true,
				Error:     fmt.Sprintf("權限不足，無法寫入 %s。請以系統管理員身分執行此程式。", configPath),
			})
			return
		}
		utils.WriteJSONError(w, http.StatusInternalServerError, "寫入 Config.ini 失敗: %v", err)
		return
	}

	handlerLogger.Printf("成功更新解析度設定: %s", configPath)
	utils.WriteJSON(w, http.StatusOK, utils.APIResponse{OK: true})
}

func HandleCheckAppUpdate(w http.ResponseWriter, r *http.Request) {
	result, err := selfupdate.Check()
	if err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, result)
}

// ==================================================================
// 【MODIFIED】: THIS IS THE CORRECTED FUNCTION
// ==================================================================
func HandleApplyAppUpdate(w http.ResponseWriter, r *http.Request) {
	var req selfupdate.AppVersionInfo
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONError(w, http.StatusBadRequest, "無效的請求: %v", err)
		return
	}

	if err := selfupdate.Apply(req); err != nil {
		utils.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 1. 正常地寫入成功回應到緩衝區
	// 注意：我們在這裡手動設定 header 和 encode，而不是用 utils.WriteJSON
	// 因為我們需要在寫入後手動 Flush
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "更新程序已啟動，應用程式即將關閉。",
	})

	// 2. 【核心】強制將緩衝區的內容發送給客戶端
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// 3. 在一個新的 goroutine 中延遲觸發關閉，確保 HTTP 回應有足夠時間送達
	go func() {
		time.Sleep(500 * time.Millisecond)
		TriggerShutdown()
	}()
}
