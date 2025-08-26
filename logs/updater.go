// updater.go
package main

import (
	"log"
	"os"
	"os/exec"
	"time"
)

func main() {
	log.Println("更新小幫手啟動...")

	if len(os.Args) < 3 {
		log.Println("錯誤：需要提供舊檔案路徑和新檔案路徑。")
		time.Sleep(5 * time.Second) // 暫停讓使用者看到錯誤
		return
	}

	oldPath := os.Args[1]
	newPath := os.Args[2]

	log.Println("舊檔案:", oldPath)
	log.Println("新檔案:", newPath)

	// 等待主程式完全退出並釋放檔案鎖定
	time.Sleep(2 * time.Second)

	// 刪除舊檔案
	log.Println("正在移除舊版本...")
	if err := os.Remove(oldPath); err != nil {
		log.Printf("移除舊檔案失敗: %v。可能是權限不足或檔案仍被佔用。", err)
		time.Sleep(5 * time.Second)
		return
	}
	log.Println("舊版本已移除。")

	// 將新檔案更名為舊檔案
	log.Println("正在套用新版本...")
	if err := os.Rename(newPath, oldPath); err != nil {
		log.Printf("更名新檔案失敗: %v。", err)
		time.Sleep(5 * time.Second)
		return
	}
	log.Println("新版本已套用。")

	// 重新啟動主程式
	log.Println("正在重新啟動應用程式...")
	cmd := exec.Command(oldPath)
	if err := cmd.Start(); err != nil {
		log.Printf("重新啟動應用程式失敗: %v。", err)
		time.Sleep(5 * time.Second)
	}

	log.Println("更新完成，小幫手即將退出。")
}