package render

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rasros/lx/pkg/lx/core"
	"github.com/rasros/lx/pkg/lx/sources"
	"github.com/rasros/lx/pkg/lx/templatex"
)

// countingBody stands in for an HTTP response body: a stream that declares its
// length and counts what is read from it.
type countingBody struct {
	r      io.Reader
	length int64
	read   int64
	closed bool
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	return n, err
}
func (c *countingBody) Close() error         { c.closed = true; return nil }
func (c *countingBody) ContentLength() int64 { return c.length }

// zeroStream yields zeros up to a limit, standing in for a body too large to
// hold.
type zeroStream struct{ left int64 }

func (z *zeroStream) Read(p []byte) (int, error) {
	if z.left <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.left {
		n = z.left
	}
	clear(p[:n])
	z.left -= n
	return int(n), nil
}

func streamInput(name string, body *countingBody) sources.InputFile {
	return sources.InputFile{
		Path:    name,
		AbsPath: name,
		Size:    -1, // as a URL is, until it is fetched
		ModTime: time.Unix(0, 0),
		Open:    func(context.Context) (io.ReadCloser, error) { return body, nil },
	}
}

func renderStream(t *testing.T, file sources.InputFile, cfg core.RenderOptions) string {
	t.Helper()
	engine, err := templatex.Compile(core.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	proc := NewProcessor(engine, core.Plan{TotalFiles: 1})
	var buf bytes.Buffer
	item := PreparedItem{Raw: FileItem{File: file, Opts: cfg}, Section: &core.SectionContext{}, FileIndexGlobal: 1}
	if _, err := proc.RenderPrepared(context.Background(), &buf, item, nil); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func mediaFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "cmd", "lx", "testdata", "media", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestProcessorReadsOnlyMediaHeadersFromStream(t *testing.T) {
	// A PNG's header, then what may as well be gigabytes of pixels. Describing
	// it takes the header alone.
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")
	png = binary.BigEndian.AppendUint32(png, 640)
	png = binary.BigEndian.AppendUint32(png, 480)
	png = append(png, 8, 2, 0, 0, 0)
	const declared = 4 << 30
	body := &countingBody{
		r:      io.MultiReader(bytes.NewReader(png), &zeroStream{left: declared - int64(len(png))}),
		length: declared,
	}

	out := renderStream(t, streamInput("https://example.com/huge.png", body),
		core.RenderOptions{Head: -1, ExtractMedia: true})
	if !strings.Contains(out, "Image: 640x480, rgb, 8-bit") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if body.read > 1<<20 {
		t.Errorf("read %d bytes of a %d-byte stream for its header", body.read, int64(declared))
	}
	if !body.closed {
		t.Error("the stream was not closed")
	}
}

func TestProcessorStopsMatroskaStreamAfterTracks(t *testing.T) {
	// The sample with its segment made open-ended, as a live stream's is, and
	// clusters following it without end.
	mkv := mediaFixture(t, "sample.mkv")
	copy(mkv[44:52], []byte{0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	const declared = 1 << 40
	body := &countingBody{
		r:      io.MultiReader(bytes.NewReader(mkv), &zeroStream{left: 256 << 20}),
		length: declared,
	}

	out := renderStream(t, streamInput("https://example.com/live.mkv", body),
		core.RenderOptions{Head: -1, ExtractMedia: true})
	for _, want := range []string{"Container: mkv", "Video: h264, 32x32", "Audio: aac, 8000 Hz, mono"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if body.read > 1<<20 {
		t.Errorf("read %d bytes of an endless stream", body.read)
	}
}

func TestProcessorFindsTrailingMoovInStream(t *testing.T) {
	// The sample's mdat, which the moov follows, grown to 64 MiB. Without a way
	// to seek, the stream is read through to reach the moov, and must still
	// describe the file as the local copy does.
	mp4 := mediaFixture(t, "sample.mp4")
	const mdatAt, moovAt = 40, 5303
	if string(mp4[mdatAt+4:mdatAt+8]) != "mdat" || string(mp4[moovAt+4:moovAt+8]) != "moov" {
		t.Fatal("fixture layout changed")
	}
	const payload = 64 << 20
	head := append([]byte{}, mp4[:mdatAt]...)
	head = binary.BigEndian.AppendUint32(head, 8+payload)
	head = append(head, "mdat"...)
	moov := mp4[moovAt:]
	total := int64(len(head)) + payload + int64(len(moov))

	body := &countingBody{
		r:      io.MultiReader(bytes.NewReader(head), &zeroStream{left: payload}, bytes.NewReader(moov)),
		length: total,
	}
	out := renderStream(t, streamInput("https://example.com/video.mp4", body),
		core.RenderOptions{Head: -1, ExtractMedia: true})
	for _, want := range []string{
		"Container: mp4",
		"Duration: 00:00:01.000",
		"Video: h264, 32x32, 10.00 fps",
		"Audio: aac, 8000 Hz, mono",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if body.read != total {
		t.Errorf("read %d of %d bytes", body.read, total)
	}
}

func TestProcessorCountsLargeStreamFromItsStart(t *testing.T) {
	// Compact mode estimates the rows of content past ExactCountLimit from its
	// start, so a large stream is not read for a header line.
	line := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde\n")
	const declared = 128 << 20
	body := &countingBody{
		r:      io.LimitReader(&repeatReader{line: line}, declared),
		length: declared,
	}
	out := renderStream(t, streamInput("https://example.com/big.log", body), core.RenderOptions{})
	if !strings.Contains(out, "~2097152 rows") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if body.read > 1<<20 {
		t.Errorf("read %d bytes of a %d-byte stream for a row estimate", body.read, int64(declared))
	}
}

type repeatReader struct {
	line []byte
	off  int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.line[r.off]
		r.off = (r.off + 1) % len(r.line)
	}
	return len(p), nil
}

func TestProcessorSurvivesLyingContentLength(t *testing.T) {
	// A server declares 1 TiB and sends 2 KB. Every way of rendering it shows
	// what arrived, and none allocates for what was declared.
	text := strings.Repeat("func f() {}\n", 170)
	cases := []struct {
		name string
		cfg  core.RenderOptions
		want string
	}{
		{"whole", core.RenderOptions{Head: -1}, text},
		{"compact", core.RenderOptions{}, "(170 rows)"},
		{"skeleton", core.RenderOptions{Head: -1, SkeletonFunctions: true}, "(170 rows, function signatures)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			body := &countingBody{r: strings.NewReader(text), length: 1 << 40}
			out := renderStream(t, streamInput("https://example.com/huge.go", body), tc.cfg)
			runtime.ReadMemStats(&after)
			if !strings.Contains(out, tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, out)
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 256<<20 {
				t.Errorf("allocated %d bytes for %d bytes of content", alloc, len(text))
			}
		})
	}
}
