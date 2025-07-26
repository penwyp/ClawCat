# 实时更新管道开发计划

## 1. 功能概述

实时更新管道是 claudecat 的核心基础设施，负责监控文件变化、处理新数据、更新统计信息并刷新 UI。该系统需要实现 10 秒数据刷新周期、0.75Hz UI 刷新率，并通过批量更新等优化确保高性能和流畅的用户体验。

### 1.1 核心需求

- **文件监控**: 实时监测 JSONL 文件变化
- **增量处理**: 只处理新增的数据，避免重复计算
- **批量更新**: 累积多个更新后统一处理
- **性能优化**: 确保低延迟和高吞吐量
- **容错机制**: 处理文件锁定、读取失败等异常情况

## 2. 技术设计

### 2.1 架构设计

```go
// UpdatePipeline 实时更新管道
type UpdatePipeline struct {
    // 核心组件
    watcher         *FileWatcher      // 文件监控器
    reader          *StreamReader     // 流式读取器
    processor       *DataProcessor    // 数据处理器
    aggregator      *UpdateAggregator // 更新聚合器
    dispatcher      *EventDispatcher  // 事件分发器
    
    // 配置
    config          PipelineConfig
    
    // 状态管理
    state           PipelineState
    metrics         PipelineMetrics
    
    // 并发控制
    updateQueue     chan Update
    batchTicker     *time.Ticker
    stopCh          chan struct{}
    wg              sync.WaitGroup
}

// PipelineConfig 管道配置
type PipelineConfig struct {
    // 更新频率
    DataRefreshInterval   time.Duration // 10秒
    UIRefreshRate         float64       // 0.75Hz
    BatchSize             int           // 批量大小
    BatchTimeout          time.Duration // 批量超时
    
    // 性能配置
    MaxConcurrency        int
    BufferSize            int
    EnableCompression     bool
    
    // 文件监控
    WatchPaths            []string
    FilePatterns          []string
    IgnorePatterns        []string
}

// Update 更新数据
type Update struct {
    Type        UpdateType
    Source      string
    Data        interface{}
    Timestamp   time.Time
    Priority    Priority
}

// UpdateType 更新类型
type UpdateType string

const (
    UpdateTypeNewEntry    UpdateType = "new_entry"
    UpdateTypeFileChange  UpdateType = "file_change"
    UpdateTypeStats       UpdateType = "stats"
    UpdateTypeConfig      UpdateType = "config"
)

// PipelineState 管道状态
type PipelineState struct {
    Running         bool
    LastUpdate      time.Time
    ProcessedCount  int64
    ErrorCount      int64
    CurrentFiles    map[string]FileState
}

// FileState 文件状态
type FileState struct {
    Path            string
    Size            int64
    LastModified    time.Time
    LastRead        time.Time
    ReadPosition    int64
    Checksum        string
}
```

### 2.2 文件监控器设计

```go
// FileWatcher 增强的文件监控器
type FileWatcher struct {
    paths           []string
    patterns        []string
    events          chan FileEvent
    errors          chan error
    
    // 状态跟踪
    fileStates      map[string]*FileState
    mu              sync.RWMutex
    
    // 性能优化
    debouncer       *Debouncer
    cache           *FileCache
}

// FileEvent 文件事件
type FileEvent struct {
    Type            EventType
    Path            string
    OldState        *FileState
    NewState        *FileState
    Changes         []Change
}

// EventType 事件类型
type EventType string

const (
    EventCreate     EventType = "create"
    EventModify     EventType = "modify"
    EventDelete     EventType = "delete"
    EventRename     EventType = "rename"
    EventTruncate   EventType = "truncate"
)

// Watch 开始监控
func (fw *FileWatcher) Watch() error
func (fw *FileWatcher) Stop() error
func (fw *FileWatcher) AddPath(path string) error
func (fw *FileWatcher) RemovePath(path string) error
func (fw *FileWatcher) GetFileState(path string) *FileState
```

### 2.3 流式数据处理器

```go
// StreamReader 流式读取器
type StreamReader struct {
    file            *os.File
    scanner         *bufio.Scanner
    position        int64
    buffer          *RingBuffer
    decoder         JSONDecoder
    
    // 性能优化
    readAhead       bool
    bufferSize      int
    maxLineLength   int
}

// DataProcessor 数据处理器
type DataProcessor struct {
    validators      []Validator
    transformers    []Transformer
    enrichers       []Enricher
    
    // 并发处理
    workerPool      *WorkerPool
    resultChan      chan ProcessedData
}

// ProcessedData 处理后的数据
type ProcessedData struct {
    Original        []byte
    Entry           models.UsageEntry
    Metadata        map[string]interface{}
    ProcessingTime  time.Duration
}

// Process 处理数据
func (dp *DataProcessor) Process(raw []byte) (*ProcessedData, error) {
    start := time.Now()
    
    // 1. 验证
    if err := dp.validate(raw); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 2. 解析
    var entry models.UsageEntry
    if err := dp.decode(raw, &entry); err != nil {
        return nil, fmt.Errorf("decode failed: %w", err)
    }
    
    // 3. 转换
    if err := dp.transform(&entry); err != nil {
        return nil, fmt.Errorf("transform failed: %w", err)
    }
    
    // 4. 增强
    metadata := dp.enrich(&entry)
    
    return &ProcessedData{
        Original:       raw,
        Entry:          entry,
        Metadata:       metadata,
        ProcessingTime: time.Since(start),
    }, nil
}
```

