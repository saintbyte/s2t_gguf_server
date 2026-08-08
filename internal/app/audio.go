package app

import (
	"bytes"
	"fmt"
	"io"

	wav "github.com/go-audio/wav"
)

const targetRate = 16000

// MaxUpload — лимит размера загружаемого аудио (можно переопределить флагом
// -max_upload_mb или конфигом).
var MaxUpload = int64(200 << 20) // 200 MB

// LoadWav reads a WAV file, downmixes to mono and resamples to 16 kHz float32 PCM.
func LoadWav(r io.Reader) ([]float32, int, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxUpload))
	if err != nil {
		return nil, 0, err
	}
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("пустое тело запроса")
	}

	d := wav.NewDecoder(bytes.NewReader(data))
	if !d.IsValidFile() {
		return nil, 0, fmt.Errorf("не похоже на WAV-файл")
	}
	buf, err := d.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("не удалось прочитать WAV: %w", err)
	}

	in := buf.AsFloat32Buffer().Data
	inRate := buf.Format.SampleRate
	inCh := buf.Format.NumChannels

	mono := in
	if inCh > 1 {
		mono = make([]float32, len(in)/inCh)
		for i := range mono {
			var acc float32
			for c := 0; c < inCh; c++ {
				acc += in[i*inCh+c]
			}
			mono[i] = acc / float32(inCh)
		}
	}

	if inRate == targetRate {
		return mono, targetRate, nil
	}
	if inRate == 0 {
		return nil, 0, fmt.Errorf("неизвестная частота дискретизации")
	}

	// Линейная интерполяция до 16 kHz.
	outLen := int(float64(len(mono)) * float64(targetRate) / float64(inRate))
	out := make([]float32, outLen)
	ratio := float64(inRate) / float64(targetRate)
	for i := 0; i < outLen; i++ {
		pos := float64(i) * ratio
		idx := int(pos)
		if idx+1 >= len(mono) {
			out[i] = mono[len(mono)-1]
			continue
		}
		frac := float32(pos - float64(idx))
		out[i] = mono[idx]*(1-frac) + mono[idx+1]*frac
	}
	return out, targetRate, nil
}
