package pipeline

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// BatchAggregator 批量聚合器
type BatchAggregator struct {
	inputChannel  chan ProcessedData
	outputChannel chan BatchUpdateEvent
	errorChannel  chan error
	batches       map[Priority]*PriorityBatch
	config        BatchConfig
	stats         *BatchStats
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	isRunning     bool
	wg            sync.WaitGroup
}

// BatchConfig 批量配置
type BatchConfig struct {
	MaxBatchSize      int           `json:"max_batch_size"`
	MaxWaitTime       time.Duration `json:"max_wait_time"`
	FlushInterval     time.Duration `json:"flush_interval"`
	PriorityEnabled   bool          `json:"priority_enabled"`
	BufferSize        int           `json:"buffer_size"`
	HighPriorityMax   int           `json:"high_priority_max"`
	NormalPriorityMax int           `json:"normal_priority_max"`
	LowPriorityMax    int           `json:"low_priority_max"`
	ForceFlushSize    int           `json:"force_flush_size"`
}

// DefaultBatchConfig 默认批量配置
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize:      100,
		MaxWaitTime:       5 * time.Second,
		FlushInterval:     1 * time.Second,
		PriorityEnabled:   true,
		BufferSize:        1000,
		HighPriorityMax:   20,
		NormalPriorityMax: 50,
		LowPriorityMax:    100,
		ForceFlushSize:    500,
	}
}

