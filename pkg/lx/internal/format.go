package internal

import (
	"bytes"
	"encoding/binary"
	"path"
	"strings"
)

// Kind groups formats by what a reader of the bundle can do with them.
type Kind int

const (
	KindImage Kind = iota + 1
	KindAudio
	KindVideo
	// KindMarkup is a format that is text underneath, such as SVG. It has a MIME
	// type for templates that embed it on purpose, but is otherwise read as text.
	KindMarkup
)

// Strength says how much a signature proves on its own.
type Strength int

const (
	// Weak signatures are short or common enough to turn up at the start of
	// ordinary text, so they count only for content that is already binary.
	Weak Strength = iota + 1
	// Strong signatures cannot be mistaken, and override the extension.
	Strong
)

type signature struct {
	strength Strength
	match    func(header []byte) bool
}

// Format is everything lx knows about a file format, in one registry entry
// so the extension, MIME, sniffer and parser tables cannot disagree.
type Format struct {
	Name string
	Kind Kind
	MIME string
	Exts []string

	// family groups formats sharing a container layout, such as the ISO
	// brands. Within a family the extension is the better guide.
	family     string
	signatures []signature
	parse      func(*byteReader) *MediaInfo
}

// Verify reports whether the header plausibly is this format: a strong
// signature matches, or the parser makes sense of it.
func (f *Format) Verify(header []byte) bool {
	if f == nil {
		return false
	}
	for _, s := range f.signatures {
		if s.strength == Strong && s.match(header) {
			return true
		}
	}
	return f.parse != nil && f.parse(headerReader(header)) != nil
}

// IsImage reports whether the format is a picture, which renders embedded
// rather than as text. It is false for a nil format.
func (f *Format) IsImage() bool { return f != nil && f.Kind == KindImage }

// IsMedia reports whether the format has metadata worth describing.
func (f *Format) IsMedia() bool {
	return f != nil && (f.Kind == KindImage || f.Kind == KindAudio || f.Kind == KindVideo)
}

func magicAt(off int, want string) func([]byte) bool {
	return func(h []byte) bool {
		return len(h) >= off+len(want) && string(h[off:off+len(want)]) == want
	}
}

func strong(match func([]byte) bool) signature { return signature{Strong, match} }
func weak(match func([]byte) bool) signature   { return signature{Weak, match} }

// isoBrands lists the major and compatible brands an ftyp box declares.
func isoBrands(h []byte) []string {
	if len(h) < 12 {
		return nil
	}
	brands := []string{strings.TrimSpace(string(h[8:12]))}
	end := int(binary.BigEndian.Uint32(h[:4]))
	if end > len(h) {
		end = len(h)
	}
	for off := 16; off+4 <= end; off += 4 {
		brands = append(brands, strings.TrimSpace(string(h[off:off+4])))
	}
	return brands
}

// isoBrand matches an ISO base media file declaring one of the brands, or any
// ftyp box when none are given. Formats sharing brands are told apart by
// registry order, the specific one first.
func isoBrand(brands ...string) func([]byte) bool {
	return func(h []byte) bool {
		if !magicAt(4, "ftyp")(h) {
			return false
		}
		if len(brands) == 0 {
			return true
		}
		for _, got := range isoBrands(h) {
			for _, b := range brands {
				if got == b {
					return true
				}
			}
		}
		return false
	}
}

func isWebM(h []byte) bool {
	return magicAt(0, "\x1a\x45\xdf\xa3")(h) && matroskaFlavour(headerReader(h)) == "webm"
}

func headerReader(h []byte) *byteReader {
	return &byteReader{r: bytes.NewReader(h), size: int64(len(h))}
}

