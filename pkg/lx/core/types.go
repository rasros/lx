package core

import (
	"text/template"
	"time"
)

// GlobalContext holds metadata about the entire execution.
type GlobalContext struct {
	TotalFiles        int
	TotalSize         int64
	SizeUnknown       bool
	TotalWrittenBytes int64
	TotalRows         int64
	TokenEstimate     int64
	TotalSections     int
	WorkDir           string
	Metadata          map[string]string

	SkippedBySize int
	BinaryFiles   int
	FailedFiles   int
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
	Stats        *template.Template

	// Policy the output format implies, resolved once at compile time so
	// renderers do not branch on the format name.
	EmbedImages   bool
	UseSeparators bool
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
	StatsTemplate        string `yaml:"stats_template"`

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
	Global           GlobalContext
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
	Global     GlobalContext
	IsImplicit bool
}

// PromptContext represents the data provided to custom text prompt templates.
type PromptContext struct {
	Body    string
	Global  GlobalContext
	Section SectionContext
}

// TreeContext represents the data provided to tree templates.
type TreeContext struct {
	Body    string
	Global  GlobalContext
	Section SectionContext
}

// MetaContext represents the data provided to metadata block templates.
type MetaContext struct {
	Body    string
	Fields  map[string]string
	Global  GlobalContext
	Section SectionContext
}

// HeaderContext represents the data provided to the overall output header.
type HeaderContext struct {
	Global GlobalContext
}

// FooterContext represents the data provided to the overall output footer.
type FooterContext struct {
	Global GlobalContext
}

// StatsContext represents the data provided to the statistics summary template.
type StatsContext struct {
	Global       GlobalContext
	ColorEnabled bool
}

// TokenCounter estimates token count from content.
type TokenCounter func(size int64, content interface{}) int64

// Tokenizer estimates token count from content.
type Tokenizer interface {
	Estimate(size int64, content interface{}) int64
}
