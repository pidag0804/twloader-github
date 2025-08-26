// twloader-tool/utils/responses.go
package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

var responseLogger = log.New(os.Stdout, "RESPONSE | ", log.LstdFlags)

type APIResponse struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Path      string `json:"path,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	NeedAdmin bool   `json:"needAdmin,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		responseLogger.Printf("編碼 JSON 回應時發生錯誤: %v", err)
	}
}

func WriteJSONError(w http.ResponseWriter, status int, format string, args ...interface{}) {
	errMsg := fmt.Sprintf(format, args...)
	responseLogger.Println("錯誤:", errMsg)
	WriteJSON(w, status, APIResponse{OK: false, Error: errMsg})
}
