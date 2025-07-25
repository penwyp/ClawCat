package pipeline

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// EventDispatcher 事件分发器
type EventDispatcher struct {
	handlers        map[reflect.Type][]EventHandler
	eventChannel    chan interface{}
	errorChannel    chan error
	config          DispatcherConfig
	stats          *DispatcherStats
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	mu             sync.RWMutex
	isRunning      bool
	handlerPool    sync.Pool
	middleware     []MiddlewareFunc
}

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	WorkerCount       int           `json:"worker_count"`
	BufferSize        int           `json:"buffer_size"`
	HandlerTimeout    time.Duration `json:"handler_timeout"`
	EnableMiddleware  bool          `json:"enable_middleware"`
	EnableRecovery    bool          `json:"enable_recovery"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	EnableMetrics     bool          `json:"enable_metrics"`
	QueueWarningSize  int           `json:"queue_warning_size"`
}

// DefaultDispatcherConfig 默认分发器配置
func DefaultDispatcherConfig() DispatcherConfig {
	return DispatcherConfig{
		WorkerCount:      8,
		BufferSize:       2000,
		HandlerTimeout:   30 * time.Second,
		EnableMiddleware: true,
		EnableRecovery:   true,
		MaxRetries:       3,
		RetryDelay:       500 * time.Millisecond,
		EnableMetrics:    true,
		QueueWarningSize: 1500,
	}
}

// DispatcherStats 分发器统计
type DispatcherStats struct {
	TotalEvents       int64            `json:"total_events"`
	ProcessedEvents   int64            `json:"processed_events"`
	FailedEvents      int64            `json:"failed_events"`
	RetryEvents       int64            `json:"retry_events"`
	HandlerErrors     int64            `json:"handler_errors"`
	PanicRecoveries   int64            `json:"panic_recoveries"`
	AverageProcessTime time.Duration   `json:"average_process_time"`
	TotalProcessTime  time.Duration    `json:"total_process_time"`
	StartTime         time.Time        `json:"start_time"`
	LastEventTime     time.Time        `json:"last_event_time"`
	ActiveWorkers     int32            `json:"active_workers"`
	QueueSize         int32            `json:"queue_size"`
	HandlerStats      map[string]int64 `json:"handler_stats"`
}

// MiddlewareFunc 中间件函数
type MiddlewareFunc func(event interface{}, next func(interface{}) error) error

// NewEventDispatcher 创建事件分发器
func NewEventDispatcher(config DispatcherConfig) *EventDispatcher {
	ctx, cancel := context.WithCancel(context.Background())

	ed := &EventDispatcher{
		handlers:     make(map[reflect.Type][]EventHandler),
		eventChannel: make(chan interface{}, config.BufferSize),
		errorChannel: make(chan error, 100),
		config:       config,
		stats:        &DispatcherStats{
			StartTime:    time.Now(),
			HandlerStats: make(map[string]int64),
		},
		ctx:    ctx,
		cancel: cancel,
		handlerPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{})
			},
		},
	}

	return ed
}

// RegisterHandler 注册事件处理器
func (ed *EventDispatcher) RegisterHandler(eventType reflect.Type, handler EventHandler) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	ed.handlers[eventType] = append(ed.handlers[eventType], handler)
	
	// 初始化处理器统计
	handlerName := ed.getHandlerName(handler)
	if ed.config.EnableMetrics {
		ed.stats.HandlerStats[handlerName] = 0
	}

	log.Printf("Registered handler %s for event type %s", handlerName, eventType.String())
}

// RegisterHandlerFunc 注册事件处理函数
func (ed *EventDispatcher) RegisterHandlerFunc(eventType reflect.Type, handlerFunc func(interface{}) error) {
	ed.RegisterHandler(eventType, EventHandlerFunc(handlerFunc))
}

// UnregisterHandler 注销事件处理器
func (ed *EventDispatcher) UnregisterHandler(eventType reflect.Type, handler EventHandler) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	handlers, exists := ed.handlers[eventType]
	if !exists {
		return
	}

	// 查找并移除处理器
	for i, h := range handlers {
		if reflect.DeepEqual(h, handler) {
			ed.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}

	// 如果没有处理器了，删除整个条目
	if len(ed.handlers[eventType]) == 0 {
		delete(ed.handlers, eventType)
	}
}

// AddMiddleware 添加中间件
func (ed *EventDispatcher) AddMiddleware(middleware MiddlewareFunc) {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	ed.middleware = append(ed.middleware, middleware)
}

// Start 启动事件分发器
func (ed *EventDispatcher) Start() error {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if ed.isRunning {
		return fmt.Errorf("event dispatcher is already running")
	}

	ed.isRunning = true

	// 启动工作协程
	for i := 0; i < ed.config.WorkerCount; i++ {
		ed.wg.Add(1)
		go ed.worker(i)
	}

	// 启动监控协程
	if ed.config.EnableMetrics {
		ed.wg.Add(1)
		go ed.monitor()
	}

	log.Printf("Event dispatcher started with %d workers", ed.config.WorkerCount)
	return nil
}

// Stop 停止事件分发器
func (ed *EventDispatcher) Stop() error {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if !ed.isRunning {
		return fmt.Errorf("event dispatcher is not running")
	}

	ed.isRunning = false
	ed.cancel()

	// 关闭事件通道
	close(ed.eventChannel)

	// 等待所有工作协程完成
	ed.wg.Wait()

	// 关闭错误通道
	close(ed.errorChannel)

	log.Println("Event dispatcher stopped")
	return nil
}

// Dispatch 分发事件
func (ed *EventDispatcher) Dispatch(event interface{}) error {
	if !ed.IsRunning() {
		return fmt.Errorf("event dispatcher is not running")
	}

	atomic.AddInt64(&ed.stats.TotalEvents, 1)
	atomic.AddInt32(&ed.stats.QueueSize, 1)

	select {
	case ed.eventChannel <- event:
		ed.stats.LastEventTime = time.Now()
		
		// 检查队列大小警告
		if ed.config.QueueWarningSize > 0 {
			queueSize := atomic.LoadInt32(&ed.stats.QueueSize)
			if int(queueSize) >= ed.config.QueueWarningSize {
				log.Printf("Warning: Event queue size is high (%d)", queueSize)
			}
		}
		
		return nil
	case <-ed.ctx.Done():
		return ed.ctx.Err()
	default:
		atomic.AddInt64(&ed.stats.FailedEvents, 1)
		return fmt.Errorf("event channel full, dropping event")
	}
}

// DispatchAsync 异步分发事件
func (ed *EventDispatcher) DispatchAsync(event interface{}) {
	go func() {
		if err := ed.Dispatch(event); err != nil {
			ed.sendError(fmt.Errorf("async dispatch failed: %w", err))
		}
	}()
}

// worker 工作协程
func (ed *EventDispatcher) worker(workerID int) {
	defer ed.wg.Done()

	log.Printf("Event dispatcher worker %d started", workerID)
	defer log.Printf("Event dispatcher worker %d stopped", workerID)

	for {
		select {
		case <-ed.ctx.Done():
			return

		case event, ok := <-ed.eventChannel:
			if !ok {
				return
			}

			atomic.AddInt32(&ed.stats.ActiveWorkers, 1)
			atomic.AddInt32(&ed.stats.QueueSize, -1)
			
			err := ed.handleEvent(event)
			
			atomic.AddInt32(&ed.stats.ActiveWorkers, -1)

			if err != nil {
				atomic.AddInt64(&ed.stats.FailedEvents, 1)
				ed.sendError(fmt.Errorf("worker %d: %w", workerID, err))
			} else {
				atomic.AddInt64(&ed.stats.ProcessedEvents, 1)
			}
		}
	}
}

// handleEvent 处理事件
func (ed *EventDispatcher) handleEvent(event interface{}) error {
	startTime := time.Now()
	defer func() {
		processingTime := time.Since(startTime)
		atomic.AddInt64((*int64)(&ed.stats.TotalProcessTime), int64(processingTime))
	}()

	// 设置处理超时
	ctx, cancel := context.WithTimeout(ed.ctx, ed.config.HandlerTimeout)
	defer cancel()

	return ed.handleEventWithRetry(ctx, event)
}

// handleEventWithRetry 带重试的事件处理
func (ed *EventDispatcher) handleEventWithRetry(ctx context.Context, event interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= ed.config.MaxRetries; attempt++ {
		if attempt > 0 {
			atomic.AddInt64(&ed.stats.RetryEvents, 1)
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(ed.config.RetryDelay):
			}
		}

		if err := ed.handleEventOnce(ctx, event); err != nil {
			lastErr = err
			log.Printf("Event handling attempt %d failed: %v", attempt+1, err)
			continue
		}

		return nil
	}

	return fmt.Errorf("event handling failed after %d attempts: %w", ed.config.MaxRetries+1, lastErr)
}

// handleEventOnce 执行一次事件处理
func (ed *EventDispatcher) handleEventOnce(ctx context.Context, event interface{}) error {
	eventType := reflect.TypeOf(event)

	ed.mu.RLock()
	handlers, exists := ed.handlers[eventType]
	middleware := ed.middleware
	ed.mu.RUnlock()

	if !exists || len(handlers) == 0 {
		log.Printf("No handlers registered for event type: %s", eventType.String())
		return nil
	}

	for _, handler := range handlers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := ed.executeHandler(ctx, event, handler, middleware); err != nil {
			atomic.AddInt64(&ed.stats.HandlerErrors, 1)
			return fmt.Errorf("handler failed: %w", err)
		}

		// 更新处理器统计
		if ed.config.EnableMetrics {
			handlerName := ed.getHandlerName(handler)
			atomic.AddInt64(ed.stats.HandlerStats[handlerName], 1)
		}
	}

	return nil
}

// executeHandler 执行处理器
func (ed *EventDispatcher) executeHandler(ctx context.Context, event interface{}, handler EventHandler, middleware []MiddlewareFunc) (err error) {
	// 恢复机制
	if ed.config.EnableRecovery {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&ed.stats.PanicRecoveries, 1)
				err = fmt.Errorf("handler panic recovered: %v", r)
				log.Printf("Recovered from handler panic: %v", r)
			}
		}()
	}

	// 应用中间件
	if ed.config.EnableMiddleware && len(middleware) > 0 {
		return ed.applyMiddleware(event, handler, middleware, 0)
	}

	// 直接执行处理器
	return handler.Handle(event)
}

// applyMiddleware 应用中间件
func (ed *EventDispatcher) applyMiddleware(event interface{}, handler EventHandler, middleware []MiddlewareFunc, index int) error {
	if index >= len(middleware) {
		return handler.Handle(event)
	}

	return middleware[index](event, func(e interface{}) error {
		return ed.applyMiddleware(e, handler, middleware, index+1)
	})
}

// monitor 监控协程
func (ed *EventDispatcher) monitor() {
	defer ed.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ed.ctx.Done():
			return
		case <-ticker.C:
			ed.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (ed *EventDispatcher) updateStats() {
	processedEvents := atomic.LoadInt64(&ed.stats.ProcessedEvents)
	totalProcessTime := time.Duration(atomic.LoadInt64((*int64)(&ed.stats.TotalProcessTime)))

	if processedEvents > 0 {
		ed.stats.AverageProcessTime = totalProcessTime / time.Duration(processedEvents)
	}

	// 记录队列大小
	queueSize := atomic.LoadInt32(&ed.stats.QueueSize)
	if ed.config.QueueWarningSize > 0 && int(queueSize) >= ed.config.QueueWarningSize {
		log.Printf("Queue size warning: %d events pending", queueSize)
	}
}

// sendError 发送错误
func (ed *EventDispatcher) sendError(err error) {
	select {
	case ed.errorChannel <- err:
	default:
		log.Printf("Error channel full, dropping error: %v", err)
	}
}

// getHandlerName 获取处理器名称
func (ed *EventDispatcher) getHandlerName(handler EventHandler) string {
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() == reflect.Ptr {
		handlerType = handlerType.Elem()
	}
	return handlerType.Name()
}

// GetStats 获取统计信息
func (ed *EventDispatcher) GetStats() DispatcherStats {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	stats := *ed.stats
	stats.TotalEvents = atomic.LoadInt64(&ed.stats.TotalEvents)
	stats.ProcessedEvents = atomic.LoadInt64(&ed.stats.ProcessedEvents)
	stats.FailedEvents = atomic.LoadInt64(&ed.stats.FailedEvents)
	stats.RetryEvents = atomic.LoadInt64(&ed.stats.RetryEvents)
	stats.HandlerErrors = atomic.LoadInt64(&ed.stats.HandlerErrors)
	stats.PanicRecoveries = atomic.LoadInt64(&ed.stats.PanicRecoveries)
	stats.ActiveWorkers = atomic.LoadInt32(&ed.stats.ActiveWorkers)
	stats.QueueSize = atomic.LoadInt32(&ed.stats.QueueSize)
	stats.TotalProcessTime = time.Duration(atomic.LoadInt64((*int64)(&ed.stats.TotalProcessTime)))

	if stats.ProcessedEvents > 0 {
		stats.AverageProcessTime = stats.TotalProcessTime / time.Duration(stats.ProcessedEvents)
	}

	// 复制处理器统计
	stats.HandlerStats = make(map[string]int64)
	for name, count := range ed.stats.HandlerStats {
		stats.HandlerStats[name] = atomic.LoadInt64(&count)
	}

	return stats
}

// IsRunning 检查是否正在运行
func (ed *EventDispatcher) IsRunning() bool {
	ed.mu.RLock()
	defer ed.mu.RUnlock()
	return ed.isRunning
}

// GetHandlers 获取已注册的处理器
func (ed *EventDispatcher) GetHandlers() map[reflect.Type][]EventHandler {
	ed.mu.RLock()
	defer ed.mu.RUnlock()

	result := make(map[reflect.Type][]EventHandler)
	for eventType, handlers := range ed.handlers {
		result[eventType] = make([]EventHandler, len(handlers))
		copy(result[eventType], handlers)
	}

	return result
}

// Errors 获取错误通道
func (ed *EventDispatcher) Errors() <-chan error {
	return ed.errorChannel
}

// SetHandlerTimeout 设置处理器超时
func (ed *EventDispatcher) SetHandlerTimeout(timeout time.Duration) {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	ed.config.HandlerTimeout = timeout
}

// GetQueueSize 获取当前队列大小
func (ed *EventDispatcher) GetQueueSize() int {
	return int(atomic.LoadInt32(&ed.stats.QueueSize))
}

// 内置中间件实现

// LoggingMiddleware 日志中间件
func LoggingMiddleware(event interface{}, next func(interface{}) error) error {
	start := time.Now()
	eventType := reflect.TypeOf(event).String()
	
	log.Printf("Processing event: %s", eventType)
	
	err := next(event)
	
	duration := time.Since(start)
	if err != nil {
		log.Printf("Event %s failed after %v: %v", eventType, duration, err)
	} else {
		log.Printf("Event %s completed in %v", eventType, duration)
	}
	
	return err
}

// MetricsMiddleware 指标中间件
func MetricsMiddleware(event interface{}, next func(interface{}) error) error {
	start := time.Now()
	
	err := next(event)
	
	duration := time.Since(start)
	eventType := reflect.TypeOf(event).String()
	
	// 这里可以发送指标到监控系统
	log.Printf("Metrics: %s processed in %v (success: %t)", eventType, duration, err == nil)
	
	return err
}

// CircuitBreakerMiddleware 熔断器中间件
type CircuitBreakerMiddleware struct {
	failures    int64
	lastFailure time.Time
	mu          sync.RWMutex
}

func NewCircuitBreakerMiddleware() *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{}
}

func (cb *CircuitBreakerMiddleware) Middleware(event interface{}, next func(interface{}) error) error {
	cb.mu.RLock()
	failures := cb.failures
	lastFailure := cb.lastFailure
	cb.mu.RUnlock()

	// 简单的熔断逻辑
	if failures >= 5 && time.Since(lastFailure) < 1*time.Minute {
		return fmt.Errorf("circuit breaker open: too many failures")
	}

	err := next(event)
	
	cb.mu.Lock()
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
	} else {
		cb.failures = 0 // 重置失败计数
	}
	cb.mu.Unlock()

	return err
}