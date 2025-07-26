package fileio

import (
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/claudecat/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRawToUsageEntry_ConversationLogFormat(t *testing.T) {
	// Test data representing a Claude Code assistant message with usage data
	jsonData := `{
		"type": "assistant",
		"timestamp": "2024-03-15T10:30:00Z",
		"sessionId": "session-123",
		"requestId": "req-456",
		"message": {
			"id": "msg-789",
			"model": "claude-3-sonnet-20240229",
			"role": "assistant",
			"usage": {
				"input_tokens": 1000,
				"output_tokens": 500,
				"cache_creation_input_tokens": 200,
				"cache_read_input_tokens": 100
			}
		}
	}`

	var rawData map[string]interface{}
	err := sonic.Unmarshal([]byte(jsonData), &rawData)
	require.NoError(t, err)

	entry, err := convertRawToUsageEntry(rawData, models.CostModeCalculated)
	require.NoError(t, err)

	// Verify the conversion
	assert.Equal(t, "claude-3-sonnet-20240229", entry.Model)
	assert.Equal(t, 1000, entry.InputTokens)
	assert.Equal(t, 500, entry.OutputTokens)
	assert.Equal(t, 200, entry.CacheCreationTokens)
	assert.Equal(t, 100, entry.CacheReadTokens)
	assert.Equal(t, 1800, entry.TotalTokens)
	assert.Equal(t, "msg-789", entry.MessageID)
	assert.Equal(t, "req-456", entry.RequestID)
	assert.Equal(t, "session-123", entry.SessionID)

	// Verify timestamp
	expectedTime, _ := time.Parse(time.RFC3339, "2024-03-15T10:30:00Z")
	assert.Equal(t, expectedTime, entry.Timestamp)

	// Verify cost calculation (approximate)
	assert.Greater(t, entry.CostUSD, 0.0)
}

func TestConvertRawToUsageEntry_LegacyFormat(t *testing.T) {
	// Test data representing legacy format
	jsonData := `{
		"timestamp": "2024-03-15T10:30:00Z",
		"model": "claude-3-haiku-20240307",
		"usage": {
			"input_tokens": 500,
			"output_tokens": 250,
			"cache_creation_tokens": 50,
			"cache_read_tokens": 25
		},
		"message_id": "legacy-msg-123",
		"request_id": "legacy-req-456"
	}`

	var rawData map[string]interface{}
	err := sonic.Unmarshal([]byte(jsonData), &rawData)
	require.NoError(t, err)

	entry, err := convertRawToUsageEntry(rawData, models.CostModeCalculated)
	require.NoError(t, err)

	// Verify the conversion
	assert.Equal(t, "claude-3-haiku-20240307", entry.Model)
	assert.Equal(t, 500, entry.InputTokens)
	assert.Equal(t, 250, entry.OutputTokens)
	assert.Equal(t, 50, entry.CacheCreationTokens)
	assert.Equal(t, 25, entry.CacheReadTokens)
	assert.Equal(t, 825, entry.TotalTokens)
	assert.Equal(t, "legacy-msg-123", entry.MessageID)
	assert.Equal(t, "legacy-req-456", entry.RequestID)
}

func TestConvertRawToUsageEntry_NonAssistantMessage(t *testing.T) {
	// Test data representing a user message (should be skipped)
	jsonData := `{
		"type": "user",
		"timestamp": "2024-03-15T10:30:00Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "Hello"}]
		}
	}`

	var rawData map[string]interface{}
	err := sonic.Unmarshal([]byte(jsonData), &rawData)
	require.NoError(t, err)

	_, err = convertRawToUsageEntry(rawData, models.CostModeCalculated)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an assistant message")
}
