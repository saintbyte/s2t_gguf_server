package app

import (
	"sync"

	whisper "github.com/CrispStrobe/CrispASR/bindings/go"
)

type segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Words []word  `json:"words,omitempty"`
}

type word struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Conf  float32 `json:"conf"`
}

type Response struct {
	Text      string    `json:"text"`
	Duration  float64   `json:"duration"`
	ElapsedMs float64   `json:"elapsed_ms"`
	Segments  []segment `json:"segments"`
}

type Server struct {
	mu      sync.Mutex
	Session Transcriber
}

// Transcriber абстрагирует сессию распознавания, чтобы её можно было
// подменить в тестах.
type Transcriber interface {
	Transcribe(pcm []float32) (*whisper.TranscribeResult, error)
}

func (s *Server) transcribe(pcm []float32) (*Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.Session.Transcribe(pcm)
	if err != nil {
		return nil, err
	}

	out := &Response{Segments: make([]segment, 0, len(res.Segments))}
	for _, seg := range res.Segments {
		out.Text += seg.Text
		segOut := segment{
			Start: float64(seg.T0) / 100,
			End:   float64(seg.T1) / 100,
			Text:  seg.Text,
		}
		for _, w := range seg.Words {
			segOut.Words = append(segOut.Words, word{
				Text:  w.Text,
				Start: float64(w.T0) / 100,
				End:   float64(w.T1) / 100,
				Conf:  w.P,
			})
		}
		out.Segments = append(out.Segments, segOut)
	}
	if n := len(res.Segments); n > 0 {
		out.Duration = float64(res.Segments[n-1].T1) / 100
	}
	return out, nil
}
