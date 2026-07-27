package internal

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func pngFile(width, height uint32, bitDepth, colorType byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	buf.Write(be32(13))
	buf.WriteString("IHDR")
	buf.Write(be32(width))
	buf.Write(be32(height))
	buf.Write([]byte{bitDepth, colorType, 0, 0, 0})
	buf.Write(be32(0)) // crc
	return buf.Bytes()
}

func TestMediaMetadataPNG(t *testing.T) {
	cases := []struct {
		name      string
		colorType byte
		bitDepth  byte
		want      string
	}{
		{"truecolour with alpha", 6, 8, "Image: 800x600, rgba, 8-bit"},
		{"truecolour", 2, 8, "Image: 800x600, rgb, 8-bit"},
		{"palette", 3, 4, "Image: 800x600, indexed, 4-bit"},
		{"greyscale", 0, 16, "Image: 800x600, grayscale, 16-bit"},
		{"greyscale with alpha", 4, 8, "Image: 800x600, grayscale+alpha, 8-bit"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := describe(t, "png", pngFile(800, 600, c.bitDepth, c.colorType))
			if !strings.Contains(got, c.want) {
				t.Errorf("got:\n%s\nwant %q", got, c.want)
			}
		})
	}
}

// A picture has no bitrate, even when it is an animation with a duration.
func TestMediaMetadataImageHasNoBitrate(t *testing.T) {
	if got := describe(t, "png", pngFile(16, 16, 8, 6)); strings.Contains(got, "Bitrate") {
		t.Errorf("got:\n%s", got)
	}
}

// jpegSegment builds a marker segment with its two length bytes included.
func jpegSegment(marker byte, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	out := []byte{0xff, marker}
	out = append(out, be16(uint16(len(body)+2))...)
	return append(out, body...)
}

func jpegFile(marker byte, width, height uint16, components byte, extra ...[]byte) []byte {
	frame := jpegSegment(marker, []byte{8}, be16(height), be16(width), []byte{components})
	return append(append([]byte{0xff, 0xd8}, bytes.Join(extra, nil)...), frame...)
}

func TestMediaMetadataJPEG(t *testing.T) {
	cases := []struct {
		name       string
		marker     byte
		components byte
		want       string
	}{
		{"baseline", 0xc0, 3, "Image: 1024x768, ycbcr, 8-bit"},
		{"progressive", 0xc2, 3, "Image: 1024x768, ycbcr, 8-bit"},
		{"greyscale", 0xc0, 1, "Image: 1024x768, grayscale, 8-bit"},
		{"cmyk", 0xc0, 4, "Image: 1024x768, cmyk, 8-bit"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := describe(t, "jpeg", jpegFile(c.marker, 1024, 768, c.components))
			if !strings.Contains(got, c.want) {
				t.Errorf("got:\n%s\nwant %q", got, c.want)
			}
		})
	}
}

// The frame header sits behind whatever metadata the encoder wrote first.
func TestMediaMetadataJPEGSkipsLeadingSegments(t *testing.T) {
	exif := jpegSegment(0xe1, []byte("Exif\x00\x00"), make([]byte, 64))
	comment := jpegSegment(0xfe, []byte("a comment"))
	got := describe(t, "jpeg", jpegFile(0xc0, 320, 240, 3, exif, comment))

	if !strings.Contains(got, "320x240") {
		t.Errorf("got:\n%s", got)
	}
}

// A Huffman table shares the marker range with the frame headers but is not one.
func TestMediaMetadataJPEGIgnoresHuffmanTable(t *testing.T) {
	table := jpegSegment(0xc4, make([]byte, 20))
	got := describe(t, "jpeg", jpegFile(0xc0, 64, 48, 3, table))

	if !strings.Contains(got, "64x48") {
		t.Errorf("got:\n%s", got)
	}
}

func gifFile(width, height uint16, frames int, delay uint16) []byte {
	var buf bytes.Buffer
	buf.WriteString("GIF89a")
	buf.Write(le16(width))
	buf.Write(le16(height))
	buf.Write([]byte{0x00, 0, 0}) // no global colour table, 1-bit depth

	for range frames {
		// A graphic control extension carries the delay for the frame after it.
		buf.Write([]byte{0x21, 0xf9, 4, 0})
		buf.Write(le16(delay))
		buf.Write([]byte{0, 0})

		buf.Write([]byte{0x2c})
		buf.Write(zeros(8))
		buf.Write([]byte{0x00})    // no local colour table
		buf.Write([]byte{2})       // LZW minimum code size
		buf.Write([]byte{1, 0, 0}) // one sub-block of one byte, then the terminator
	}

	buf.Write([]byte{0x3b}) // trailer
	return buf.Bytes()
}

