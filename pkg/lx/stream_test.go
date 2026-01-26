package lx

import (
	"context"
	"strings"
	"testing"
)

func TestStream_Integration(t *testing.T) {
	cfg := NewConfig()
	stream, _ := NewStream(cfg, RunnerConfig{Head: -1, Tail: -1})

	stream.AddSection("Header Test")
	stream.AddFile(NewBufferInputFile("A.txt", []byte("Content A")))
	stream.AddPrompt("Analyze this")

	var buf strings.Builder
	err := stream.Execute(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := buf.String()

	expected := []string{
		"## Header Test",
		"Content A",
		"Analyze this",
	}

	for _, exp := range expected {
		if !strings.Contains(got, exp) {
			t.Errorf("Output missing %q", exp)
		}
	}
}

func TestStream_ContextCancellation(t *testing.T) {
	cfg := NewConfig()
	stream, _ := NewStream(cfg, RunnerConfig{Head: -1})
	stream.WithConcurrency(2)

	for i := 0; i < 50; i++ {
		stream.AddFile(NewBufferInputFile("file.txt", []byte("data")))
	}

	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	var buf strings.Builder
	err := stream.Execute(ctx, &buf)

	if err != nil && err != context.Canceled {
		t.Errorf("Expected nil or Canceled, got: %v", err)
	}
}

type MockTokenizer struct{}

func (m MockTokenizer) Estimate(size int64, _ interface{}) int64 { return 999 }

func TestStream_TokenizerIntegration(t *testing.T) {
	cfg := NewConfig()
	stream, _ := NewStream(cfg, RunnerConfig{Head: -1})
	stream.WithTokenizer(MockTokenizer{})

	stream.AddFile(NewBufferInputFile("test.txt", []byte("abc")))

	stats := stream.GetGlobalContext()

	if stats.TokenEstimate != 999 {
		t.Errorf("Expected custom token estimate 999, got %d", stats.TokenEstimate)
	}
}
