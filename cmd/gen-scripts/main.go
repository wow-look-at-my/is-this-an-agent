// Command gen-scripts writes the standalone shell scripts from the Go roster
// and scripts/engine.sh.
//
//	go run ./cmd/gen-scripts
//
// Run it from the repository root after changing the roster or the engine.
// TestGeneratedScriptsAreUpToDate fails the build if the committed scripts do
// not match what this writes, so CI catches a forgotten regeneration rather
// than shipping a script that silently disagrees with the library.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	agent "github.com/wow-look-at-my/is-this-an-agent"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gen-scripts: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	const dir = "scripts"
	if _, err := os.Stat(filepath.Join(dir, "engine.sh")); err != nil {
		return fmt.Errorf("run me from the repository root: %w", err)
	}
	for name, body := range agent.GeneratedScripts() {
		path := filepath.Join(dir, name)
		// 0o755: these are meant to be executed directly, and a script that
		// arrives without its exec bit is a script nobody can run.
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	return nil
}
