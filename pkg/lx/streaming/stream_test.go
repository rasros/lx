package streaming

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
)

func TestStream_ContextCancellation(t *testing.T) {
	cfg := core.NewConfig()
	opts := core.RenderOptions{Head: -1}
	stream, _ := NewStream(cfg)
	stream.WithConcurrency(2)

	for i := 0; i < 50; i++ {
		stream.AddFile(sources.NewBufferInputFile("file.txt", []byte("data")), opts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf strings.Builder
	_, err := stream.Execute(ctx, &buf)
	if err != nil && err != context.Canceled {
		t.Errorf("Expected nil or Canceled, got: %v", err)
	}
}

func TestStream_Prepare_SizeUnknownForNegativeSizeFile(t *testing.T) {
	cfg := core.NewConfig()
	opts := core.RenderOptions{Head: -1}
	stream, _ := NewStream(cfg)

	stream.AddFile(sources.NewBufferInputFile("known.txt", []byte("abc")), opts)
	stream.AddFile(sources.InputFile{
		Path: "https://example.com/doc",
		Size: -1,
		Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("")), nil },
	}, opts)

	g := stream.Plan()
	if !g.SizeUnknown {
		t.Error("SizeUnknown = false, want true when a file's size is unavailable")
	}
	if g.TotalSize != 3 {
		t.Errorf("TotalSize = %d, want 3 (only the known-size file counted)", g.TotalSize)
	}
}

func TestStream_MaxSizeKeepsUnknownSizeInputs(t *testing.T) {
	cfg := core.NewConfig()
	opts := core.RenderOptions{Head: -1, MaxSize: 10}
	stream, _ := NewStream(cfg)

	// Remote inputs report a negative size until they are fetched.
	f := sources.NewBufferInputFile("remote.txt", []byte("fetched later"))
	f.Size = -1
	stream.AddFile(f, opts)

	if n := stream.Plan().TotalFiles; n != 1 {
		t.Errorf("TotalFiles = %d, want 1 (unknown size must not be skipped)", n)
	}
}

func tokenEstimateFor(t *testing.T, content string) int64 {
	t.Helper()
	opts := core.RenderOptions{Head: -1}
	stream, err := NewStream(core.NewConfig())
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}
	stream.AddFile(sources.NewBufferInputFile("sample.txt", []byte(content)), opts)

	var buf strings.Builder
	report, err := stream.Execute(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	return report.WrittenTokenEstimate
}

func TestStream_TokenEstimateReflectsContentShape(t *testing.T) {
	prose := strings.Repeat("hello world ", 60)
	symbols := strings.Repeat(`{"a":1},`, 90)
	if len(prose) != len(symbols) {
		t.Fatalf("fixture sizes differ: %d vs %d", len(prose), len(symbols))
	}

	proseTokens := tokenEstimateFor(t, prose)
	symbolTokens := tokenEstimateFor(t, symbols)

	if symbolTokens <= proseTokens {
		t.Errorf("symbol-dense estimate %d <= prose estimate %d, want greater for equal byte counts",
			symbolTokens, proseTokens)
	}

	sizeOnly := int64(len(prose)) / 4
	if proseTokens == sizeOnly && symbolTokens == sizeOnly {
		t.Error("estimates match bytes/4 exactly, want a content-derived estimate")
	}
}

func TestStream_TokenEstimateCoversAllEmittedBytes(t *testing.T) {
	opts := core.RenderOptions{Head: -1}
	stream, err := NewStream(core.NewConfig())
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}
	// One token per byte, so totals can be compared against bytes exactly.
	stream.WithTokenizer(func(size int64, _ interface{}) int64 { return size })

	stream.AddSection("First")
	stream.AddFile(sources.NewBufferInputFile("a.txt", []byte("content a")), opts)
	stream.AddPrompt("Analyze this")
	stream.AddSection("Second")
	stream.AddFile(sources.NewBufferInputFile("b.txt", []byte("content b")), opts)

	var buf strings.Builder
	global, err := stream.Execute(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if global.WrittenTokenEstimate != global.TotalWrittenBytes {
		t.Errorf("WrittenTokenEstimate = %d, TotalWrittenBytes = %d; want equal with a one-token-per-byte tokenizer",
			global.WrittenTokenEstimate, global.TotalWrittenBytes)
	}
	if global.TotalWrittenBytes != int64(buf.Len()) {
		t.Errorf("TotalWrittenBytes = %d, want %d", global.TotalWrittenBytes, buf.Len())
	}
}

