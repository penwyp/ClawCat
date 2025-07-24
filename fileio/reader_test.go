package fileio

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	// Create temporary test file
	tempFile := createTestJSONLFile(t, []string{
		`{"type":"message","timestamp":"2025-01-24T10:00:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_tokens":0,"cache_read_tokens":0}}`,
	})
	defer os.Remove(tempFile)

	reader, err := NewReader(tempFile)
	require.NoError(t, err)
	require.NotNil(t, reader)

	assert.Equal(t, tempFile, reader.filepath)
	assert.NotNil(t, reader.file)
	assert.NotNil(t, reader.scanner)

	err = reader.Close()
	assert.NoError(t, err)
}

func TestNewReader_NonExistentFile(t *testing.T) {
	reader, err := NewReader("/nonexistent/file.jsonl")
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestReader_ReadEntries(t *testing.T) {
	testLines := []string{
		`{"type":"message","timestamp":"2025-01-24T10:00:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_tokens":0,"cache_read_tokens":0}}`,
		`{"type":"message","timestamp":"2025-01-24T10:01:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":200,"output_tokens":75,"cache_creation_tokens":20,"cache_read_tokens":10}}`,
		`{"type":"other","timestamp":"2025-01-24T10:02:00Z"}`, // Should be skipped
		`invalid json line`, // Should be skipped
		`{"type":"message","timestamp":"2025-01-24T10:03:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_tokens":0,"cache_read_tokens":0}}`, // Should be skipped (no tokens)
	}

	tempFile := createTestJSONLFile(t, testLines)
	defer os.Remove(tempFile)

	reader, err := NewReader(tempFile)
	require.NoError(t, err)
	defer reader.Close()

	entries, errors := reader.ReadEntries()

	var collectedEntries []string
	var collectedErrors []error

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				goto done
			}
			collectedEntries = append(collectedEntries, entry.Model)
		case err := <-errors:
			if err != nil {
				collectedErrors = append(collectedErrors, err)
			}
		case <-time.After(100 * time.Millisecond):
			goto done
		}
	}

done:
	// Should have processed 2 valid entries (first two lines)
	assert.Len(t, collectedEntries, 2)
	assert.Equal(t, "claude-3-5-sonnet-20241022", collectedEntries[0])
	assert.Equal(t, "claude-3-5-sonnet-20241022", collectedEntries[1])

	// Should not have any errors for this test
	assert.Empty(t, collectedErrors)
}

func TestReader_ReadAll(t *testing.T) {
	testLines := []string{
		`{"type":"message","timestamp":"2025-01-24T10:00:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_tokens":0,"cache_read_tokens":0}}`,
		`{"type":"message","timestamp":"2025-01-24T10:01:00Z","model":"claude-3-5-haiku-20241022","usage":{"input_tokens":200,"output_tokens":75,"cache_creation_tokens":20,"cache_read_tokens":10}}`,
	}

	tempFile := createTestJSONLFile(t, testLines)
	defer os.Remove(tempFile)

	reader, err := NewReader(tempFile)
	require.NoError(t, err)
	defer reader.Close()

	entries, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// Check first entry
	assert.Equal(t, "claude-3-5-sonnet-20241022", entries[0].Model)
	assert.Equal(t, 100, entries[0].InputTokens)
	assert.Equal(t, 50, entries[0].OutputTokens)
	assert.Equal(t, 150, entries[0].TotalTokens)
	assert.True(t, entries[0].CostUSD > 0) // Should have calculated cost

	// Check second entry
	assert.Equal(t, "claude-3-5-haiku-20241022", entries[1].Model)
	assert.Equal(t, 200, entries[1].InputTokens)
	assert.Equal(t, 75, entries[1].OutputTokens)
	assert.Equal(t, 20, entries[1].CacheCreationTokens)
	assert.Equal(t, 10, entries[1].CacheReadTokens)
	assert.Equal(t, 305, entries[1].TotalTokens)
}

func TestConvertToUsageEntry(t *testing.T) {
	timestamp, _ := time.Parse(time.RFC3339, "2025-01-24T10:00:00Z")
	msg := &RawMessage{
		Type:      "message",
		Timestamp: timestamp,
		Model:     "claude-3-5-sonnet-20241022",
		Usage: struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheCreationTokens int `json:"cache_creation_tokens"`
			CacheReadTokens     int `json:"cache_read_tokens"`
		}{
			InputTokens:         100,
			OutputTokens:        50,
			CacheCreationTokens: 20,
			CacheReadTokens:     10,
		},
	}

	entry, err := convertToUsageEntry(msg)
	require.NoError(t, err)

	assert.Equal(t, timestamp, entry.Timestamp)
	assert.Equal(t, "claude-3-5-sonnet-20241022", entry.Model)
	assert.Equal(t, 100, entry.InputTokens)
	assert.Equal(t, 50, entry.OutputTokens)
	assert.Equal(t, 20, entry.CacheCreationTokens)
	assert.Equal(t, 10, entry.CacheReadTokens)
	assert.Equal(t, 180, entry.TotalTokens)
	assert.True(t, entry.CostUSD > 0)
}

func TestConvertToUsageEntry_MissingModel(t *testing.T) {
	msg := &RawMessage{
		Type:  "message",
		Model: "",
	}

	_, err := convertToUsageEntry(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing model information")
}

func TestReadConversationFile(t *testing.T) {
	testLines := []string{
		`{"type":"message","timestamp":"2025-01-24T10:00:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_tokens":0,"cache_read_tokens":0}}`,
		`{"type":"message","timestamp":"2025-01-24T10:01:00Z","model":"claude-3-5-haiku-20241022","usage":{"input_tokens":200,"output_tokens":75,"cache_creation_tokens":0,"cache_read_tokens":0}}`,
	}

	tempFile := createTestJSONLFile(t, testLines)
	defer os.Remove(tempFile)

	entries, err := ReadConversationFile(tempFile)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "claude-3-5-sonnet-20241022", entries[0].Model)
	assert.Equal(t, "claude-3-5-haiku-20241022", entries[1].Model)
}

func TestReadConversationFile_NonExistent(t *testing.T) {
	entries, err := ReadConversationFile("/nonexistent/file.jsonl")
	assert.Error(t, err)
	assert.Nil(t, entries)
}

func TestStreamConversationFile(t *testing.T) {
	testLines := []string{
		`{"type":"message","timestamp":"2025-01-24T10:00:00Z","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_tokens":0,"cache_read_tokens":0}}`,
	}

	tempFile := createTestJSONLFile(t, testLines)
	defer os.Remove(tempFile)

	entries, errors := StreamConversationFile(tempFile)

	// Should receive one entry
	entry := <-entries
	assert.Equal(t, "claude-3-5-sonnet-20241022", entry.Model)

	// Channel should close
	_, ok := <-entries
	assert.False(t, ok)

	// Should not have errors
	select {
	case err := <-errors:
		assert.NoError(t, err)
	default:
		// No error, which is good
	}
}

func TestStreamConversationFile_NonExistent(t *testing.T) {
	entries, errors := StreamConversationFile("/nonexistent/file.jsonl")

	// Should receive error
	err := <-errors
	assert.Error(t, err)

	// Entries channel should be closed
	_, ok := <-entries
	assert.False(t, ok)
}

// Helper function to create temporary JSONL test files
func createTestJSONLFile(t *testing.T, lines []string) string {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.jsonl")

	content := ""
	for _, line := range lines {
		content += line + "\n"
	}

	err := os.WriteFile(tempFile, []byte(content), 0644)
	require.NoError(t, err)

	return tempFile
}
