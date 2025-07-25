package pipeline

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EnhancedFileWatcher 增强文件监控器
type EnhancedFileWatcher struct {
	watcher       *fsnotify.Watcher
	fileStates    map[string]*FileState
	eventChannel  chan FileEvent
	config        WatcherConfig
	mu            sync.RWMutex
	isRunning     bool
	debounceTimer map[string]*time.Timer
	patterns      []string
	ignorePatterns []string
}

// WatcherConfig 监控器配置
type WatcherConfig struct {
	DebounceInterval time.Duration `json:"debounce_interval"`
	ChecksumEnabled  bool         `json:"checksum_enabled"`
	MaxFileSize      int64        `json:"max_file_size"`
	PollInterval     time.Duration `json:"poll_interval"`
	BufferSize       int          `json:"buffer_size"`
}

// DefaultWatcherConfig 默认监控器配置
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		DebounceInterval: 500 * time.Millisecond,
		ChecksumEnabled:  true,
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		PollInterval:     1 * time.Second,
		BufferSize:       100,
	}
}

// NewEnhancedFileWatcher 创建增强文件监控器
func NewEnhancedFileWatcher(config WatcherConfig) (*EnhancedFileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	efw := &EnhancedFileWatcher{
		watcher:       watcher,
		fileStates:    make(map[string]*FileState),
		eventChannel:  make(chan FileEvent, config.BufferSize),
		config:        config,
		debounceTimer: make(map[string]*time.Timer),
	}

	return efw, nil
}

// AddPath 添加监控路径
func (efw *EnhancedFileWatcher) AddPath(path string) error {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		// 递归添加目录中的文件
		return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && efw.shouldWatch(walkPath) {
				if err := efw.watcher.Add(walkPath); err != nil {
					log.Printf("Failed to watch file %s: %v", walkPath, err)
					return nil // 继续处理其他文件
				}

				// 初始化文件状态
				if err := efw.initFileState(walkPath, info); err != nil {
					log.Printf("Failed to init file state for %s: %v", walkPath, err)
				}
			}

			return nil
		})
	} else {
		// 单个文件
		if efw.shouldWatch(path) {
			if err := efw.watcher.Add(path); err != nil {
				return fmt.Errorf("failed to watch file: %w", err)
			}

			return efw.initFileState(path, info)
		}
	}

	return nil
}

// RemovePath 移除监控路径
func (efw *EnhancedFileWatcher) RemovePath(path string) error {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	if err := efw.watcher.Remove(path); err != nil {
		return fmt.Errorf("failed to remove path: %w", err)
	}

	delete(efw.fileStates, path)
	
	// 清理防抖定时器
	if timer, exists := efw.debounceTimer[path]; exists {
		timer.Stop()
		delete(efw.debounceTimer, path)
	}

	return nil
}

// SetPatterns 设置文件模式
func (efw *EnhancedFileWatcher) SetPatterns(patterns, ignorePatterns []string) {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	efw.patterns = patterns
	efw.ignorePatterns = ignorePatterns
}

// Start 启动文件监控
func (efw *EnhancedFileWatcher) Start() error {
	efw.mu.Lock()
	if efw.isRunning {
		efw.mu.Unlock()
		return fmt.Errorf("watcher is already running")
	}
	efw.isRunning = true
	efw.mu.Unlock()

	go efw.watchLoop()
	return nil
}

// Stop 停止文件监控
func (efw *EnhancedFileWatcher) Stop() error {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	if !efw.isRunning {
		return fmt.Errorf("watcher is not running")
	}

	efw.isRunning = false
	
	// 停止所有防抖定时器
	for _, timer := range efw.debounceTimer {
		timer.Stop()
	}
	efw.debounceTimer = make(map[string]*time.Timer)

	close(efw.eventChannel)
	return efw.watcher.Close()
}

// Events 获取事件通道
func (efw *EnhancedFileWatcher) Events() <-chan FileEvent {
	return efw.eventChannel
}

