package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"

	whisper "github.com/CrispStrobe/CrispASR/bindings/go"
	"github.com/saintbyte/s2t_gguf_server/internal/app"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		slog.Error("конфиг", "err", err)
		os.Exit(1)
	}

	addr := flag.String("addr", cfg.Addr, "адрес HTTP-сервера")
	model := flag.String("model", cfg.Model, "путь к GGUF-модели")
	threads := flag.Int("threads", cfg.Threads, "число потоков ggml")
	maxUploadMB := flag.Int("max_upload_mb", cfg.MaxUploadMB, "лимит загрузки аудио, МБ")
	logLevel := flag.String("log_level", cfg.LogLevel, "уровень логирования: debug, info, warn, error")
	flag.Parse()

	if *maxUploadMB > 0 {
		app.MaxUpload = int64(*maxUploadMB) << 20
	}

	lvl, err := app.ParseLogLevel(*logLevel)
	if err != nil {
		slog.Error("неизвестный уровень логирования", "level", *logLevel)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)

	if _, err := os.Stat(*model); err != nil {
		slog.Error("модель не найдена",
			"model", *model,
			"hint", "запустите scripts/download-model.sh")
		os.Exit(1)
	}

	// Загружаем модель при старте (auto-detect бэкенда по GGUF-метаданным).
	sess, err := whisper.SessionOpen(*model, *threads)
	if err != nil {
		slog.Error("не удалось открыть модель", "err", err)
		os.Exit(1)
	}
	defer sess.Close()
	slog.Info("модель загружена", "model", filepath.Base(*model))

	s := &app.Server{Session: sess}

	httpSrv := app.NewApp(s, "./static")

	slog.Info("слушаю", "addr", *addr)
	if err := httpSrv.Listen(*addr); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}