## 3. 实现步骤

### 3.1 创建更新管道

**文件**: `pipeline/pipeline.go`

```go
package pipeline

import (
    "context"
    "sync"
    "time"
    "github.com/penwyp/claudecat/models"
    "github.com/penwyp/claudecat/config"
)

// NewUpdatePipeline 创建更新管道
func NewUpdatePipeline(cfg *config.Config) (*UpdatePipeline, error) {
    pipelineConfig := PipelineConfig{
        DataRefreshInterval: 10 * time.Second,
        UIRefreshRate:       0.75,
        BatchSize:           100,
        BatchTimeout:        500 * time.Millisecond,
        MaxConcurrency:      runtime.NumCPU(),
        BufferSize:          1000,
        WatchPaths:          cfg.Data.Paths,
        FilePatterns:        []string{"*.jsonl"},
    }
    
    pipeline := &UpdatePipeline{
        config:      pipelineConfig,
        updateQueue: make(chan Update, pipelineConfig.BufferSize),
        stopCh:      make(chan struct{}),
        state: PipelineState{
            CurrentFiles: make(map[string]FileState),
        },
    }
    
    // 初始化组件
    if err := pipeline.initComponents(); err != nil {
        return nil, fmt.Errorf("failed to init components: %w", err)
    }
    
    return pipeline, nil
}

// initComponents 初始化组件
func (p *UpdatePipeline) initComponents() error {
    // 创建文件监控器
    p.watcher = NewFileWatcher(p.config.WatchPaths, p.config.FilePatterns)
    
    // 创建流式读取器
    p.reader = NewStreamReader()
    
    // 创建数据处理器
    p.processor = NewDataProcessor(p.config.MaxConcurrency)
    
    // 创建更新聚合器
    p.aggregator = NewUpdateAggregator(p.config.BatchSize, p.config.BatchTimeout)
    
    // 创建事件分发器
    p.dispatcher = NewEventDispatcher()
    
    return nil
}

// Start 启动管道
func (p *UpdatePipeline) Start(ctx context.Context) error {
    if p.state.Running {
        return fmt.Errorf("pipeline already running")
    }
    
    p.state.Running = true
    p.state.LastUpdate = time.Now()
    
    // 启动各个组件
    p.wg.Add(4)
    
    // 1. 文件监控协程
    go p.runFileWatcher(ctx)
    
    // 2. 数据处理协程
    go p.runDataProcessor(ctx)
    
    // 3. 批量更新协程
    go p.runBatchUpdater(ctx)
    
    // 4. UI 刷新协程
    go p.runUIRefresher(ctx)
    
    // 启动定期数据刷新
    go p.runPeriodicRefresh(ctx)
    
    return nil
}

// runFileWatcher 运行文件监控器
func (p *UpdatePipeline) runFileWatcher(ctx context.Context) {
    defer p.wg.Done()
    
    // 启动文件监控
    eventCh, errorCh := p.watcher.Start()
    
    for {
        select {
        case <-ctx.Done():
            p.watcher.Stop()
            return
            
        case event := <-eventCh:
            p.handleFileEvent(event)
            
        case err := <-errorCh:
            p.handleWatchError(err)
        }
    }
}

// handleFileEvent 处理文件事件
func (p *UpdatePipeline) handleFileEvent(event FileEvent) {
    switch event.Type {
    case EventModify, EventCreate:
        // 读取新增内容
        newData, err := p.readNewContent(event.Path, event.OldState, event.NewState)
        if err != nil {
            p.logError("Failed to read new content", err)
            return
        }
        
        // 发送到处理队列
        for _, data := range newData {
            update := Update{
                Type:      UpdateTypeNewEntry,
                Source:    event.Path,
                Data:      data,
                Timestamp: time.Now(),
                Priority:  PriorityNormal,
            }
            
            select {
            case p.updateQueue <- update:
            default:
                // 队列满，记录丢失
                p.metrics.DroppedUpdates.Inc()
            }
        }
        
    case EventDelete:
        // 处理文件删除
        p.handleFileDelete(event.Path)
    }
}

// readNewContent 读取新增内容
func (p *UpdatePipeline) readNewContent(path string, oldState, newState *FileState) ([][]byte, error) {
    // 获取或创建读取器
    reader, err := p.reader.GetReader(path)
    if err != nil {
        return nil, err
    }
    
    // 定位到上次读取位置
    startPos := int64(0)
    if oldState != nil {
        startPos = oldState.ReadPosition
    }
    
    // 读取新内容
    newLines, err := reader.ReadFrom(startPos)
    if err != nil {
        return nil, err
    }
    
    // 更新文件状态
    p.updateFileState(path, newState.Size)
    
    return newLines, nil
}

// runDataProcessor 运行数据处理器
func (p *UpdatePipeline) runDataProcessor(ctx context.Context) {
    defer p.wg.Done()
    
    for {
        select {
        case <-ctx.Done():
            return
            
        case update := <-p.updateQueue:
            p.processUpdate(update)
        }
    }
}

// processUpdate 处理更新
func (p *UpdatePipeline) processUpdate(update Update) {
    start := time.Now()
    
    switch update.Type {
    case UpdateTypeNewEntry:
        // 处理新条目
        if data, ok := update.Data.([]byte); ok {
            processed, err := p.processor.Process(data)
            if err != nil {
                p.metrics.ProcessingErrors.Inc()
                return
            }
            
            // 发送到聚合器
            p.aggregator.Add(processed)
            
            // 更新指标
            p.metrics.ProcessedEntries.Inc()
            p.metrics.ProcessingTime.Observe(time.Since(start).Seconds())
        }
    }
}

// runBatchUpdater 运行批量更新器
func (p *UpdatePipeline) runBatchUpdater(ctx context.Context) {
    defer p.wg.Done()
    
    ticker := time.NewTicker(p.config.BatchTimeout)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            // 处理剩余的批次
            p.flushBatch()
            return
            
        case <-ticker.C:
            p.flushBatch()
        }
    }
}

// flushBatch 刷新批次
func (p *UpdatePipeline) flushBatch() {
    batch := p.aggregator.GetBatch()
    if len(batch) == 0 {
        return
    }
    
    // 创建批量更新事件
    event := BatchUpdateEvent{
        Entries:   batch,
        Timestamp: time.Now(),
        BatchSize: len(batch),
    }
    
    // 分发事件
    p.dispatcher.Dispatch(event)
    
    // 更新指标
    p.metrics.BatchesProcessed.Inc()
    p.metrics.BatchSize.Observe(float64(len(batch)))
}

// runUIRefresher UI 刷新器
func (p *UpdatePipeline) runUIRefresher(ctx context.Context) {
    defer p.wg.Done()
    
    // 计算刷新间隔 (0.75Hz = 1.33秒)
    interval := time.Duration(float64(time.Second) / p.config.UIRefreshRate)
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
            
        case <-ticker.C:
            p.refreshUI()
        }
    }
}

// refreshUI 刷新 UI
func (p *UpdatePipeline) refreshUI() {
    // 获取最新状态
    status := p.GetStatus()
    
    // 创建 UI 更新事件
    event := UIRefreshEvent{
        Status:    status,
        Timestamp: time.Now(),
    }
    
    // 分发到 UI 组件
    p.dispatcher.Dispatch(event)
    
    // 更新指标
    p.metrics.UIRefreshes.Inc()
}

// runPeriodicRefresh 定期数据刷新
func (p *UpdatePipeline) runPeriodicRefresh(ctx context.Context) {
    ticker := time.NewTicker(p.config.DataRefreshInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
            
        case <-ticker.C:
            p.performFullRefresh()
        }
    }
}

// performFullRefresh 执行完整刷新
func (p *UpdatePipeline) performFullRefresh() {
    start := time.Now()
    
    // 重新扫描所有文件
    for _, path := range p.config.WatchPaths {
        files, err := p.discoverFiles(path)
        if err != nil {
            p.logError("Failed to discover files", err)
            continue
        }
        
        for _, file := range files {
            // 检查文件是否有更新
            if p.shouldRefreshFile(file) {
                event := FileEvent{
                    Type:     EventModify,
                    Path:     file,
                    OldState: p.getFileState(file),
                    NewState: p.getCurrentFileState(file),
                }
                
                p.handleFileEvent(event)
            }
        }
    }
    
    p.metrics.FullRefreshTime.Observe(time.Since(start).Seconds())
}
```

