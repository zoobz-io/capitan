package examples

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExamplesBuild verifies all example files can be compiled.
// Examples use //go:build ignore so they don't affect coverage metrics.
// This test uses 'go build' to verify they compile correctly.
func TestExamplesBuild(t *testing.T) {
	examples := []string{
		"logging-evolution/before/main.go",
		"logging-evolution/after/main.go",
		"async-notifications/before/main.go",
		"async-notifications/after/main.go",
		"decoupled-services/before/main.go",
		"decoupled-services/after/main.go",
	}

	examplesDir := getExamplesDir(t)

	for _, example := range examples {
		t.Run(example, func(t *testing.T) {
			filePath := filepath.Join(examplesDir, example)

			// Verify file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Fatalf("example file does not exist: %s", filePath)
			}

			// Use 'go build' to verify the file compiles
			// -o /dev/null prevents creating an executable
			cmd := exec.Command("go", "build", "-o", "/dev/null", filePath)
			cmd.Dir = examplesDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("example %s failed to build: %v\n%s", example, err, output)
			}
		})
	}
}

func getExamplesDir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	return dir
}
