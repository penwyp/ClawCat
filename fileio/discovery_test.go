package fileio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverFiles(t *testing.T) {
	// Create temporary directory with test files
	tempDir := t.TempDir()

	// Create JSONL files
	jsonlFiles := []string{
		"conversation1.jsonl",
		"conversation2.jsonl",
		"data.jsonl",
	}

	for _, filename := range jsonlFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Create non-JSONL files that should be ignored
	otherFiles := []string{
		"readme.txt",
		"data.json",
		"config.yaml",
	}

	for _, filename := range otherFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	// Test directory discovery
	files, err := DiscoverFiles(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 3)

	// Verify all JSONL files were found
	for _, jsonlFile := range jsonlFiles {
		expectedPath := filepath.Join(tempDir, jsonlFile)
		assert.Contains(t, files, expectedPath)
	}
}

func TestDiscoverFiles_SingleFile(t *testing.T) {
	// Create a single JSONL file
	tempDir := t.TempDir()
	jsonlFile := filepath.Join(tempDir, "test.jsonl")
	err := os.WriteFile(jsonlFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Test single file discovery
	files, err := DiscoverFiles(jsonlFile)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, jsonlFile, files[0])
}

func TestDiscoverFiles_NonJSONLFile(t *testing.T) {
	// Create a non-JSONL file
	tempDir := t.TempDir()
	textFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(textFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Test that non-JSONL file is not discovered
	files, err := DiscoverFiles(textFile)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestDiscoverFiles_NonExistentPath(t *testing.T) {
	// Test with non-existent path
	_, err := DiscoverFiles("/nonexistent/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path does not exist")
}

func TestDiscoverFiles_EmptyDirectory(t *testing.T) {
	// Create empty directory
	tempDir := t.TempDir()

	files, err := DiscoverFiles(tempDir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestDiscoverFiles_Subdirectories(t *testing.T) {
	// Create directory structure with subdirectories
	tempDir := t.TempDir()
	subDir1 := filepath.Join(tempDir, "sub1")
	subDir2 := filepath.Join(tempDir, "sub2")
	
	err := os.MkdirAll(subDir1, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(subDir2, 0755)
	require.NoError(t, err)

	// Create JSONL files in different locations
	files := map[string]string{
		filepath.Join(tempDir, "root.jsonl"):   "root",
		filepath.Join(subDir1, "sub1.jsonl"):   "sub1",
		filepath.Join(subDir2, "sub2.jsonl"):   "sub2",
		filepath.Join(subDir1, "nested.jsonl"): "nested",
	}

	for path, content := range files {
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Discover all files
	discovered, err := DiscoverFiles(tempDir)
	require.NoError(t, err)
	assert.Len(t, discovered, 4)

	// Verify all files were found
	for path := range files {
		assert.Contains(t, discovered, path)
	}
}

func TestDiscoverFiles_CaseInsensitive(t *testing.T) {
	// Create files with different case extensions
	tempDir := t.TempDir()
	
	testFiles := []string{
		"lower.jsonl",
		"UPPER.JSONL",
		"Mixed.JsOnL",
	}

	for _, filename := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// All files should be discovered regardless of case
	files, err := DiscoverFiles(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 3)
}