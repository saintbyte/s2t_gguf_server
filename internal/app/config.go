package app

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config описывает настройки сервера.
type Config struct {
	Addr        string `yaml:"addr"`
	Model       string `yaml:"model"`
	Threads     int    `yaml:"threads"`
	MaxUploadMB int    `yaml:"max_upload_mb"`
}

// DefaultConfig возвращает настройки по умолчанию.
func DefaultConfig() Config {
	return Config{
		Addr:        ":8080",
		Model:       "models/gigaam-v3-e2e-rnnt-q4_k.gguf",
		Threads:     4,
		MaxUploadMB: 200,
	}
}

// configPaths — места поиска конфига по порядку: сначала системный каталог,
// затем текущая директория.
var configPaths = []string{
	"/etc/s2t_gguf_server/config.yaml",
	"config.yaml",
}

// LoadConfig находит первый существующий файл в configPaths и читает его.
// Если ни одного нет — возвращает настройки по умолчанию.
func LoadConfig() (Config, error) {
	return loadConfig(configPaths...)
}

func loadConfig(paths ...string) (Config, error) {
	cfg := DefaultConfig()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return cfg, fmt.Errorf("config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config %s: %w", path, err)
		}
		return cfg, nil
	}
	return cfg, nil
}
