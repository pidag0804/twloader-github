//go:build windows

// twloader-tool/api/static.go
package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"twloader-tool/utils"
)

var (
	indexHTML []byte
	styleCSS  []byte
	scriptJS  []byte
)

const (
	remoteIndexURL  = "http://tlmoo.com/twloader/down/index.html"
	remoteStyleURL  = "http://tlmoo.com/twloader/down/style.css"
	remoteScriptURL = "http://tlmoo.com/twloader/down/script.js"
)

// FetchStaticAssets 保持不大寫，因為它是套件內部使用的函式
func FetchStaticAssets() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 3)
	cacheBuster := fmt.Sprintf("?v=%d", time.Now().Unix())
	wg.Add(3)
	go func() {
		defer wg.Done()
		var err error
		indexHTML, err = utils.DownloadFile(remoteIndexURL + cacheBuster)
		if err != nil {
			errChan <- err
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		styleCSS, err = utils.DownloadFile(remoteStyleURL + cacheBuster)
		if err != nil {
			errChan <- err
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		scriptJS, err = utils.DownloadFile(remoteScriptURL + cacheBuster)
		if err != nil {
			errChan <- err
		}
	}()
	wg.Wait()
	close(errChan)
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}

// ServeIndex 處理主頁面請求 (維持大寫)
func ServeIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

// ServeCSS 處理 CSS 檔案請求 (維持大寫)
func ServeCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(styleCSS)
}

// ServeJS 處理 JavaScript 檔案請求 (維持大寫)
func ServeJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Write(scriptJS)
}
