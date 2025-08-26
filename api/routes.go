//go:build windows

// twloader-tool/api/routes.go
package api

import (
	"net/http"
	"runtime"
)

// RegisterRoutes 集中註冊所有 API 路由
func RegisterRoutes(mux *http.ServeMux) {
	// 靜態資源
	mux.HandleFunc("GET /", ServeIndex)
	mux.HandleFunc("GET /static/style.css", ServeCSS)
	mux.HandleFunc("GET /static/script.js", ServeJS)

	// WebSocket
	mux.HandleFunc("GET /ws", HandleWebSocket)
	mux.HandleFunc("GET /ws/chat", HandleChatWebSocket) // <-- 將聊天室路由加回來

	// 核心優化 API
	mux.HandleFunc("GET /api/items/{category}", HandleGetItems)
	mux.HandleFunc("POST /api/install", HandleInstall)
	mux.HandleFunc("POST /api/uninstall", HandleUninstall)
	mux.HandleFunc("POST /api/status", HandleGetStatus)
	mux.HandleFunc("GET /api/get-initial-state", HandleGetInitialState)

	// 遊戲內容更新 API
	mux.HandleFunc("POST /api/check-updates", HandleCheckUpdates)
	mux.HandleFunc("POST /api/apply-updates", HandleApplyUpdates)

	// 遊戲啟動與路徑設定
	mux.HandleFunc("POST /api/launch/{mode}", HandleLaunch)
	mux.HandleFunc("POST /api/select-path", HandleSelectPath)
	mux.HandleFunc("POST /api/reset-path", HandleResetPath)

	// 遊戲主程式更新 API
	mux.HandleFunc("GET /api/game-update-status", HandleGetGameUpdateStatus)
	mux.HandleFunc("POST /api/run-game-patcher", HandleRunGamePatcher)

	// 應用程式自我更新 API
	mux.HandleFunc("GET /api/check-app-update", HandleCheckAppUpdate)
	mux.HandleFunc("POST /api/apply-app-update", HandleApplyAppUpdate)

	// 解析度調整 API
	mux.HandleFunc("GET /api/resolution-config", HandleGetResolutionConfig)
	mux.HandleFunc("POST /api/resolution-config", HandleSetResolutionConfig)

	// Windows 專用提權 API
	if runtime.GOOS == "windows" {
		mux.HandleFunc("POST /api/relaunch-admin", HandleRelaunchAdmin)
	}
}
