package lx

import (
	"bytes"
	"strings"
	"testing"
)

func TestProcessor_RenderFile(t *testing.T) {
	cfg := NewConfig()
	engine, _ := CompileTemplates(cfg)

	global := GlobalContext{TotalFiles: 1}

	proc := newProcessor(engine, global, nil)
	file := NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))

	file.Config = RunnerConfig{Head: 2, Tail: 0}

	scratch := make([]byte, 1024)

	item := preparedItem{
		raw:              file,
		section:          &SectionContext{},
		fileIndexGlobal:  1,
		fileIndexSection: 1,
	}

	var buf bytes.Buffer
	err := proc.RenderPrepared(&buf, item, scratch)
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()

	if !strings.Contains(got, "1\n2\n") {
		t.Errorf("Output missing expected head (1, 2). Got:\n%s", got)
	}
	if strings.Contains(got, "3\n") {
		t.Errorf("Output contains excluded line (3). Got:\n%s", got)
	}
}

func TestRenderFormats(t *testing.T) {
	// Setup standard file data
	file := NewBufferInputFile("test.go", []byte("package main\nfunc main() {}"))
	// IMPORTANT: Set Head to -1 to ensure IsCompactView is false.
	// Default (0,0) triggers compact view logic in render.go.
	file.Config = RunnerConfig{Head: -1}

	global := GlobalContext{TotalFiles: 1, TotalSize: 100}

	tests := []struct {
		name         string
		format       string
		expectedTags []string
	}{
		{
			name:   "XML Format",
			format: "xml",
			expectedTags: []string{
				"<document index=",
				"<source>test.go</source>",
				"<document_content>",
				"package main",
				"</document>",
			},
		},
		{
			name:   "HTML Format",
			format: "html",
			expectedTags: []string{
				"<article id=\"file-",
				"<header>",
				"<strong>test.go</strong>",
				"<code class=\"language-go\">",
			},
		},
		{
			name:   "Markdown Format",
			format: "markdown",
			expectedTags: []string{
				"```go",
				"package main",
				"```",
				"test.go (",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.OutputFormat = tt.format
			engine, err := CompileTemplates(cfg)
			if err != nil {
				t.Fatal(err)
			}

			proc := newProcessor(engine, global, nil)
			scratch := make([]byte, 1024)

			// Prepare Item
			item := preparedItem{
				raw:              file,
				section:          &SectionContext{},
				fileIndexGlobal:  1,
				fileIndexSection: 1,
			}

			var buf bytes.Buffer
			if err := proc.RenderPrepared(&buf, item, scratch); err != nil {
				t.Fatalf("RenderPrepared failed: %v", err)
			}

			output := buf.String()
			for _, tag := range tt.expectedTags {
				if !strings.Contains(output, tag) {
					t.Errorf("Output for %s missing expected tag/content: %q\nGot:\n%s", tt.format, tag, output)
				}
			}
		})
	}
}
