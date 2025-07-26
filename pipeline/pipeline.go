package pipeline

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/ui/components"
)

// RealtimeUpdatePipeline 实时更新管道
type RealtimeUpdatePipeline struct {
	// 核心组件
	fileWatcher     *EnhancedFileWatcher
	streamReader    *StreamReader
	dataProcessor   *DataProcessor
	batchAggregator *BatchAggregator
	eventDispatcher *EventDispatcher

	// 配置和状态
	config  PipelineConfig
	state   *PipelineState
	metrics *PipelineMetrics

	// 上下文和同步
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	isRunning bool

	// UI更新
	metricsCalculator *calculations.MetricsCalculator
	progressBar       *components.ProgressBar
	statisticsTable   *components.StatisticsTable

	// 错误处理
	errorChannel chan error
}

// NewRealtimeUpdatePipeline 创建实时更新管道
func NewRealtimeUpdatePipeline(config PipelineConfig) (*RealtimeUpdatePipeline, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建核心组件
	fileWatcher, err := NewEnhancedFileWatcher(DefaultWatcherConfig())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	streamReader, err := NewStreamReader("", DefaultStreamConfig())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stream reader: %w", err)
	}

	dataProcessor := NewDataProcessor(DefaultProcessorConfig())
	batchAggregator := NewBatchAggregator(DefaultBatchConfig())
	eventDispatcher := NewEventDispatcher(DefaultDispatcherConfig())

	// 创建管道实例
	pipeline := &RealtimeUpdatePipeline{
		fileWatcher:     fileWatcher,
		streamReader:    streamReader,
		dataProcessor:   dataProcessor,
		batchAggregator: batchAggregator,
		eventDispatcher: eventDispatcher,
		config:          config,
		state: &PipelineState{
			Running:      false,
			LastUpdate:   time.Now(),
			CurrentFiles: make(map[string]FileState),
		},
		metrics:      NewPipelineMetrics(),
		ctx:          ctx,
		cancel:       cancel,
		errorChannel: make(chan error, 100),
	}

	// 初始化UI组件
	if err := pipeline.initializeUIComponents(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize UI components: %w", err)
	}

	// 设置组件间连接
	if err := pipeline.setupComponentConnections(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup component connections: %w", err)
	}

	// 注册事件处理器
	pipeline.registerEventHandlers()

	return pipeline, nil
}

// Start 启动管道
func (rp *RealtimeUpdatePipeline) Start() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isRunning {
		return fmt.Errorf("pipeline is already running")
	}

	if logging.GetLogger() != nil {
		logging.GetLogger().Info("Starting realtime update pipeline...")
	}

	// 启动核心组件
	components := []struct {
		name      string
		startFunc func() error
	}{
		{"event dispatcher", rp.eventDispatcher.Start},
		{"data processor", rp.dataProcessor.Start},
		{"batch aggregator", rp.batchAggregator.Start},
		{"stream reader", rp.streamReader.Start},
		{"file watcher", rp.fileWatcher.Start},
	}

	for _, comp := range components {
		if err := comp.startFunc(); err != nil {
			log.Printf("Failed to start %s: %v", comp.name, err)
			if stopErr := rp.stop(); stopErr != nil {
				log.Printf("Failed to stop pipeline during cleanup: %v", stopErr)
			} // 清理已启动的组件
			return fmt.Errorf("failed to start %s: %w", comp.name, err)
		}
		log.Printf("Started %s", comp.name)
	}

	// 添加监控路径
	for _, path := range rp.config.WatchPaths {
		if err := rp.fileWatcher.AddPath(path); err != nil {
			log.Printf("Failed to add watch path %s: %v", path, err)
		}
	}

	// 设置文件模式
	rp.fileWatcher.SetPatterns(rp.config.FilePatterns, rp.config.IgnorePatterns)

	// 启动管道协调器
	rp.wg.Add(1)
	go rp.coordinator()

	// 启动错误处理器
	rp.wg.Add(1)
	go rp.errorHandler()

	// 启动指标收集器
	rp.wg.Add(1)
	go rp.metricsCollector()

	rp.isRunning = true
	rp.state.Running = true
	rp.state.LastUpdate = time.Now()

	log.Println("Realtime update pipeline started successfully")
	return nil
}

// Stop 停止管道
func (rp *RealtimeUpdatePipeline) Stop() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	return rp.stop()
}