### 3.2 优化的文件监控器

**文件**: `pipeline/file_watcher.go`

```go
package pipeline

import (
    "crypto/md5"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sync"
    "time"
    "github.com/fsnotify/fsnotify"
)

// EnhancedFileWatcher 增强的文件监控器
type EnhancedFileWatcher struct {
    watcher       *fsnotify.Watcher
    paths         []string
    patterns      []string
    
    // 事件处理
    eventCh       chan FileEvent
    errorCh       chan error
    
    // 状态管理
    fileStates    map[string]*FileState
    stateMu       sync.RWMutex
    
    // 性能优化
    debouncer     *EventDebouncer
    checksumCache *ChecksumCache
    
    // 配置
    config        WatcherConfig
}

// WatcherConfig 监控器配置
type WatcherConfig struct {
    DebounceDelay    time.Duration
    ChecksumEnabled  bool
    IgnoreHidden     bool
    MaxFileSize      int64
}

// NewEnhancedFileWatcher 创建增强的文件监控器
func NewEnhancedFileWatcher(paths, patterns []string) (*EnhancedFileWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }
    
    fw := &EnhancedFileWatcher{
        watcher:       watcher,
        paths:         paths,
        patterns:      patterns,
        eventCh:       make(chan FileEvent, 100),
        errorCh:       make(chan error, 10),
        fileStates:    make(map[string]*FileState),
        checksumCache: NewChecksumCache(100),
        config: WatcherConfig{
            DebounceDelay:   100 * time.Millisecond,
            ChecksumEnabled: true,
            IgnoreHidden:    true,
            MaxFileSize:     1 << 30, // 1GB
        },
    }
    
    fw.debouncer = NewEventDebouncer(fw.config.DebounceDelay, fw.handleDebouncedEvent)
    
    return fw, nil
}

// Start 启动监控
func (fw *EnhancedFileWatcher) Start() (<-chan FileEvent, <-chan error) {
    // 添加监控路径
    for _, path := range fw.paths {
        if err := fw.addPath(path); err != nil {
            fw.errorCh <- fmt.Errorf("failed to watch %s: %w", path, err)
        }
    }
    
    // 初始扫描
    fw.initialScan()
    
    // 启动事件处理
    go fw.eventLoop()
    
    return fw.eventCh, fw.errorCh
}

// eventLoop 事件循环
func (fw *EnhancedFileWatcher) eventLoop() {
    for {
        select {
        case event, ok := <-fw.watcher.Events:
            if !ok {
                return
            }
            fw.handleRawEvent(event)
            
        case err, ok := <-fw.watcher.Errors:
            if !ok {
                return
            }
            fw.errorCh <- err
        }
    }
}

// handleRawEvent 处理原始事件
func (fw *EnhancedFileWatcher) handleRawEvent(event fsnotify.Event) {
    // 过滤不需要的文件
    if !fw.shouldWatch(event.Name) {
        return
    }
    
    // 防抖处理
    fw.debouncer.Add(event.Name, event)
}

// handleDebouncedEvent 处理防抖后的事件
func (fw *EnhancedFileWatcher) handleDebouncedEvent(path string, events []interface{}) {
    // 合并多个事件
    var lastOp fsnotify.Op
    for _, e := range events {
        if event, ok := e.(fsnotify.Event); ok {
            lastOp |= event.Op
        }
    }
    
    // 获取文件状态
    oldState := fw.getFileState(path)
    newState, err := fw.getCurrentFileState(path)
    if err != nil && !os.IsNotExist(err) {
        fw.errorCh <- err
        return
    }
    
    // 生成文件事件
    var eventType EventType
    
    switch {
    case lastOp&fsnotify.Create == fsnotify.Create:
        eventType = EventCreate
    case lastOp&fsnotify.Remove == fsnotify.Remove:
        eventType = EventDelete
    case lastOp&fsnotify.Rename == fsnotify.Rename:
        eventType = EventRename
    case lastOp&fsnotify.Write == fsnotify.Write:
        eventType = EventModify
        
        // 检查是否被截断
        if oldState != nil && newState != nil && newState.Size < oldState.Size {
            eventType = EventTruncate
        }
    }
    
    // 检查是否真的有变化
    if eventType == EventModify && fw.config.ChecksumEnabled {
        if oldState != nil && newState != nil && oldState.Checksum == newState.Checksum {
            // 文件内容没有变化
            return
        }
    }
    
    // 生成变化列表
    changes := fw.detectChanges(oldState, newState)
    
    // 发送事件
    fw.eventCh <- FileEvent{
        Type:     eventType,
        Path:     path,
        OldState: oldState,
        NewState: newState,
        Changes:  changes,
    }
    
    // 更新状态
    if newState != nil {
        fw.updateFileState(path, newState)
    } else {
        fw.removeFileState(path)
    }
}

// detectChanges 检测具体变化
func (fw *EnhancedFileWatcher) detectChanges(oldState, newState *FileState) []Change {
    if oldState == nil || newState == nil {
        return nil
    }
    
    changes := []Change{}
    
    // 大小变化
    if oldState.Size != newState.Size {
        changes = append(changes, Change{
            Type:  ChangeSize,
            OldValue: oldState.Size,
            NewValue: newState.Size,
        })
    }
    
    // 时间变化
    if !oldState.LastModified.Equal(newState.LastModified) {
        changes = append(changes, Change{
            Type:  ChangeModTime,
            OldValue: oldState.LastModified,
            NewValue: newState.LastModified,
        })
    }
    
    // 内容变化
    if oldState.Checksum != newState.Checksum {
        changes = append(changes, Change{
            Type:  ChangeContent,
            OldValue: oldState.Checksum,
            NewValue: newState.Checksum,
        })
    }
    
    return changes
}

// getCurrentFileState 获取当前文件状态
func (fw *EnhancedFileWatcher) getCurrentFileState(path string) (*FileState, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }
    
    state := &FileState{
        Path:         path,
        Size:         info.Size(),
        LastModified: info.ModTime(),
        LastRead:     time.Now(),
    }
    
    // 计算校验和
    if fw.config.ChecksumEnabled && info.Size() < fw.config.MaxFileSize {
        checksum, err := fw.calculateChecksum(path)
        if err == nil {
            state.Checksum = checksum
        }
    }
    
    return state, nil
}

// calculateChecksum 计算文件校验和
func (fw *EnhancedFileWatcher) calculateChecksum(path string) (string, error) {
    // 检查缓存
    if cached, ok := fw.checksumCache.Get(path); ok {
        return cached, nil
    }
    
    file, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer file.Close()
    
    hash := md5.New()
    if _, err := io.Copy(hash, file); err != nil {
        return "", err
    }
    
    checksum := fmt.Sprintf("%x", hash.Sum(nil))
    
    // 更新缓存
    fw.checksumCache.Set(path, checksum)
    
    return checksum, nil
}

// EventDebouncer 事件防抖器
type EventDebouncer struct {
    delay       time.Duration
    callback    func(string, []interface{})
    pending     map[string]*debounceItem
    mu          sync.Mutex
}

type debounceItem struct {
    events []interface{}
    timer  *time.Timer
}

// NewEventDebouncer 创建事件防抖器
func NewEventDebouncer(delay time.Duration, callback func(string, []interface{})) *EventDebouncer {
    return &EventDebouncer{
        delay:    delay,
        callback: callback,
        pending:  make(map[string]*debounceItem),
    }
}

// Add 添加事件
func (d *EventDebouncer) Add(key string, event interface{}) {
    d.mu.Lock()
    defer d.mu.Unlock()
    
    item, exists := d.pending[key]
    if !exists {
        item = &debounceItem{
            events: []interface{}{event},
        }
        d.pending[key] = item
    } else {
        item.events = append(item.events, event)
        item.timer.Stop()
    }
    
    // 设置定时器
    item.timer = time.AfterFunc(d.delay, func() {
        d.mu.Lock()
        defer d.mu.Unlock()
        
        if item, ok := d.pending[key]; ok {
            delete(d.pending, key)
            d.callback(key, item.events)
        }
    })
}
```

