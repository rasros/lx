package streaming

import (
	"context"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
)

func TestStream_Integration(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1, Tail: -1})

	stream.AddSection("Header Test")
	stream.AddFile(sources.NewBufferInputFile("A.txt", []byte("Content A")))
	stream.AddPrompt("Analyze this")

	var buf strings.Builder
	err := stream.Execute(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := buf.String()
	expected := []string{"## Header Test", "Content A", "Analyze this"}
	for _, exp := range expected {
		if !strings.Contains(got, exp) {
			t.Errorf("Output missing %q", exp)
		}
	}
}

func TestStream_ContextCancellation(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1})
	stream.WithConcurrency(2)

	for i := 0; i < 50; i++ {
		stream.AddFile(sources.NewBufferInputFile("file.txt", []byte("data")))
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
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1})
	stream.WithTokenizer(MockTokenizer{})

	stream.AddFile(sources.NewBufferInputFile("test.txt", []byte("abc")))

	stats := stream.GetGlobalContext()
	if stats.TokenEstimate != 999 {
		t.Errorf("Expected custom token estimate 999, got %d", stats.TokenEstimate)
	}
}

func TestStream_Prepare_Counts(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1})

	stream.AddSection("S1")
	stream.AddFile(sources.NewBufferInputFile("a.txt", []byte("aaa")))
	stream.AddFile(sources.NewBufferInputFile("b.txt", []byte("bb")))
	stream.AddSection("S2")
	stream.AddFile(sources.NewBufferInputFile("c.txt", []byte("c")))

	g := stream.GetGlobalContext()
	if g.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", g.TotalFiles)
	}
	if g.TotalSections != 2 {
		t.Errorf("TotalSections = %d, want 2", g.TotalSections)
	}
	if g.TotalSize != 6 {
		t.Errorf("TotalSize = %d, want 6", g.TotalSize)
	}
}

func TestStream_TotalRows_AfterExecute(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1, Tail: -1})

	stream.AddFile(sources.NewBufferInputFile("a.txt", []byte("one\ntwo\nthree")))
	stream.AddFile(sources.NewBufferInputFile("b.txt", []byte("alpha\nbeta")))

	var buf strings.Builder
	if err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	g := stream.GetGlobalContext()
	if g.TotalRows != 5 {
		t.Errorf("TotalRows = %d, want 5", g.TotalRows)
	}
}

func TestStream_MaxSizeSkipsOversizedFiles(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1, MaxSize: 10})

	stream.AddFile(sources.NewBufferInputFile("small.txt", []byte("tiny")))
	stream.AddFile(sources.NewBufferInputFile("large.txt", []byte(strings.Repeat("x", 64))))

	var buf strings.Builder
	if err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "small.txt") {
		t.Error("Output missing small.txt, want it kept under the limit")
	}
	if strings.Contains(got, "large.txt") {
		t.Error("Output contains large.txt, want it skipped by MaxSize")
	}
	if n := stream.GetGlobalContext().TotalFiles; n != 1 {
		t.Errorf("TotalFiles = %d, want 1", n)
	}
}

func TestStream_MaxSizeKeepsUnknownSizeInputs(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1, MaxSize: 10})

	// Remote inputs report a negative size until they are fetched.
	f := sources.NewBufferInputFile("remote.txt", []byte("fetched later"))
	f.Size = -1
	stream.AddFile(f)

	if n := stream.GetGlobalContext().TotalFiles; n != 1 {
		t.Errorf("TotalFiles = %d, want 1 (unknown size must not be skipped)", n)
	}
}

func TestStream_MaxSizeUnsetKeepsEverything(t *testing.T) {
	cfg := core.NewConfig()
	stream, _ := NewStream(cfg, core.RunnerConfig{Head: -1})

	stream.AddFile(sources.NewBufferInputFile("large.txt", []byte(strings.Repeat("x", 4096))))

	if n := stream.GetGlobalContext().TotalFiles; n != 1 {
		t.Errorf("TotalFiles = %d, want 1", n)
	}
}