// stop 内部停止方法（不加锁）
func (rp *RealtimeUpdatePipeline) stop() error {
	if !rp.isRunning {
		return fmt.Errorf("pipeline is not running")
	}

	log.Println("Stopping realtime update pipeline...")

	rp.isRunning = false
	rp.cancel()

	// 停止核心组件（逆序停止）
	components := []struct {
		name     string
		stopFunc func() error
	}{
		{"file watcher", rp.fileWatcher.Stop},
		{"stream reader", rp.streamReader.Stop},
		{"batch aggregator", rp.batchAggregator.Stop},
		{"data processor", rp.dataProcessor.Stop},
		{"event dispatcher", rp.eventDispatcher.Stop},
	}

	for _, comp := range components {
		if err := comp.stopFunc(); err != nil {
			log.Printf("Failed to stop %s: %v", comp.name, err)
		} else {
			log.Printf("Stopped %s", comp.name)
		}
	}

	// 等待所有协程完成
	rp.wg.Wait()

	// 关闭错误通道
	close(rp.errorChannel)

	rp.state.Running = false
	rp.state.LastUpdate = time.Now()

	log.Println("Realtime update pipeline stopped")
	return nil
}

// coordinator 管道协调器
func (rp *RealtimeUpdatePipeline) coordinator() {
	defer rp.wg.Done()

	log.Println("Pipeline coordinator started")
	defer log.Println("Pipeline coordinator stopped")

	uiRefreshTicker := time.NewTicker(time.Duration(1000/rp.config.UIRefreshRate) * time.Millisecond)
	defer uiRefreshTicker.Stop()

	dataRefreshTicker := time.NewTicker(rp.config.DataRefreshInterval)
	defer dataRefreshTicker.Stop()

	for {
		select {
		case <-rp.ctx.Done():
			return

		case <-uiRefreshTicker.C:
			rp.refreshUI()

		case <-dataRefreshTicker.C:
			rp.refreshData()

		case fileEvent := <-rp.fileWatcher.Events():
			rp.handleFileEvent(fileEvent)

		case processedData := <-rp.dataProcessor.Output():
			rp.handleProcessedData(processedData)

		case batchEvent := <-rp.batchAggregator.Output():
			rp.handleBatchEvent(batchEvent)
		}
	}
}

// errorHandler 错误处理器
func (rp *RealtimeUpdatePipeline) errorHandler() {
	defer rp.wg.Done()

	log.Println("Pipeline error handler started")
	defer log.Println("Pipeline error handler stopped")

	for {
		select {
		case <-rp.ctx.Done():
			return

		case err := <-rp.errorChannel:
			rp.handleError(err)

		case event := <-rp.fileWatcher.Events():
			// Process file event
			rp.handleFileEvent(event)

		case err := <-rp.streamReader.Errors():
			rp.handleError(fmt.Errorf("stream reader: %w", err))

		case err := <-rp.dataProcessor.Errors():
			rp.handleError(fmt.Errorf("data processor: %w", err))

		case err := <-rp.batchAggregator.Errors():
			rp.handleError(fmt.Errorf("batch aggregator: %w", err))

		case err := <-rp.eventDispatcher.Errors():
			rp.handleError(fmt.Errorf("event dispatcher: %w", err))
		}
	}
}

// metricsCollector 指标收集器
func (rp *RealtimeUpdatePipeline) metricsCollector() {
	defer rp.wg.Done()

	log.Println("Pipeline metrics collector started")
	defer log.Println("Pipeline metrics collector stopped")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rp.ctx.Done():
			return

		case <-ticker.C:
			rp.collectMetrics()
		}
	}
}

// handleFileEvent 处理文件事件
func (rp *RealtimeUpdatePipeline) handleFileEvent(event FileEvent) {
	log.Printf("File event: %s %s", event.Type, event.Path)

	// 更新文件状态
	rp.mu.Lock()
	if event.NewState != nil {
		rp.state.CurrentFiles[event.Path] = *event.NewState
	} else {
		delete(rp.state.CurrentFiles, event.Path)
	}
	rp.state.LastUpdate = time.Now()
	rp.mu.Unlock()

	// 分发文件事件
	rp.eventDispatcher.DispatchAsync(event)

	// 更新指标
	rp.metrics.ProcessedEntries.Inc()
}

// handleProcessedData 处理已处理的数据
func (rp *RealtimeUpdatePipeline) handleProcessedData(data ProcessedData) {
	// 发送到批量聚合器
	select {
	case rp.batchAggregator.Input() <- data:
	default:
		log.Printf("Batch aggregator input channel full, dropping data")
		rp.metrics.DroppedUpdates.Inc()
	}

	// 更新指标
	rp.metrics.ProcessedEntries.Inc()
	rp.metrics.ProcessingTime.Observe(float64(data.ProcessingTime.Nanoseconds()))
}