// formats is ordered for sniffing: where two signatures overlap, the more
// specific comes first.
var formats = []*Format{
	{Name: "png", Kind: KindImage, MIME: "image/png", Exts: []string{".png"},
		signatures: []signature{strong(magicAt(0, "\x89PNG\r\n\x1a\n"))}, parse: parsePNG},
	{Name: "jpeg", Kind: KindImage, MIME: "image/jpeg", Exts: []string{".jpg", ".jpeg"},
		signatures: []signature{strong(magicAt(0, "\xff\xd8\xff"))}, parse: parseJPEG},
	{Name: "gif", Kind: KindImage, MIME: "image/gif", Exts: []string{".gif"},
		signatures: []signature{strong(magicAt(0, "GIF87a")), strong(magicAt(0, "GIF89a"))}, parse: parseGIF},
	{Name: "bmp", Kind: KindImage, MIME: "image/bmp", Exts: []string{".bmp"},
		signatures: []signature{weak(magicAt(0, "BM"))}, parse: parseBMP},
	{Name: "tiff", Kind: KindImage, MIME: "image/tiff", Exts: []string{".tif", ".tiff"},
		signatures: []signature{strong(magicAt(0, "II*\x00")), strong(magicAt(0, "MM\x00*"))}, parse: parseTIFF},
	{Name: "ico", Kind: KindImage, MIME: "image/x-icon", Exts: []string{".ico"},
		signatures: []signature{weak(magicAt(0, "\x00\x00\x01\x00"))}, parse: parseICO},
	{Name: "flac", Kind: KindAudio, MIME: "audio/flac", Exts: []string{".flac"},
		signatures: []signature{strong(magicAt(0, "fLaC"))}, parse: parseFLAC},
	{Name: "wav", Kind: KindAudio, MIME: "audio/wav", Exts: []string{".wav"},
		signatures: []signature{strong(func(h []byte) bool { return magicAt(0, "RIFF")(h) && magicAt(8, "WAVE")(h) })},
		parse:      parseWAV},
	{Name: "webp", Kind: KindImage, MIME: "image/webp", Exts: []string{".webp"},
		signatures: []signature{strong(func(h []byte) bool { return magicAt(0, "RIFF")(h) && magicAt(8, "WEBP")(h) })},
		parse:      parseWebP},
	{Name: "webm", Kind: KindVideo, MIME: "video/webm", Exts: []string{".webm"},
		signatures: []signature{strong(isWebM)}, family: "matroska", parse: parseMatroska},
	{Name: "mkv", Kind: KindVideo, MIME: "video/x-matroska", Exts: []string{".mkv"},
		signatures: []signature{strong(magicAt(0, "\x1a\x45\xdf\xa3"))}, family: "matroska", parse: parseMatroska},
	// AVIF and HEIC are the same boxes as an mp4, holding a still rather than a
	// track.
	{Name: "avif", Kind: KindImage, MIME: "image/avif", Exts: []string{".avif"},
		signatures: []signature{strong(isoBrand("avif", "avis"))}, family: "iso", parse: parseMP4},
	{Name: "heic", Kind: KindImage, MIME: "image/heic", Exts: []string{".heic"},
		signatures: []signature{strong(isoBrand("heic", "heix", "hevc", "hevx", "mif1", "msf1"))}, family: "iso", parse: parseMP4},
	{Name: "mov", Kind: KindVideo, MIME: "video/quicktime", Exts: []string{".mov"},
		signatures: []signature{strong(isoBrand("qt"))}, family: "iso", parse: parseMP4},
	{Name: "m4a", Kind: KindAudio, MIME: "audio/mp4", Exts: []string{".m4a"},
		signatures: []signature{strong(isoBrand("M4A"))}, family: "iso", parse: parseMP4},
	{Name: "mp4", Kind: KindVideo, MIME: "video/mp4", Exts: []string{".mp4", ".m4v"},
		signatures: []signature{strong(isoBrand())}, family: "iso", parse: parseMP4},
	// A sync word is only eleven set bits, so a bare frame header is weak; an ID3
	// tag is not.
	{Name: "mp3", Kind: KindAudio, MIME: "audio/mpeg", Exts: []string{".mp3"},
		signatures: []signature{strong(magicAt(0, "ID3")), weak(looksLikeMP3)}, parse: parseMP3},
	// SVG reads better as text; it is here for its MIME type, for dataURI.
	{Name: "svg", Kind: KindMarkup, MIME: "image/svg+xml", Exts: []string{".svg"}},
}

var (
	formatsByExt  = map[string]*Format{}
	formatsByName = map[string]*Format{}
)

func init() {
	for _, f := range formats {
		formatsByName[f.Name] = f
		for _, ext := range f.Exts {
			formatsByExt[ext] = f
		}
	}
}

// FormatByExt returns the format the path's extension names, or nil.
func FormatByExt(p string) *Format {
	return formatsByExt[strings.ToLower(path.Ext(p))]
}

// FormatByName returns the format with the given container name, or nil.
func FormatByName(name string) *Format {
	return formatsByName[name]
}

// SniffLen is how much of a file's start Classify needs to see.
const SniffLen = 1024

// sniffFormat identifies a format from the header alone. Strong signatures
// are tried first, and weak ones count only for binary content the format's
// parser accepts.
func sniffFormat(header []byte) (*Format, Strength) {
	if len(header) == 0 {
		return nil, 0
	}
	if f := sniffWith(header, Strong); f != nil {
		return f, Strong
	}
	if IsBinary(header) {
		if f := sniffWith(header, Weak); f != nil && f.Verify(header) {
			return f, Weak
		}
	}
	return nil, 0
}

func sniffWith(header []byte, strength Strength) *Format {
	for _, f := range formats {
		for _, s := range f.signatures {
			if s.strength == strength && s.match(header) {
				return f
			}
		}
	}
	return nil
}

// Classify decides a file's format from its name and first SniffLen bytes,
// or returns nil. Text content is never a registered format; a strong
// signature overrides the extension except within a family; otherwise the
// extension decides, then a weak signature. An empty header leaves it to the
// extension.
func Classify(p string, header []byte) *Format {
	byExt := FormatByExt(p)
	if len(header) == 0 {
		return byExt
	}
	if !IsBinary(header) {
		return nil
	}
	sniffed, strength := sniffFormat(header)
	if strength == Strong {
		if byExt != nil && byExt.family != "" && byExt.family == sniffed.family {
			return byExt
		}
		return sniffed
	}
	if byExt != nil {
		return byExt
	}
	return sniffed
}
