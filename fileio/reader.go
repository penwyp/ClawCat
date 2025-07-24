package fileio

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/ClawCat/models"
)

type Reader struct {
	file     *os.File
	scanner  *bufio.Scanner
	filepath string
}

type RawMessage struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
	Usage     struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		CacheCreationTokens int `json:"cache_creation_tokens"`
		CacheReadTokens     int `json:"cache_read_tokens"`
	} `json:"usage"`
}

func NewReader(filepath string) (*Reader, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filepath, err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 64KB initial, 1MB max

	return &Reader{
		file:     file,
		scanner:  scanner,
		filepath: filepath,
	}, nil
}

func (r *Reader) ReadEntries() (<-chan models.UsageEntry, <-chan error) {
	entries := make(chan models.UsageEntry, 100)
	errors := make(chan error, 1)

	go func() {
		defer close(entries)
		defer close(errors)

		lineNum := 0
		for r.scanner.Scan() {
			lineNum++
			line := r.scanner.Bytes()

			var msg RawMessage
			if err := sonic.Unmarshal(line, &msg); err != nil {
				continue // Skip invalid JSON lines
			}

			// Only process "message" type entries with usage data
			if msg.Type == "message" && (msg.Usage.InputTokens > 0 || msg.Usage.OutputTokens > 0) {
				entry, err := convertToUsageEntry(&msg)
				if err != nil {
					continue // Skip entries that can't be converted
				}
				entries <- entry
			}
		}

		if err := r.scanner.Err(); err != nil {
			errors <- fmt.Errorf("scanner error at line %d: %w", lineNum, err)
		}
	}()

	return entries, errors
}

func (r *Reader) ReadAll() ([]models.UsageEntry, error) {
	var entries []models.UsageEntry
	entriesCh, errorsCh := r.ReadEntries()

	for {
		select {
		case entry, ok := <-entriesCh:
			if !ok {
				return entries, nil
			}
			entries = append(entries, entry)
		case err := <-errorsCh:
			if err != nil {
				return entries, err
			}
		}
	}
}

func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

func convertToUsageEntry(msg *RawMessage) (models.UsageEntry, error) {
	if msg.Model == "" {
		return models.UsageEntry{}, fmt.Errorf("missing model information")
	}

	totalTokens := msg.Usage.InputTokens + msg.Usage.OutputTokens + 
		msg.Usage.CacheCreationTokens + msg.Usage.CacheReadTokens

	entry := models.UsageEntry{
		Timestamp:           msg.Timestamp,
		Model:               msg.Model,
		InputTokens:         msg.Usage.InputTokens,
		OutputTokens:        msg.Usage.OutputTokens,
		CacheCreationTokens: msg.Usage.CacheCreationTokens,
		CacheReadTokens:     msg.Usage.CacheReadTokens,
		TotalTokens:         totalTokens,
	}

	// Calculate cost using the pricing model
	pricing := models.GetPricing(msg.Model)
	entry.CostUSD = entry.CalculateCost(pricing)
	return entry, nil
}

// ReadConversationFile is a convenience function to read an entire conversation file
func ReadConversationFile(filepath string) ([]models.UsageEntry, error) {
	reader, err := NewReader(filepath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return reader.ReadAll()
}

// StreamConversationFile is a convenience function to stream a conversation file
func StreamConversationFile(filepath string) (<-chan models.UsageEntry, <-chan error) {
	reader, err := NewReader(filepath)
	if err != nil {
		errors := make(chan error, 1)
		entries := make(chan models.UsageEntry)
		close(entries)
		errors <- err
		close(errors)
		return entries, errors
	}

	// Note: Caller is responsible for closing the reader
	// This is a limitation of the streaming approach
	return reader.ReadEntries()
}