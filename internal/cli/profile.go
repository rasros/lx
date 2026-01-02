package cli

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

func setupProfiling(parsed *ParsedArgs) (func(), error) {
	var onExit []func()

	cleanup := func() {
		for i := len(onExit) - 1; i >= 0; i-- {
			onExit[i]()
		}
	}

	if path, ok := parsed.Globals["cpuprofile"]; ok {
		f, err := os.Create(path)
		if err != nil {
			return cleanup, fmt.Errorf("create cpu profile: %w", err)
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return cleanup, fmt.Errorf("start cpu profile: %w", err)
		}

		onExit = append(onExit, func() {
			pprof.StopCPUProfile()
			f.Close()
		})
	}

	if path, ok := parsed.Globals["memprofile"]; ok {
		onExit = append(onExit, func() {
			f, err := os.Create(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "create mem profile: %v\n", err)
				return
			}
			defer f.Close()

			runtime.GC()

			if err := pprof.WriteHeapProfile(f); err != nil {
				fmt.Fprintf(os.Stderr, "write mem profile: %v\n", err)
			}
		})
	}

	return cleanup, nil
}
