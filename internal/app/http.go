package app

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) handleTranscribe(c *fiber.Ctx) error {
	start := time.Now()
	var body io.Reader

	if strings.HasPrefix(c.Get("Content-Type"), "multipart/form-data") {
		fh, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "поле 'file' не найдено: " + err.Error(),
			})
		}
		f, err := fh.Open()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "не удалось открыть файл: " + err.Error(),
			})
		}
		defer f.Close()
		body = f
	} else {
		body = bytes.NewReader(c.Body())
	}

	pcm, inRate, err := LoadWav(body)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if len(pcm) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "в аудио нет сэмплов"})
	}

	slog.Info("транскрибация",
		"samples", len(pcm),
		"rate", inRate,
		"duration_s", float64(len(pcm))/float64(inRate))

	res, err := s.transcribe(pcm)
	if err != nil {
		slog.Error("ошибка транскрибации",
			"err", err,
			"elapsed_ms", elapsedMs(start))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "ошибка транскрибации: " + err.Error(),
		})
	}

	res.ElapsedMs = elapsedMs(start)
	slog.Info("транскрибация завершена",
		"elapsed_ms", res.ElapsedMs,
		"text_len", len(res.Text),
		"segments", len(res.Segments))

	return c.JSON(res)
}

// elapsedMs возвращает время с момента start в миллисекундах.
func elapsedMs(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}

// requestLogger логирует каждый запрос с временем выполнения.
func requestLogger(c *fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	slog.Info("http",
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"elapsed_ms", elapsedMs(start))
	return err
}

func NewApp(s *Server, staticDir string) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit:             int(MaxUpload),
		DisableStartupMessage: true,
	})
	app.Use(requestLogger)
	app.Post("/api/transcribe", s.handleTranscribe)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Static("/", staticDir)
	return app
}
