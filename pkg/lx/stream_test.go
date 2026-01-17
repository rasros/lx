package lx

import (
	"context"
	"strings"
	"testing"
)

func TestStream_FluentAPI(t *testing.T) {
	cfg := NewConfig()
	runCfg := RunnerConfig{Head: -1, Tail: -1}

	stream, err := NewStream(cfg, runCfg)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	content := []byte("Hello Library")
	file := NewBufferInputFile("test.txt", content)

	stream.AddSection("Intro").AddFile(file).AddPrompt("Final Step")

	if len(stream.items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(stream.items))
	}
}

func TestStream_Execute(t *testing.T) {
	cfg := NewConfig()
	stream, _ := NewStream(cfg, RunnerConfig{Head: -1, Tail: -1})

	stream.AddSection("Header Test")

	var buf strings.Builder
	err := stream.Execute(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "## Header Test") {
		t.Errorf("Output missing section header, got: %s", got)
	}
}
