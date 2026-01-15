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

	// Processor is stateless regarding runner config
	proc := NewProcessor(engine, global)

	// Create a 5-line file with a specific config attached
	file := NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))
	file.Config = RunnerConfig{Head: 2}

	// Create scratch buffer required by Render
	scratch := make([]byte, 1024)

	var buf bytes.Buffer
	err := proc.Render(&buf, file, 1, scratch)
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	// Should only have first 2 lines based on Head: 2
	if !strings.Contains(got, "1\n2\n") || strings.Contains(got, "3\n") {
		t.Errorf("Slicing failed, got:\n%s", got)
	}
}
