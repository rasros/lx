package main

import (
	"testing"
)

func TestGoldenSystemContext(t *testing.T) {
	dir := setupSectionsFixture(t)
	runTestGolden(t, dir, []goldenTestCase{
		{name: "160_system_context_only", args: []string{"--system-context"}},
		{name: "161_system_context_before_files", args: []string{"--system-context", "main.go"}},
		{name: "162_system_context_after_files", args: []string{"main.go", "--system-context"}},
		{name: "163_system_context_xml", args: []string{"--xml", "--system-context", "main.go"}},
		{name: "164_system_context_bare", args: []string{"--bare", "--system-context", "main.go"}},
		{name: "165_system_context_html", args: []string{"--html", "--system-context", "main.go"}},
		{name: "166_system_context_between_sections", args: []string{
			"-s", "Code", "main.go", "-s", "Environment", "--system-context",
		}},
		{name: "167_system_context_twice", args: []string{"--system-context", "main.go", "--system-context"}},
		{name: "168_system_context_with_tree", args: []string{"--system-context", "-t", "src"}},
		{name: "169_system_context_short_flag", args: []string{"-E", "main.go"}},
	})
}