### 3.3 批量更新聚合器

**文件**: `pipeline/aggregator.go`

```go
package pipeline

import (
    "sync"
    "time"
)

// UpdateAggregator 更新聚合器
type UpdateAggregator struct {
    batchSize    int
    timeout      time.Duration
    
    // 当前批次
    currentBatch []ProcessedData
    batchMu      sync.Mutex
    
    // 统计信息
    stats        AggregatorStats
}

// AggregatorStats 聚合器统计
type AggregatorStats struct {
    TotalBatches   int64
    TotalItems     int64
    AverageBatchSize float64
    LastBatchTime  time.Time
}

// NewUpdateAggregator 创建更新聚合器
func NewUpdateAggregator(batchSize int, timeout time.Duration) *UpdateAggregator {
    return &UpdateAggregator{
        batchSize:    batchSize,
        timeout:      timeout,
        currentBatch: make([]ProcessedData, 0, batchSize),
    }
}

// Add 添加数据到批次
func (ua *UpdateAggregator) Add(data *ProcessedData) {
    ua.batchMu.Lock()
    defer ua.batchMu.Unlock()
    
    ua.currentBatch = append(ua.currentBatch, *data)
    ua.stats.TotalItems++
    
    // 检查是否需要刷新
    if len(ua.currentBatch) >= ua.batchSize {
        // 批次已满，需要外部调用 GetBatch
    }
}

// GetBatch 获取当前批次
func (ua *UpdateAggregator) GetBatch() []ProcessedData {
    ua.batchMu.Lock()
    defer ua.batchMu.Unlock()
    
    if len(ua.currentBatch) == 0 {
        return nil
    }
    
    // 复制批次数据
    batch := make([]ProcessedData, len(ua.currentBatch))
    copy(batch, ua.currentBatch)
    
    // 重置当前批次
    ua.currentBatch = ua.currentBatch[:0]
    
    // 更新统计
    ua.stats.TotalBatches++
    ua.stats.LastBatchTime = time.Now()
    ua.updateAverageBatchSize(len(batch))
    
    return batch
}

// updateAverageBatchSize 更新平均批次大小
func (ua *UpdateAggregator) updateAverageBatchSize(size int) {
    // 使用指数移动平均
    alpha := 0.1
    ua.stats.AverageBatchSize = alpha*float64(size) + (1-alpha)*ua.stats.AverageBatchSize
}

// GetStats 获取统计信息
func (ua *UpdateAggregator) GetStats() AggregatorStats {
    ua.batchMu.Lock()
    defer ua.batchMu.Unlock()
    
    return ua.stats
}

// OptimizedAggregator 优化的聚合器（支持优先级和压缩）
type OptimizedAggregator struct {
    *UpdateAggregator
    
    // 优先级队列
    priorityQueue PriorityQueue
    
    // 压缩
    compressor    Compressor
    compressionEnabled bool
}

// AddWithPriority 添加带优先级的数据
func (oa *OptimizedAggregator) AddWithPriority(data *ProcessedData, priority Priority) {
    item := &PriorityItem{
        Data:     data,
        Priority: priority,
        Index:    -1,
    }
    
    heap.Push(&oa.priorityQueue, item)
    
    // 检查是否需要刷新高优先级数据
    if priority == PriorityHigh && oa.priorityQueue.Len() > 10 {
        // 触发立即刷新
    }
}

// GetOptimizedBatch 获取优化的批次
func (oa *OptimizedAggregator) GetOptimizedBatch() []ProcessedData {
    batch := []ProcessedData{}
    
    // 先处理高优先级
    for oa.priorityQueue.Len() > 0 && len(batch) < oa.batchSize {
        item := heap.Pop(&oa.priorityQueue).(*PriorityItem)
        batch = append(batch, *item.Data)
    }
    
    // 压缩批次数据
    if oa.compressionEnabled && len(batch) > 50 {
        batch = oa.compressBatch(batch)
    }
    
    return batch
}

// compressBatch 压缩批次
func (oa *OptimizedAggregator) compressBatch(batch []ProcessedData) []ProcessedData {
    // 实现数据压缩逻辑
    // 例如：合并相似的条目，去除冗余数据
    compressed := []ProcessedData{}
    
    // 按模型分组
    grouped := make(map[string][]ProcessedData)
    for _, data := range batch {
        model := data.Entry.Model
        grouped[model] = append(grouped[model], data)
    }
    
    // 合并每组数据
    for model, group := range grouped {
        if len(group) == 1 {
            compressed = append(compressed, group[0])
        } else {
            // 合并多个条目
            merged := oa.mergeEntries(model, group)
            compressed = append(compressed, merged)
        }
    }
    
    return compressed
}
```

