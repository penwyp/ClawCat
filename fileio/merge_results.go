package fileio

import (
	"fmt"

	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
)

// MergeResultsWithDedup combines results from concurrent loading with deduplication
func MergeResultsWithDedup(results []FileResult, deduplicationSet map[string]bool) ([]models.UsageEntry, []map[string]interface{}, []error) {
	var allEntries []models.UsageEntry
	var allRawEntries []map[string]interface{}
	var errors []error
	duplicatesSkipped := 0

	// Calculate total capacity needed
	totalEntries := 0
	totalRaw := 0
	for _, result := range results {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.FilePath, result.Error))
			continue
		}
		totalEntries += len(result.Entries)
		totalRaw += len(result.RawEntries)
	}

	// Pre-allocate slices
	allEntries = make([]models.UsageEntry, 0, totalEntries)
	if totalRaw > 0 {
		allRawEntries = make([]map[string]interface{}, 0, totalRaw)
	}

	// Merge results with deduplication
	for _, result := range results {
		if result.Error == nil {
			// Process entries with deduplication
			for _, entry := range result.Entries {
				// Check for deduplication
				if entry.MessageID != "" && entry.RequestID != "" {
					key := fmt.Sprintf("%s:%s", entry.MessageID, entry.RequestID)
					if deduplicationSet[key] {
						// Skip duplicate entry
						duplicatesSkipped++
						continue
					}
					// Mark as seen
					deduplicationSet[key] = true
				}
				allEntries = append(allEntries, entry)
			}

			// Raw entries don't need deduplication
			if result.RawEntries != nil {
				allRawEntries = append(allRawEntries, result.RawEntries...)
			}
		}
	}

	if duplicatesSkipped > 0 {
		logging.LogInfof("Deduplication: skipped %d duplicate entries across all files", duplicatesSkipped)
	}

	return allEntries, allRawEntries, errors
}