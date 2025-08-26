// twloader-tool/optimizer/updater.go
package optimizer

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"twloader-tool/game"
	"twloader-tool/utils"
)

const (
	plusUpdateListURL   = "https://www.tlmoo.com/twloader/PackageInfo/PlusInfo2.txt"
	plusUPUpdateListURL = "https://www.tlmoo.com/twloader/PackageInfo/PlusUPInfo2.txt"
)

var updaterLogger = log.New(os.Stdout, "UPDATER | ", log.LstdFlags)

func CheckForUpdates(mode, customPath string) ([]UpdateItem, error) {
	basePath := customPath
	if basePath == "" {
		var err error
		basePath, err = game.ResolveBasePath()
		if err != nil {
			return nil, err
		}
	}

	var listURL string
	if mode == "plus" {
		listURL = plusUpdateListURL
	} else if mode == "plusup" {
		listURL = plusUPUpdateListURL
	} else {
		return nil, fmt.Errorf("無效的模式")
	}

	return parseUpdateList(listURL, basePath)
}

func parseUpdateList(url, basePath string) ([]UpdateItem, error) {
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("無法下載列表: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("伺服器錯誤: %s", resp.Status)
	}

	var itemsToUpdate []UpdateItem
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), ";")
		parts := strings.Split(line, ",")
		if len(parts) != 6 {
			continue
		}

		enabled, _ := strconv.Atoi(parts[5])
		if enabled != 1 {
			continue
		}

		sizeExpected, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			updaterLogger.Printf("警告: 無法解析大小 '%s' 於行: %s", parts[1], line)
			continue
		}

		relativePath := filepath.FromSlash(parts[2])
		fullPath := filepath.Join(basePath, relativePath)

		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) || info.Size() != sizeExpected {
			item := UpdateItem{
				Name:         parts[0],
				SizeExpected: sizeExpected,
				RelativePath: relativePath,
				Path:         fullPath,
				URL:          parts[3],
				BackupURL:    parts[4],
			}
			itemsToUpdate = append(itemsToUpdate, item)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("讀取列表內容時發生錯誤: %w", err)
	}
	updaterLogger.Printf("檢查完成，找到 %d 個需要更新的檔案。", len(itemsToUpdate))
	return itemsToUpdate, nil
}

func ApplyUpdates(ctx context.Context, items []UpdateItem) (updatedFiles []string, failedUpdates []FailedUpdate, permissionError bool) {
	var wg sync.WaitGroup
	var mutex sync.Mutex

	semaphore := make(chan struct{}, 4)

	for _, item := range items {
		wg.Add(1)
		go func(item UpdateItem) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := downloadAndUpdateFile(ctx, item)

			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				if os.IsPermission(err) {
					permissionError = true
				}
				failedUpdates = append(failedUpdates, FailedUpdate{Path: item.RelativePath, Error: err.Error()})
			} else {
				updatedFiles = append(updatedFiles, item.RelativePath)
			}
		}(item)
	}
	wg.Wait()

	return
}

func downloadAndUpdateFile(ctx context.Context, item UpdateItem) error {
	updaterLogger.Printf("正在更新檔案: %s", item.RelativePath)
	data, err := utils.DownloadWithRetries(ctx, item.URL, item.BackupURL)
	if err != nil {
		return fmt.Errorf("下載失敗: %w", err)
	}

	targetDir := filepath.Dir(item.Path)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("建立目錄失敗: %w", err)
	}

	tempFile, err := os.CreateTemp(targetDir, "update_*.tmp")
	if err != nil {
		return fmt.Errorf("建立暫存檔失敗: %w", err)
	}

	_, writeErr := tempFile.Write(data)
	closeErr := tempFile.Close()

	if writeErr != nil {
		os.Remove(tempFile.Name())
		return fmt.Errorf("寫入暫存檔失敗: %w", writeErr)
	}
	if closeErr != nil {
		os.Remove(tempFile.Name())
		return fmt.Errorf("關閉暫存檔失敗: %w", closeErr)
	}

	if err := os.Rename(tempFile.Name(), item.Path); err != nil {
		os.Remove(tempFile.Name()) // Try to clean up
		// Attempt to copy and delete if rename fails (e.g., across different drives)
		if err := os.WriteFile(item.Path, data, 0666); err != nil {
			return fmt.Errorf("更名和寫入檔案均失敗: %w", err)
		}
	}

	updaterLogger.Printf("成功更新: %s", item.RelativePath)
	return nil
}

func InstallItem(ctx context.Context, item OptimizationItem, targetDir string) (int64, error) {
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		return 0, err
	}

	finalPath := filepath.Join(targetDir, item.TargetFile)
	updaterLogger.Printf("開始安裝 '%s' 到 '%s'", item.Name, finalPath)

	data, err := utils.DownloadWithRetries(ctx, item.FileURL, "")
	if err != nil {
		return 0, fmt.Errorf("下載 '%s' 失敗: %w", item.Name, err)
	}

	tempFile, err := os.CreateTemp(targetDir, "dl_*.tmp")
	if err != nil {
		return 0, fmt.Errorf("無法建立暫存檔: %w", err)
	}
	defer os.Remove(tempFile.Name())

	bytesWritten, err := tempFile.Write(data)
	if err != nil {
		tempFile.Close()
		return 0, fmt.Errorf("寫入暫存檔失敗: %w", err)
	}
	tempFile.Close()

	if err := os.Rename(tempFile.Name(), finalPath); err != nil {
		// Fallback for cross-device rename error
		if err := os.WriteFile(finalPath, data, 0666); err != nil {
			return 0, fmt.Errorf("覆蓋最終檔案失敗: %w", err)
		}
	}

	return int64(bytesWritten), nil
}

func UninstallItem(item OptimizationItem, targetDir string) error {
	filePath := filepath.Join(targetDir, item.TargetFile)
	updaterLogger.Printf("準備移除檔案: %s", filePath)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		updaterLogger.Printf("檔案不存在，視為移除成功: %s", filePath)
		return nil
	}

	return os.Remove(filePath)
}