### 3.4 事件分发器

**文件**: `pipeline/dispatcher.go`

```go
package pipeline

import (
    "sync"
    "reflect"
)

// EventDispatcher 事件分发器
type EventDispatcher struct {
    handlers map[reflect.Type][]EventHandler
    mu       sync.RWMutex
    
    // 性能优化
    handlerCache map[reflect.Type][]*cachedHandler
    eventPool    sync.Pool
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

// NewEventDispatcher 创建事件分发器
func NewEventDispatcher() *EventDispatcher {
    return &EventDispatcher{
        handlers:     make(map[reflect.Type][]EventHandler),
        handlerCache: make(map[reflect.Type][]*cachedHandler),
        eventPool: sync.Pool{
            New: func() interface{} {
                return &eventWrapper{}
            },
        },
    }
}

// Subscribe 订阅事件
func (ed *EventDispatcher) Subscribe(eventType interface{}, handler EventHandler) {
    ed.mu.Lock()
    defer ed.mu.Unlock()
    
    t := reflect.TypeOf(eventType)
    ed.handlers[t] = append(ed.handlers[t], handler)
    
    // 清除缓存
    delete(ed.handlerCache, t)
}

// Unsubscribe 取消订阅
func (ed *EventDispatcher) Unsubscribe(eventType interface{}, handler EventHandler) {
    ed.mu.Lock()
    defer ed.mu.Unlock()
    
    t := reflect.TypeOf(eventType)
    handlers := ed.handlers[t]
    
    for i, h := range handlers {
        if h == handler {
            ed.handlers[t] = append(handlers[:i], handlers[i+1:]...)
            break
        }
    }
    
    // 清除缓存
    delete(ed.handlerCache, t)
}

// Dispatch 分发事件
func (ed *EventDispatcher) Dispatch(event interface{}) {
    t := reflect.TypeOf(event)
    
    // 获取处理器
    handlers := ed.getHandlers(t)
    if len(handlers) == 0 {
        return
    }
    
    // 并发处理
    var wg sync.WaitGroup
    wg.Add(len(handlers))
    
    for _, handler := range handlers {
        go func(h EventHandler) {
            defer wg.Done()
            
            if err := h.Handle(event); err != nil {
                // 记录错误
                ed.logError(err)
            }
        }(handler)
    }
    
    // 等待所有处理器完成
    wg.Wait()
}

// DispatchAsync 异步分发事件
func (ed *EventDispatcher) DispatchAsync(event interface{}) {
    go ed.Dispatch(event)
}

// getHandlers 获取处理器（带缓存）
func (ed *EventDispatcher) getHandlers(t reflect.Type) []EventHandler {
    ed.mu.RLock()
    
    // 检查缓存
    if cached, ok := ed.handlerCache[t]; ok {
        ed.mu.RUnlock()
        
        handlers := make([]EventHandler, len(cached))
        for i, ch := range cached {
            handlers[i] = ch.handler
        }
        return handlers
    }
    
    // 获取原始处理器
    handlers := ed.handlers[t]
    ed.mu.RUnlock()
    
    if len(handlers) == 0 {
        return nil
    }
    
    // 创建缓存
    ed.mu.Lock()
    cached := make([]*cachedHandler, len(handlers))
    for i, h := range handlers {
        cached[i] = &cachedHandler{handler: h}
    }
    ed.handlerCache[t] = cached
    ed.mu.Unlock()
    
    return handlers
}

// TypedEventDispatcher 类型安全的事件分发器
type TypedEventDispatcher[T any] struct {
    handlers []func(T) error
    mu       sync.RWMutex
}

// NewTypedEventDispatcher 创建类型安全的事件分发器
func NewTypedEventDispatcher[T any]() *TypedEventDispatcher[T] {
    return &TypedEventDispatcher[T]{
        handlers: make([]func(T) error, 0),
    }
}

// Subscribe 订阅事件
func (ted *TypedEventDispatcher[T]) Subscribe(handler func(T) error) {
    ted.mu.Lock()
    defer ted.mu.Unlock()
    
    ted.handlers = append(ted.handlers, handler)
}

// Dispatch 分发事件
func (ted *TypedEventDispatcher[T]) Dispatch(event T) {
    ted.mu.RLock()
    handlers := make([]func(T) error, len(ted.handlers))
    copy(handlers, ted.handlers)
    ted.mu.RUnlock()
    
    for _, handler := range handlers {
        if err := handler(event); err != nil {
            // 处理错误
        }
    }
}
```

