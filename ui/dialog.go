// twloader-tool/ui/dialog.go
package ui

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/faiface/mainthread"
	"github.com/sqweek/dialog"
)

var (
	logger             = log.New(os.Stdout, "UI | ", log.LstdFlags)
	dialogMutex        = &sync.Mutex{}
	dialogRequestChan  = make(chan bool)
	dialogResponseChan = make(chan dialogResponse)
)

type dialogResponse struct {
	path string
	err  error
}

func GUIManager(readyChan chan<- bool) {
	readyChan <- true
	for range dialogRequestChan {
		var path string
		var err error
		mainthread.Call(func() {
			path, err = dialog.Directory().Title("請選擇 TWLoader 主安裝資料夾 (例如 C:\\Program Files (x86)\\TWLoader)").Browse()
		})
		select {
		case dialogResponseChan <- dialogResponse{path: path, err: err}:
		case <-time.After(2 * time.Second):
			logger.Println("警告: dialogResponseChan 發送超時")
		}
	}
	logger.Println("GUI 管理器已關閉。")
}

func CloseGUIManager() {
	close(dialogRequestChan)
}

func SelectDirectory() (string, error) {
	if !dialogMutex.TryLock() {
		return "", fmt.Errorf("一個選擇視窗已在執行中")
	}
	defer dialogMutex.Unlock()

	dialogRequestChan <- true
	resp := <-dialogResponseChan

	if resp.err == dialog.ErrCancelled {
		return "", nil // User cancelled, not an error
	}
	return resp.path, resp.err
}
