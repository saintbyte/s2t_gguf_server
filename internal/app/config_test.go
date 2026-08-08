package app

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadConfig_Defaults проверяет дефолты, когда ни одного конфига нет.
func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := loadConfig("/nonexistent/a.yaml", "/nonexistent/b.yaml")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.Addr != ":8080" || cfg.Model != "models/gigaam-v3-e2e-rnnt-q4_k.gguf" ||
		cfg.Threads != 4 || cfg.MaxUploadMB != 200 {
		t.Fatalf("ожидались значения по умолчанию, получено %+v", cfg)
	}
}

// TestLoadConfig_FromFile проверяет применение значений из YAML.
func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.yaml",
		"addr: :9999\nmodel: /some/model.gguf\nthreads: 8\nmax_upload_mb: 500\n")
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.Addr != ":9999" || cfg.Model != "/some/model.gguf" ||
		cfg.Threads != 8 || cfg.MaxUploadMB != 500 {
		t.Fatalf("конфиг не применился: %+v", cfg)
	}
}

// TestLoadConfig_FirstWins проверяет приоритет первого найденного файла
// (вслед за поиском в /etc/s2t_gguf_server, затем в текущей директории).
func TestLoadConfig_FirstWins(t *testing.T) {
	dir := t.TempDir()
	first := writeConfig(t, dir, "first.yaml", "addr: :1000\n")
	second := writeConfig(t, dir, "second.yaml", "addr: :2000\nthreads: 8\n")
	cfg, err := loadConfig(first, second)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cfg.Addr != ":1000" {
		t.Fatalf("ожидался первый файл, получено %+v", cfg)
	}
	if cfg.Threads != 4 {
		t.Fatalf("неуказанные поля должны браться из дефолтов, получено %+v", cfg)
	}
}

// TestLoadConfig_Invalid проверяет ошибку на битом YAML.
func TestLoadConfig_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "config.yaml", "addr: [не: корректно\n")
	if _, err := loadConfig(path); err == nil {
		t.Fatal("ожидалась ошибка для битого YAML")
	}
}
