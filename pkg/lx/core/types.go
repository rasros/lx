package core

import (
	"strconv"
	"text/template"
	"time"
)

// Plan is what a Stream is about to render, known before rendering starts.
// It is the only view of the whole output templates get, as .Global.
type Plan struct {
	TotalFiles  int
	TotalSize   int64
	SizeUnknown bool
	// TokenEstimate is estimated from the sizes of the files alone.
	TokenEstimate int64
	TotalSections int
	WorkDir       string

	SkippedBySize int
}

// Report is what rendering a Stream turned out to be: its Plan and what
// only rendering finds out.
type Report struct {
	Plan
	TotalWrittenBytes int64
	TotalRows         int64
	// WrittenTokenEstimate is estimated from the written output, where the
	// Plan's TokenEstimate is estimated from file sizes alone.
	WrittenTokenEstimate int64
	BinaryFiles          int
	// FailedFiles counts every Failure: the ones added to the Stream and the
	// files it could not read.
	FailedFiles int
}

// RenderOptions decide how a file is rendered. Head and Tail count lines from
// the start and end; a negative Head renders the whole file, and zero for both
// renders only the header.
type RenderOptions struct {
	Head        int
	Tail        int
	LineNumbers bool
	// MaxSize skips files larger than this many bytes; zero disables it.
	MaxSize int64
	// SkeletonFunctions and SkeletonTypes reduce source files to signatures
	// and definitions.
	SkeletonFunctions bool
	SkeletonTypes     bool
	// ExtractDocuments converts documents to text, and HTML to Markdown.
	ExtractDocuments bool
	// ExtractMedia replaces audio and video with their container metadata.
	ExtractMedia bool
}

// TemplateEngine holds the parsed text/template instances.
type TemplateEngine struct {
	FileContent *template.Template
	FileError   *template.Template
	FileBinary  *template.Template
	FileCompact *template.Template
	FileImage   *template.Template

	Section *template.Template
	Prompt  *template.Template
	Tree    *template.Template
	Meta    *template.Template

	SectionHeader *template.Template
	SectionFooter *template.Template

	OutputHeader *template.Template
	OutputFooter *template.Template

	// Policy the output format implies, resolved once at compile time so
	// renderers do not branch on the format name.
	EmbedImages   bool
	UseSeparators bool

	// Dir is what relative paths in templates, such as dataURI's, resolve
	// against. Empty means the process's working directory.
	Dir string
}

// Config represents the core library configuration. The yaml tags are the
// documented config-file keys, so this struct is the single definition of that
// schema.
type Config struct {
	FileContentTemplate string `yaml:"file_content_template"`
	FileErrorTemplate   string `yaml:"file_error_template"`
	FileBinaryTemplate  string `yaml:"file_binary_template"`
	FileCompactTemplate string `yaml:"file_compact_template"`
	FileHeaderTemplate  string `yaml:"file_header_template"`

	SectionTemplate string `yaml:"section_template"`
	PromptTemplate  string `yaml:"prompt_template"`
	TreeTemplate    string `yaml:"tree_template"`
	MetaTemplate    string `yaml:"meta_template"`

	SectionHeaderTemplate string `yaml:"section_header_template"`
	SectionFooterTemplate string `yaml:"section_footer_template"`

	OutputHeaderTemplate string `yaml:"output_header_template"`
	OutputFooterTemplate string `yaml:"output_footer_template"`

	OutputFormat string `yaml:"output_format"`
}

// FileContext represents the data provided to file-level templates.
type FileContext struct {
	Path             string
	AbsPath          string
	Size             int64
	OriginalSize     int64
	ModTime          time.Time
	TotalRows        int
	ReadError        string
	IsError          bool
	IsEstimate       bool
	Language         string
	Content          interface{}
	IsBinary         bool
	IsImage          bool
	DataURI          string
	IsCompactView    bool
	FileIndex        int
	SectionFileIndex int
	Global           Plan
	Section          SectionContext
	SkeletonMode     string
	ConvertedFrom    string

	tokenCounter TokenCounter
}

// TokenEstimate estimates the tokens in the rendered content. It is computed
// when a template asks for it, so templates that never do skip the scan.
func (f FileContext) TokenEstimate() int64 {
	tc := f.tokenCounter
	if tc == nil {
		tc = DefaultTokenCounter
	}
	return tc(f.Size, f.Content)
}

// SetTokenCounter sets the estimator behind TokenEstimate.
func (f *FileContext) SetTokenCounter(tc TokenCounter) {
	f.tokenCounter = tc
}

// SectionContext represents the data provided to section templates.
type SectionContext struct {
	Body       string
	Index      int
	TotalFiles int
	TotalSize  int64
	Global     Plan
	IsImplicit bool
}

// PromptContext represents the data provided to custom text prompt templates.
type PromptContext struct {
	Body    string
	Global  Plan
	Section SectionContext
}

// TreeContext represents the data provided to tree templates.
type TreeContext struct {
	Body    string
	Global  Plan
	Section SectionContext
}

// MetaContext represents the data provided to metadata block templates.
type MetaContext struct {
	Body    string
	Fields  map[string]string
	Global  Plan
	Section SectionContext
}

// HeaderContext represents the data provided to the overall output header.
type HeaderContext struct {
	Global Plan
}

// FooterContext represents the data provided to the overall output footer.
type FooterContext struct {
	Global Plan
}

// TokenCounter estimates token count from content.
type TokenCounter func(size int64, content interface{}) int64

// Tokenizer estimates token count from content.
type Tokenizer interface {
	Estimate(size int64, content interface{}) int64
}

// Failure is an input that could not be read, so it is missing from the
// output. Discovery returns the failures it meets and a Stream reports the
// ones added to it, or met rendering, in output order, so each failure is
// counted once, in the Stream's FailedFiles.
type Failure struct {
	Kind FailureKind
	// Path names the input the same way however it was reached: its
	// absolute path, an archive's absolute path or URL followed by "/" and
	// the entry's path in it, a URL, or "-" for stdin.
	Path string
	// Display is the input as output shows it.
	Display string
	Err     error
}

// FailureKind is the step an input failed at.
type FailureKind int

const (
	// FailedStat is a named path that could not be stat'd.
	FailedStat FailureKind = iota
	// FailedURL is a URL that could not be parsed or downloaded.
	FailedURL
	// FailedWalk is a directory or file below a named one that could not be
	// read or stat'd.
	FailedWalk
	// FailedArchive is an archive that could not be expanded.
	FailedArchive
	// FailedArchiveEntry is an entry of an archive that could not be read.
	FailedArchiveEntry
	// FailedRead is a file that could not be opened or read, as it rendered,
	// or stdin that could not be read.
	FailedRead
)

func (k FailureKind) String() string {
	switch k {
	case FailedStat:
		return "stat"
	case FailedURL:
		return "url"
	case FailedWalk:
		return "walk"
	case FailedArchive:
		return "archive"
	case FailedArchiveEntry:
		return "archive entry"
	case FailedRead:
		return "read"
	}
	return "FailureKind(" + strconv.Itoa(int(k)) + ")"
}
