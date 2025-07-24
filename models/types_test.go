package models

import (
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageEntry_CalculateTotalTokens(t *testing.T) {
	tests := []struct {
		name  string
		entry UsageEntry
		want  int
	}{
		{
			name: "all token types",
			entry: UsageEntry{
				InputTokens:         100,
				OutputTokens:        50,
				CacheCreationTokens: 20,
				CacheReadTokens:     10,
			},
			want: 180,
		},
		{
			name: "only input and output",
			entry: UsageEntry{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			want: 1500,
		},
		{
			name:  "zero tokens",
			entry: UsageEntry{},
			want:  0,
		},
		{
			name: "large numbers",
			entry: UsageEntry{
				InputTokens:         1_000_000,
				OutputTokens:        500_000,
				CacheCreationTokens: 250_000,
				CacheReadTokens:     50_000,
			},
			want: 1_800_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.CalculateTotalTokens()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUsageEntry_CalculateCost(t *testing.T) {
	tests := []struct {
		name    string
		entry   UsageEntry
		pricing ModelPricing
		want    float64
	}{
		{
			name: "sonnet pricing with 1M tokens",
			entry: UsageEntry{
				Model:        ModelSonnet,
				InputTokens:  1_000_000,
				OutputTokens: 500_000,
			},
			pricing: GetPricing(ModelSonnet),
			want:    3.0 + 7.5, // $3 + $7.50
		},
		{
			name: "opus pricing with mixed tokens",
			entry: UsageEntry{
				Model:               ModelOpus,
				InputTokens:         100_000,
				OutputTokens:        50_000,
				CacheCreationTokens: 20_000,
				CacheReadTokens:     10_000,
			},
			pricing: GetPricing(ModelOpus),
			want:    1.5 + 3.75 + 0.375 + 0.01875, // $1.50 + $3.75 + $0.375 + $0.01875
		},
		{
			name: "zero tokens",
			entry: UsageEntry{
				Model: ModelHaiku,
			},
			pricing: GetPricing(ModelHaiku),
			want:    0.0,
		},
		{
			name: "small token counts",
			entry: UsageEntry{
				Model:        ModelHaiku,
				InputTokens:  100,
				OutputTokens: 50,
			},
			pricing: GetPricing(ModelHaiku),
			want:    0.00008 + 0.0002, // Very small costs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.CalculateCost(tt.pricing)
			assert.InDelta(t, tt.want, got, 0.000001)
		})
	}
}

func TestSessionBlock_AddEntry(t *testing.T) {
	session := &SessionBlock{
		StartTime: time.Now(),
		EndTime:   time.Now().Add(SessionDuration),
	}

	entry1 := UsageEntry{
		Model:        ModelSonnet,
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
		CostUSD:      0.01,
	}

	entry2 := UsageEntry{
		Model:        ModelSonnet,
		InputTokens:  2000,
		OutputTokens: 1000,
		TotalTokens:  3000,
		CostUSD:      0.02,
	}

	entry3 := UsageEntry{
		Model:        ModelOpus,
		InputTokens:  500,
		OutputTokens: 250,
		TotalTokens:  750,
		CostUSD:      0.05,
	}

	// Add entries
	session.AddEntry(entry1)
	session.AddEntry(entry2)
	session.AddEntry(entry3)

	// Check totals
	assert.Equal(t, 0.08, session.TotalCost)
	assert.Equal(t, 5250, session.TotalTokens)

	// Check model stats
	assert.Len(t, session.ModelStats, 2)

	sonnetStats := session.ModelStats[ModelSonnet]
	assert.Equal(t, 3000, sonnetStats.InputTokens)
	assert.Equal(t, 1500, sonnetStats.OutputTokens)
	assert.Equal(t, 4500, sonnetStats.TotalTokens)
	assert.Equal(t, 0.03, sonnetStats.Cost)

	opusStats := session.ModelStats[ModelOpus]
	assert.Equal(t, 500, opusStats.InputTokens)
	assert.Equal(t, 250, opusStats.OutputTokens)
	assert.Equal(t, 750, opusStats.TotalTokens)
	assert.Equal(t, 0.05, opusStats.Cost)
}

func TestSessionBlock_CalculateTotals(t *testing.T) {
	session := &SessionBlock{
		ModelStats: map[string]ModelStat{
			ModelSonnet: {
				InputTokens:  1000,
				OutputTokens: 500,
				TotalTokens:  1500,
				Cost:         0.01,
			},
			ModelHaiku: {
				InputTokens:  2000,
				OutputTokens: 1000,
				TotalTokens:  3000,
				Cost:         0.02,
			},
		},
	}

	session.CalculateTotals()

	assert.Equal(t, 0.03, session.TotalCost)
	assert.Equal(t, 4500, session.TotalTokens)
}

func TestUsageEntry_JSONMarshaling(t *testing.T) {
	entry := UsageEntry{
		Timestamp:           time.Now().Truncate(time.Second),
		Model:               ModelSonnet,
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 100,
		CacheReadTokens:     50,
		TotalTokens:         1650,
		CostUSD:             0.01,
	}

	// Marshal
	data, err := sonic.Marshal(entry)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Unmarshal
	var decoded UsageEntry
	err = sonic.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, entry.Timestamp.Unix(), decoded.Timestamp.Unix())
	assert.Equal(t, entry.Model, decoded.Model)
	assert.Equal(t, entry.InputTokens, decoded.InputTokens)
	assert.Equal(t, entry.OutputTokens, decoded.OutputTokens)
	assert.Equal(t, entry.CacheCreationTokens, decoded.CacheCreationTokens)
	assert.Equal(t, entry.CacheReadTokens, decoded.CacheReadTokens)
	assert.Equal(t, entry.TotalTokens, decoded.TotalTokens)
	assert.InDelta(t, entry.CostUSD, decoded.CostUSD, 0.000001)
}

func TestSessionBlock_JSONMarshaling(t *testing.T) {
	session := SessionBlock{
		StartTime:   time.Now().Truncate(time.Second),
		EndTime:     time.Now().Add(SessionDuration).Truncate(time.Second),
		IsGap:       false,
		TotalCost:   0.05,
		TotalTokens: 5000,
		ModelStats: map[string]ModelStat{
			ModelSonnet: {
				InputTokens:  3000,
				OutputTokens: 2000,
				TotalTokens:  5000,
				Cost:         0.05,
			},
		},
	}

	// Marshal
	data, err := sonic.Marshal(session)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Unmarshal
	var decoded SessionBlock
	err = sonic.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, session.StartTime.Unix(), decoded.StartTime.Unix())
	assert.Equal(t, session.EndTime.Unix(), decoded.EndTime.Unix())
	assert.Equal(t, session.IsGap, decoded.IsGap)
	assert.InDelta(t, session.TotalCost, decoded.TotalCost, 0.000001)
	assert.Equal(t, session.TotalTokens, decoded.TotalTokens)
	assert.Len(t, decoded.ModelStats, 1)
	assert.Equal(t, session.ModelStats[ModelSonnet], decoded.ModelStats[ModelSonnet])
}
