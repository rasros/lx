package internal

import (
	"encoding/binary"
	"fmt"
)

// pngColorModels are the colour types a PNG header can name; the bit depth it
// carries is per channel, not per pixel.
var pngColorModels = map[byte]string{
	0: "grayscale", 2: "rgb", 3: "indexed", 4: "grayscale+alpha", 6: "rgba",
}

func parsePNG(b *byteReader) *MediaInfo {
	if !b.magic(0, "\x89PNG\r\n\x1a\n") || !b.magic(12, "IHDR") {
		return nil
	}
	p, ok := b.at(16, 10)
	if !ok {
		return nil
	}
	return &MediaInfo{Image: &ImageInfo{
		Width:      int(binary.BigEndian.Uint32(p[:4])),
		Height:     int(binary.BigEndian.Uint32(p[4:8])),
		BitDepth:   int(p[8]),
		ColorModel: pngColorModels[p[9]],
	}}
}

// parseJPEG walks the marker segments to the frame header, which is the only
// place the dimensions are recorded.
func parseJPEG(b *byteReader) *MediaInfo {
	if !b.magic(0, "\xff\xd8") {
		return nil
	}

	for off := int64(2); off+4 <= b.size; {
		head, ok := b.at(off, 4)
		if !ok || head[0] != 0xff {
			return nil
		}
		marker := head[1]
		length := int64(binary.BigEndian.Uint16(head[2:4]))

		switch {
		case marker == 0xd8 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7):
			off += 2 // standalone markers carry no payload
			continue
		case isJPEGFrameMarker(marker):
			p, ok := b.at(off+4, 6)
			if !ok {
				return nil
			}
			return &MediaInfo{Image: &ImageInfo{
				Height:     int(binary.BigEndian.Uint16(p[1:3])),
				Width:      int(binary.BigEndian.Uint16(p[3:5])),
				BitDepth:   int(p[0]),
				ColorModel: jpegColorModel(p[5]),
			}}
		case marker == 0xda: // the scan begins; no header follows it
			return nil
		}

		if length < 2 {
			return nil
		}
		off += 2 + length
	}
	return nil
}

// isJPEGFrameMarker reports whether the marker starts a frame header. The
// arithmetic markers in the same range are not frames.
func isJPEGFrameMarker(marker byte) bool {
	if marker < 0xc0 || marker > 0xcf {
		return false
	}
	switch marker {
	case 0xc4, 0xc8, 0xcc:
		return false
	}
	return true
}

func jpegColorModel(components byte) string {
	switch components {
	case 1:
		return "grayscale"
	case 3:
		return "ycbcr"
	case 4:
		return "cmyk"
	}
	return ""
}

// parseGIF reads the logical screen descriptor, then walks the blocks to total
// the animation, since a GIF's length is the sum of its frame delays.
func parseGIF(b *byteReader) *MediaInfo {
	if !b.magic(0, "GIF87a") && !b.magic(0, "GIF89a") {
		return nil
	}
	p, ok := b.at(6, 7)
	if !ok {
		return nil
	}

	img := &ImageInfo{
		Width:      int(binary.LittleEndian.Uint16(p[:2])),
		Height:     int(binary.LittleEndian.Uint16(p[2:4])),
		ColorModel: "indexed",
		BitDepth:   int(p[4]&0x7) + 1,
	}

	off := int64(13)
	if p[4]&0x80 != 0 { // a global colour table follows the descriptor
		off += 3 * (1 << (int(p[4]&0x7) + 1))
	}

	frames, delay := b.gifBlocks(off)
	img.Frames = frames

	info := &MediaInfo{Image: img}
	if frames > 1 && delay > 0 {
		// Delays are in hundredths of a second.
		info.Duration = secondsToDuration(delay, 100)
	}
	return info
}

func (b *byteReader) gifBlocks(off int64) (frames int, delay uint64) {
	for off < b.size {
		head, ok := b.at(off, 1)
		if !ok {
			return frames, delay
		}

		switch head[0] {
		case 0x2c: // an image descriptor, so one more frame
			frames++
			desc, ok := b.at(off+1, 9)
			if !ok {
				return frames, delay
			}
			off += 10
			if desc[8]&0x80 != 0 { // a local colour table
				off += 3 * (1 << (int(desc[8]&0x7) + 1))
			}
			off++ // the LZW minimum code size
			off = b.gifSkipSubBlocks(off)
		case 0x21: // an extension
			label, ok := b.at(off+1, 1)
			if !ok {
				return frames, delay
			}
			if label[0] == 0xf9 { // graphic control, which holds the frame delay
				if gce, ok := b.at(off+3, 4); ok {
					delay += uint64(binary.LittleEndian.Uint16(gce[1:3]))
				}
			}
			off = b.gifSkipSubBlocks(off + 2)
		default: // the trailer, or something unparsable
			return frames, delay
		}
	}
	return frames, delay
}

// gifSkipSubBlocks steps over a chain of length-prefixed blocks, which is how
// GIF delimits every variable-length payload.
func (b *byteReader) gifSkipSubBlocks(off int64) int64 {
	for off < b.size {
		length, ok := b.at(off, 1)
		if !ok || length[0] == 0 {
			return off + 1
		}
		off += 1 + int64(length[0])
	}
	return off
}

