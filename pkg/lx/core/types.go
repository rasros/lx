package core

import (
	"text/template"
	"time"
)

// GlobalContext holds metadata about the entire execution.
type GlobalContext struct {
	TotalFiles        int
	TotalSize         int64
	TotalWrittenBytes int64
	TotalRows         int64
	TokenEstimate     int64
	TotalSections     int
	WorkDir           string
	Metadata          map[string]string
}

// RunnerConfig defines slicing and formatting state for the rendering processor.
type RunnerConfig struct {
	Head              int
	Tail              int
	MaxSize           int64
	LineNumbers       bool
	SkeletonFunctions bool
	SkeletonTypes     bool
	ExpandArchives    bool
	ExtractDocuments  bool
	ShowHidden        bool
	FollowDirSymlinks bool
	SkipFileSymlinks  bool
	NoIgnore          bool
}

// TemplateEngine holds the parsed text/template instances.
type TemplateEngine struct {
	FileContent *template.Template
	FileError   *template.Template
	FileBinary  *template.Template
	FileCompact *template.Template

	Section *template.Template
	Prompt  *template.Template
	Tree    *template.Template

	SectionHeader *template.Template
	SectionFooter *template.Template

	OutputHeader *template.Template
	OutputFooter *template.Template
	Stats        *template.Template
}

// Config represents the core library configuration.
type Config struct {
	FileContentTemplate string
	FileErrorTemplate   string
	FileBinaryTemplate  string
	FileCompactTemplate string
	FileHeaderTemplate  string

	SectionTemplate string
	PromptTemplate  string
	TreeTemplate    string

	SectionHeaderTemplate string
	SectionFooterTemplate string

	OutputHeaderTemplate string
	OutputFooterTemplate string
	StatsTemplate        string

	OutputFormat string
}

// FileContext represents the data provided to file-level templates.
type FileContext struct {
	Path             string
	AbsPath          string
	Size             int64
	ModTime          time.Time
	TotalRows        int
	ReadError        string
	IsError          bool
	TokenEstimate    int64
	IsEstimate       bool
	Language         string
	Content          interface{}
	IsBinary         bool
	IsImage          bool
	IsCompactView    bool
	FileIndex        int
	SectionFileIndex int
	Global           GlobalContext
	Section          SectionContext
	SkeletonMode     string
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