## 4. 测试计划

### 4.1 单元测试

```go
// pipeline/pipeline_test.go

func TestUpdatePipeline_ProcessUpdate(t *testing.T) {
    pipeline := createTestPipeline(t)
    
    // 创建测试数据
    testData := []byte(`{"type":"message","usage":{"input_tokens":100}}`)
    
    update := Update{
        Type:      UpdateTypeNewEntry,
        Data:      testData,
        Timestamp: time.Now(),
    }
    
    // 处理更新
    pipeline.processUpdate(update)
    
    // 验证处理结果
    assert.Equal(t, int64(1), pipeline.metrics.ProcessedEntries.Count())
}

func TestFileWatcher_Debouncing(t *testing.T) {
    watcher := createTestWatcher(t)
    
    eventCount := 0
    watcher.OnEvent = func(event FileEvent) {
        eventCount++
    }
    
    // 快速触发多个事件
    for i := 0; i < 10; i++ {
        watcher.handleRawEvent(fsnotify.Event{
            Name: "test.jsonl",
            Op:   fsnotify.Write,
        })
        time.Sleep(10 * time.Millisecond)
    }
    
    // 等待防抖
    time.Sleep(200 * time.Millisecond)
    
    // 应该只触发一次
    assert.Equal(t, 1, eventCount)
}

func TestUpdateAggregator_Batching(t *testing.T) {
    aggregator := NewUpdateAggregator(10, 100*time.Millisecond)
    
    // 添加数据
    for i := 0; i < 25; i++ {
        aggregator.Add(&ProcessedData{
            Entry: models.UsageEntry{
                TotalTokens: i * 100,
            },
        })
    }
    
    // 获取批次
    batch1 := aggregator.GetBatch()
    assert.Len(t, batch1, 25)
    
    // 再次获取应该为空
    batch2 := aggregator.GetBatch()
    assert.Len(t, batch2, 0)
}
```

