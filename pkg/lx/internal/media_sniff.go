package internal

import "strings"

// sniffContainer identifies a container from the bytes at the start of the
// file, by the same registry signatures Classify uses.
func sniffContainer(b *byteReader) string {
	n := b.size
	if n > SniffLen {
		n = SniffLen
	}
	header, ok := b.at(0, n)
	if !ok {
		return ""
	}
	if f, _ := sniffFormat(header); f.IsMedia() {
		return f.Name
	}
	return ""
}

// matroskaFlavour reads the DocType the EBML header declares, since WebM is
// Matroska with a narrower profile and nothing else tells the two apart.
func matroskaFlavour(b *byteReader) string {
	_, start, stop, ok := b.ebmlElement(0)
	if !ok {
		return "mkv"
	}

	flavour := "mkv"
	b.ebmlChildren(start, stop, func(id uint64, s, e int64) {
		if id == mkvDocType && strings.Contains(string(b.ebmlBytes(s, e)), "webm") {
			flavour = "webm"
		}
	})
	return flavour
}

// looksLikeMP3 checks for a valid frame header at offset zero, the only
// place a bare sync word is trusted.
func looksLikeMP3(header []byte) bool {
	if len(header) < 4 || header[0] != 0xff || header[1]&0xe0 != 0xe0 {
		return false
	}
	if header[1]>>3&0x3 == 1 || header[1]>>1&0x3 == 0 {
		return false // a reserved version or layer
	}
	bitrateIndex := header[2] >> 4 & 0xf
	return bitrateIndex != 0 && bitrateIndex != 15 && header[2]>>2&0x3 != 3
}
