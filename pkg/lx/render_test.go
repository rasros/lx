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

	proc := NewProcessor(engine, global, nil) // Fixed: nil for onError callback
	file := NewBufferInputFile("slice.txt", []byte("1\n2\n3\n4\n5\n"))

	// Set the config on the file itself, which RenderPrepared uses
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

	// Expect lines 1 and 2
	if !strings.Contains(got, "1\n2\n") {
		t.Errorf("Output missing expected head (1, 2). Got:\n%s", got)
	}
	// Expect NO line 3
	if strings.Contains(got, "3\n") {
		t.Errorf("Output contains excluded line (3). Got:\n%s", got)
	}
}