### 4.2 性能测试

```go
// pipeline/benchmark_test.go

func BenchmarkPipeline_ProcessEntry(b *testing.B) {
    pipeline := createBenchmarkPipeline(b)
    testData := generateTestEntry()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        update := Update{
            Type: UpdateTypeNewEntry,
            Data: testData,
        }
        pipeline.processUpdate(update)
    }
}

func BenchmarkFileWatcher_ChecksumCalculation(b *testing.B) {
    watcher := createTestWatcher(b)
    
    // 创建测试文件
    testFile := createTempFile(b, 1<<20) // 1MB
    defer os.Remove(testFile)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = watcher.calculateChecksum(testFile)
    }
}

func BenchmarkEventDispatcher_Dispatch(b *testing.B) {
    dispatcher := NewEventDispatcher()
    
    // 注册多个处理器
    for i := 0; i < 10; i++ {
        dispatcher.Subscribe(TestEvent{}, EventHandlerFunc(func(e interface{}) error {
            // 模拟处理
            time.Sleep(1 * time.Microsecond)
            return nil
        }))
    }
    
    event := TestEvent{ID: 1}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        dispatcher.Dispatch(event)
    }
}
```

### 4.3 集成测试

```go
// integration/pipeline_test.go

func TestPipeline_EndToEnd(t *testing.T) {
    // 创建测试环境
    tempDir := createTempDir(t)
    defer os.RemoveAll(tempDir)
    
    // 创建管道
    config := &config.Config{
        Data: config.Data{
            Paths: []string{tempDir},
        },
    }
    
    pipeline, err := NewUpdatePipeline(config)
    assert.NoError(t, err)
    
    // 启动管道
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    err = pipeline.Start(ctx)
    assert.NoError(t, err)
    
    // 创建测试文件
    testFile := filepath.Join(tempDir, "test.jsonl")
    
    // 模拟数据写入
    updates := 0
    pipeline.OnUpdate = func(batch []ProcessedData) {
        updates += len(batch)
    }
    
    // 写入数据
    for i := 0; i < 100; i++ {
        appendToFile(t, testFile, generateTestEntry())
        time.Sleep(10 * time.Millisecond)
    }
    
    // 等待处理
    time.Sleep(1 * time.Second)
    
    // 验证所有数据都被处理
    assert.Equal(t, 100, updates)
}

func TestPipeline_Performance(t *testing.T) {
    pipeline := createHighPerformancePipeline(t)
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    pipeline.Start(ctx)
    
    // 高速数据生成
    const targetThroughput = 10000 // 条目/秒
    
    start := time.Now()
    processed := int64(0)
    
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
                pipeline.updateQueue <- Update{
                    Type: UpdateTypeNewEntry,
                    Data: generateTestEntry(),
                }
            }
        }
    }()
    
    // 监控处理速度
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            duration := time.Since(start)
            throughput := float64(processed) / duration.Seconds()
            
            t.Logf("Processed %d entries in %v", processed, duration)
            t.Logf("Throughput: %.2f entries/sec", throughput)
            
            assert.Greater(t, throughput, float64(targetThroughput)*0.9)
            return
            
        case <-ticker.C:
            processed = pipeline.state.ProcessedCount
        }
    }
}
```