func TestMediaMetadataGIFAnimation(t *testing.T) {
	got := describe(t, "gif", gifFile(320, 240, 12, 10))
	for _, want := range []string{"Image: 320x240, indexed, 1-bit, 12 frames", "Duration: 00:00:01.200"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// One frame is a still, so neither a frame count nor a duration applies.
func TestMediaMetadataGIFStill(t *testing.T) {
	got := describe(t, "gif", gifFile(48, 48, 1, 0))
	if strings.Contains(got, "frames") || strings.Contains(got, "Duration") {
		t.Errorf("got:\n%s", got)
	}
	if !strings.Contains(got, "Image: 48x48") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataGIF87a(t *testing.T) {
	data := gifFile(8, 8, 1, 0)
	copy(data, "GIF87a")
	if got := describe(t, "gif", data); !strings.Contains(got, "8x8") {
		t.Errorf("got:\n%s", got)
	}
}

func webpFile(chunk string, payload []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	buf.Write(le32(uint32(12 + len(payload))))
	buf.WriteString("WEBP")
	buf.WriteString(chunk)
	buf.Write(le32(uint32(len(payload))))
	buf.Write(payload)
	return buf.Bytes()
}

func TestMediaMetadataWebPLossy(t *testing.T) {
	// A frame tag and sync code precede the dimensions.
	payload := append([]byte{0, 0, 0, 0x9d, 0x01, 0x2a}, bytes.Join([][]byte{
		le16(640), le16(480),
	}, nil)...)

	if got := describe(t, "webp", webpFile("VP8 ", payload)); !strings.Contains(got, "Image: 640x480, ycbcr") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataWebPLossless(t *testing.T) {
	// Both dimensions are fourteen bits wide and stored one less than they are.
	bits := uint32(639) | uint32(479)<<14
	payload := append([]byte{0x2f}, binary.LittleEndian.AppendUint32(nil, bits)...)

	if got := describe(t, "webp", webpFile("VP8L", payload)); !strings.Contains(got, "Image: 640x480, rgba") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataWebPExtended(t *testing.T) {
	payload := append(zeros(4), []byte{0x7f, 0x01, 0, 0xdf, 0, 0}...) // 384x224
	if got := describe(t, "webp", webpFile("VP8X", payload)); !strings.Contains(got, "384x224") {
		t.Errorf("got:\n%s", got)
	}
}

func bmpFile(headerSize uint32, width, height int32, bits uint16) []byte {
	var buf bytes.Buffer
	buf.WriteString("BM")
	buf.Write(zeros(12))
	buf.Write(le32(headerSize))
	if headerSize == 12 {
		buf.Write(le16(uint16(width)))
		buf.Write(le16(uint16(height)))
		buf.Write(le16(1))
		buf.Write(le16(bits))
		return buf.Bytes()
	}
	buf.Write(le32(uint32(width)))
	buf.Write(le32(uint32(height)))
	buf.Write(le16(1))
	buf.Write(le16(bits))
	return buf.Bytes()
}

func TestMediaMetadataBMP(t *testing.T) {
	if got := describe(t, "bmp", bmpFile(40, 200, 100, 24)); !strings.Contains(got, "Image: 200x100, rgb, 24-bit") {
		t.Errorf("got:\n%s", got)
	}
}

// A negative height means the rows are stored top down, not that the image is.
func TestMediaMetadataBMPTopDown(t *testing.T) {
	if got := describe(t, "bmp", bmpFile(40, 200, -100, 32)); !strings.Contains(got, "200x100") {
		t.Errorf("got:\n%s", got)
	}
}

// The oldest header is twelve bytes and stores its dimensions in half the width.
func TestMediaMetadataBMPCoreHeader(t *testing.T) {
	if got := describe(t, "bmp", bmpFile(12, 64, 32, 8)); !strings.Contains(got, "Image: 64x32, rgb, 8-bit") {
		t.Errorf("got:\n%s", got)
	}
}

func icoFile(sizes [][2]byte) []byte {
	var buf bytes.Buffer
	buf.Write(le16(0))
	buf.Write(le16(1))
	buf.Write(le16(uint16(len(sizes))))
	for _, s := range sizes {
		buf.Write([]byte{s[0], s[1], 0, 0, 1, 0})
		buf.Write(le16(32)) // bits per pixel
		buf.Write(zeros(8))
	}
	return buf.Bytes()
}

// An icon is a bundle, so the largest image in it is the one worth naming.
func TestMediaMetadataICOReportsLargest(t *testing.T) {
	got := describe(t, "ico", icoFile([][2]byte{{16, 16}, {48, 48}, {32, 32}}))
	if !strings.Contains(got, "Image: 48x48, 3 sizes, 32-bit") {
		t.Errorf("got:\n%s", got)
	}
}

// Zero means 256, which is the one size that will not fit in the byte.
func TestMediaMetadataICOZeroMeans256(t *testing.T) {
	if got := describe(t, "ico", icoFile([][2]byte{{0, 0}})); !strings.Contains(got, "256x256") {
		t.Errorf("got:\n%s", got)
	}
}

// The byte order a TIFF declares applies to every field in it, so the builder
// takes it as a parameter rather than assuming one.
func putU16(order binary.ByteOrder, v uint16) []byte {
	b := make([]byte, 2)
	order.PutUint16(b, v)
	return b
}

func putU32(order binary.ByteOrder, v uint32) []byte {
	b := make([]byte, 4)
	order.PutUint32(b, v)
	return b
}

func tiffFile(order binary.ByteOrder, width, height uint32) []byte {
	var buf bytes.Buffer
	if order == binary.LittleEndian {
		buf.WriteString("II")
	} else {
		buf.WriteString("MM")
	}
	buf.Write(putU16(order, 42))
	buf.Write(putU32(order, 8))

	entries := [][3]uint32{{256, 4, width}, {257, 4, height}, {258, 3, 8}}
	buf.Write(putU16(order, uint16(len(entries))))
	for _, e := range entries {
		buf.Write(putU16(order, uint16(e[0])))
		buf.Write(putU16(order, uint16(e[1])))
		buf.Write(putU32(order, 1))
		if e[1] == 3 {
			buf.Write(putU16(order, uint16(e[2])))
			buf.Write(zeros(2))
			continue
		}
		buf.Write(putU32(order, e[2]))
	}
	return buf.Bytes()
}

func TestMediaMetadataTIFF(t *testing.T) {
	for _, order := range []struct {
		name      string
		byteOrder binary.ByteOrder
	}{
		{"little endian", binary.LittleEndian},
		{"big endian", binary.BigEndian},
	} {
		t.Run(order.name, func(t *testing.T) {
			got := describe(t, "tiff", tiffFile(order.byteOrder, 1200, 900))
			if !strings.Contains(got, "Image: 1200x900, rgb, 8-bit") {
				t.Errorf("got:\n%s", got)
			}
		})
	}
}

// An RGB image's bit depth is three values, too wide for the field, so the tag
// holds an offset to them instead.
func TestMediaMetadataTIFFBitDepthArray(t *testing.T) {
	order := binary.LittleEndian
	var buf bytes.Buffer
	buf.WriteString("II")
	buf.Write(putU16(order, 42))
	buf.Write(putU32(order, 8))
	buf.Write(putU16(order, 3))

	samplesAt := uint32(8 + 2 + 3*12)
	for _, e := range [][4]uint32{{256, 4, 1, 64}, {257, 4, 1, 48}, {258, 3, 3, samplesAt}} {
		buf.Write(putU16(order, uint16(e[0])))
		buf.Write(putU16(order, uint16(e[1])))
		buf.Write(putU32(order, e[2]))
		buf.Write(putU32(order, e[3]))
	}
	buf.Write(putU16(order, 8))
	buf.Write(putU16(order, 8))
	buf.Write(putU16(order, 8))

	if got := describe(t, "tiff", buf.Bytes()); !strings.Contains(got, "Image: 64x48, rgb, 8-bit") {
		t.Errorf("got:\n%s", got)
	}
}

// An AVIF is an ISO container holding a property rather than a track.
func TestMediaMetadataAVIF(t *testing.T) {
	ispe := box("ispe", zeros(4), be32(1280), be32(720))
	meta := box("meta", zeros(4), box("iprp", box("ipco", ispe)))
	data := append(box("ftyp", []byte("avif"), zeros(4)), meta...)

	if got := describe(t, "avif", data); !strings.Contains(got, "Image: 1280x720") {
		t.Errorf("got:\n%s", got)
	}
}

func TestMediaMetadataImagesDegradeToContainer(t *testing.T) {
	cases := []struct {
		name, container string
		data            []byte
	}{
		{"png without header", "png", []byte("\x89PNG\r\n\x1a\n")},
		{"not a png", "png", []byte("plain text")},
		{"jpeg without a frame", "jpeg", []byte{0xff, 0xd8, 0xff, 0xda, 0x00, 0x02}},
		{"truncated gif", "gif", []byte("GIF89a\x00")},
		{"webp without a chunk", "webp", []byte("RIFF\x00\x00\x00\x00WEBP")},
		{"bmp too short", "bmp", []byte("BM")},
		{"ico with no entries", "ico", []byte{0, 0, 1, 0, 0, 0}},
		{"tiff with a bad magic", "tiff", []byte("II\x00\x00\x08\x00\x00\x00")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describe(t, c.container, c.data); got != "Container: "+c.container {
				t.Errorf("got:\n%s", got)
			}
		})
	}
}