func TestStream_PerFileTokenEstimateIsContentDerived(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "{{ .TokenEstimate }}"
	opts := core.RenderOptions{Head: -1}
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	content := strings.Repeat(`{"a":1},`, 60)
	stream.AddFile(sources.NewBufferInputFile("dense.json", []byte(content)), opts)

	var buf strings.Builder
	if _, err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got, err := strconv.ParseInt(strings.TrimSpace(buf.String()), 10, 64)
	if err != nil {
		t.Fatalf("parse rendered estimate %q: %v", buf.String(), err)
	}

	sizeOnly := int64(len(content)) / 4
	if got == sizeOnly {
		t.Errorf("per-file TokenEstimate = %d, which is exactly bytes/4; want a content-derived estimate", got)
	}
	if got <= sizeOnly {
		t.Errorf("per-file TokenEstimate = %d, want greater than bytes/4 (%d) for symbol-dense content", got, sizeOnly)
	}
}

// A stream renders once, and its plan follows what is added up to then.
func TestStream_ExecutesOnce(t *testing.T) {
	stream, err := NewStream(core.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	opts := core.RenderOptions{Head: -1}
	stream.AddFile(sources.NewBufferInputFile("a.txt", []byte("a")), opts)
	if n := stream.Plan().TotalFiles; n != 1 {
		t.Fatalf("planned %d files, want 1", n)
	}
	stream.AddFile(sources.NewBufferInputFile("b.txt", []byte("b")), opts)

	report, err := stream.Execute(context.Background(), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalFiles != 2 || report.TotalRows != 2 {
		t.Errorf("report %+v, want 2 files of 1 row each", report)
	}
	if _, err := stream.Execute(context.Background(), io.Discard); !errors.Is(err, ErrExecuted) {
		t.Errorf("second Execute = %v, want ErrExecuted", err)
	}
}

func TestStream_OriginalSizeIsPreConversionSize(t *testing.T) {
	cfg := core.NewConfig()
	cfg.FileContentTemplate = "{{ .OriginalSize }} {{ .Size }}"
	opts := core.RenderOptions{Head: -1, SkeletonFunctions: true}
	stream, err := NewStream(cfg)
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	src := "package main\n\nfunc Alpha() int {\n\treturn 1\n}\n\nfunc Beta() int {\n\treturn 2\n}\n"
	stream.AddFile(sources.NewBufferInputFile("main.go", []byte(src)), opts)

	var buf strings.Builder
	if _, err := stream.Execute(context.Background(), &buf); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var original, rendered int64
	if _, err := fmt.Sscanf(strings.TrimSpace(buf.String()), "%d %d", &original, &rendered); err != nil {
		t.Fatalf("parse %q: %v", buf.String(), err)
	}

	if original != int64(len(src)) {
		t.Errorf("OriginalSize = %d, want %d (size on disk)", original, len(src))
	}
	if rendered >= original {
		t.Errorf("Size = %d, want less than OriginalSize %d after the skeleton filter", rendered, original)
	}
}

// Each rendered piece is estimated as a whole, so templates that write in
// several chunks get the same estimate as ones that write once.
func TestStream_TokenEstimateIgnoresWriteChunking(t *testing.T) {
	type templates struct{ file, sectionHeader, outputHeader, outputFooter string }
	estimate := func(tmpl templates) (int64, string) {
		cfg := core.NewConfig()
		cfg.FileContentTemplate = tmpl.file
		cfg.SectionHeaderTemplate = tmpl.sectionHeader
		cfg.OutputHeaderTemplate = tmpl.outputHeader
		cfg.OutputFooterTemplate = tmpl.outputFooter
		opts := core.RenderOptions{Head: -1}
		stream, err := NewStream(cfg)
		if err != nil {
			t.Fatalf("NewStream failed: %v", err)
		}
		// One token per estimate, so the total shows how many pieces the
		// output was estimated in.
		stream.WithTokenizer(func(int64, interface{}) int64 { return 1 })
		stream.AddSection("S")
		stream.AddFile(sources.NewBufferInputFile("a.txt", []byte("content a")), opts)
		stream.AddFile(sources.NewBufferInputFile("b.txt", []byte("content b")), opts)
		var buf strings.Builder
		report, err := stream.Execute(context.Background(), &buf)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		return report.WrittenTokenEstimate, buf.String()
	}

	chunked, chunkedOut := estimate(templates{
		file:          `{{ .Path }}:{{ .Content }}:{{ .Path }}`,
		sectionHeader: `<{{ .Body }}>{{ .Index }}`,
		outputHeader:  `files={{ .Global.TotalFiles }};`,
		outputFooter:  `end={{ .Global.TotalFiles }};`,
	})
	whole, wholeOut := estimate(templates{
		file:          `{{ printf "%s:%s:%s" .Path .Content .Path }}`,
		sectionHeader: `{{ printf "<%s>%d" .Body .Index }}`,
		outputHeader:  `{{ printf "files=%d;" .Global.TotalFiles }}`,
		outputFooter:  `{{ printf "end=%d;" .Global.TotalFiles }}`,
	})
	if chunkedOut != wholeOut {
		t.Fatalf("templates render differently:\n%q\n%q", chunkedOut, wholeOut)
	}
	if chunked != whole {
		t.Errorf("chunked templates estimated %d tokens, single writes %d; want equal", chunked, whole)
	}
}

func TestStream_SkeletonFilesCapWorkers(t *testing.T) {
	stream, _ := NewStream(core.NewConfig())
	stream.WithConcurrency(8)
	stream.AddFile(sources.NewBufferInputFile("a.go", []byte("package a")), core.RenderOptions{Head: -1})
	if got := stream.workers(); got != 8 {
		t.Fatalf("workers = %d without skeletons, want 8", got)
	}
	stream.AddFile(sources.NewBufferInputFile("b.go", []byte("package b")), core.RenderOptions{Head: -1, SkeletonTypes: true})
	if got := stream.workers(); got != maxSkeletonWorkers {
		t.Fatalf("workers = %d with skeletons, want %d", got, maxSkeletonWorkers)
	}
}

// Files that cannot be opened and failures added to the stream are reported
// in output order, whichever worker rendered them first, and each is counted
// once.
func TestStream_FailuresInOutputOrder(t *testing.T) {
	stream, err := NewStream(core.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for i := range 64 {
		path := "missing-" + strconv.Itoa(i)
		want = append(want, path)
		if i%3 == 0 {
			stream.AddFailure(core.Failure{Kind: core.FailedStat, Path: path, Err: errors.New("gone")})
			continue
		}
		stream.AddFile(sources.InputFile{Path: path, Open: func(context.Context) (io.ReadCloser, error) {
			return nil, fmt.Errorf("open %s: gone", path)
		}}, core.RenderOptions{Head: -1})
	}
	var got []string
	stream.WithConcurrency(8).WithOnFailure(func(f core.Failure) {
		got = append(got, f.Path)
	})
	report, err := stream.Execute(context.Background(), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("reported %v, want %v", got, want)
	}
	if n := report.FailedFiles; n != len(want) {
		t.Errorf("FailedFiles = %d, want %d", n, len(want))
	}
}

// Converters that fail are logged in output order too.
func TestStream_ConverterFailuresInOutputOrder(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	stream, err := NewStream(core.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for i := range 64 {
		// Not a zip, so the document converter fails on it.
		f := sources.NewBufferInputFile("doc-"+strconv.Itoa(i)+".docx", []byte("not a document"))
		want = append(want, f.Path)
		stream.AddFile(f, core.RenderOptions{Head: -1, ExtractDocuments: true})
	}
	stream.WithConcurrency(8).WithLogger(log)
	if _, err := stream.Execute(context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, line := range strings.Split(logs.String(), "\n") {
		if !strings.Contains(line, `msg="Converter failed, keeping raw content"`) {
			continue
		}
		_, path, _ := strings.Cut(line, " path=")
		path, _, _ = strings.Cut(path, " ")
		got = append(got, path)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("logged %v, want %v", got, want)
	}
}
