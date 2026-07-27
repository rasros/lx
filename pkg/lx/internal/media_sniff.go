package internal

import "strings"

// sniffContainer identifies a container from the bytes it puts at a fixed
// offset. Unlike markup, whose shape can only be guessed at, every one of these
// formats declares itself, so this is a lookup rather than a heuristic.
func sniffContainer(b *byteReader) string {
	switch {
	case b.magic(0, "\x89PNG\r\n\x1a\n"):
		return "png"
	case b.magic(0, "\xff\xd8\xff"):
		return "jpeg"
	case b.magic(0, "GIF87a"), b.magic(0, "GIF89a"):
		return "gif"
	case b.magic(0, "BM"):
		return "bmp"
	case b.magic(0, "II*\x00"), b.magic(0, "MM\x00*"):
		return "tiff"
	case b.magic(0, "\x00\x00\x01\x00"):
		return "ico"
	case b.magic(0, "fLaC"):
		return "flac"
	case b.magic(0, "RIFF") && b.magic(8, "WAVE"):
		return "wav"
	case b.magic(0, "RIFF") && b.magic(8, "WEBP"):
		return "webp"
	case b.magic(0, "\x1a\x45\xdf\xa3"):
		return matroskaFlavour(b)
	case b.magic(4, "ftyp"):
		return isoBrandContainer(b)
	case b.magic(0, "ID3"), looksLikeMP3(b):
		return "mp3"
	}
	return ""
}

// isoBrandContainer reads the brand an ISO container declares, which is the only
// thing distinguishing formats that share the box layout entirely.
func isoBrandContainer(b *byteReader) string {
	brand, ok := b.at(8, 4)
	if !ok {
		return "mp4"
	}
	switch strings.TrimSpace(string(brand)) {
	case "qt":
		return "mov"
	case "M4A":
		return "m4a"
	case "avif", "avis":
		return "avif"
	case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
		return "heic"
	}
	return "mp4"
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

// looksLikeMP3 checks for a frame header at the very start of the file. A sync
// word is only eleven set bits, common enough in arbitrary binary that it is
// worth trusting nowhere but offset zero, and only when the fields behind it
// hold values that exist.
func looksLikeMP3(b *byteReader) bool {
	header, ok := b.at(0, 4)
	if !ok || header[0] != 0xff || header[1]&0xe0 != 0xe0 {
		return false
	}
	if header[1]>>3&0x3 == 1 || header[1]>>1&0x3 == 0 {
		return false // a reserved version or layer
	}
	bitrateIndex := header[2] >> 4 & 0xf
	return bitrateIndex != 0 && bitrateIndex != 15 && header[2]>>2&0x3 != 3
}
