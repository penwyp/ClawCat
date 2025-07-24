package fileio

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPathDiscovery(t *testing.T) {
	pd := NewPathDiscovery()
	require.NotNil(t, pd)

	// Should have default search paths
	assert.NotEmpty(t, pd.searchPaths)

	// Should have default filters
	assert.Len(t, pd.filters, 2)
}

func TestPathDiscovery_AddSearchPath(t *testing.T) {
	pd := NewPathDiscovery()
	initialCount := len(pd.searchPaths)

	pd.AddSearchPath("/custom/path")
	assert.Len(t, pd.searchPaths, initialCount+1)
	assert.Contains(t, pd.searchPaths, "/custom/path")
}

func TestPathDiscovery_AddSearchPath_WithTilde(t *testing.T) {
	pd := NewPathDiscovery()
	pd.AddSearchPath("~/test/path")

	// Should expand the tilde
	homeDir, _ := os.UserHomeDir()
	expectedPath := filepath.Join(homeDir, "test/path")
	assert.Contains(t, pd.searchPaths, expectedPath)
}

func TestPathDiscovery_AddFilter(t *testing.T) {
	pd := NewPathDiscovery()
	initialCount := len(pd.filters)

	customFilter := func(path string) bool { return true }
	pd.AddFilter(customFilter)

	assert.Len(t, pd.filters, initialCount+1)
}

func TestPathDiscovery_Discover(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "test-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create a fake conversation file
	conversationFile := filepath.Join(projectDir, "conversation-12345-67890.jsonl")
	err = os.WriteFile(conversationFile, []byte(`{"type":"message","model":"claude-3-5-sonnet-20241022"}`), 0644)
	require.NoError(t, err)

	// Create path discovery with our test directory
	pd := NewPathDiscovery()
	pd.searchPaths = []string{tempDir} // Override with test directory

	discovered, err := pd.Discover()
	require.NoError(t, err)

	// Should find our test project
	assert.Len(t, discovered, 1)
	assert.Equal(t, projectDir, discovered[0].Path)
	assert.Equal(t, 1, discovered[0].ProjectCount)
	assert.True(t, discovered[0].Size > 0)
}

func TestPathDiscovery_Discover_EmptyDirectory(t *testing.T) {
	// Create empty directory
	tempDir := t.TempDir()
	emptyDir := filepath.Join(tempDir, "empty-project")
	err := os.MkdirAll(emptyDir, 0755)
	require.NoError(t, err)

	pd := NewPathDiscovery()
	pd.searchPaths = []string{tempDir}

	discovered, err := pd.Discover()
	require.NoError(t, err)

	// Should not find the empty directory
	assert.Empty(t, discovered)
}

func TestPathDiscovery_Discover_NonExistentPath(t *testing.T) {
	pd := NewPathDiscovery()
	pd.searchPaths = []string{"/nonexistent/path"}

	discovered, err := pd.Discover()
	require.NoError(t, err)

	// Should handle non-existent paths gracefully
	assert.Empty(t, discovered)
}

func TestGetDefaultSearchPaths(t *testing.T) {
	paths := getDefaultSearchPaths()
	assert.NotEmpty(t, paths)

	// Should have platform-specific paths
	switch runtime.GOOS {
	case "darwin":
		found := false
		for _, path := range paths {
			if filepath.Base(filepath.Dir(path)) == ".claude" ||
				filepath.Base(filepath.Dir(path)) == ".config" ||
				filepath.Base(filepath.Dir(filepath.Dir(path))) == "Application Support" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have macOS-specific paths")

	case "linux":
		found := false
		for _, path := range paths {
			if filepath.Base(filepath.Dir(path)) == ".claude" ||
				filepath.Base(filepath.Dir(path)) == ".config" ||
				filepath.Base(filepath.Dir(filepath.Dir(path))) == "share" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have Linux-specific paths")

	case "windows":
		found := false
		for _, path := range paths {
			if filepath.Base(filepath.Dir(path)) == "claude" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have Windows-specific paths")
	}
}

func TestIsValidClaudeDirectory(t *testing.T) {
	// Test empty directory
	emptyDir := t.TempDir()
	assert.False(t, isValidClaudeDirectory(emptyDir))

	// Test directory with conversation file
	validDir := t.TempDir()
	conversationFile := filepath.Join(validDir, "conversation-12345-67890.jsonl")
	err := os.WriteFile(conversationFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.True(t, isValidClaudeDirectory(validDir))

	// Test directory with UUID-like filename
	validDir2 := t.TempDir()
	uuidFile := filepath.Join(validDir2, "550e8400-e29b-41d4-a716-446655440000")
	err = os.WriteFile(uuidFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.True(t, isValidClaudeDirectory(validDir2))

	// Test directory with regular files only
	invalidDir := t.TempDir()
	regularFile := filepath.Join(invalidDir, "regular.txt")
	err = os.WriteFile(regularFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.False(t, isValidClaudeDirectory(invalidDir))
}

func TestHasConversationFiles(t *testing.T) {
	// Test directory without conversation files
	noConvDir := t.TempDir()
	regularFile := filepath.Join(noConvDir, "regular.txt")
	err := os.WriteFile(regularFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.False(t, hasConversationFiles(noConvDir))

	// Test directory with JSONL file
	jsonlDir := t.TempDir()
	jsonlFile := filepath.Join(jsonlDir, "conversation.jsonl")
	err = os.WriteFile(jsonlFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.True(t, hasConversationFiles(jsonlDir))

	// Test directory with UUID-like filename
	uuidDir := t.TempDir()
	uuidFile := filepath.Join(uuidDir, "550e8400-e29b-41d4-a716-446655440000")
	err = os.WriteFile(uuidFile, []byte("test"), 0644)
	require.NoError(t, err)
	assert.True(t, hasConversationFiles(uuidDir))
}

func TestGetPathStats(t *testing.T) {
	// Create test directory with conversation files
	testDir := t.TempDir()

	// Create conversation files
	files := []string{
		"conversation-1.jsonl",
		"conversation-2.jsonl",
		"550e8400-e29b-41d4-a716-446655440000",
	}

	totalExpectedSize := int64(0)
	for _, filename := range files {
		filePath := filepath.Join(testDir, filename)
		content := []byte("test content for " + filename)
		err := os.WriteFile(filePath, content, 0644)
		require.NoError(t, err)
		totalExpectedSize += int64(len(content))
	}

	// Create a regular file that should be ignored
	regularFile := filepath.Join(testDir, "regular.txt")
	err := os.WriteFile(regularFile, []byte("ignored"), 0644)
	require.NoError(t, err)

	stats, err := getPathStats(testDir)
	require.NoError(t, err)

	assert.Equal(t, testDir, stats.Path)
	assert.Equal(t, 3, stats.ProjectCount) // Only conversation files counted
	assert.Equal(t, totalExpectedSize, stats.Size)
	assert.False(t, stats.LastModified.IsZero())
}

func TestGetPathStats_NonExistentPath(t *testing.T) {
	_, err := getPathStats("/nonexistent/path")
	assert.Error(t, err)
}

func TestDiscoverDataPaths(t *testing.T) {
	// This is an integration test that uses the actual file system
	// It may not find any paths on a clean system, which is fine
	paths, err := DiscoverDataPaths()
	require.NoError(t, err)

	// Should return slice (may be empty on systems without Claude data)
	assert.NotNil(t, paths)

	// If paths are found, they should be valid directories
	for _, path := range paths {
		info, err := os.Stat(path)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	}
}
