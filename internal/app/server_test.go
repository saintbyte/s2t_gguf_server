package app

import (
	"errors"
	"sync"
	"testing"

	whisper "github.com/CrispStrobe/CrispASR/bindings/go"
)

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

// TestTranscribe_Segments проверяет полное преобразование результата
// распознавания в Response: склейку текста, тайминги, слова и длительность.
func TestTranscribe_Segments(t *testing.T) {
	s := &Server{Session: &fakeTranscriber{result: &whisper.TranscribeResult{
		Segments: []whisper.TranscribeSegment{
			{Text: "привет ", T0: 0, T1: 50, Words: []whisper.TranscribeWord{
				{Text: "привет", T0: 0, T1: 50, P: 0.9},
			}},
			{Text: "мир", T0: 50, T1: 100, Words: []whisper.TranscribeWord{
				{Text: "мир", T0: 50, T1: 100, P: 0.8},
			}},
		},
	}}}

	res, err := s.transcribe(nil)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if want := "привет мир"; res.Text != want {
		t.Fatalf("текст: ожидалось %q, получено %q", want, res.Text)
	}
	if res.Duration != 1.0 {
		t.Fatalf("длительность: ожидалось 1.0, получено %v", res.Duration)
	}
	if len(res.Segments) != 2 {
		t.Fatalf("сегментов: ожидалось 2, получено %d", len(res.Segments))
	}
	seg := res.Segments[0]
	if seg.Start != 0 || seg.End != 0.5 {
		t.Fatalf("сегмент 0: ожидалось [0, 0.5], получено [%v, %v]", seg.Start, seg.End)
	}
	if len(seg.Words) != 1 || seg.Words[0].Text != "привет" || seg.Words[0].Conf != 0.9 {
		t.Fatalf("неверные слова сегмента: %+v", seg.Words)
	}
}

// TestTranscribe_Empty проверяет пустой результат распознавания.
func TestTranscribe_Empty(t *testing.T) {
	s := &Server{Session: &fakeTranscriber{result: &whisper.TranscribeResult{}}}
	res, err := s.transcribe(nil)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if res.Text != "" || res.Duration != 0 {
		t.Fatalf("ожидались пустые text/duration, получено %+v", res)
	}
	if res.Segments == nil || len(res.Segments) != 0 {
		t.Fatalf("ожидался пустой (не nil) массив segments")
	}
}

// TestTranscribe_SegmentWithoutWords проверяет, что у сегмента без слов
// Words остаётся nil.
func TestTranscribe_SegmentWithoutWords(t *testing.T) {
	s := &Server{Session: &fakeTranscriber{result: &whisper.TranscribeResult{
		Segments: []whisper.TranscribeSegment{
			{Text: "привет", T0: 0, T1: 100},
		},
	}}}
	res, err := s.transcribe(nil)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if res.Segments[0].Words != nil {
		t.Fatalf("ожидались nil words, получено %+v", res.Segments[0].Words)
	}
}

// TestTranscribe_Error проверяет проброс ошибки сессии.
func TestTranscribe_Error(t *testing.T) {
	s := &Server{Session: &fakeTranscriber{err: errors.New("session exploded")}}
	if _, err := s.transcribe(nil); err == nil {
		t.Fatal("ожидалась ошибка сессии")
	}
}

// TestTranscribe_Concurrent проверяет, что вызовы transcribe сериализуются
// (мьютекс) и не падают при одновременном доступе.
func TestTranscribe_Concurrent(t *testing.T) {
	s := &Server{Session: &fakeTranscriber{result: &whisper.TranscribeResult{
		Segments: []whisper.TranscribeSegment{{Text: "x", T0: 0, T1: 50}},
	}}}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.transcribe(nil)
			if err != nil {
				t.Errorf("неожиданная ошибка: %v", err)
				return
			}
			if len(res.Segments) != 1 {
				t.Errorf("ожидался 1 сегмент, получено %d", len(res.Segments))
			}
		}()
	}
	wg.Wait()
}