// PriorityBatch 优先级批次
type PriorityBatch struct {
	Priority  Priority        `json:"priority"`
	Items     []ProcessedData `json:"items"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	MaxSize   int             `json:"max_size"`
	IsReady   bool            `json:"is_ready"`
}

// BatchStats 批量统计
type BatchStats struct {
	TotalBatches     int64         `json:"total_batches"`
	TotalItems       int64         `json:"total_items"`
	AverageBatchSize float64       `json:"average_batch_size"`
	MaxBatchSize     int           `json:"max_batch_size"`
	MinBatchSize     int           `json:"min_batch_size"`
	FlushCount       int64         `json:"flush_count"`
	TimeoutFlushes   int64         `json:"timeout_flushes"`
	SizeFlushes      int64         `json:"size_flushes"`
	PriorityFlushes  int64         `json:"priority_flushes"`
	AverageWaitTime  time.Duration `json:"average_wait_time"`
	TotalWaitTime    time.Duration `json:"total_wait_time"`
	StartTime        time.Time     `json:"start_time"`
	LastFlushTime    time.Time     `json:"last_flush_time"`
	CurrentBatches   int32         `json:"current_batches"`
}

// NewBatchAggregator 创建批量聚合器
func NewBatchAggregator(config BatchConfig) *BatchAggregator {
	ctx, cancel := context.WithCancel(context.Background())

	ba := &BatchAggregator{
		inputChannel:  make(chan ProcessedData, config.BufferSize),
		outputChannel: make(chan BatchUpdateEvent, 100),
		errorChannel:  make(chan error, 50),
		batches:       make(map[Priority]*PriorityBatch),
		config:        config,
		stats:         &BatchStats{StartTime: time.Now()},
		ctx:           ctx,
		cancel:        cancel,
	}

	// 初始化优先级批次
	if config.PriorityEnabled {
		ba.batches[PriorityHigh] = &PriorityBatch{
			Priority:  PriorityHigh,
			Items:     make([]ProcessedData, 0, config.HighPriorityMax),
			CreatedAt: time.Now(),
			MaxSize:   config.HighPriorityMax,
		}
		ba.batches[PriorityNormal] = &PriorityBatch{
			Priority:  PriorityNormal,
			Items:     make([]ProcessedData, 0, config.NormalPriorityMax),
			CreatedAt: time.Now(),
			MaxSize:   config.NormalPriorityMax,
		}
		ba.batches[PriorityLow] = &PriorityBatch{
			Priority:  PriorityLow,
			Items:     make([]ProcessedData, 0, config.LowPriorityMax),
			CreatedAt: time.Now(),
			MaxSize:   config.LowPriorityMax,
		}
	} else {
		ba.batches[PriorityNormal] = &PriorityBatch{
			Priority:  PriorityNormal,
			Items:     make([]ProcessedData, 0, config.MaxBatchSize),
			CreatedAt: time.Now(),
			MaxSize:   config.MaxBatchSize,
		}
	}

	return ba
}

// Start 启动批量聚合器
func (ba *BatchAggregator) Start() error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if ba.isRunning {
		return fmt.Errorf("batch aggregator is already running")
	}

	ba.isRunning = true

	// 启动聚合协程
	ba.wg.Add(1)
	go ba.aggregateLoop()

	// 启动定时刷新协程
	ba.wg.Add(1)
	go ba.flushLoop()

	// 启动统计协程
	ba.wg.Add(1)
	go ba.statsLoop()

	return nil
}

// Stop 停止批量聚合器
func (ba *BatchAggregator) Stop() error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if !ba.isRunning {
		return fmt.Errorf("batch aggregator is not running")
	}

	ba.isRunning = false
	ba.cancel()

	// 最后一次刷新所有批次
	ba.flushAllBatches()

	// 关闭输入通道
	close(ba.inputChannel)

	// 等待所有协程完成
	ba.wg.Wait()

	// 关闭输出通道
	close(ba.outputChannel)
	close(ba.errorChannel)

	return nil
}

// Input 获取输入通道
func (ba *BatchAggregator) Input() chan<- ProcessedData {
	return ba.inputChannel
}

// Output 获取输出通道
func (ba *BatchAggregator) Output() <-chan BatchUpdateEvent {
	return ba.outputChannel
}

// Errors 获取错误通道
func (ba *BatchAggregator) Errors() <-chan error {
	return ba.errorChannel
}

// aggregateLoop 聚合循环
func (ba *BatchAggregator) aggregateLoop() {
	defer ba.wg.Done()

	log.Println("Batch aggregator started")
	defer log.Println("Batch aggregator stopped")

	for {
		select {
		case <-ba.ctx.Done():
			return

		case data, ok := <-ba.inputChannel:
			if !ok {
				return
			}

			if err := ba.addToBatch(data); err != nil {
				ba.sendError(fmt.Errorf("failed to add to batch: %w", err))
			}
		}
	}
}

// flushLoop 刷新循环
func (ba *BatchAggregator) flushLoop() {
	defer ba.wg.Done()

	ticker := time.NewTicker(ba.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ba.ctx.Done():
			return

		case <-ticker.C:
			ba.checkAndFlushBatches()
		}
	}
}

// statsLoop 统计循环
func (ba *BatchAggregator) statsLoop() {
	defer ba.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ba.ctx.Done():
			return

		case <-ticker.C:
			ba.updateStats()
		}
	}
}

// addToBatch 添加到批次
func (ba *BatchAggregator) addToBatch(data ProcessedData) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	// 确定优先级
	priority := ba.determinePriority(data)

	batch, exists := ba.batches[priority]
	if !exists {
		return fmt.Errorf("batch for priority %d not found", priority)
	}

	// 添加数据到批次
	batch.Items = append(batch.Items, data)
	batch.UpdatedAt = time.Now()
	atomic.AddInt64(&ba.stats.TotalItems, 1)

	// 检查是否需要刷新
	if len(batch.Items) >= batch.MaxSize {
		if err := ba.flushBatch(priority); err != nil {
			return fmt.Errorf("failed to flush batch: %w", err)
		}
		atomic.AddInt64(&ba.stats.SizeFlushes, 1)
	}

	// 检查总数是否达到强制刷新阈值
	totalItems := ba.getTotalItems()
	if totalItems >= ba.config.ForceFlushSize {
		ba.flushAllBatches()
		log.Printf("Force flushed all batches due to size limit (%d items)", totalItems)
	}

	return nil
}

// determinePriority 确定优先级
func (ba *BatchAggregator) determinePriority(data ProcessedData) Priority {
	if !ba.config.PriorityEnabled {
		return PriorityNormal
	}

	// 基于元数据或其他因素确定优先级
	if metadata, ok := data.Metadata["priority"]; ok {
		if priority, ok := metadata.(Priority); ok {
			return priority
		}
	}

	// 基于成本确定优先级
	if data.Entry.CostUSD > 1.0 {
		return PriorityHigh
	} else if data.Entry.CostUSD > 0.1 {
		return PriorityNormal
	}

	return PriorityLow
}

// checkAndFlushBatches 检查并刷新批次
func (ba *BatchAggregator) checkAndFlushBatches() {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	now := time.Now()
	flushed := false

	for priority, batch := range ba.batches {
		if len(batch.Items) == 0 {
			continue
		}

		// 检查超时
		waitTime := now.Sub(batch.CreatedAt)
		if waitTime >= ba.config.MaxWaitTime {
			if err := ba.flushBatch(priority); err != nil {
				ba.sendError(fmt.Errorf("failed to flush batch on timeout: %w", err))
			} else {
				atomic.AddInt64(&ba.stats.TimeoutFlushes, 1)
				flushed = true
			}
		}
	}

	if flushed {
		log.Printf("Flushed batches due to timeout")
	}
}

// flushBatch 刷新单个批次
func (ba *BatchAggregator) flushBatch(priority Priority) error {
	batch, exists := ba.batches[priority]
	if !exists || len(batch.Items) == 0 {
		return nil
	}

	// 创建批次事件
	batchEvent := BatchUpdateEvent{
		Entries:   batch.Items,
		Timestamp: time.Now(),
		BatchSize: len(batch.Items),
	}

	// 更新统计
	atomic.AddInt64(&ba.stats.TotalBatches, 1)
	atomic.AddInt64(&ba.stats.FlushCount, 1)
	ba.stats.LastFlushTime = time.Now()

	waitTime := batchEvent.Timestamp.Sub(batch.CreatedAt)
	atomic.AddInt64((*int64)(&ba.stats.TotalWaitTime), int64(waitTime))

	// 更新批次大小统计
	batchSize := len(batch.Items)
	if batchSize > ba.stats.MaxBatchSize {
		ba.stats.MaxBatchSize = batchSize
	}
	if ba.stats.MinBatchSize == 0 || batchSize < ba.stats.MinBatchSize {
		ba.stats.MinBatchSize = batchSize
	}

	// 发送事件
	select {
	case ba.outputChannel <- batchEvent:
	default:
		return fmt.Errorf("output channel full, dropping batch")
	}

	// 重置批次
	batch.Items = batch.Items[:0]
	batch.CreatedAt = time.Now()
	batch.UpdatedAt = batch.CreatedAt

	log.Printf("Flushed batch with %d items (priority: %d)", batchSize, priority)
	return nil
}

// flushAllBatches 刷新所有批次
func (ba *BatchAggregator) flushAllBatches() {
	priorities := ba.getSortedPriorities()

	for _, priority := range priorities {
		if err := ba.flushBatch(priority); err != nil {
			ba.sendError(fmt.Errorf("failed to flush batch %d: %w", priority, err))
		}
	}
}

// getSortedPriorities 获取排序后的优先级列表
func (ba *BatchAggregator) getSortedPriorities() []Priority {
	priorities := make([]Priority, 0, len(ba.batches))
	for priority := range ba.batches {
		priorities = append(priorities, priority)
	}

	// 按优先级排序（高优先级优先）
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i] > priorities[j]
	})

	return priorities
}

// getTotalItems 获取总项目数
func (ba *BatchAggregator) getTotalItems() int {
	total := 0
	for _, batch := range ba.batches {
		total += len(batch.Items)
	}
	return total
}

// updateStats 更新统计信息
func (ba *BatchAggregator) updateStats() {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	totalBatches := atomic.LoadInt64(&ba.stats.TotalBatches)
	totalItems := atomic.LoadInt64(&ba.stats.TotalItems)

	if totalBatches > 0 {
		ba.stats.AverageBatchSize = float64(totalItems) / float64(totalBatches)
	}

	totalWaitTime := time.Duration(atomic.LoadInt64((*int64)(&ba.stats.TotalWaitTime)))
	if totalBatches > 0 {
		ba.stats.AverageWaitTime = totalWaitTime / time.Duration(totalBatches)
	}

	// 更新当前批次数
	currentBatches := int32(0)
	for _, batch := range ba.batches {
		if len(batch.Items) > 0 {
			currentBatches++
		}
	}
	atomic.StoreInt32(&ba.stats.CurrentBatches, currentBatches)
}

// sendError 发送错误
func (ba *BatchAggregator) sendError(err error) {
	select {
	case ba.errorChannel <- err:
	default:
		log.Printf("Error channel full, dropping error: %v", err)
	}
}

// GetStats 获取统计信息
func (ba *BatchAggregator) GetStats() BatchStats {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	stats := *ba.stats
	stats.TotalBatches = atomic.LoadInt64(&ba.stats.TotalBatches)
	stats.TotalItems = atomic.LoadInt64(&ba.stats.TotalItems)
	stats.FlushCount = atomic.LoadInt64(&ba.stats.FlushCount)
	stats.TimeoutFlushes = atomic.LoadInt64(&ba.stats.TimeoutFlushes)
	stats.SizeFlushes = atomic.LoadInt64(&ba.stats.SizeFlushes)
	stats.PriorityFlushes = atomic.LoadInt64(&ba.stats.PriorityFlushes)
	stats.CurrentBatches = atomic.LoadInt32(&ba.stats.CurrentBatches)
	stats.TotalWaitTime = time.Duration(atomic.LoadInt64((*int64)(&ba.stats.TotalWaitTime)))

	if stats.TotalBatches > 0 {
		stats.AverageBatchSize = float64(stats.TotalItems) / float64(stats.TotalBatches)
		stats.AverageWaitTime = stats.TotalWaitTime / time.Duration(stats.TotalBatches)
	}

	return stats
}

// IsRunning 检查是否正在运行
func (ba *BatchAggregator) IsRunning() bool {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.isRunning
}

// GetCurrentBatches 获取当前批次状态
func (ba *BatchAggregator) GetCurrentBatches() map[Priority]*PriorityBatch {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	result := make(map[Priority]*PriorityBatch)
	for priority, batch := range ba.batches {
		// 返回副本以避免并发修改
		batchCopy := &PriorityBatch{
			Priority:  batch.Priority,
			Items:     make([]ProcessedData, len(batch.Items)),
			CreatedAt: batch.CreatedAt,
			UpdatedAt: batch.UpdatedAt,
			MaxSize:   batch.MaxSize,
			IsReady:   len(batch.Items) >= batch.MaxSize,
		}
		copy(batchCopy.Items, batch.Items)
		result[priority] = batchCopy
	}

	return result
}

// ForceFlush 强制刷新所有批次
func (ba *BatchAggregator) ForceFlush() error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if !ba.isRunning {
		return fmt.Errorf("batch aggregator is not running")
	}

	ba.flushAllBatches()
	atomic.AddInt64(&ba.stats.PriorityFlushes, 1)

	log.Println("Force flushed all batches")
	return nil
}

// SetMaxBatchSize 设置最大批次大小
func (ba *BatchAggregator) SetMaxBatchSize(priority Priority, maxSize int) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	batch, exists := ba.batches[priority]
	if !exists {
		return fmt.Errorf("batch for priority %d not found", priority)
	}

	batch.MaxSize = maxSize

	// 如果当前批次大小超过新的最大值，立即刷新
	if len(batch.Items) >= maxSize {
		return ba.flushBatch(priority)
	}

	return nil
}