// handleBatchEvent 处理批次事件
func (rp *RealtimeUpdatePipeline) handleBatchEvent(event BatchUpdateEvent) {
	log.Printf("Batch event: %d entries", event.BatchSize)

	// 更新实时计算
	if rp.metricsCalculator != nil {
		for _, data := range event.Entries {
			rp.metricsCalculator.UpdateWithNewEntry(data.Entry)
		}
	}

	// 分发批次事件
	rp.eventDispatcher.DispatchAsync(event)

	// 更新指标
	rp.metrics.BatchesProcessed.Inc()
	rp.metrics.BatchSize.Observe(float64(event.BatchSize))

	rp.mu.Lock()
	rp.state.ProcessedCount += int64(event.BatchSize)
	rp.state.LastUpdate = time.Now()
	rp.mu.Unlock()
}

// handleError 处理错误
func (rp *RealtimeUpdatePipeline) handleError(err error) {
	log.Printf("Pipeline error: %v", err)

	rp.mu.Lock()
	rp.state.ErrorCount++
	rp.mu.Unlock()

	rp.metrics.ProcessingErrors.Inc()

	// 可以在这里添加错误恢复逻辑
	// 例如重启失败的组件或发送告警
}

// refreshUI 刷新UI
func (rp *RealtimeUpdatePipeline) refreshUI() {
	if rp.metricsCalculator == nil {
		return
	}

	// 更新进度条
	if rp.progressBar != nil {
		// 这里可以根据实际需求更新进度
		metrics := rp.metricsCalculator.Calculate()
		rp.progressBar.Update(float64(metrics.CurrentTokens))
	}

	// 更新统计表格
	if rp.statisticsTable != nil {
		metrics := rp.metricsCalculator.Calculate()
		rp.statisticsTable.Update(metrics)
	}

	// 分发UI刷新事件
	refreshEvent := UIRefreshEvent{
		Status:    rp.GetStatus(),
		Timestamp: time.Now(),
	}
	rp.eventDispatcher.DispatchAsync(refreshEvent)

	rp.metrics.UIRefreshes.Inc()
}

// refreshData 刷新数据
func (rp *RealtimeUpdatePipeline) refreshData() {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		rp.metrics.FullRefreshTime.Observe(float64(duration.Nanoseconds()))
	}()

	// 强制刷新批次聚合器
	if err := rp.batchAggregator.ForceFlush(); err != nil {
		log.Printf("Failed to force flush batch aggregator: %v", err)
	}

	log.Println("Data refresh completed")
}

// collectMetrics 收集指标
func (rp *RealtimeUpdatePipeline) collectMetrics() {
	// 收集队列深度
	queueDepth := float64(rp.eventDispatcher.GetQueueSize())
	rp.metrics.QueueDepth.Observe(queueDepth)
	rp.metrics.CurrentQueueSize.Set(queueDepth)

	// 收集活跃工作者数量
	dispatcherStats := rp.eventDispatcher.GetStats()
	rp.metrics.ActiveWorkers.Set(float64(dispatcherStats.ActiveWorkers))

	// 收集内存使用情况（简化实现）
	// 实际实现中可以使用runtime包获取真实内存使用
	rp.metrics.MemoryUsage.Set(float64(len(rp.state.CurrentFiles)))
}

// setupComponentConnections 设置组件间连接
func (rp *RealtimeUpdatePipeline) setupComponentConnections() error {
	// 连接流读取器和数据处理器
	go func() {
		for data := range rp.streamReader.Data() {
			select {
			case rp.dataProcessor.Input() <- data:
			case <-rp.ctx.Done():
				return
			default:
				log.Printf("Data processor input channel full, dropping data")
				rp.metrics.DroppedUpdates.Inc()
			}
		}
	}()

	return nil
}

