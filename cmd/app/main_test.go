package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/saintbyte/s2t_gguf_server/internal/app"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	whisper "github.com/CrispStrobe/CrispASR/bindings/go"
)

// buildWav собирает минимальный 16-битный PCM WAV из межсемпловых int16.
func buildWav(sampleRate, channels int, samples []int16) []byte {
	buf := new(bytes.Buffer)
	blockAlign := 2 * channels
	byteRate := sampleRate * blockAlign
	dataSize := len(samples) * 2

	le := binary.LittleEndian
	write := func(s string) { buf.WriteString(s) }
	put32 := func(v uint32) {
		var b [4]byte
		le.PutUint32(b[:], v)
		buf.Write(b[:])
	}
	put16 := func(v uint16) {
		var b [2]byte
		le.PutUint16(b[:], v)
		buf.Write(b[:])
	}

	write("RIFF")
	put32(uint32(36 + dataSize))
	write("WAVE")
	write("fmt ")
	put32(16)
	put16(1) // PCM
	put16(uint16(channels))
	put32(uint32(sampleRate))
	put32(uint32(byteRate))
	put16(uint16(blockAlign))
	put16(16) // bits per sample
	write("data")
	put32(uint32(dataSize))
	for _, s := range samples {
		put16(uint16(s))
	}
	return buf.Bytes()
}

// TestLoadWav_EmptyBody проверяет ошибку на пустом теле.
func TestLoadWav_EmptyBody(t *testing.T) {
	_, _, err := app.LoadWav(strings.NewReader(""))
	if err == nil {
		t.Fatal("ожидалась ошибка для пустого тела")
	}
}

// TestLoadWav_Invalid проверяет отбраковку не-WAV содержимого.
func TestLoadWav_Invalid(t *testing.T) {
	_, _, err := app.LoadWav(strings.NewReader("это точно не wav"))
	if err == nil {
		t.Fatal("ожидалась ошибка для не-WAV файла")
	}
}

