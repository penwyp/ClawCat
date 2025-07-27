package sessions

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/penwyp/claudecat/models"
)

// SessionAnalyzer creates session blocks and detects limits
type SessionAnalyzer struct {
	sessionDurationHours int
	sessionDuration      time.Duration
}

// NewSessionAnalyzer creates a new session analyzer with the specified duration
func NewSessionAnalyzer(sessionDurationHours int) *SessionAnalyzer {
	if sessionDurationHours <= 0 {
		sessionDurationHours = 5 // Default to 5 hours
	}

	return &SessionAnalyzer{
		sessionDurationHours: sessionDurationHours,
		sessionDuration:      time.Duration(sessionDurationHours) * time.Hour,
	}
}

// TransformToBlocks processes entries and creates session blocks
func (sa *SessionAnalyzer) TransformToBlocks(entries []models.UsageEntry) []models.SessionBlock {
	if len(entries) == 0 {
		return []models.SessionBlock{}
	}

	// Sort entries by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	var blocks []models.SessionBlock
	var currentBlock *models.SessionBlock

	for _, entry := range entries {
		// Check if we need a new block
		if currentBlock == nil || sa.shouldCreateNewBlock(currentBlock, entry) {
			// Close current block
			if currentBlock != nil {
				sa.finalizeBlock(currentBlock)
				blocks = append(blocks, *currentBlock)

				// Check for gap
				if gap := sa.checkForGap(currentBlock, entry); gap != nil {
					blocks = append(blocks, *gap)
				}
			}

			// Create new block
			currentBlock = sa.createNewBlock(entry)
		}

		// Add entry to current block
		sa.addEntryToBlock(currentBlock, entry)
	}

	// Finalize last block
	if currentBlock != nil {
		sa.finalizeBlock(currentBlock)
		blocks = append(blocks, *currentBlock)
	}

	// Mark active blocks
	sa.markActiveBlocks(blocks)

	return blocks
}

// DetectLimits detects token limit messages from raw JSONL entries
func (sa *SessionAnalyzer) DetectLimits(rawEntries []map[string]interface{}) []models.LimitMessage {
	var limits []models.LimitMessage

	for _, rawData := range rawEntries {
		if limitInfo := sa.detectSingleLimit(rawData); limitInfo != nil {
			limits = append(limits, *limitInfo)
		}
	}

	return limits
}

// shouldCreateNewBlock checks if a new block is needed
func (sa *SessionAnalyzer) shouldCreateNewBlock(block *models.SessionBlock, entry models.UsageEntry) bool {
	if entry.Timestamp.After(block.EndTime) || entry.Timestamp.Equal(block.EndTime) {
		return true
	}

	if len(block.Entries) > 0 {
		lastEntry := block.Entries[len(block.Entries)-1]
		if entry.Timestamp.Sub(lastEntry.Timestamp) >= sa.sessionDuration {
			return true
		}
	}

	return false
}

// roundToHour rounds timestamp to the nearest full hour in UTC
func (sa *SessionAnalyzer) roundToHour(timestamp time.Time) time.Time {
	utc := timestamp.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), 0, 0, 0, time.UTC)
}

// createNewBlock creates a new session block
func (sa *SessionAnalyzer) createNewBlock(entry models.UsageEntry) *models.SessionBlock {
	startTime := sa.roundToHour(entry.Timestamp)
	endTime := startTime.Add(sa.sessionDuration)

	block := &models.SessionBlock{
		StartTime:         startTime,
		EndTime:           endTime,
		Entries:           []models.UsageEntry{},
		TokenCounts:       models.TokenCounts{},
		IsActive:          false,
		IsGap:             false,
		PerModelStats:     make(map[string]map[string]any),
		Models:            []string{},
		SentMessagesCount: 0,
		CostUSD:           0.0,
		LimitMessages:     []models.LimitMessage{},

		// Legacy fields for backward compatibility
		ModelStats:  make(map[string]models.ModelStat),
		TotalCost:   0.0,
		TotalTokens: 0,
	}

	block.GenerateID()
	return block
}

// addEntryToBlock adds entry to block and aggregates data per model
func (sa *SessionAnalyzer) addEntryToBlock(block *models.SessionBlock, entry models.UsageEntry) {
	block.Entries = append(block.Entries, entry)

	// Normalize model name
	rawModel := entry.Model
	if rawModel == "" {
		rawModel = "unknown"
	}
	model := models.NormalizeModelName(rawModel)

	// Initialize per-model stats if not exists
	if _, exists := block.PerModelStats[model]; !exists {
		block.PerModelStats[model] = map[string]any{
			"input_tokens":          0,
			"output_tokens":         0,
			"cache_creation_tokens": 0,
			"cache_read_tokens":     0,
			"cost_usd":              0.0,
			"entries_count":         0,
		}
	}

	// Update per-model stats
	modelStats := block.PerModelStats[model]
	modelStats["input_tokens"] = modelStats["input_tokens"].(int) + entry.InputTokens
	modelStats["output_tokens"] = modelStats["output_tokens"].(int) + entry.OutputTokens
	modelStats["cache_creation_tokens"] = modelStats["cache_creation_tokens"].(int) + entry.CacheCreationTokens
	modelStats["cache_read_tokens"] = modelStats["cache_read_tokens"].(int) + entry.CacheReadTokens
	modelStats["cost_usd"] = modelStats["cost_usd"].(float64) + entry.CostUSD
	modelStats["entries_count"] = modelStats["entries_count"].(int) + 1

	// Update token counts
	block.TokenCounts.InputTokens += entry.InputTokens
	block.TokenCounts.OutputTokens += entry.OutputTokens
	block.TokenCounts.CacheCreationTokens += entry.CacheCreationTokens
	block.TokenCounts.CacheReadTokens += entry.CacheReadTokens

	// Update aggregated cost
	block.CostUSD += entry.CostUSD

	// Update model tracking (prevent duplicates)
	if model != "" && !contains(block.Models, model) {
		block.Models = append(block.Models, model)
	}

	// Increment sent messages count
	block.SentMessagesCount++

	// Update legacy fields for backward compatibility
	if _, exists := block.ModelStats[model]; !exists {
		block.ModelStats[model] = models.ModelStat{}
	}

	legacyStats := block.ModelStats[model]
	legacyStats.InputTokens += entry.InputTokens
	legacyStats.OutputTokens += entry.OutputTokens
	legacyStats.CacheCreationTokens += entry.CacheCreationTokens
	legacyStats.CacheReadTokens += entry.CacheReadTokens
	legacyStats.TotalTokens += entry.TotalTokens
	legacyStats.Cost += entry.CostUSD
	block.ModelStats[model] = legacyStats
}

