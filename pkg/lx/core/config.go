package core

// NewConfig returns a default configuration.
func NewConfig() *Config {
	return &Config{
		OutputFormat:       "markdown",
		IgnoreEnabled:      true,
		IgnoreHidden:       true,
		IgnoreFileSymlinks: false,
		IgnoreDirSymlinks:  true,
		ExtractDocuments:   true,
	}
}

// Merge applies non-zero fields from src to dst.
func Merge(dst *Config, src *Config) {
	if src.FileContentTemplate != "" {
		dst.FileContentTemplate = src.FileContentTemplate
	}
	if src.FileErrorTemplate != "" {
		dst.FileErrorTemplate = src.FileErrorTemplate
	}
	if src.FileBinaryTemplate != "" {
		dst.FileBinaryTemplate = src.FileBinaryTemplate
	}
	if src.FileCompactTemplate != "" {
		dst.FileCompactTemplate = src.FileCompactTemplate
	}
	if src.FileHeaderTemplate != "" {
		dst.FileHeaderTemplate = src.FileHeaderTemplate
	}

	if src.SectionTemplate != "" {
		dst.SectionTemplate = src.SectionTemplate
	}
	if src.PromptTemplate != "" {
		dst.PromptTemplate = src.PromptTemplate
	}
	if src.TreeTemplate != "" {
		dst.TreeTemplate = src.TreeTemplate
	}

	if src.SectionHeaderTemplate != "" {
		dst.SectionHeaderTemplate = src.SectionHeaderTemplate
	}
	if src.SectionFooterTemplate != "" {
		dst.SectionFooterTemplate = src.SectionFooterTemplate
	}

	if src.OutputHeaderTemplate != "" {
		dst.OutputHeaderTemplate = src.OutputHeaderTemplate
	}
	if src.OutputFooterTemplate != "" {
		dst.OutputFooterTemplate = src.OutputFooterTemplate
	}
	if src.StatsTemplate != "" {
		dst.StatsTemplate = src.StatsTemplate
	}

	if src.OutputFormat != "" {
		dst.OutputFormat = src.OutputFormat
	}
	if !src.IgnoreDirSymlinks {
		dst.IgnoreDirSymlinks = false
	}
	if src.IgnoreFileSymlinks {
		dst.IgnoreFileSymlinks = true
	}
	if !src.IgnoreHidden {
		dst.IgnoreHidden = false
	}
}
