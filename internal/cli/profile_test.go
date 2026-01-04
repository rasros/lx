package cli

import (
	"os"
	"testing"
)

func TestSetupProfiling_NoOp(t *testing.T) {
	parsed := &ParsedArgs{
		Globals: make(map[string]string),
	}

	cleanup, err := setupProfiling(parsed)
	if err != nil {
		t.Fatalf("setupProfiling unexpected error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup function should not be nil")
	}
	cleanup()
}

func TestSetupProfiling_CPU(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "cpu_prof")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())

	parsed := &ParsedArgs{
		Globals: map[string]string{
			"cpuprofile": tmpFile.Name(),
		},
	}

	cleanup, err := setupProfiling(parsed)
	if err != nil {
		return
	}
	defer cleanup()

	if _, err := os.Stat(tmpFile.Name()); os.IsNotExist(err) {
		t.Errorf("Expected cpuprofile file to be created at %s", tmpFile.Name())
	}
}