// finalizeBlock sets actual end time and calculates totals
func (sa *SessionAnalyzer) finalizeBlock(block *models.SessionBlock) {
	if len(block.Entries) > 0 {
		lastEntry := block.Entries[len(block.Entries)-1]
		block.ActualEndTime = &lastEntry.Timestamp
	}

	// Update sent messages count
	block.SentMessagesCount = len(block.Entries)

	// Update legacy totals
	block.TotalCost = block.CostUSD
	block.TotalTokens = block.TokenCounts.TotalTokens()
}

// checkForGap checks for inactivity gap between blocks
func (sa *SessionAnalyzer) checkForGap(lastBlock *models.SessionBlock, nextEntry models.UsageEntry) *models.SessionBlock {
	if lastBlock.ActualEndTime == nil {
		return nil
	}

	gapDuration := nextEntry.Timestamp.Sub(*lastBlock.ActualEndTime)

	if gapDuration >= sa.sessionDuration {
		gapBlock := &models.SessionBlock{
			StartTime:         *lastBlock.ActualEndTime,
			EndTime:           nextEntry.Timestamp,
			ActualEndTime:     nil,
			IsGap:             true,
			Entries:           []models.UsageEntry{},
			TokenCounts:       models.TokenCounts{},
			CostUSD:           0.0,
			Models:            []string{},
			PerModelStats:     make(map[string]map[string]any),
			SentMessagesCount: 0,
			LimitMessages:     []models.LimitMessage{},

			// Legacy fields
			ModelStats:  make(map[string]models.ModelStat),
			TotalCost:   0.0,
			TotalTokens: 0,
		}

		gapBlock.ID = fmt.Sprintf("gap-%s", lastBlock.ActualEndTime.Format(time.RFC3339))
		return gapBlock
	}

	return nil
}

// markActiveBlocks marks blocks as active if they're still ongoing
func (sa *SessionAnalyzer) markActiveBlocks(blocks []models.SessionBlock) {
	currentTime := time.Now()

	for i := range blocks {
		if !blocks[i].IsGap && blocks[i].EndTime.After(currentTime) {
			blocks[i].IsActive = true
		}
	}
}

// detectSingleLimit detects token limit messages from a single JSONL entry
func (sa *SessionAnalyzer) detectSingleLimit(rawData map[string]interface{}) *models.LimitMessage {
	entryType, ok := rawData["type"].(string)
	if !ok {
		return nil
	}

	switch entryType {
	case "system":
		return sa.processSystemMessage(rawData)
	case "user":
		return sa.processUserMessage(rawData)
	}

	return nil
}

// processSystemMessage processes system messages for limit detection
func (sa *SessionAnalyzer) processSystemMessage(rawData map[string]interface{}) *models.LimitMessage {
	content, ok := rawData["content"].(string)
	if !ok {
		return nil
	}

	contentLower := strings.ToLower(content)
	if !strings.Contains(contentLower, "limit") && !strings.Contains(contentLower, "rate") {
		return nil
	}

	timestampStr, ok := rawData["timestamp"].(string)
	if !ok {
		return nil
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil
	}

	// Check for Opus-specific limit
	if sa.isOpusLimit(contentLower) {
		return &models.LimitMessage{
			Message:   content,
			Timestamp: timestamp,
			Type:      "opus_limit",
		}
	}

	// General system limit
	return &models.LimitMessage{
		Message:   content,
		Timestamp: timestamp,
		Type:      "system_limit",
	}
}

// processUserMessage processes user messages for tool result limit detection
func (sa *SessionAnalyzer) processUserMessage(rawData map[string]interface{}) *models.LimitMessage {
	message, ok := rawData["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	contentList, ok := message["content"].([]interface{})
	if !ok {
		return nil
	}

	for _, item := range contentList {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if itemType, ok := itemMap["type"].(string); ok && itemType == "tool_result" {
				if content, ok := itemMap["content"].(string); ok {
					contentLower := strings.ToLower(content)
					if strings.Contains(contentLower, "limit") || strings.Contains(contentLower, "rate") {
						timestampStr, ok := rawData["timestamp"].(string)
						if !ok {
							continue
						}

						timestamp, err := time.Parse(time.RFC3339, timestampStr)
						if err != nil {
							continue
						}

						return &models.LimitMessage{
							Message:   content,
							Timestamp: timestamp,
							Type:      "tool_result_limit",
						}
					}
				}
			}
		}
	}

	return nil
}

// isOpusLimit checks if the content indicates an Opus-specific limit
func (sa *SessionAnalyzer) isOpusLimit(contentLower string) bool {
	opusPatterns := []string{
		"opus",
		"per day",
		"daily",
		"messages per day",
	}

	for _, pattern := range opusPatterns {
		if strings.Contains(contentLower, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
