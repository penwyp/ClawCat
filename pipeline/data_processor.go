package pipeline

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// DataProcessor 数据处理器
type DataProcessor struct {
	validators    []Validator
	transformers  []Transformer
	enrichers     []Enricher
	inputChannel  chan ProcessedData
	outputChannel chan ProcessedData
	errorChannel  chan error
	config        ProcessorConfig
	stats         *ProcessorStats
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	isRunning     bool
	mu            sync.RWMutex
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	WorkerCount      int           `json:"worker_count"`
	BufferSize       int           `json:"buffer_size"`
	ProcessTimeout   time.Duration `json:"process_timeout"`
	RetryAttempts    int           `json:"retry_attempts"`
	RetryDelay       time.Duration `json:"retry_delay"`
	ValidateInput    bool          `json:"validate_input"`
	EnrichData       bool          `json:"enrich_data"`
	FailOnError      bool          `json:"fail_on_error"`
	CollectMetrics   bool          `json:"collect_metrics"`
}

// DefaultProcessorConfig 默认处理器配置
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		WorkerCount:    4,
		BufferSize:     1000,
		ProcessTimeout: 5 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     100 * time.Millisecond,
		ValidateInput:  true,
		EnrichData:     true,
		FailOnError:    false,
		CollectMetrics: true,
	}
}

// ProcessorStats 处理器统计
type ProcessorStats struct {
	ProcessedCount    int64         `json:"processed_count"`
	ErrorCount        int64         `json:"error_count"`
	RetryCount        int64         `json:"retry_count"`
	ValidationErrors  int64         `json:"validation_errors"`
	TransformErrors   int64         `json:"transform_errors"`
	TotalProcessTime  time.Duration `json:"total_process_time"`
	AverageProcessTime time.Duration `json:"average_process_time"`
	StartTime         time.Time     `json:"start_time"`
	LastProcessTime   time.Time     `json:"last_process_time"`
	ActiveWorkers     int32         `json:"active_workers"`
}

// NewDataProcessor 创建数据处理器
func NewDataProcessor(config ProcessorConfig) *DataProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &DataProcessor{
		inputChannel:  make(chan ProcessedData, config.BufferSize),
		outputChannel: make(chan ProcessedData, config.BufferSize),
		errorChannel:  make(chan error, 100),
		config:        config,
		stats:         &ProcessorStats{StartTime: time.Now()},
		ctx:           ctx,
		cancel:        cancel,
	}
}

// AddValidator 添加验证器
func (dp *DataProcessor) AddValidator(validator Validator) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.validators = append(dp.validators, validator)
}

// AddTransformer 添加转换器
func (dp *DataProcessor) AddTransformer(transformer Transformer) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.transformers = append(dp.transformers, transformer)
}

// AddEnricher 添加增强器
func (dp *DataProcessor) AddEnricher(enricher Enricher) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.enrichers = append(dp.enrichers, enricher)
}

// Start 启动数据处理器
func (dp *DataProcessor) Start() error {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if dp.isRunning {
		return fmt.Errorf("data processor is already running")
	}

	dp.isRunning = true

	// 启动工作协程
	for i := 0; i < dp.config.WorkerCount; i++ {
		dp.wg.Add(1)
		go dp.worker(i)
	}

	// 启动统计协程
	if dp.config.CollectMetrics {
		dp.wg.Add(1)
		go dp.statsCollector()
	}

	return nil
}

// Stop 停止数据处理器
func (dp *DataProcessor) Stop() error {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if !dp.isRunning {
		return fmt.Errorf("data processor is not running")
	}

	dp.isRunning = false
	dp.cancel()

	// 关闭输入通道
	close(dp.inputChannel)

	// 等待所有工作协程完成
	dp.wg.Wait()

	// 关闭输出通道
	close(dp.outputChannel)
	close(dp.errorChannel)

	return nil
}

// Input 获取输入通道
func (dp *DataProcessor) Input() chan<- ProcessedData {
	return dp.inputChannel
}

// Output 获取输出通道
func (dp *DataProcessor) Output() <-chan ProcessedData {
	return dp.outputChannel
}

// Errors 获取错误通道
func (dp *DataProcessor) Errors() <-chan error {
	return dp.errorChannel
}

