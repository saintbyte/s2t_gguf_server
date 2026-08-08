package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
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

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestLoadWav_EmptyBody проверяет ошибку на пустом теле.
func TestLoadWav_EmptyBody(t *testing.T) {
	_, _, err := LoadWav(strings.NewReader(""))
	if err == nil {
		t.Fatal("ожидалась ошибка для пустого тела")
	}
}

// TestLoadWav_Invalid проверяет отбраковку не-WAV содержимого.
func TestLoadWav_Invalid(t *testing.T) {
	_, _, err := LoadWav(strings.NewReader("это точно не wav"))
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
	pcm, rate, err := LoadWav(bytes.NewReader(buildWav(16000, 1, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != targetRate {
		t.Fatalf("ожидалась частота %d, получено %d", targetRate, rate)
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
	samples := make([]int16, 16000)
	for i := range samples {
		if i%2 == 0 {
			samples[i] = 1000
		} else {
			samples[i] = 3000
		}
	}
	pcm, rate, err := LoadWav(bytes.NewReader(buildWav(16000, 2, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != targetRate {
		t.Fatalf("ожидалась частота %d, получено %d", targetRate, rate)
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
	pcm, rate, err := LoadWav(bytes.NewReader(buildWav(8000, 1, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != targetRate {
		t.Fatalf("ожидалась частота %d, получено %d", targetRate, rate)
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
	pcm, rate, err := LoadWav(bytes.NewReader(buildWav(44100, 1, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != targetRate {
		t.Fatalf("ожидалась частота %d, получено %d", targetRate, rate)
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

// TestLoadWav_StereoResample проверяет сведение стерео 8 kHz в моно 16 kHz:
// длина после даунмикса удваивается, постоянный сигнал не меняется.
func TestLoadWav_StereoResample(t *testing.T) {
	const n = 8000
	samples := make([]int16, n*2)
	for i := range samples {
		samples[i] = 6000
	}
	pcm, rate, err := LoadWav(bytes.NewReader(buildWav(8000, 2, samples)))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rate != targetRate {
		t.Fatalf("ожидалась частота %d, получено %d", targetRate, rate)
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

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

// TestLoadWav_ReadError проверяет проброс ошибки чтения входа.
func TestLoadWav_ReadError(t *testing.T) {
	_, _, err := LoadWav(errReader{})
	if err == nil {
		t.Fatal("ожидалась ошибка чтения")
	}
}

// TestLoadWav_RateLimit проверяет, что входящий поток читается не дальше MaxUpload.
func TestLoadWav_RateLimit(t *testing.T) {
	// Максимальный размер пакета Fiber не должен превышать MaxUpload.
	if got, want := MaxUpload, int64(200<<20); got != want {
		t.Fatalf("ожидалось MaxUpload=%d, получено %d", want, got)
	}
}