// TestLoadWav_Mono16k проверяет проходной путь: моно 16 kHz без изменений.
func TestLoadWav_Mono16k(t *testing.T) {
	samples := make([]int16, 16000)
	for i := range samples {
		samples[i] = int16(i*7 - 4000)
	}
	pcm, rate, err := app.LoadWav(bytes.NewReader(buildWav(16000, 1, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("ожидалась частота 16000, получено %d", rate)
	}
	if len(pcm) != len(samples) {
		t.Fatalf("ожидалось %d сэмплов, получено %d", len(samples), len(pcm))
	}
	for _, idx := range []int{0, 100, len(samples) - 1} {
		want := float32(samples[idx]) / 32768
		if diff := absf(pcm[idx] - want); diff > 1e-6 {
			t.Fatalf("семпл %d: ожидалось %v, получено %v", idx, want, pcm[idx])
		}
	}
}

// TestLoadWav_StereoDownmix проверяет сведение стерео в моно (усреднение каналов).
func TestLoadWav_StereoDownmix(t *testing.T) {
	// Чётные семплы = 1000, нечётные = 3000, среднее 2000.
	samples := make([]int16, 16000)
	for i := range samples {
		if i%2 == 0 {
			samples[i] = 1000
		} else {
			samples[i] = 3000
		}
	}
	pcm, rate, err := app.LoadWav(bytes.NewReader(buildWav(16000, 2, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("ожидалась частота 16000, получено %d", rate)
	}
	if want, got := len(samples)/2, len(pcm); want != got {
		t.Fatalf("ожидалось %d моно-сэмплов, получено %d", want, got)
	}
	want := float32(2000) / 32768
	for i := range pcm {
		if diff := absf(pcm[i] - want); diff > 1e-6 {
			t.Fatalf("семпл %d: ожидалось %v, получено %v", i, want, pcm[i])
		}
	}
}

// TestLoadWav_Resample8k проверяет передискретизацию 8 kHz -> 16 kHz
// (длина удваивается, постоянный сигнал не меняется).
func TestLoadWav_Resample8k(t *testing.T) {
	const n = 8000
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = 10000
	}
	pcm, rate, err := app.LoadWav(bytes.NewReader(buildWav(8000, 1, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("ожидалась частота 16000, получено %d", rate)
	}
	if want, got := n*2, len(pcm); want != got {
		t.Fatalf("ожидалось %d сэмплов, получено %d", want, got)
	}
	want := float32(10000) / 32768
	for i := range pcm {
		if diff := absf(pcm[i] - want); diff > 1e-4 {
			t.Fatalf("семпл %d: ожидалось %v, получено %v", i, want, pcm[i])
		}
	}
}

// TestLoadWav_Resample44100 проверяет передискретизацию 44.1 kHz -> 16 kHz.
func TestLoadWav_Resample44100(t *testing.T) {
	const n = 44100
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = -8000
	}
	pcm, rate, err := app.LoadWav(bytes.NewReader(buildWav(44100, 1, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("ожидалась частота 16000, получено %d", rate)
	}
	if want, got := 16000, len(pcm); want != got {
		t.Fatalf("ожидалось %d сэмплов, получено %d", want, got)
	}
	want := float32(-8000) / 32768
	for i := range pcm {
		if diff := absf(pcm[i] - want); diff > 1e-4 {
			t.Fatalf("семпл %d: ожидалось %v, получено %v", i, want, pcm[i])
		}
	}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

// TestLoadWav_ReadError проверяет проброс ошибки чтения входа.
func TestLoadWav_ReadError(t *testing.T) {
	_, _, err := app.LoadWav(errReader{})
	if err == nil {
		t.Fatal("ожидалась ошибка чтения")
	}
}

// TestLoadWav_StereoResample проверяет сведение стерео 8 kHz в моно 16 kHz:
// длина после даунмикса удваивается, постоянный сигнал не меняется.
func TestLoadWav_StereoResample(t *testing.T) {
	const n = 8000
	samples := make([]int16, n*2)
	for i := range samples {
		samples[i] = 6000
	}
	pcm, rate, err := app.LoadWav(bytes.NewReader(buildWav(8000, 2, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("ожидалась частота 16000, получено %d", rate)
	}
	if want, got := n*2, len(pcm); want != got {
		t.Fatalf("ожидалось %d сэмплов, получено %d", want, got)
	}
	want := float32(6000) / 32768
	for i := range pcm {
		if diff := absf(pcm[i] - want); diff > 1e-4 {
			t.Fatalf("семпл %d: ожидалось %v, получено %v", i, want, pcm[i])
		}
	}
}

// fakeTranscriber подменяет сессию распознавания.
type fakeTranscriber struct {
	result *whisper.TranscribeResult
	err    error
}

func (f *fakeTranscriber) Transcribe(pcm []float32) (*whisper.TranscribeResult, error) {
	return f.result, f.err
}

type errSentinel struct{}

func (errSentinel) Error() string { return "session exploded" }

var errFail error = errSentinel{}

// doRequest выполняет запрос через Fiber-приложение с подменённой сессией.
func doRequest(t *testing.T, res *whisper.TranscribeResult, tErr error, req *http.Request) *http.Response {
	t.Helper()
	s := &app.Server{Session: &fakeTranscriber{result: res, err: tErr}}
	app := app.NewApp(s, staticDir())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("запрос не выполнен: %v", err)
	}
	return resp
}

// staticDir возвращает путь к каталогу со статикой проекта (корень модуля/static).
func staticDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "static")
}

// readBody читает и закрывает тело ответа.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("не удалось прочитать тело: %v", err)
	}
	return buf.Bytes()
}

// multipartRequest собирает multipart/form-data запрос с полем file.
func multipartRequest(t *testing.T, file []byte, filename string) *http.Request {
	t.Helper()
	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(file); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// TestHandler_OK проверяет полный путь: WAV -> транскрибация -> JSON.
func TestHandler_OK(t *testing.T) {
	samples := make([]int16, 16000)
	for i := range samples {
		samples[i] = int16(i)
	}
	wav := buildWav(16000, 1, samples)

	res := &whisper.TranscribeResult{
		Segments: []whisper.TranscribeSegment{
			{Text: "привет ", T0: 0, T1: 50, Words: []whisper.TranscribeWord{
				{Text: "привет", T0: 0, T1: 50, P: 0.9},
			}},
			{Text: "мир", T0: 50, T1: 100, Words: []whisper.TranscribeWord{
				{Text: "мир", T0: 50, T1: 100, P: 0.8},
			}},
		},
	}

	resp := doRequest(t, res, nil, multipartRequest(t, wav, "speech.wav"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался 200, получено %d: %s", resp.StatusCode, readBody(t, resp))
	}

	var got app.Response
	if err := json.Unmarshal(readBody(t, resp), &got); err != nil {
		t.Fatalf("не-JSON ответ: %v", err)
	}
	if want := "привет мир"; got.Text != want {
		t.Fatalf("текст: ожидалось %q, получено %q", want, got.Text)
	}
	if got.Duration != 1.0 {
		t.Fatalf("длительность: ожидалось 1.0, получено %v", got.Duration)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("сегментов: ожидалось 2, получено %d", len(got.Segments))
	}
	seg := got.Segments[0]
	if seg.Start != 0 || seg.End != 0.5 {
		t.Fatalf("сегмент 0: ожидалось [0, 0.5], получено [%v, %v]", seg.Start, seg.End)
	}
	if len(seg.Words) != 1 || seg.Words[0].Text != "привет" || seg.Words[0].Conf != 0.9 {
		t.Fatalf("неверные слова сегмента: %+v", seg.Words)
	}
}

// TestHandler_RawBody проверяет отправку сырого WAV в теле запроса (без multipart).
func TestHandler_RawBody(t *testing.T) {
	samples := make([]int16, 16000)
	for i := range samples {
		samples[i] = int16(i)
	}
	wav := buildWav(16000, 1, samples)

	res := &whisper.TranscribeResult{
		Segments: []whisper.TranscribeSegment{
			{Text: "привет ", T0: 0, T1: 100},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", bytes.NewReader(wav))
	req.Header.Set("Content-Type", "audio/wav")

	resp := doRequest(t, res, nil, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался 200, получено %d: %s", resp.StatusCode, readBody(t, resp))
	}

	body := readBody(t, resp)
	var got app.Response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("не-JSON ответ: %v", err)
	}
	if want := "привет "; got.Text != want {
		t.Fatalf("текст: ожидалось %q, получено %q", want, got.Text)
	}
	if got.Duration != 1.0 {
		t.Fatalf("длительность: ожидалось 1.0, получено %v", got.Duration)
	}
	if got.Segments[0].Words != nil {
		t.Fatalf("ожидались nil words у сегмента без слов, получено %+v", got.Segments[0].Words)
	}
	if strings.Contains(string(body), `"words"`) {
		t.Fatalf("в JSON не должно быть ключа words: %s", body)
	}
}

// TestHandler_BadWav проверяет 400 на не-WAV файле.
func TestHandler_BadWav(t *testing.T) {
	resp := doRequest(t, nil, nil, multipartRequest(t, []byte("not a wav"), "bad.wav"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался 400, получено %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(string(body), "не похоже на WAV") {
		t.Fatalf("ожидалось сообщение об ошибке WAV, получено: %s", body)
	}
}

// TestHandler_MissingFile проверяет 400 при отсутствии поля file.
func TestHandler_MissingFile(t *testing.T) {
	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	if err := w.WriteField("other", "x"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp := doRequest(t, nil, nil, req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался 400, получено %d", resp.StatusCode)
	}
	if !strings.Contains(string(readBody(t, resp)), "'file'") {
		t.Fatalf("ожидалось сообщение про поле 'file'")
	}
}

// TestHandler_SessionError проверяет 500 при ошибке распознавания.
func TestHandler_SessionError(t *testing.T) {
	samples := make([]int16, 16000)
	wav := buildWav(16000, 1, samples)

	resp := doRequest(t, nil, errFail, multipartRequest(t, wav, "speech.wav"))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("ожидался 500, получено %d", resp.StatusCode)
	}
	if !strings.Contains(string(readBody(t, resp)), "ошибка транскрибации") {
		t.Fatalf("ожидалось сообщение об ошибке транскрибации")
	}
}

// TestHandler_EmptyAudio проверяет 400 на WAV без сэмплов.
func TestHandler_EmptyAudio(t *testing.T) {
	wav := buildWav(16000, 1, nil)
	resp := doRequest(t, nil, nil, multipartRequest(t, wav, "empty.wav"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался 400, получено %d", resp.StatusCode)
	}
	if !strings.Contains(string(readBody(t, resp)), "нет сэмплов") {
		t.Fatalf("ожидалось сообщение про пустое аудио")
	}
}

// TestHandler_EmptyResult проверяет пустой результат распознавания:
// 200, text="" и непустой массив segments в JSON.
func TestHandler_EmptyResult(t *testing.T) {
	samples := make([]int16, 16000)
	wav := buildWav(16000, 1, samples)

	resp := doRequest(t, &whisper.TranscribeResult{}, nil, multipartRequest(t, wav, "speech.wav"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался 200, получено %d: %s", resp.StatusCode, readBody(t, resp))
	}

	body := readBody(t, resp)
	var got app.Response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("не-JSON ответ: %v", err)
	}
	if got.Text != "" || got.Duration != 0 {
		t.Fatalf("ожидались пустые text/duration, получено %+v", got)
	}
	if got.Segments == nil {
		t.Fatalf("ожидался пустой (не nil) массив segments")
	}
	if !strings.Contains(string(body), `"segments":[]`) {
		t.Fatalf("ожидался 'segments':[] в JSON: %s", body)
	}
}

// TestHealth проверяет GET /health.
func TestHealth(t *testing.T) {
	resp := doRequest(t, nil, nil, httptest.NewRequest(http.MethodGet, "/health", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался 200, получено %d", resp.StatusCode)
	}
	if got := string(readBody(t, resp)); got != "ok" {
		t.Fatalf("ожидалось 'ok', получено %q", got)
	}
}

// TestIndex проверяет, что GET / отдаёт веб-интерфейс.
func TestIndex(t *testing.T) {
	resp := doRequest(t, nil, nil, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался 200, получено %d", resp.StatusCode)
	}
	if !strings.Contains(string(readBody(t, resp)), "Транскрибация") {
		t.Fatalf("на главной странице нет заголовка")
	}
}