// watchLoop 监控循环
func (efw *EnhancedFileWatcher) watchLoop() {
	for {
		select {
		case event, ok := <-efw.watcher.Events:
			if !ok {
				return
			}
			efw.handleFSEvent(event)

		case err, ok := <-efw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)

		default:
			efw.mu.RLock()
			if !efw.isRunning {
				efw.mu.RUnlock()
				return
			}
			efw.mu.RUnlock()
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// handleFSEvent 处理文件系统事件
func (efw *EnhancedFileWatcher) handleFSEvent(event fsnotify.Event) {
	if !efw.shouldWatch(event.Name) {
		return
	}

	// 使用防抖处理
	efw.debounceEvent(event.Name, func() {
		efw.processFileEvent(event)
	})
}

// debounceEvent 防抖处理事件
func (efw *EnhancedFileWatcher) debounceEvent(path string, handler func()) {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	// 取消之前的定时器
	if timer, exists := efw.debounceTimer[path]; exists {
		timer.Stop()
	}

	// 创建新的定时器
	efw.debounceTimer[path] = time.AfterFunc(efw.config.DebounceInterval, handler)
}

// processFileEvent 处理文件事件
func (efw *EnhancedFileWatcher) processFileEvent(event fsnotify.Event) {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	path := event.Name
	oldState := efw.fileStates[path]

	// 获取当前文件状态
	newState, err := efw.getFileState(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to get file state for %s: %v", path, err)
		}
		newState = nil
	}

	// 确定事件类型
	var eventType EventType
	var changes []Change

	if oldState == nil && newState != nil {
		eventType = EventCreate
	} else if oldState != nil && newState == nil {
		eventType = EventDelete
	} else if oldState != nil && newState != nil {
		eventType = EventModify
		changes = efw.detectChanges(oldState, newState)
		
		// 如果没有实际变化，忽略事件
		if len(changes) == 0 {
			return
		}
	} else {
		return // 无效状态
	}

	// 创建文件事件
	fileEvent := FileEvent{
		Type:     eventType,
		Path:     path,
		OldState: oldState,
		NewState: newState,
		Changes:  changes,
	}

	// 更新文件状态
	if newState != nil {
		efw.fileStates[path] = newState
	} else {
		delete(efw.fileStates, path)
	}

	// 发送事件
	select {
	case efw.eventChannel <- fileEvent:
	default:
		log.Printf("Event channel full, dropping event for %s", path)
	}
}

// shouldWatch 检查是否应该监控此文件
func (efw *EnhancedFileWatcher) shouldWatch(path string) bool {
	// 检查忽略模式
	for _, pattern := range efw.ignorePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
	}

	// 检查包含模式
	if len(efw.patterns) == 0 {
		return true // 没有模式则监控所有文件
	}

	for _, pattern := range efw.patterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}

	return false
}

// initFileState 初始化文件状态
func (efw *EnhancedFileWatcher) initFileState(path string, info os.FileInfo) error {
	state := &FileState{
		Path:         path,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		LastRead:     time.Now(),
		ReadPosition: 0,
	}

	// 计算校验和
	if efw.config.ChecksumEnabled && info.Size() <= efw.config.MaxFileSize {
		checksum, err := efw.calculateChecksum(path)
		if err != nil {
			log.Printf("Failed to calculate checksum for %s: %v", path, err)
		} else {
			state.Checksum = checksum
		}
	}

	efw.fileStates[path] = state
	return nil
}

// getFileState 获取文件当前状态
func (efw *EnhancedFileWatcher) getFileState(path string) (*FileState, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	state := &FileState{
		Path:         path,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		LastRead:     time.Now(),
		ReadPosition: 0, // 新状态重置读取位置
	}

	// 计算校验和
	if efw.config.ChecksumEnabled && info.Size() <= efw.config.MaxFileSize {
		checksum, err := efw.calculateChecksum(path)
		if err != nil {
			log.Printf("Failed to calculate checksum for %s: %v", path, err)
		} else {
			state.Checksum = checksum
		}
	}

	return state, nil
}

// detectChanges 检测文件变化
func (efw *EnhancedFileWatcher) detectChanges(oldState, newState *FileState) []Change {
	var changes []Change

	// 大小变化
	if oldState.Size != newState.Size {
		changes = append(changes, Change{
			Type:     ChangeSize,
			OldValue: oldState.Size,
			NewValue: newState.Size,
		})
	}

	// 修改时间变化
	if !oldState.LastModified.Equal(newState.LastModified) {
		changes = append(changes, Change{
			Type:     ChangeModTime,
			OldValue: oldState.LastModified,
			NewValue: newState.LastModified,
		})
	}

	// 内容变化（通过校验和）
	if efw.config.ChecksumEnabled && 
		oldState.Checksum != "" && newState.Checksum != "" &&
		oldState.Checksum != newState.Checksum {
		changes = append(changes, Change{
			Type:     ChangeContent,
			OldValue: oldState.Checksum,
			NewValue: newState.Checksum,
		})
	}

	return changes
}

// calculateChecksum 计算文件校验和
func (efw *EnhancedFileWatcher) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GetFileState 获取文件状态（外部接口）
func (efw *EnhancedFileWatcher) GetFileState(path string) (*FileState, bool) {
	efw.mu.RLock()
	defer efw.mu.RUnlock()

	state, exists := efw.fileStates[path]
	if !exists {
		return nil, false
	}

	// 返回副本以避免并发修改
	stateCopy := *state
	return &stateCopy, true
}

// GetAllFileStates 获取所有文件状态
func (efw *EnhancedFileWatcher) GetAllFileStates() map[string]FileState {
	efw.mu.RLock()
	defer efw.mu.RUnlock()

	states := make(map[string]FileState)
	for path, state := range efw.fileStates {
		states[path] = *state // 复制值
	}

	return states
}

// UpdateReadPosition 更新文件读取位置
func (efw *EnhancedFileWatcher) UpdateReadPosition(path string, position int64) error {
	efw.mu.Lock()
	defer efw.mu.Unlock()

	state, exists := efw.fileStates[path]
	if !exists {
		return fmt.Errorf("file state not found for path: %s", path)
	}

	state.ReadPosition = position
	state.LastRead = time.Now()
	return nil
}

// IsRunning 检查是否正在运行
func (efw *EnhancedFileWatcher) IsRunning() bool {
	efw.mu.RLock()
	defer efw.mu.RUnlock()
	return efw.isRunning
}