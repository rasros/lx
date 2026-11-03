package internal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// countingReader serves data and counts what it hands out, so a test can tell
// how much of a stream was read.
type countingReader struct {
	r      io.Reader
	n      int64
	closed bool
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.closed {
		return 0, errors.New("read after close")
	}
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func (c *countingReader) Close() error { c.closed = true; return nil }

// pattern is a stream whose byte at offset i is byte(i % 251), so any slice of
// it can be checked without keeping the whole.
type pattern struct{ off, limit int64 }

func (p *pattern) Read(b []byte) (int, error) {
	if p.limit >= 0 && p.off >= p.limit {
		return 0, io.EOF
	}
	if p.limit >= 0 && int64(len(b)) > p.limit-p.off {
		b = b[:p.limit-p.off]
	}
	for i := range b {
		b[i] = byte((p.off + int64(i)) % 251)
	}
	p.off += int64(len(b))
	return len(b), nil
}

func checkPattern(t *testing.T, got []byte, off int64) {
	t.Helper()
	for i, c := range got {
		if want := byte((off + int64(i)) % 251); c != want {
			t.Fatalf("byte %d = %d, want %d", off+int64(i), c, want)
		}
	}
}

func TestLazyReaderRandomAccess(t *testing.T) {
	src := &countingReader{r: &pattern{limit: 1 << 20}}
	r := NewLazyReader(src, -1)

	buf := make([]byte, 100)
	if n, err := r.ReadAt(buf, 5000); n != 100 || err != nil {
		t.Fatalf("ReadAt beyond buffer = %d, %v", n, err)
	}
	checkPattern(t, buf, 5000)
	read := src.n
	if read >= 1<<20 {
		t.Fatalf("read the whole stream (%d) for one small read", read)
	}

	// Below what is buffered: served from memory, the stream untouched.
	if n, err := r.ReadAt(buf, 10); n != 100 || err != nil {
		t.Fatalf("ReadAt below buffer = %d, %v", n, err)
	}
	checkPattern(t, buf, 10)
	if src.n != read {
		t.Errorf("a buffered read took %d more bytes from the stream", src.n-read)
	}

	// Further on: read on demand.
	if n, err := r.ReadAt(buf, 300_000); n != 100 || err != nil {
		t.Fatalf("ReadAt further on = %d, %v", n, err)
	}
	checkPattern(t, buf, 300_000)
}

func TestLazyReaderEOFAndSize(t *testing.T) {
	r := NewLazyReader(strings.NewReader("hello world"), -1)

	buf := make([]byte, 8)
	n, err := r.ReadAt(buf, 6)
	if n != 5 || err != io.EOF || string(buf[:n]) != "world" {
		t.Fatalf("ReadAt across the end = %d %q, %v", n, buf[:n], err)
	}
	if n, err := r.ReadAt(buf, 11); n != 0 || err != io.EOF {
		t.Fatalf("ReadAt at the end = %d, %v", n, err)
	}
	if got := r.Size(); got != 11 {
		t.Errorf("Size = %d, want 11", got)
	}
}

func TestLazyReaderUndeclaredSizeReadsToTheEnd(t *testing.T) {
	src := &countingReader{r: &pattern{limit: 300_000}}
	r := NewLazyReader(src, -1)
	if got := r.Size(); got != 300_000 {
		t.Fatalf("Size = %d, want 300000", got)
	}
	if src.n != 300_000 {
		t.Errorf("read %d bytes, want the whole stream", src.n)
	}
	all := make([]byte, 300_000)
	if n, err := r.ReadAt(all, 0); n != len(all) || err != nil {
		t.Fatalf("ReadAt all = %d, %v", n, err)
	}
	checkPattern(t, all, 0)
}

func TestLazyReaderDeclaredSize(t *testing.T) {
	// The declared size is reported without reading, and bounds the stream:
	// whatever follows it is never read.
	src := &countingReader{r: &pattern{limit: -1}}
	r := NewLazyReader(src, 1000)
	if got := r.Size(); got != 1000 {
		t.Fatalf("Size = %d, want 1000", got)
	}
	if src.n != 0 {
		t.Errorf("Size read %d bytes of a declared stream", src.n)
	}
	buf := make([]byte, 100)
	if n, err := r.ReadAt(buf, 950); n != 50 || err != io.EOF {
		t.Fatalf("ReadAt across the declared end = %d, %v", n, err)
	}
	if src.n > 1000 {
		t.Errorf("read %d bytes, past the declared 1000", src.n)
	}
}

func TestLazyReaderStreamShorterThanDeclared(t *testing.T) {
	// A declared length is a claim. A stream that ends sooner cleanly shrinks
	// to what arrived.
	r := NewLazyReader(&failingReader{data: []byte("short"), err: io.EOF}, 100)
	buf := make([]byte, 10)
	n, err := r.ReadAt(buf, 0)
	if n != 5 || err != io.EOF {
		t.Fatalf("ReadAt = %d, %v; want 5, EOF", n, err)
	}
	if n, err := r.ReadAt(buf[:5], 0); n != 5 || err != nil || string(buf[:5]) != "short" {
		t.Fatalf("ReadAt of what arrived = %d %q, %v", n, buf[:5], err)
	}
	if got := r.Size(); got != 5 {
		t.Errorf("Size = %d after the stream ended, want 5", got)
	}
}

func TestLazyReaderStreamCutOff(t *testing.T) {
	// An HTTP body shorter than its Content-Length ends with ErrUnexpectedEOF:
	// a read past what arrived fails, rather than passing for the end.
	r := NewLazyReader(&failingReader{data: []byte("short"), err: io.ErrUnexpectedEOF}, 100)
	buf := make([]byte, 10)
	if n, err := r.ReadAt(buf, 0); n != 5 || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadAt = %d, %v; want 5, ErrUnexpectedEOF", n, err)
	}
}

