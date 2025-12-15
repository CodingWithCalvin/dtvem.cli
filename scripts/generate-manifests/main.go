// Script to generate manifest files from upstream sources.
// Run with: go run ./scripts/generate-manifests [node|python|ruby|all]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./scripts/generate-manifests [node|python|ruby|all]")
		os.Exit(1)
	}

	runtime := os.Args[1]

	// Determine output directory (relative to repo root)
	outputDir := "src/internal/manifest/data"
	if len(os.Args) > 2 {
		outputDir = os.Args[2]
	}

	switch runtime {
	case "node":
		if err := generateNodeManifest(outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Node.js manifest: %v\n", err)
			os.Exit(1)
		}
	case "python":
		if err := generatePythonManifest(outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Python manifest: %v\n", err)
			os.Exit(1)
		}
	case "ruby":
		if err := generateRubyManifest(outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Ruby manifest: %v\n", err)
			os.Exit(1)
		}
	case "all":
		var errors []string
		if err := generateNodeManifest(outputDir); err != nil {
			errors = append(errors, fmt.Sprintf("Node.js: %v", err))
		}
		if err := generatePythonManifest(outputDir); err != nil {
			errors = append(errors, fmt.Sprintf("Python: %v", err))
		}
		if err := generateRubyManifest(outputDir); err != nil {
			errors = append(errors, fmt.Sprintf("Ruby: %v", err))
		}
		if len(errors) > 0 {
			fmt.Fprintf(os.Stderr, "Errors:\n%s\n", strings.Join(errors, "\n"))
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown runtime: %s\n", runtime)
		os.Exit(1)
	}

	fmt.Println("Done!")
}

// Manifest represents our manifest JSON structure
type Manifest struct {
	Schema   string                            `json:"$schema,omitempty"`
	Version  int                               `json:"version"`
	Versions map[string]map[string]*Download   `json:"versions"`
}

// Download contains URL and SHA256 for a binary
type Download struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// writeManifest writes a manifest to a JSON file
func writeManifest(m *Manifest, outputDir, filename string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	path := filepath.Join(outputDir, filename)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Add trailing newline
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
}
