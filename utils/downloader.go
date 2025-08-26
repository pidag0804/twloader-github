// twloader-tool/utils/downloader.go
package utils

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	connectTimeout = 10 * time.Second
	requestTimeout = 60 * time.Second
	maxRetries     = 2
	retryBaseDelay = 500 * time.Millisecond
)

var downloaderLogger = log.New(os.Stdout, "DOWNLOADER | ", log.LstdFlags)

func DownloadFile(url string) ([]byte, error) {
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("伺服器回應錯誤狀態: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func DownloadWithRetries(ctx context.Context, primaryURL string, backupURL string) ([]byte, error) {
	urlsToTry := []string{primaryURL}
	if backupURL != "" && backupURL != "0" {
		urlsToTry = append(urlsToTry, backupURL)
	}

	var lastErr error
	for _, url := range urlsToTry {
		body, err := downloadAttempt(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		downloaderLogger.Printf("從 %s 下載失敗: %v. 嘗試下一個 URL...", url, err)
	}

	return nil, fmt.Errorf("所有下載嘗試均失敗: %w", lastErr)
}

func downloadAttempt(ctx context.Context, url string) ([]byte, error) {
	var body []byte
	var lastErr error
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: connectTimeout,
			}).DialContext,
		},
		Timeout: requestTimeout,
	}
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			delay := time.Duration(i) * retryBaseDelay
			time.Sleep(delay)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("無法建立請求: %w", err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP 請求失敗: %w", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("不正確的狀態碼: %s", resp.Status)
			continue
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("讀取回應內容失敗: %w", err)
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("在 %d 次重試後仍然失敗: %w", maxRetries, lastErr)
}