func TestLazyReaderLyingLengthAllocatesWhatArrives(t *testing.T) {
	// 1 TiB declared, 2 KB sent. Neither a read across the whole declared
	// length nor reading it all may allocate on the claim's strength.
	data := bytes.Repeat([]byte("x"), 2048)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	r := NewLazyReader(bytes.NewReader(data), 1<<40)
	got, err := ReadAllAt(r, r.Size())
	if err != io.EOF || !bytes.Equal(got, data) {
		t.Fatalf("ReadAllAt = %d bytes, %v; want the 2048 that arrived", len(got), err)
	}
	tail := make([]byte, 16)
	if n, err := r.ReadAt(tail, 1<<40-16); n != 0 || err != io.EOF {
		t.Errorf("ReadAt at the declared end = %d, %v", n, err)
	}
	lines := NewLineSource(NewLazyReader(bytes.NewReader(data), 1<<40), 1<<40)
	if all, err := lines.All(); err != nil || len(all) != len(data) {
		t.Errorf("LineSource.All = %d bytes, %v", len(all), err)
	}

	runtime.ReadMemStats(&after)
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 3*maxPrealloc {
		t.Errorf("allocated %d bytes for 2 KB of content", alloc)
	}
}

type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func TestLazyReaderStreamError(t *testing.T) {
	boom := errors.New("connection reset")
	r := NewLazyReader(&failingReader{data: []byte("0123456789"), err: boom}, -1)

	buf := make([]byte, 4)
	if n, err := r.ReadAt(buf, 2); n != 4 || err != nil || string(buf) != "2345" {
		t.Fatalf("ReadAt before the failure = %d %q, %v", n, buf, err)
	}
	if n, err := r.ReadAt(buf, 8); n != 2 || !errors.Is(err, boom) {
		t.Fatalf("ReadAt across the failure = %d, %v; want 2, %v", n, err, boom)
	}
	if n, err := r.ReadAt(buf, 20); n != 0 || !errors.Is(err, boom) {
		t.Fatalf("ReadAt past the failure = %d, %v", n, err)
	}
	// Bytes before the failure are still served.
	if n, err := r.ReadAt(buf, 0); n != 4 || err != nil || string(buf) != "0123" {
		t.Fatalf("ReadAt after the failure = %d %q, %v", n, buf, err)
	}
}

func TestLazyReaderCloseStopsReading(t *testing.T) {
	src := &countingReader{r: &pattern{limit: -1}}
	r := NewLazyReader(src, -1)

	buf := make([]byte, 10)
	if _, err := r.ReadAt(buf, 0); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if !src.closed {
		t.Error("Close did not close the stream")
	}
	read := src.n

	// Buffered bytes stay readable; nothing more is read.
	if n, err := r.ReadAt(buf, 5); n != 10 || err != nil {
		t.Errorf("ReadAt of buffered bytes after Close = %d, %v", n, err)
	}
	if n, err := r.ReadAt(buf, 1<<20); n != 0 || err == nil {
		t.Errorf("ReadAt past the buffer after Close = %d, %v; want an error", n, err)
	}
	if src.n != read {
		t.Errorf("read %d bytes after Close", src.n-read)
	}
}