// worker 工作协程
func (dp *DataProcessor) worker(workerID int) {
	defer dp.wg.Done()

	log.Printf("Data processor worker %d started", workerID)
	defer log.Printf("Data processor worker %d stopped", workerID)

	for {
		select {
		case <-dp.ctx.Done():
			return

		case data, ok := <-dp.inputChannel:
			if !ok {
				return
			}

			atomic.AddInt32(&dp.stats.ActiveWorkers, 1)
			err := dp.processData(&data)
			atomic.AddInt32(&dp.stats.ActiveWorkers, -1)

			if err != nil {
				atomic.AddInt64(&dp.stats.ErrorCount, 1)
				
				if dp.config.FailOnError {
					dp.sendError(fmt.Errorf("worker %d: %w", workerID, err))
					continue
				} else {
					log.Printf("Worker %d processing error (continuing): %v", workerID, err)
				}
			}

			// 发送处理后的数据
			select {
			case dp.outputChannel <- data:
				atomic.AddInt64(&dp.stats.ProcessedCount, 1)
				dp.stats.LastProcessTime = time.Now()
			case <-dp.ctx.Done():
				return
			default:
				log.Printf("Output channel full, dropping data")
			}
		}
	}
}

// processData 处理数据
func (dp *DataProcessor) processData(data *ProcessedData) error {
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime)
		atomic.AddInt64((*int64)(&dp.stats.TotalProcessTime), int64(processingTime))
	}()

	// 设置处理超时
	ctx, cancel := context.WithTimeout(dp.ctx, dp.config.ProcessTimeout)
	defer cancel()

	return dp.processWithRetry(ctx, data)
}

// processWithRetry 带重试的处理
func (dp *DataProcessor) processWithRetry(ctx context.Context, data *ProcessedData) error {
	var lastErr error

	for attempt := 0; attempt <= dp.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			atomic.AddInt64(&dp.stats.RetryCount, 1)
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(dp.config.RetryDelay):
			}
		}

		if err := dp.processOnce(ctx, data); err != nil {
			lastErr = err
			log.Printf("Processing attempt %d failed: %v", attempt+1, err)
			continue
		}

		return nil
	}

	return fmt.Errorf("processing failed after %d attempts: %w", dp.config.RetryAttempts+1, lastErr)
}

// processOnce 执行一次处理
func (dp *DataProcessor) processOnce(ctx context.Context, data *ProcessedData) error {
	// 1. 验证输入
	if dp.config.ValidateInput {
		if err := dp.validateData(ctx, data); err != nil {
			atomic.AddInt64(&dp.stats.ValidationErrors, 1)
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// 2. 数据转换
	if err := dp.transformData(ctx, data); err != nil {
		atomic.AddInt64(&dp.stats.TransformErrors, 1)
		return fmt.Errorf("transformation failed: %w", err)
	}

	// 3. 数据增强
	if dp.config.EnrichData {
		if err := dp.enrichData(ctx, data); err != nil {
			log.Printf("Enrichment failed (continuing): %v", err)
			// 增强失败不是致命错误，继续处理
		}
	}

	return nil
}

// validateData 验证数据
func (dp *DataProcessor) validateData(ctx context.Context, data *ProcessedData) error {
	dp.mu.RLock()
	validators := dp.validators
	dp.mu.RUnlock()

	for _, validator := range validators {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := validator.Validate(data.Original); err != nil {
			return fmt.Errorf("validator failed: %w", err)
		}
	}

	return nil
}

// transformData 转换数据
func (dp *DataProcessor) transformData(ctx context.Context, data *ProcessedData) error {
	dp.mu.RLock()
	transformers := dp.transformers
	dp.mu.RUnlock()

	for _, transformer := range transformers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := transformer.Transform(&data.Entry); err != nil {
			return fmt.Errorf("transformer failed: %w", err)
		}
	}

	return nil
}

// enrichData 增强数据
func (dp *DataProcessor) enrichData(ctx context.Context, data *ProcessedData) error {
	dp.mu.RLock()
	enrichers := dp.enrichers
	dp.mu.RUnlock()

	if data.Metadata == nil {
		data.Metadata = make(map[string]interface{})
	}

	for _, enricher := range enrichers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		enrichment := enricher.Enrich(&data.Entry)
		for key, value := range enrichment {
			data.Metadata[key] = value
		}
	}

	return nil
}

