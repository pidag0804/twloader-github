// twloader-tool/api/websocket.go
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- 全域變數 ---
var (
	logger       = log.New(os.Stdout, "API_WS | ", log.LstdFlags)
	ShutdownChan = make(chan struct{})
	upgrader     = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

// --- 聊天室相關結構 ---
type ClientProfile struct {
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
	Gender     string `json:"gender"`
	HideAvatar bool   `json:"hideAvatar"`
}
type ClientMessage struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}
type ServerMessage struct {
	Type    string         `json:"type"`
	Content interface{}    `json:"content,omitempty"`
	Profile *ClientProfile `json:"profile,omitempty"`
	Time    time.Time      `json:"time,omitempty"`
}
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	profile ClientProfile
	// 【新增】記錄客戶端上次發送訊息的時間
	lastMessageTime time.Time
}
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			logger.Printf("使用者 '%s' 加入聊天室", client.profile.Nickname)
			h.broadcastUserList()
		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				logger.Printf("使用者 '%s' 離開聊天室", client.profile.Nickname)
			}
			h.mutex.Unlock()
			h.broadcastUserList()
		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

func (h *Hub) broadcastUserList() {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	var userList []ClientProfile
	for client := range h.clients {
		userList = append(userList, client.profile)
	}
	msg := ServerMessage{Type: "userList", Content: userList}
	messageBytes, _ := json.Marshal(msg)
	for client := range h.clients {
		// 預防 channel 已關閉導致 panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Printf("發送 userList 失敗: %v", r)
				}
			}()
			client.send <- messageBytes
		}()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Printf("readPump 錯誤: %v", err)
			}
			break
		}
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			logger.Printf("無法解析訊息: %v", err)
			continue
		}
		switch clientMsg.Type {
		case "updateProfile":
			if profileData, ok := clientMsg.Content.(map[string]interface{}); ok {
				c.hub.mutex.Lock()
				if nickname, ok := profileData["nickname"].(string); ok {
					c.profile.Nickname = nickname
				}
				if avatar, ok := profileData["avatar"].(string); ok {
					c.profile.Avatar = avatar
				}
				if gender, ok := profileData["gender"].(string); ok {
					c.profile.Gender = gender
				}
				if hideAvatar, ok := profileData["hideAvatar"].(bool); ok {
					c.profile.HideAvatar = hideAvatar
				}
				c.hub.mutex.Unlock()
				c.hub.broadcastUserList()
			}
		case "chatMessage":
			// --- 【主要修改區域】 ---
			// 檢查距離上次發言是否已超過一分鐘
			if time.Since(c.lastMessageTime) < time.Minute {
				// 如果時間不夠，發送錯誤訊息給該客戶端並跳過
				errorMsg := ServerMessage{
					Type:    "rateLimitError",
					Content: "發言太頻繁，請一分鐘後再試。",
					Time:    time.Now(),
				}
				msgBytes, _ := json.Marshal(errorMsg)
				c.send <- msgBytes
				continue // 跳過這次訊息廣播
			}

			// 驗證通過，處理訊息
			if content, ok := clientMsg.Content.(string); ok && content != "" {
				// 更新最後發言時間
				c.lastMessageTime = time.Now()

				// 廣播訊息
				serverMsg := ServerMessage{Type: "chatMessage", Content: content, Profile: &c.profile, Time: time.Now()}
				messageBytes, _ := json.Marshal(serverMsg)
				c.hub.broadcast <- messageBytes
			}
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			logger.Printf("writePump 錯誤: %v", err)
			return
		}
	}
}

var chatHub = newHub()

func init() {
	go chatHub.run()
}

// HandleChatWebSocket 是給 `/ws/chat` 路由的新處理器
func HandleChatWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Printf("Chat WebSocket 升級失敗: %v", err)
		return
	}
	client := &Client{
		hub:     chatHub,
		conn:    conn,
		send:    make(chan []byte, 256),
		profile: ClientProfile{Nickname: "User" + time.Now().Format("150405"), Gender: "Male"},
		// lastMessageTime 的初始值為 time.Time 的零值，所以第一次發言必定成功
	}
	client.hub.register <- client
	go client.writePump()
	go client.readPump()
}

// HandleWebSocket 是舊有的，用於偵測網頁關閉的處理器
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Printf("Main WebSocket 升級失敗: %v", err)
		return
	}
	defer conn.Close()
	logger.Println("前端主連線已建立。程式將在網頁關閉時自動結束。")

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			logger.Printf("偵測到主連線中斷: %v", err)
			break
		}
	}
	// 在此處呼叫大寫的 TriggerShutdown
	TriggerShutdown()
}

// TriggerShutdown 觸發程式關閉
func TriggerShutdown() {
	logger.Println("正在觸發程式關閉信號...")
	select {
	case _, ok := <-ShutdownChan:
		if !ok {
			logger.Println("關閉信號已被觸發，無需重複。")
			return
		}
	default:
		close(ShutdownChan)
	}
}