func parseWebP(b *byteReader) *MediaInfo {
	if !b.magic(0, "RIFF") || !b.magic(8, "WEBP") {
		return nil
	}

	var (
		img    *ImageInfo
		frames int
	)
	b.riffChunks(12, func(id string, body, length int64) {
		switch id {
		case "VP8 ": // lossy: dimensions follow the frame tag and sync code
			if p, ok := b.at(body+6, 4); ok && img == nil {
				img = &ImageInfo{
					Width:      int(binary.LittleEndian.Uint16(p[:2]) & 0x3fff),
					Height:     int(binary.LittleEndian.Uint16(p[2:4]) & 0x3fff),
					ColorModel: "ycbcr",
				}
			}
		case "VP8L": // lossless: 14 bits each, packed
			if p, ok := b.at(body+1, 4); ok && img == nil {
				bits := binary.LittleEndian.Uint32(p)
				img = &ImageInfo{
					Width:      int(bits&0x3fff) + 1,
					Height:     int(bits>>14&0x3fff) + 1,
					ColorModel: "rgba",
				}
			}
		case "VP8X": // an extended file states its canvas up front
			if p, ok := b.at(body+4, 6); ok {
				img = &ImageInfo{
					Width:  int(uint32(p[0])|uint32(p[1])<<8|uint32(p[2])<<16) + 1,
					Height: int(uint32(p[3])|uint32(p[4])<<8|uint32(p[5])<<16) + 1,
				}
			}
		case "ANMF":
			frames++
		}
	})

	if img == nil {
		return nil
	}
	img.Frames = frames
	return &MediaInfo{Image: img}
}

func parseBMP(b *byteReader) *MediaInfo {
	if !b.magic(0, "BM") {
		return nil
	}
	// A BITMAPCOREHEADER is 12 bytes and stores 16-bit dimensions; everything
	// later is at least 40 and stores them as signed 32-bit, height negative when
	// the rows run top down. The two are read separately because a core-header
	// file is too short to hold the longer one.
	size, ok := b.at(14, 4)
	if !ok {
		return nil
	}
	if binary.LittleEndian.Uint32(size) == 12 {
		p, ok := b.at(18, 8)
		if !ok {
			return nil
		}
		return &MediaInfo{Image: &ImageInfo{
			Width:      int(binary.LittleEndian.Uint16(p[:2])),
			Height:     int(binary.LittleEndian.Uint16(p[2:4])),
			BitDepth:   int(binary.LittleEndian.Uint16(p[6:8])),
			ColorModel: "rgb",
		}}
	}

	p, ok := b.at(14, 16)
	if !ok {
		return nil
	}
	height := int(int32(binary.LittleEndian.Uint32(p[8:12])))
	if height < 0 {
		height = -height
	}
	return &MediaInfo{Image: &ImageInfo{
		Width:      int(int32(binary.LittleEndian.Uint32(p[4:8]))),
		Height:     height,
		BitDepth:   int(binary.LittleEndian.Uint16(p[14:16])),
		ColorModel: "rgb",
	}}
}

// parseICO reports the largest image in the file, an icon being a bundle of
// sizes rather than one picture.
func parseICO(b *byteReader) *MediaInfo {
	head, ok := b.at(0, 6)
	if !ok || binary.LittleEndian.Uint16(head[:2]) != 0 ||
		binary.LittleEndian.Uint16(head[2:4]) != 1 {
		return nil
	}
	count := int64(binary.LittleEndian.Uint16(head[4:6]))
	if count == 0 {
		return nil
	}

	img := &ImageInfo{}
	for i := int64(0); i < count; i++ {
		entry, ok := b.at(6+i*16, 16)
		if !ok {
			break
		}
		// A zero in either dimension means 256, which does not fit in the byte.
		width, height := int(entry[0]), int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		if width*height > img.Width*img.Height {
			img.Width, img.Height = width, height
			img.BitDepth = int(binary.LittleEndian.Uint16(entry[6:8]))
		}
	}

	if count > 1 {
		img.ColorModel = fmt.Sprintf("%d sizes", count)
	}
	return &MediaInfo{Image: img}
}

// parseTIFF reads the tags of the first directory, whose byte order the file
// declares in its header.
func parseTIFF(b *byteReader) *MediaInfo {
	head, ok := b.at(0, 8)
	if !ok {
		return nil
	}

	var order binary.ByteOrder
	switch string(head[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil
	}
	if order.Uint16(head[2:4]) != 42 {
		return nil
	}

	dir := int64(order.Uint32(head[4:8]))
	count, ok := b.at(dir, 2)
	if !ok {
		return nil
	}

	img := &ImageInfo{ColorModel: "rgb"}
	for i := int64(0); i < int64(order.Uint16(count)); i++ {
		entry, ok := b.at(dir+2+i*12, 12)
		if !ok {
			break
		}
		value, ok := b.tiffValue(entry, order)
		if !ok {
			continue
		}
		switch order.Uint16(entry[:2]) {
		case 256:
			img.Width = value
		case 257:
			img.Height = value
		case 258:
			img.BitDepth = value
		}
	}

	if img.Width == 0 && img.Height == 0 {
		return nil
	}
	return &MediaInfo{Image: img}
}

// tiffValue reads a tag's first value. A field only holds the value itself when
// it fits in four bytes; otherwise it holds an offset to where the values are,
// which is the case for the three samples of an RGB image's bit depth.
func (b *byteReader) tiffValue(entry []byte, order binary.ByteOrder) (int, bool) {
	short := order.Uint16(entry[2:4]) == 3
	width := int64(4)
	if short {
		width = 2
	}

	body := entry[8:12]
	if int64(order.Uint32(entry[4:8]))*width > 4 {
		fetched, ok := b.at(int64(order.Uint32(entry[8:12])), width)
		if !ok {
			return 0, false
		}
		body = fetched
	}

	if short {
		return int(order.Uint16(body[:2])), true
	}
	return int(order.Uint32(body[:4])), true
}