// sendError 发送错误
func (dp *DataProcessor) sendError(err error) {
	select {
	case dp.errorChannel <- err:
	default:
		log.Printf("Error channel full, dropping error: %v", err)
	}
}

// statsCollector 统计收集器
func (dp *DataProcessor) statsCollector() {
	defer dp.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dp.ctx.Done():
			return
		case <-ticker.C:
			dp.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (dp *DataProcessor) updateStats() {
	processed := atomic.LoadInt64(&dp.stats.ProcessedCount)
	totalProcessTime := time.Duration(atomic.LoadInt64((*int64)(&dp.stats.TotalProcessTime)))

	if processed > 0 {
		dp.stats.AverageProcessTime = totalProcessTime / time.Duration(processed)
	}
}

// GetStats 获取统计信息
func (dp *DataProcessor) GetStats() ProcessorStats {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	stats := *dp.stats
	stats.ProcessedCount = atomic.LoadInt64(&dp.stats.ProcessedCount)
	stats.ErrorCount = atomic.LoadInt64(&dp.stats.ErrorCount)
	stats.RetryCount = atomic.LoadInt64(&dp.stats.RetryCount)
	stats.ValidationErrors = atomic.LoadInt64(&dp.stats.ValidationErrors)
	stats.TransformErrors = atomic.LoadInt64(&dp.stats.TransformErrors)
	stats.ActiveWorkers = atomic.LoadInt32(&dp.stats.ActiveWorkers)
	stats.TotalProcessTime = time.Duration(atomic.LoadInt64((*int64)(&dp.stats.TotalProcessTime)))

	if stats.ProcessedCount > 0 {
		stats.AverageProcessTime = stats.TotalProcessTime / time.Duration(stats.ProcessedCount)
	}

	return stats
}

// IsRunning 检查是否正在运行
func (dp *DataProcessor) IsRunning() bool {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return dp.isRunning
}

// ProcessBatch 批量处理数据
func (dp *DataProcessor) ProcessBatch(batch []ProcessedData) ([]ProcessedData, error) {
	if !dp.IsRunning() {
		return nil, fmt.Errorf("data processor is not running")
	}

	results := make([]ProcessedData, 0, len(batch))
	errors := make([]error, 0)

	for i, data := range batch {
		dataCopy := data // 避免循环变量问题
		
		if err := dp.processData(&dataCopy); err != nil {
			errors = append(errors, fmt.Errorf("item %d: %w", i, err))
			if dp.config.FailOnError {
				return results, fmt.Errorf("batch processing failed at item %d: %w", i, err)
			}
		} else {
			results = append(results, dataCopy)
		}
	}

	if len(errors) > 0 && !dp.config.FailOnError {
		log.Printf("Batch processing completed with %d errors", len(errors))
	}

	return results, nil
}

// 内置验证器实现
type JSONValidator struct{}

func (jv *JSONValidator) Validate(data []byte) error {
	var entry models.UsageEntry
	if err := entry.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}
	return nil
}

// 内置转换器实现
type TimestampNormalizer struct{}

func (tn *TimestampNormalizer) Transform(entry *models.UsageEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	return nil
}

// 内置增强器实现
type MetadataEnricher struct{}

func (me *MetadataEnricher) Enrich(entry *models.UsageEntry) map[string]interface{} {
	return map[string]interface{}{
		"processing_timestamp": time.Now(),
		"token_density":       float64(entry.TotalTokens) / math.Max(1, float64(len(entry.Content))),
		"cost_per_token":      entry.CostUSD / math.Max(0.001, float64(entry.TotalTokens)),
	}
}

// CostCalculator 成本计算转换器
type CostCalculator struct {
	ModelPricing map[string]ModelPricing
}

type ModelPricing struct {
	InputTokenPrice  float64 `json:"input_token_price"`
	OutputTokenPrice float64 `json:"output_token_price"`
}

func (cc *CostCalculator) Transform(entry *models.UsageEntry) error {
	pricing, exists := cc.ModelPricing[entry.Model]
	if !exists {
		return nil // 跳过未知模型
	}

	// 重新计算成本
	inputCost := float64(entry.PromptTokens) * pricing.InputTokenPrice / 1000000
	outputCost := float64(entry.CompletionTokens) * pricing.OutputTokenPrice / 1000000
	entry.CostUSD = inputCost + outputCost

	return nil
}