// registerEventHandlers 注册事件处理器
func (rp *RealtimeUpdatePipeline) registerEventHandlers() {
	// 注册文件事件处理器
	rp.eventDispatcher.RegisterHandlerFunc(
		reflect.TypeOf(FileEvent{}),
		func(event interface{}) error {
			fileEvent := event.(FileEvent)
			log.Printf("Handling file event: %s %s", fileEvent.Type, fileEvent.Path)
			return nil
		},
	)

	// 注册批次事件处理器
	rp.eventDispatcher.RegisterHandlerFunc(
		reflect.TypeOf(BatchUpdateEvent{}),
		func(event interface{}) error {
			batchEvent := event.(BatchUpdateEvent)
			log.Printf("Handling batch event: %d entries", batchEvent.BatchSize)
			return nil
		},
	)

	// 注册UI刷新事件处理器
	rp.eventDispatcher.RegisterHandlerFunc(
		reflect.TypeOf(UIRefreshEvent{}),
		func(event interface{}) error {
			// UI刷新处理逻辑
			return nil
		},
	)

	// 添加中间件
	rp.eventDispatcher.AddMiddleware(LoggingMiddleware)
	if rp.config.EnableCompression {
		rp.eventDispatcher.AddMiddleware(MetricsMiddleware)
	}
}

// initializeUIComponents 初始化UI组件
func (rp *RealtimeUpdatePipeline) initializeUIComponents() error {
	// 初始化实时计算器
	rp.metricsCalculator = calculations.NewMetricsCalculator(time.Now(), nil)

	// 初始化进度条
	rp.progressBar = components.NewProgressBar("Progress", 0, 100)

	// 初始化统计表格
	rp.statisticsTable = components.NewStatisticsTable(80)

	return nil
}

// GetStatus 获取管道状态
func (rp *RealtimeUpdatePipeline) GetStatus() *PipelineState {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	// 返回状态副本
	status := *rp.state
	status.CurrentFiles = make(map[string]FileState)
	for k, v := range rp.state.CurrentFiles {
		status.CurrentFiles[k] = v
	}

	return &status
}

// GetMetrics 获取管道指标
func (rp *RealtimeUpdatePipeline) GetMetrics() *PipelineMetrics {
	return rp.metrics
}

// GetComponentStats 获取组件统计信息
func (rp *RealtimeUpdatePipeline) GetComponentStats() map[string]interface{} {
	return map[string]interface{}{
		"file_watcher":     rp.fileWatcher.GetAllFileStates(),
		"stream_reader":    rp.streamReader.GetStats(),
		"data_processor":   rp.dataProcessor.GetStats(),
		"batch_aggregator": rp.batchAggregator.GetStats(),
		"event_dispatcher": rp.eventDispatcher.GetStats(),
	}
}

// IsRunning 检查管道是否正在运行
func (rp *RealtimeUpdatePipeline) IsRunning() bool {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.isRunning
}

// AddWatchPath 添加监控路径
func (rp *RealtimeUpdatePipeline) AddWatchPath(path string) error {
	return rp.fileWatcher.AddPath(path)
}

// RemoveWatchPath 移除监控路径
func (rp *RealtimeUpdatePipeline) RemoveWatchPath(path string) error {
	return rp.fileWatcher.RemovePath(path)
}

// UpdateConfig 更新配置
func (rp *RealtimeUpdatePipeline) UpdateConfig(config PipelineConfig) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	rp.config = config

	// 更新文件模式
	rp.fileWatcher.SetPatterns(config.FilePatterns, config.IgnorePatterns)

	return nil
}

// Errors 获取错误通道
func (rp *RealtimeUpdatePipeline) Errors() <-chan error {
	return rp.errorChannel
}

// NewPipelineMetrics 创建管道指标
func NewPipelineMetrics() *PipelineMetrics {
	return &PipelineMetrics{
		ProcessedEntries: &SimpleCounter{},
		DroppedUpdates:   &SimpleCounter{},
		ProcessingErrors: &SimpleCounter{},
		BatchesProcessed: &SimpleCounter{},
		UIRefreshes:      &SimpleCounter{},
		ProcessingTime:   &SimpleHistogram{},
		BatchSize:        &SimpleHistogram{},
		QueueDepth:       &SimpleHistogram{},
		FullRefreshTime:  &SimpleHistogram{},
		CurrentQueueSize: &SimpleGauge{},
		ActiveWorkers:    &SimpleGauge{},
		MemoryUsage:      &SimpleGauge{},
	}
}

// 简单的指标实现
type SimpleCounter struct {
	value int64
	mu    sync.RWMutex
}

func (c *SimpleCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *SimpleCounter) Add(delta float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += int64(delta)
}

func (c *SimpleCounter) Value() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

type SimpleHistogram struct{}

func (h *SimpleHistogram) Observe(value float64) {
	// 简化实现，实际中应该记录分布信息
}

type SimpleGauge struct {
	value float64
	mu    sync.RWMutex
}

func (g *SimpleGauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

func (g *SimpleGauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

func (g *SimpleGauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}
