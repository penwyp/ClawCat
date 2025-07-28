package fileio

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/claudecat/models"
)

// hasAssistantMessages checks if a file contains assistant messages
func hasAssistantMessages(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	// Check first 50 lines for assistant messages
	for scanner.Scan() && lineCount < 50 {
		line := scanner.Text()
		lineCount++

		if strings.TrimSpace(line) == "" {
			continue
		}

		var data map[string]interface{}
		if err := sonic.Unmarshal([]byte(line), &data); err != nil {
			continue
		}

		// Check if this is an assistant message with usage data
		if typeStr, ok := data["type"].(string); ok && typeStr == "assistant" {
			if message, ok := data["message"].(map[string]interface{}); ok {
				if usage, ok := message["usage"].(map[string]interface{}); ok {
					// Check if usage has any tokens
					for _, field := range []string{"input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"} {
						if val, ok := usage[field]; ok {
							if tokens, ok := val.(float64); ok && tokens > 0 {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// extractProjectFromPath extracts the project name from a Claude projects directory path
// For example: /Users/user/.claude/projects/-Users-user-Dat-MoviePilot/conversation.jsonl -> MoviePilot
func extractProjectFromPath(filePath string) string {
	// Get the directory path
	dir := filepath.Dir(filePath)

	// Get the project directory name (last component)
	projectDir := filepath.Base(dir)

	// Handle the special format where paths are converted to dashes
	// Format: -Users-user-path-to-project
	if strings.HasPrefix(projectDir, "-") {
		// Split by dash and get the last meaningful part
		parts := strings.Split(projectDir, "-")
		if len(parts) > 0 {
			// Return the last non-empty part
			for i := len(parts) - 1; i >= 0; i-- {
				if parts[i] != "" {
					return parts[i]
				}
			}
		}
	}

	// If not in the expected format, just return the directory name
	return projectDir
}

// convertRawToUsageEntry converts raw JSON data to a UsageEntry with cost calculation
func convertRawToUsageEntry(data map[string]interface{}, mode models.CostMode) (models.UsageEntry, error) {
	entry, hasUsage := extractUsageEntry(data)
	if !hasUsage {
		// Check the type to provide better error message
		if typeStr, ok := data["type"].(string); ok && typeStr == "user" {
			return entry, fmt.Errorf("not an assistant message: type=%s", typeStr)
		}
		return entry, fmt.Errorf("no usage data found")
	}

	// Extract additional fields for testing/legacy compatibility
	if sessionID, ok := data["sessionId"].(string); ok {
		entry.SessionID = sessionID
	}
	if sessionID, ok := data["session_id"].(string); ok {
		entry.SessionID = sessionID
	}
	if msgID, ok := data["message_id"].(string); ok && entry.MessageID == "" {
		entry.MessageID = msgID
	}
	if reqID, ok := data["request_id"].(string); ok && entry.RequestID == "" {
		entry.RequestID = reqID
	}
	// Also try top-level requestId for conversation log format
	if reqID, ok := data["requestId"].(string); ok && entry.RequestID == "" {
		entry.RequestID = reqID
	}

	// Calculate cost
	pricing := models.GetPricing(entry.Model)
	entry.CostUSD = entry.CalculateCost(pricing)

	// Don't normalize model name in tests - preserve original
	// entry.NormalizeModel()

	return entry, nil
}

// extractUsageEntry extracts usage entry from JSON data
func extractUsageEntry(data map[string]interface{}) (models.UsageEntry, bool) {
	var entry models.UsageEntry
	var hasUsage bool

	// Extract timestamp
	if timestampStr, ok := data["timestamp"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			entry.Timestamp = ts
		} else {
			return entry, false
		}
	} else {
		return entry, false
	}

	// Handle different message types
	typeStr, hasType := data["type"].(string)

	if typeStr == "assistant" {
		// Claude Code session format
		if message, ok := data["message"].(map[string]interface{}); ok {
			// Extract model
			if model, ok := message["model"].(string); ok {
				entry.Model = model
			}

			// Extract message ID
			if id, ok := message["id"].(string); ok {
				entry.MessageID = id
			}

			// Extract usage
			if usage, ok := message["usage"].(map[string]interface{}); ok {
				if val, ok := usage["input_tokens"]; ok {
					entry.InputTokens = int(val.(float64))
					hasUsage = true
				}
				if val, ok := usage["output_tokens"]; ok {
					entry.OutputTokens = int(val.(float64))
					hasUsage = true
				}
				if val, ok := usage["cache_creation_input_tokens"]; ok {
					entry.CacheCreationTokens = int(val.(float64))
				}
				if val, ok := usage["cache_read_input_tokens"]; ok {
					entry.CacheReadTokens = int(val.(float64))
				}
			}
		}
	} else if typeStr == "message" || !hasType {
		// Direct API format or legacy format (no type field)
		if model, ok := data["model"].(string); ok {
			entry.Model = model
		}

		if usage, ok := data["usage"].(map[string]interface{}); ok {
			if val, ok := usage["input_tokens"]; ok {
				entry.InputTokens = int(val.(float64))
				hasUsage = true
			}
			if val, ok := usage["output_tokens"]; ok {
				entry.OutputTokens = int(val.(float64))
				hasUsage = true
			}
			if val, ok := usage["cache_creation_tokens"]; ok {
				entry.CacheCreationTokens = int(val.(float64))
			}
			if val, ok := usage["cache_read_tokens"]; ok {
				entry.CacheReadTokens = int(val.(float64))
			}
		}
	}

	// Extract request ID (at top level for both message types)
	if requestID, ok := data["request_id"].(string); ok {
		entry.RequestID = requestID
	}

	// Calculate total tokens
	entry.TotalTokens = entry.InputTokens + entry.OutputTokens + entry.CacheCreationTokens + entry.CacheReadTokens

	return entry, hasUsage
}