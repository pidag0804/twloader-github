// cmd/chat-server/main.go
package main

import (
	"log"
	"net/http"
	"twloader-tool/api"
)

func main() {
	mux := http.NewServeMux()

	// 伺服器只註冊聊天室需要的 WebSocket 路由
	mux.HandleFunc("GET /ws/chat", api.HandleChatWebSocket)

	port := ":8787"
	log.Printf("獨立聊天伺服器已啟動，監聽於 http://0.0.0.0%s", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("無法啟動伺服器: %v", err)
	}
}
