package pipeline

import (
	"time"

	"github.com/penwyp/claudecat/models"
)

// UpdateType 更新类型
type UpdateType string

const (
	UpdateTypeNewEntry   UpdateType = "new_entry"
	UpdateTypeFileChange UpdateType = "file_change"
	UpdateTypeStats      UpdateType = "stats"
	UpdateTypeConfig     UpdateType = "config"
)

// Priority 优先级
type Priority int

const (
	PriorityLow    Priority = 1
	PriorityNormal Priority = 2
	PriorityHigh   Priority = 3
)

// Update 更新数据
type Update struct {
	Type      UpdateType  `json:"type"`
	Source    string      `json:"source"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	Priority  Priority    `json:"priority"`
}

// EventType 文件事件类型
type EventType string

const (
	EventCreate   EventType = "create"
	EventModify   EventType = "modify"
	EventDelete   EventType = "delete"
	EventRename   EventType = "rename"
	EventTruncate EventType = "truncate"
)

// FileEvent 文件事件
type FileEvent struct {
	Type     EventType  `json:"type"`
	Path     string     `json:"path"`
	OldState *FileState `json:"old_state,omitempty"`
	NewState *FileState `json:"new_state,omitempty"`
	Changes  []Change   `json:"changes,omitempty"`
}

// FileState 文件状态
type FileState struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	LastRead     time.Time `json:"last_read"`
	ReadPosition int64     `json:"read_position"`
	Checksum     string    `json:"checksum,omitempty"`
}

// Change 文件变化
type Change struct {
	Type     ChangeType  `json:"type"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// ChangeType 变化类型
type ChangeType string

const (
	ChangeSize    ChangeType = "size"
	ChangeModTime ChangeType = "mod_time"
	ChangeContent ChangeType = "content"
)

// ProcessedData 处理后的数据
type ProcessedData struct {
	Original       []byte                 `json:"original"`
	Entry          models.UsageEntry      `json:"entry"`
	Metadata       map[string]interface{} `json:"metadata"`
	ProcessingTime time.Duration          `json:"processing_time"`
}

// BatchUpdateEvent 批量更新事件
type BatchUpdateEvent struct {
	Entries   []ProcessedData `json:"entries"`
	Timestamp time.Time       `json:"timestamp"`
	BatchSize int             `json:"batch_size"`
}

// UIRefreshEvent UI 刷新事件
type UIRefreshEvent struct {
	Status    interface{} `json:"status"`
	Timestamp time.Time   `json:"timestamp"`
}

// PipelineState 管道状态
type PipelineState struct {
	Running        bool                 `json:"running"`
	LastUpdate     time.Time            `json:"last_update"`
	ProcessedCount int64                `json:"processed_count"`
	ErrorCount     int64                `json:"error_count"`
	CurrentFiles   map[string]FileState `json:"current_files"`
}

// PipelineConfig 管道配置
type PipelineConfig struct {
	// 更新频率
	DataRefreshInterval time.Duration `json:"data_refresh_interval"` // 10秒
	UIRefreshRate       float64       `json:"ui_refresh_rate"`       // 0.75Hz
	BatchSize           int           `json:"batch_size"`            // 批量大小
	BatchTimeout        time.Duration `json:"batch_timeout"`         // 批量超时

	// 性能配置
	MaxConcurrency    int  `json:"max_concurrency"`
	BufferSize        int  `json:"buffer_size"`
	EnableCompression bool `json:"enable_compression"`

	// 文件监控
	WatchPaths     []string `json:"watch_paths"`
	FilePatterns   []string `json:"file_patterns"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

// PipelineMetrics 管道指标
type PipelineMetrics struct {
	// 计数器
	ProcessedEntries Counter `json:"processed_entries"`
	DroppedUpdates   Counter `json:"dropped_updates"`
	ProcessingErrors Counter `json:"processing_errors"`
	BatchesProcessed Counter `json:"batches_processed"`
	UIRefreshes      Counter `json:"ui_refreshes"`

	// 直方图
	ProcessingTime  Histogram `json:"processing_time"`
	BatchSize       Histogram `json:"batch_size"`
	QueueDepth      Histogram `json:"queue_depth"`
	FullRefreshTime Histogram `json:"full_refresh_time"`

	// 仪表
	CurrentQueueSize Gauge `json:"current_queue_size"`
	ActiveWorkers    Gauge `json:"active_workers"`
	MemoryUsage      Gauge `json:"memory_usage"`
}

// Counter 计数器接口
type Counter interface {
	Inc()
	Add(float64)
	Value() int64
}

// Histogram 直方图接口
type Histogram interface {
	Observe(float64)
}

// Gauge 仪表接口
type Gauge interface {
	Set(float64)
	Inc()
	Dec()
}

// Validator 验证器接口
type Validator interface {
	Validate([]byte) error
}

// Transformer 转换器接口
type Transformer interface {
	Transform(*models.UsageEntry) error
}

// Enricher 增强器接口
type Enricher interface {
	Enrich(*models.UsageEntry) map[string]interface{}
}

// EventHandler 事件处理器接口
type EventHandler interface {
	Handle(event interface{}) error
}

// EventHandlerFunc 事件处理函数
type EventHandlerFunc func(event interface{}) error

func (f EventHandlerFunc) Handle(event interface{}) error {
	return f(event)
}
