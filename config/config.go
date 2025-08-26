// twloader-tool/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Data struct {
	CustomBasePath string `json:"customBasePath"`
}

var (
	cfg        Data
	configPath string
	mutex      = &sync.RWMutex{}
)

func Load() error {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("找不到使用者設定目錄: %w", err)
	}
	configDir := filepath.Join(userConfigDir, "TWLoaderWeb")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("無法建立設定目錄: %w", err)
	}
	configPath = filepath.Join(configDir, "config.json")

	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = Data{}
			return nil
		}
		return fmt.Errorf("無法開啟設定檔: %w", err)
	}
	defer file.Close()

	mutex.Lock()
	defer mutex.Unlock()
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		cfg = Data{} // Reset on parsing error
	}
	return nil
}

func Save(data Data) error {
	mutex.Lock()
	defer mutex.Unlock()

	cfg = data // Update global state

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("無法建立設定檔: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("無法寫入設定檔: %w", err)
	}
	return nil
}

func Get() Data {
	mutex.RLock()
	defer mutex.RUnlock()
	return cfg
}
