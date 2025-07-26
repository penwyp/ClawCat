package pipeline

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/ClawCat/models"
)

// StreamReader 流式文件读取器
type StreamReader struct {
	filePath     string
	file         *os.File
	scanner      *bufio.Scanner
	position     int64
	lastModTime  time.Time
	config       StreamConfig
	mu           sync.RWMutex
	isRunning    bool
	dataChannel  chan ProcessedData
	errorChannel chan error
	ctx          context.Context
	cancel       context.CancelFunc
}

// StreamConfig 流配置
type StreamConfig struct {
	BufferSize      int           `json:"buffer_size"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	MaxLineSize     int           `json:"max_line_size"`
	SkipEmptyLines  bool          `json:"skip_empty_lines"`
	RetryInterval   time.Duration `json:"retry_interval"`
	MaxRetries      int           `json:"max_retries"`
	FollowMode      bool          `json:"follow_mode"`      // tail -f 模式
	SeekToEnd       bool          `json:"seek_to_end"`      // 从文件末尾开始读取
	ParseJSON       bool          `json:"parse_json"`       // 是否解析JSON
	ValidateEntries bool          `json:"validate_entries"` // 是否验证条目
}

// DefaultStreamConfig 默认流配置
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		BufferSize:      1000,
		ReadTimeout:     5 * time.Second,
		MaxLineSize:     64 * 1024, // 64KB
		SkipEmptyLines:  true,
		RetryInterval:   1 * time.Second,
		MaxRetries:      3,
		FollowMode:      true,
		SeekToEnd:       false,
		ParseJSON:       true,
		ValidateEntries: true,
	}
}

// NewStreamReader 创建流读取器
func NewStreamReader(filePath string, config StreamConfig) (*StreamReader, error) {
	ctx, cancel := context.WithCancel(context.Background())

	sr := &StreamReader{
		filePath:     filePath,
		config:       config,
		dataChannel:  make(chan ProcessedData, config.BufferSize),
		errorChannel: make(chan error, 10),
		ctx:          ctx,
		cancel:       cancel,
	}

	return sr, nil
}

// Start 启动流读取器
func (sr *StreamReader) Start() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.isRunning {
		return fmt.Errorf("stream reader is already running")
	}

	if err := sr.openFile(); err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	sr.isRunning = true
	go sr.readLoop()

	return nil
}

// Stop 停止流读取器
func (sr *StreamReader) Stop() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if !sr.isRunning {
		return fmt.Errorf("stream reader is not running")
	}

	sr.isRunning = false
	sr.cancel()

	if sr.file != nil {
		sr.file.Close()
		sr.file = nil
	}

	close(sr.dataChannel)
	close(sr.errorChannel)

	return nil
}

// Data 获取数据通道
func (sr *StreamReader) Data() <-chan ProcessedData {
	return sr.dataChannel
}

// Errors 获取错误通道
func (sr *StreamReader) Errors() <-chan error {
	return sr.errorChannel
}

// SetPosition 设置读取位置
func (sr *StreamReader) SetPosition(position int64) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.file != nil {
		if _, err := sr.file.Seek(position, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to position %d: %w", position, err)
		}
		sr.position = position
		sr.scanner = bufio.NewScanner(sr.file)
		sr.configureScanner()
	}

	return nil
}

// GetPosition 获取当前读取位置
func (sr *StreamReader) GetPosition() int64 {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.position
}

// openFile 打开文件
func (sr *StreamReader) openFile() error {
	file, err := os.Open(sr.filePath)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	sr.file = file
	sr.lastModTime = info.ModTime()

	// 根据配置决定开始位置
	if sr.config.SeekToEnd {
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			log.Printf("Failed to seek to end: %v", err)
		} else {
			sr.position, _ = file.Seek(0, io.SeekCurrent)
		}
	} else if sr.position > 0 {
		if _, err := file.Seek(sr.position, io.SeekStart); err != nil {
			log.Printf("Failed to seek to position %d: %v", sr.position, err)
			sr.position = 0
		}
	}

	sr.scanner = bufio.NewScanner(file)
	sr.configureScanner()

	return nil
}

// configureScanner 配置扫描器
func (sr *StreamReader) configureScanner() {
	if sr.config.MaxLineSize > 0 {
		buffer := make([]byte, sr.config.MaxLineSize)
		sr.scanner.Buffer(buffer, sr.config.MaxLineSize)
	}
}

// readLoop 读取循环
func (sr *StreamReader) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Stream reader panic: %v", r)
			sr.sendError(fmt.Errorf("stream reader panic: %v", r))
		}
	}()

	retryCount := 0

	for {
		select {
		case <-sr.ctx.Done():
			return
		default:
			if err := sr.readOnce(); err != nil {
				retryCount++
				if retryCount >= sr.config.MaxRetries {
					sr.sendError(fmt.Errorf("max retries exceeded: %w", err))
					return
				}

				log.Printf("Read error (retry %d/%d): %v", retryCount, sr.config.MaxRetries, err)

				// 等待重试
				select {
				case <-sr.ctx.Done():
					return
				case <-time.After(sr.config.RetryInterval):
					// 尝试重新打开文件
					if sr.file != nil {
						sr.file.Close()
					}
					if err := sr.openFile(); err != nil {
						log.Printf("Failed to reopen file: %v", err)
					}
				}
			} else {
				retryCount = 0 // 重置重试计数
			}
		}
	}
}

// readOnce 执行一次读取
func (sr *StreamReader) readOnce() error {
	sr.mu.Lock()
	scanner := sr.scanner
	file := sr.file
	sr.mu.Unlock()

	if scanner == nil || file == nil {
		return fmt.Errorf("scanner or file is nil")
	}

	// 检查文件是否被修改或截断
	if err := sr.checkFileChanges(); err != nil {
		return err
	}

	// 设置读取超时
	ctx, cancel := context.WithTimeout(sr.ctx, sr.config.ReadTimeout)
	defer cancel()

	readComplete := make(chan bool, 1)
	var scanErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				scanErr = fmt.Errorf("scan panic: %v", r)
			}
			readComplete <- true
		}()

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())

			// 跳过空行
			if sr.config.SkipEmptyLines && line == "" {
				continue
			}

			// 更新位置
			sr.mu.Lock()
			sr.position += int64(len(scanner.Bytes()) + 1) // +1 for newline
			sr.mu.Unlock()

			// 处理行数据
			if err := sr.processLine(line); err != nil {
				log.Printf("Failed to process line: %v", err)
				sr.sendError(err)
			}
		}

		if err := scanner.Err(); err != nil {
			scanErr = err
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-readComplete:
		if scanErr != nil {
			return scanErr
		}

		// 如果是follow模式，等待文件更新
		if sr.config.FollowMode {
			time.Sleep(100 * time.Millisecond)
		}

		return nil
	}
}

// processLine 处理行数据
func (sr *StreamReader) processLine(line string) error {
	startTime := time.Now()

	// 解析JSON
	var entry models.UsageEntry
	if sr.config.ParseJSON {
		if err := sonic.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	// 验证条目
	if sr.config.ValidateEntries {
		if err := sr.validateEntry(&entry); err != nil {
			return fmt.Errorf("entry validation failed: %w", err)
		}
	}

	// 创建处理后的数据
	data := ProcessedData{
		Original:       []byte(line),
		Entry:          entry,
		Metadata:       sr.createMetadata(line),
		ProcessingTime: time.Since(startTime),
	}

	// 发送数据
	select {
	case sr.dataChannel <- data:
		return nil
	case <-sr.ctx.Done():
		return sr.ctx.Err()
	default:
		// 通道满了，记录警告但继续处理
		log.Printf("Data channel full, dropping entry")
		return nil
	}
}

// validateEntry 验证条目
func (sr *StreamReader) validateEntry(entry *models.UsageEntry) error {
	if entry.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}

	if entry.Model == "" {
		return fmt.Errorf("model is required")
	}

	if entry.TotalTokens <= 0 {
		return fmt.Errorf("total tokens must be positive")
	}

	if entry.CostUSD < 0 {
		return fmt.Errorf("cost cannot be negative")
	}

	return nil
}

// createMetadata 创建元数据
func (sr *StreamReader) createMetadata(line string) map[string]interface{} {
	return map[string]interface{}{
		"file_path":     sr.filePath,
		"read_position": sr.position,
		"line_length":   len(line),
		"read_time":     time.Now(),
	}
}

// checkFileChanges 检查文件变化
func (sr *StreamReader) checkFileChanges() error {
	sr.mu.Lock()
	file := sr.file
	sr.mu.Unlock()

	if file == nil {
		return fmt.Errorf("file is nil")
	}

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// 检查文件是否被截断
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current position: %w", err)
	}

	if info.Size() < currentPos {
		// 文件被截断，重新开始读取
		log.Printf("File truncated, restarting from beginning")
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to start: %w", err)
		}

		sr.mu.Lock()
		sr.position = 0
		sr.scanner = bufio.NewScanner(file)
		sr.configureScanner()
		sr.mu.Unlock()
	}

	// 更新修改时间
	sr.mu.Lock()
	sr.lastModTime = info.ModTime()
	sr.mu.Unlock()

	return nil
}

// sendError 发送错误
func (sr *StreamReader) sendError(err error) {
	select {
	case sr.errorChannel <- err:
	default:
		log.Printf("Error channel full, dropping error: %v", err)
	}
}

// IsRunning 检查是否正在运行
func (sr *StreamReader) IsRunning() bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.isRunning
}

// GetStats 获取读取统计
func (sr *StreamReader) GetStats() StreamStats {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	return StreamStats{
		FilePath:        sr.filePath,
		Position:        sr.position,
		LastModTime:     sr.lastModTime,
		IsRunning:       sr.isRunning,
		DataChannelLen:  len(sr.dataChannel),
		ErrorChannelLen: len(sr.errorChannel),
	}
}

// StreamStats 流统计信息
type StreamStats struct {
	FilePath        string    `json:"file_path"`
	Position        int64     `json:"position"`
	LastModTime     time.Time `json:"last_mod_time"`
	IsRunning       bool      `json:"is_running"`
	DataChannelLen  int       `json:"data_channel_len"`
	ErrorChannelLen int       `json:"error_channel_len"`
}

// Restart 重启流读取器
func (sr *StreamReader) Restart() error {
	if err := sr.Stop(); err != nil {
		log.Printf("Failed to stop stream reader: %v", err)
	}

	// 等待清理完成
	time.Sleep(100 * time.Millisecond)

	return sr.Start()
}

// ReadFrom 从指定位置读取指定数量的行
func (sr *StreamReader) ReadFrom(position int64, maxLines int) ([]ProcessedData, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// 临时打开文件
	file, err := os.Open(sr.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 定位到指定位置
	if _, err := file.Seek(position, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to position: %w", err)
	}

	scanner := bufio.NewScanner(file)
	if sr.config.MaxLineSize > 0 {
		buffer := make([]byte, sr.config.MaxLineSize)
		scanner.Buffer(buffer, sr.config.MaxLineSize)
	}

	var results []ProcessedData
	lineCount := 0

	for scanner.Scan() && lineCount < maxLines {
		line := strings.TrimSpace(scanner.Text())

		if sr.config.SkipEmptyLines && line == "" {
			continue
		}

		var entry models.UsageEntry
		if sr.config.ParseJSON {
			if err := sonic.Unmarshal([]byte(line), &entry); err != nil {
				log.Printf("Failed to parse JSON line: %v", err)
				continue
			}
		}

		data := ProcessedData{
			Original: []byte(line),
			Entry:    entry,
			Metadata: map[string]interface{}{
				"file_path":   sr.filePath,
				"line_number": lineCount + 1,
			},
		}

		results = append(results, data)
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return results, fmt.Errorf("scanner error: %w", err)
	}

	return results, nil
}
