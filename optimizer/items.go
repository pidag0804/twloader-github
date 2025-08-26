// twloader-tool/optimizer/items.go
package optimizer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"twloader-tool/utils"
)

const (
	encryptionKey = "TWLoader_Online_List_Key_ERdwsw_@R)(!dd)"
	encryptedURL  = "PCM4HxJeSl0oOBlCHQIIMCNHEBsyZBEOMyozABIBWDsvJUcHSBABRCd5JhwOCg=="
)

var itemsDatabase = make(map[string][]OptimizationItem)

func FetchItemsFromServer() error {
	realURL, err := utils.Decrypt(encryptedURL, encryptionKey)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(realURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("伺服器回應錯誤狀態: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&itemsDatabase); err != nil {
		return err
	}
	return nil
}

func FindItemBySlugAndCategory(category, slug string) (OptimizationItem, bool) {
	categoryItems, ok := itemsDatabase[category]
	if !ok {
		return OptimizationItem{}, false
	}
	for _, item := range categoryItems {
		if item.Slug == slug {
			return item, true
		}
	}
	return OptimizationItem{}, false
}

func GetItemsByCategory(category string) ([]OptimizationItem, bool) {
	items, ok := itemsDatabase[category]
	return items, ok
}
