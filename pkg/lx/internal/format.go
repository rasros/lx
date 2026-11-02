package internal

import (
	"bytes"
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

// Format is one entry of the registry: everything lx knows about a file format,
// so that the extension table, the MIME table, the sniffer and the parser
// cannot disagree about it.
type Format struct {
	Name string
	Kind Kind
	MIME string
	Exts []string

	signatures []signature
	parse      func(*byteReader) *MediaInfo
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

// isoBrand matches an ISO base media file whose ftyp box declares one of the
// brands, which are the only thing telling apart formats that share the box
// layout entirely. With no brands given, any ftyp box matches.
func isoBrand(brands ...string) func([]byte) bool {
	return func(h []byte) bool {
		if !magicAt(4, "ftyp")(h) {
			return false
		}
		if len(brands) == 0 {
			return true
		}
		if len(h) < 12 {
			return false
		}
		got := strings.TrimSpace(string(h[8:12]))
		for _, b := range brands {
			if got == b {
				return true
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
		signatures: []signature{strong(isWebM)}, parse: parseMatroska},
	{Name: "mkv", Kind: KindVideo, MIME: "video/x-matroska", Exts: []string{".mkv"},
		signatures: []signature{strong(magicAt(0, "\x1a\x45\xdf\xa3"))}, parse: parseMatroska},
	// AVIF and HEIC are the same boxes as an mp4, holding a still rather than a
	// track.
	{Name: "avif", Kind: KindImage, MIME: "image/avif", Exts: []string{".avif"},
		signatures: []signature{strong(isoBrand("avif", "avis"))}, parse: parseMP4},
	{Name: "heic", Kind: KindImage, MIME: "image/heic", Exts: []string{".heic"},
		signatures: []signature{strong(isoBrand("heic", "heix", "hevc", "hevx", "mif1", "msf1"))}, parse: parseMP4},
	{Name: "mov", Kind: KindVideo, MIME: "video/quicktime", Exts: []string{".mov"},
		signatures: []signature{strong(isoBrand("qt"))}, parse: parseMP4},
	{Name: "m4a", Kind: KindAudio, MIME: "audio/mp4", Exts: []string{".m4a"},
		signatures: []signature{strong(isoBrand("M4A"))}, parse: parseMP4},
	{Name: "mp4", Kind: KindVideo, MIME: "video/mp4", Exts: []string{".mp4", ".m4v"},
		signatures: []signature{strong(isoBrand())}, parse: parseMP4},
	// A sync word is only eleven set bits, so a bare frame header is weak; an ID3
	// tag is not.
	{Name: "mp3", Kind: KindAudio, MIME: "audio/mpeg", Exts: []string{".mp3"},
		signatures: []signature{strong(magicAt(0, "ID3")), weak(looksLikeMP3)}, parse: parseMP3},
	// SVG is XML, and reads better as text than as an embedded blob. It is here
	// for its MIME type, for templates that pass an svg path to dataURI on purpose.
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

// sniffFormat identifies a format from the header alone. Strong signatures are
// tried before weak ones so that a short magic cannot shadow a specific one,
// and weak signatures count only for binary content.
func sniffFormat(header []byte) (*Format, Strength) {
	if len(header) == 0 {
		return nil, 0
	}
	if f := sniffWith(header, Strong); f != nil {
		return f, Strong
	}
	if IsBinary(header) {
		if f := sniffWith(header, Weak); f != nil {
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

// Classify decides what format a file is from its name and the first bytes of
// its content (up to SniffLen), and returns nil for anything that is not a
// registered format. The content decides wherever it can:
//
//   - content that looks like text is never a registered format, whatever the
//     extension says, since a mislabelled text file (such as a Git LFS pointer)
//     is still text;
//   - a strong signature overrides the extension;
//   - otherwise the extension decides, and failing that a weak signature.
//
// An empty header carries no evidence, so the extension alone decides.
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
		return sniffed
	}
	if byExt != nil {
		return byExt
	}
	return sniffed
}
