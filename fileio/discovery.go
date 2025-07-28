package fileio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverFiles discovers JSONL files in a given path
func DiscoverFiles(path string) ([]string, error) {
	var files []string

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	if info.IsDir() {
		// Search for JSONL files in directory
		err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && strings.HasSuffix(strings.ToLower(walkPath), ".jsonl") {
				files = append(files, walkPath)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Single file
		if strings.HasSuffix(strings.ToLower(path), ".jsonl") {
			files = append(files, path)
		}
	}

	return files, nil
}