func TestLazyReaderConcurrentReadAt(t *testing.T) {
	const size = 2 << 20
	r := NewLazyReader(&pattern{limit: size}, -1)

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			buf := make([]byte, 4096)
			for i := 0; i < 64; i++ {
				off := int64((g*7919 + i*104729) % (size - len(buf)))
				n, err := r.ReadAt(buf, off)
				if n != len(buf) || err != nil {
					errs <- err
					return
				}
				for j, c := range buf {
					if c != byte((off+int64(j))%251) {
						errs <- errors.New("wrong byte")
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestLazyReaderSkipsGapsWithBoundedMemory(t *testing.T) {
	const size = 64 << 20
	src := &countingReader{r: &pattern{limit: size}}
	r := NewLazyReader(src, size)
	done := r.skipGaps()

	buf := make([]byte, 16)
	for _, off := range []int64{0, 100, 10 << 20, 10<<20 + 64, 40 << 20, size - 16} {
		if n, err := r.ReadAt(buf, off); n != len(buf) || err != nil {
			t.Fatalf("ReadAt(%d) = %d, %v", off, n, err)
		}
		checkPattern(t, buf, off)
	}
	if held := len(r.prefix) + len(r.win); held > lazyPrefixCap+lazyWindowCap {
		t.Errorf("holding %d bytes after skipping through %d", held, size)
	}

	// The header stays; a skipped gap is gone.
	if _, err := r.ReadAt(buf, 50); err != nil {
		t.Errorf("ReadAt in the prefix after skipping: %v", err)
	}
	if _, err := r.ReadAt(buf, 20<<20); !errors.Is(err, ErrLazyDiscarded) {
		t.Errorf("ReadAt in a skipped gap = %v, want ErrLazyDiscarded", err)
	}
	done()

	// A sequential walk past the window keeps only the window.
	r2 := NewLazyReader(&pattern{limit: size}, size)
	defer r2.skipGaps()()
	chunk := make([]byte, mediaReadAhead)
	for off := int64(0); off < 32<<20; off += int64(len(chunk)) {
		if n, err := r2.ReadAt(chunk, off); n != len(chunk) || err != nil {
			t.Fatalf("sequential ReadAt(%d) = %d, %v", off, n, err)
		}
	}
	checkPattern(t, chunk, 32<<20-int64(len(chunk)))
	if held := len(r2.prefix) + len(r2.win); held > lazyPrefixCap+lazyWindowCap+mediaReadAhead {
		t.Errorf("holding %d bytes after a sequential walk", held)
	}
}

func TestLazyReaderReadsWholeWithoutSkipping(t *testing.T) {
	// Without skipGaps every byte is kept, so a consumer that needs the whole
	// can read it back in any order.
	const size = 8 << 20
	r := NewLazyReader(&pattern{limit: size}, size)
	buf := make([]byte, 16)
	if _, err := r.ReadAt(buf, size-16); err != nil {
		t.Fatal(err)
	}
	all := make([]byte, size)
	if n, err := r.ReadAt(all, 0); n != size || err != nil {
		t.Fatalf("ReadAt all = %d, %v", n, err)
	}
	checkPattern(t, all, 0)
}

// --- Media through a stream ---

func TestMediaMetadataMP4TrailingMoovFromStream(t *testing.T) {
	// The moov box follows a large mdat, as most encoders write it. Read as a
	// stream it is still found, and the mdat is skipped rather than held.
	const mdatSize = 64 << 20
	full := mp4File(
		mvhd(1000, 2000),
		trak(tkhd(1920, 1080), hdlr("vide"), mdhd(1000, 2000),
			stbl(videoSampleEntry("avc1", 1920, 1080), stts(60, 33))),
	)
	ftyp := box("ftyp", []byte("isom"), zeros(4))
	moov := full[len(ftyp):]
	head := append(ftyp, be32(8+mdatSize)...)
	head = append(head, "mdat"...)
	total := int64(len(head)) + mdatSize + int64(len(moov))

	src := &countingReader{r: io.MultiReader(bytes.NewReader(head), io.LimitReader(&pattern{limit: -1}, mdatSize), bytes.NewReader(moov))}
	r := NewLazyReader(src, total)

	out, ok := MediaMetadata("mp4", r, r.Size())
	if !ok {
		t.Fatal("not recognised as media")
	}
	if !strings.Contains(string(out), "Video: h264, 1920x1080, 30.00 fps") ||
		!strings.Contains(string(out), "Duration: 00:00:02.000") {
		t.Errorf("unexpected metadata:\n%s", out)
	}
	if held := len(r.prefix) + len(r.win); held > lazyPrefixCap+lazyWindowCap {
		t.Errorf("held %d bytes of a %d-byte stream", held, total)
	}
}

// endlessClusters is a live Matroska stream after its headers: cluster after
// cluster, as good as never ending. It does end at 256 MiB, so that a walk that
// fails to stop fails the test rather than hanging it.
type endlessClusters struct {
	buf  []byte
	sent int64
}

func (e *endlessClusters) Read(p []byte) (int, error) {
	if e.sent >= 256<<20 {
		return 0, io.EOF
	}
	if len(e.buf) == 0 {
		e.buf = append(ebmlID(0x1f43b675), 0x01, 0, 0, 0, 0, 0x01, 0, 0) // 64 KiB payload
		e.buf = append(e.buf, zeros(64<<10)...)
	}
	n := copy(p, e.buf)
	e.buf = e.buf[n:]
	e.sent += int64(n)
	return n, nil
}

func TestMediaMetadataMatroskaStopsAfterTracks(t *testing.T) {
	// A live stream's segment has an unknown size and runs on forever. Once the
	// Info and Tracks are read nothing else is wanted, so the walk stops there.
	header := ebmlElem(ebmlHeader, ebmlElem(mkvDocType, []byte("matroska")))
	segment := append(ebmlID(mkvSegment), 0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	track := ebmlElem(mkvTrackEntry,
		ebmlUintElem(mkvTrackType, 1), ebmlElem(mkvCodecID, []byte("V_VP9")),
		ebmlElem(mkvVideo, ebmlUintElem(mkvPixelWidth, 640), ebmlUintElem(mkvPixelHeigh, 360)))
	body := append(ebmlElem(mkvInfo, ebmlFloatElem(mkvDuration, 1000)), ebmlElem(mkvTracks, track)...)
	prefix := append(append(header, segment...), body...)

	src := &countingReader{r: io.MultiReader(bytes.NewReader(prefix), &endlessClusters{})}
	r := NewLazyReader(src, 1<<40)
	out, ok := MediaMetadata("mkv", r, r.Size())
	if !ok || !strings.Contains(string(out), "Video: vp9, 640x360") {
		t.Fatalf("unexpected metadata (%v):\n%s", ok, out)
	}
	if src.n > 1<<20 {
		t.Errorf("read %d bytes of an endless stream", src.n)
	}
}

// --- Read-ahead ---

// ebmlLongElem writes an element with an eight-byte size, for payloads too long
// for ebmlElem.
func ebmlLongElem(id uint32, payload []byte) []byte {
	out := append(ebmlID(id), 0x01)
	out = append(out, binary.BigEndian.AppendUint64(nil, uint64(len(payload)))[1:]...)
	return append(out, payload...)
}

type countingReaderAt struct {
	r     io.ReaderAt
	calls int
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	c.calls++
	return c.r.ReadAt(p, off)
}

func TestByteReaderReadsAheadOnGIFWalk(t *testing.T) {
	data := gifFile(16, 16, 500, 4)
	ra := &countingReaderAt{r: bytes.NewReader(data)}
	out, ok := MediaMetadata("gif", ra, int64(len(data)))
	if !ok || !strings.Contains(string(out), "500 frames") {
		t.Fatalf("unexpected metadata:\n%s", out)
	}
	// Each frame is several blocks; one read per block would be thousands.
	if want := len(data)/mediaReadAhead + 4; ra.calls > want {
		t.Errorf("%d ReadAt calls over a %d-byte GIF, want at most %d", ra.calls, len(data), want)
	}
}

func TestByteReaderReadsAheadOnMatroskaWalk(t *testing.T) {
	var tracks []byte
	for i := 0; i < 200; i++ {
		tracks = append(tracks, ebmlElem(mkvTrackEntry,
			ebmlUintElem(mkvTrackType, 2), ebmlElem(mkvCodecID, []byte("A_OPUS")),
			ebmlElem(mkvAudio, ebmlFloatElem(mkvSampleRate, 48000), ebmlUintElem(mkvChannels, 2)))...)
	}
	header := ebmlElem(ebmlHeader, []byte{0x42, 0x86, 0x81, 0x01})
	segment := append(ebmlElem(mkvInfo, ebmlFloatElem(mkvDuration, 1000)), ebmlLongElem(mkvTracks, tracks)...)
	data := append(header, ebmlLongElem(mkvSegment, segment)...)
	ra := &countingReaderAt{r: bytes.NewReader(data)}
	out, ok := MediaMetadata("mkv", ra, int64(len(data)))
	if !ok || strings.Count(string(out), "Audio: opus") != 200 {
		t.Fatalf("unexpected metadata:\n%s", out)
	}
	if want := len(data)/mediaReadAhead + 4; ra.calls > want {
		t.Errorf("%d ReadAt calls over a %d-byte Matroska file, want at most %d", ra.calls, len(data), want)
	}
}

// freeBoxes is a run of MP4 free boxes just under the read-ahead apart, so
// that nearly every read-ahead straddles the end of what was read before.
type freeBoxes struct {
	left int
	buf  []byte
}

func (f *freeBoxes) Read(p []byte) (int, error) {
	if len(f.buf) == 0 {
		if f.left == 0 {
			return 0, io.EOF
		}
		f.left--
		f.buf = append(append(be32(65532), "free"...), zeros(65532-8)...)
	}
	n := copy(p, f.buf)
	f.buf = f.buf[n:]
	return n, nil
}

func TestLazyReaderBoundedWhenReadsStraddle(t *testing.T) {
	const boxes = 1024 // 64 MiB
	full := mp4File(
		mvhd(1000, 2000),
		trak(tkhd(1920, 1080), hdlr("vide"), mdhd(1000, 2000),
			stbl(videoSampleEntry("avc1", 1920, 1080), stts(60, 33))),
	)
	ftyp := box("ftyp", []byte("isom"), zeros(4))
	moov := full[len(ftyp):]
	total := int64(len(ftyp)) + boxes*65532 + int64(len(moov))

	r := NewLazyReader(io.MultiReader(bytes.NewReader(ftyp), &freeBoxes{left: boxes}, bytes.NewReader(moov)), total)
	out, ok := MediaMetadata("mp4", r, r.Size())
	if !ok || !strings.Contains(string(out), "Video: h264, 1920x1080") {
		t.Fatalf("unexpected metadata:\n%s", out)
	}
	if held := len(r.prefix) + len(r.win); held > lazyPrefixCap+lazyWindowCap+mediaReadAhead {
		t.Errorf("held %d bytes of a %d-byte stream", held, total)
	}
	if cap(r.prefix) > 2*lazyPrefixCap {
		t.Errorf("prefix grew to %d bytes", cap(r.prefix))
	}
}

func TestMediaMetadataTIFFStreamMatchesFile(t *testing.T) {
	// The bit depths come first, then 2 MiB of pixels, then the directory that
	// points back at them. A stream must not have discarded them by then.
	order := binary.LittleEndian
	var buf bytes.Buffer
	buf.WriteString("II")
	buf.Write(putU16(order, 42))
	pixels := 2 << 20
	ifdAt := uint32(8 + 6 + pixels)
	buf.Write(putU32(order, ifdAt))
	buf.Write(putU16(order, 8))
	buf.Write(putU16(order, 8))
	buf.Write(putU16(order, 8))
	buf.Write(zeros(pixels))
	buf.Write(putU16(order, 3))
	for _, e := range [][4]uint32{{256, 4, 1, 640}, {257, 4, 1, 480}, {258, 3, 3, 8}} {
		buf.Write(putU16(order, uint16(e[0])))
		buf.Write(putU16(order, uint16(e[1])))
		buf.Write(putU32(order, e[2]))
		buf.Write(putU32(order, e[3]))
	}
	data := buf.Bytes()

	local, _ := MediaMetadata("tiff", bytes.NewReader(data), int64(len(data)))
	r := NewLazyReader(bytes.NewReader(data), int64(len(data)))
	streamed, _ := MediaMetadata("tiff", r, r.Size())
	if !strings.Contains(string(local), "640x480, rgb, 8-bit") || string(streamed) != string(local) {
		t.Errorf("streamed:\n%s\nlocal:\n%s", streamed, local)
	}
}
