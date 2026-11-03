package cli

import (
	"fmt"
	"text/template"

	"github.com/rasros/lx/internal/ansi"
	"github.com/rasros/lx/pkg/lx"
	"github.com/rasros/lx/pkg/lx/templatex"
)

// StatsContext is what the stats template renders: the report of a run and
// whether the stats may be colored.
type StatsContext struct {
	Global       lx.Report
	ColorEnabled bool
}

// 128k matches GPT-4o context, 200k matches Claude Opus.
const (
	tokenWarnThreshold   int64 = 128_000
	tokenDangerThreshold int64 = 200_000
)

const defaultStatsTemplate = `{{ accent .ColorEnabled "▎" }} ` +
	`{{ commafy .Global.TotalFiles }} {{ plural .Global.TotalFiles "file" "files" }} {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.TotalRows }} {{ plural .Global.TotalRows "row" "rows" }} {{ dim .ColorEnabled "·" }} ` +
	`{{ commafy .Global.TotalSections }} {{ plural .Global.TotalSections "section" "sections" }}` +
	`{{ if not .Global.SizeUnknown }} {{ dim .ColorEnabled "·" }} {{ humanize .Global.TotalSize }}{{ end }}` + "\n" +
	`  {{ tokenLabel .Global.WrittenTokenEstimate .ColorEnabled }}` + "\n" +
	`{{ if or .Global.SkippedBySize .Global.BinaryFiles .Global.FailedFiles }}  ` +
	`{{ $sep := "" }}` +
	`{{ if .Global.SkippedBySize }}{{ commafy .Global.SkippedBySize }} skipped (size){{ $sep = printf " %s " (dim .ColorEnabled "·") }}{{ end }}` +
	`{{ if .Global.BinaryFiles }}{{ $sep }}{{ commafy .Global.BinaryFiles }} binary{{ $sep = printf " %s " (dim .ColorEnabled "·") }}{{ end }}` +
	`{{ if .Global.FailedFiles }}{{ $sep }}{{ commafy .Global.FailedFiles }} failed{{ end }}` +
	"\n" + `{{ end }}`

// compileStats parses the stats template, or the default one when tmpl is
// empty, failing on a field StatsContext lacks. It can call every function
// the other templates can, with dataURI resolving against dir, and the
// coloring helpers below.
func compileStats(tmpl, dir string) (*template.Template, error) {
	if tmpl == "" {
		tmpl = defaultStatsTemplate
	}
	t, err := template.New("stats").Funcs(templatex.Funcs(dir)).Funcs(statsFuncs()).Parse(tmpl)
	if err == nil {
		// The stats render after the output, too late to fail cleanly.
		err = templatex.CheckFields(t, StatsContext{})
	}
	if err != nil {
		return nil, fmt.Errorf("stats template: %w", err)
	}
	return t, nil
}

// statsFuncs returns the coloring helpers of the stats template.
func statsFuncs() template.FuncMap {
	return template.FuncMap{
		"accent": func(enabled bool, s string) string {
			return ansi.New(enabled).Paint(s, ansi.Cyan)
		},
		"dim": func(enabled bool, s string) string {
			return ansi.New(enabled).Paint(s, ansi.Dim)
		},
		"tokenLabel": func(n int64, enabled bool) string {
			label := fmt.Sprintf("~%s tokens", templatex.Commafy(n))
			color := ansi.Green
			switch {
			case n >= tokenDangerThreshold:
				color = ansi.Red
			case n >= tokenWarnThreshold:
				color = ansi.Yellow
			}
			return ansi.New(enabled).Paint(label, color, ansi.Bold)
		},
	}
}