## 5. 性能优化策略

### 5.1 内存池

```go
// MemoryPool 内存池
type MemoryPool struct {
    entryPool   sync.Pool
    bufferPool  sync.Pool
    slicePool   sync.Pool
}

func NewMemoryPool() *MemoryPool {
    return &MemoryPool{
        entryPool: sync.Pool{
            New: func() interface{} {
                return &models.UsageEntry{}
            },
        },
        bufferPool: sync.Pool{
            New: func() interface{} {
                return make([]byte, 4096)
            },
        },
        slicePool: sync.Pool{
            New: func() interface{} {
                return make([]ProcessedData, 0, 100)
            },
        },
    }
}
```

### 5.2 并发控制

```go
// ConcurrencyController 并发控制器
type ConcurrencyController struct {
    maxWorkers   int
    semaphore    chan struct{}
    workQueue    chan Work
    workers      []*Worker
}

func (cc *ConcurrencyController) Start() {
    for i := 0; i < cc.maxWorkers; i++ {
        worker := NewWorker(i, cc.workQueue)
        cc.workers = append(cc.workers, worker)
        go worker.Start()
    }
}
```

## 6. 监控和指标

### 6.1 性能指标

```go
// PipelineMetrics 管道指标
type PipelineMetrics struct {
    // 计数器
    ProcessedEntries   Counter
    DroppedUpdates     Counter
    ProcessingErrors   Counter
    BatchesProcessed   Counter
    UIRefreshes        Counter
    
    // 直方图
    ProcessingTime     Histogram
    BatchSize          Histogram
    QueueDepth         Histogram
    FullRefreshTime    Histogram
    
    // 仪表
    CurrentQueueSize   Gauge
    ActiveWorkers      Gauge
    MemoryUsage        Gauge
}

// 收集指标
func (p *UpdatePipeline) collectMetrics() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        p.metrics.CurrentQueueSize.Set(float64(len(p.updateQueue)))
        p.metrics.MemoryUsage.Set(float64(runtime.MemStats.HeapAlloc))
    }
}
```

## 7. 配置选项

### 7.1 管道配置

```yaml
# config.yaml
pipeline:
  # 更新频率
  data_refresh_interval: 10s
  ui_refresh_rate: 0.75
  
  # 批处理
  batch_size: 100
  batch_timeout: 500ms
  
  # 并发
  max_concurrency: 8
  buffer_size: 1000
  
  # 文件监控
  file_watcher:
    debounce_delay: 100ms
    checksum_enabled: true
    ignore_hidden: true
    max_file_size: 1GB
    
  # 性能
  performance:
    enable_compression: true
    memory_pool_enabled: true
    profile_enabled: false
```

## 8. 错误处理

### 8.1 容错机制

```go
func (p *UpdatePipeline) handleError(err error, context string) {
    p.metrics.ProcessingErrors.Inc()
    
    // 分类错误
    switch {
    case isTransientError(err):
        // 暂时性错误，重试
        p.scheduleRetry(context, err)
        
    case isCriticalError(err):
        // 严重错误，降级
        p.enterDegradedMode(err)
        
    default:
        // 一般错误，记录并继续
        p.logError(context, err)
    }
}
```

## 9. 部署清单

- [ ] 实现 `pipeline/pipeline.go`
- [ ] 实现 `pipeline/file_watcher.go`
- [ ] 实现 `pipeline/aggregator.go`
- [ ] 实现 `pipeline/dispatcher.go`
- [ ] 添加内存池优化
- [ ] 实现并发控制
- [ ] 添加性能指标
- [ ] 编写单元测试
- [ ] 编写性能测试
- [ ] 集成测试
- [ ] 压力测试
- [ ] 更新配置
- [ ] 编写文档

## 10. 未来增强

- 分布式管道支持
- 智能批处理策略
- 自适应刷新率
- 机器学习预测
- 流量控制
- 数据压缩优化
- WebSocket 实时推送
- 插件系统支持