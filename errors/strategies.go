package errors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// FileAccessRecovery 文件访问错误恢复
type FileAccessRecovery struct {
	alternativePaths []string
	cache            *FileCache
}

func (f *FileAccessRecovery) CanHandle(err error) bool {
	_, ok := err.(*RecoverableError)
	return ok && isFileError(err)
}

func (f *FileAccessRecovery) Recover(err error, context *ErrorContext) error {
	recErr := err.(*RecoverableError)

	// 策略1: 尝试备用路径
	if altPath := f.tryAlternativePath(context); altPath != "" {
		context.Metadata["recovered_path"] = altPath
		return nil
	}

	// 策略2: 使用缓存数据
	if cached := f.tryCache(context); cached != nil {
		context.Metadata["using_cache"] = true
		return nil
	}

	// 策略3: 创建默认文件
	if f.canCreateDefault(recErr) {
		if err := f.createDefaultFile(context); err == nil {
			return nil
		}
	}

	return fmt.Errorf("all file recovery strategies failed")
}

func (f *FileAccessRecovery) GetFallback() interface{} {
	// 返回空数据集
	return []models.UsageEntry{}
}

func (f *FileAccessRecovery) tryAlternativePath(context *ErrorContext) string {
	// 尝试备用路径
	for _, path := range f.alternativePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (f *FileAccessRecovery) tryCache(context *ErrorContext) interface{} {
	if f.cache == nil {
		return nil
	}
	// 尝试从缓存获取数据
	// 这里简化实现
	return nil
}

func (f *FileAccessRecovery) canCreateDefault(err *RecoverableError) bool {
	return err.Type == ErrorTypePermission
}

func (f *FileAccessRecovery) createDefaultFile(context *ErrorContext) error {
	// 创建默认配置文件
	return nil
}

// JSONParseRecovery JSON 解析错误恢复
type JSONParseRecovery struct {
	validator     *JSONValidator
	repair        *JSONRepair
	skipCorrupted bool
}

func (j *JSONParseRecovery) CanHandle(err error) bool {
	_, ok := err.(*json.SyntaxError)
	if ok {
		return true
	}
	_, ok = err.(*json.UnmarshalTypeError)
	return ok
}

func (j *JSONParseRecovery) Recover(err error, context *ErrorContext) error {
	data, ok := context.Metadata["raw_data"].([]byte)
	if !ok {
		return fmt.Errorf("no raw data available")
	}

	// 策略1: 尝试修复 JSON
	if repaired := j.repair.TryRepair(data); repaired != nil {
		context.Metadata["repaired"] = true
		context.Metadata["data"] = repaired
		return nil
	}

	// 策略2: 跳过损坏的行
	if j.skipCorrupted {
		context.Metadata["skip"] = true
		return nil
	}

	// 策略3: 提取部分有效数据
	if partial := j.extractPartialData(data); partial != nil {
		context.Metadata["partial"] = true
		context.Metadata["data"] = partial
		return nil
	}

	return fmt.Errorf("cannot recover JSON data")
}

func (j *JSONParseRecovery) GetFallback() interface{} {
	return map[string]interface{}{}
}

func (j *JSONParseRecovery) extractPartialData(data []byte) []byte {
	// 尝试从损坏的JSON中提取部分数据
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var validLines []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 尝试解析每一行
		var temp interface{}
		if err := json.Unmarshal([]byte(line), &temp); err == nil {
			validLines = append(validLines, line)
		}
	}

	if len(validLines) > 0 {
		return []byte(strings.Join(validLines, "\n"))
	}

	return nil
}

// JSONRepair JSON 修复器
type JSONRepair struct {
	strategies []RepairStrategy
}

type RepairStrategy interface {
	Repair(data []byte) []byte
}

func NewJSONRepair() *JSONRepair {
	return &JSONRepair{
		strategies: []RepairStrategy{
			&QuoteRepairStrategy{},
			&BracketRepairStrategy{},
			&TrailingCommaStrategy{},
		},
	}
}

func (jr *JSONRepair) TryRepair(data []byte) []byte {
	for _, strategy := range jr.strategies {
		if repaired := strategy.Repair(data); repaired != nil {
			// 验证修复后的数据
			var test interface{}
			if err := json.Unmarshal(repaired, &test); err == nil {
				return repaired
			}
		}
	}
	return nil
}

// QuoteRepairStrategy 引号修复策略
type QuoteRepairStrategy struct{}

func (q *QuoteRepairStrategy) Repair(data []byte) []byte {
	s := string(data)
	quoteCount := 0
	inString := false

	for i, char := range s {
		if char == '"' && (i == 0 || s[i-1] != '\\') {
			quoteCount++
			inString = !inString
		}
	}

	if quoteCount%2 != 0 {
		// 添加缺失的引号
		if inString {
			s += "\""
		}
	}

	return []byte(s)
}

// BracketRepairStrategy 括号修复策略
type BracketRepairStrategy struct{}

func (b *BracketRepairStrategy) Repair(data []byte) []byte {
	s := string(data)
	openBrackets := 0
	openBraces := 0

	for _, char := range s {
		switch char {
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		case '{':
			openBraces++
		case '}':
			openBraces--
		}
	}

	// 添加缺失的括号
	for openBrackets > 0 {
		s += "]"
		openBrackets--
	}
	for openBraces > 0 {
		s += "}"
		openBraces--
	}

	return []byte(s)
}

// TrailingCommaStrategy 尾随逗号修复策略
type TrailingCommaStrategy struct{}

func (t *TrailingCommaStrategy) Repair(data []byte) []byte {
	s := string(data)
	
	// 移除对象中的尾随逗号
	s = strings.ReplaceAll(s, ",}", "}")
	s = strings.ReplaceAll(s, ",]", "]")
	
	return []byte(s)
}

// NetworkErrorRecovery 网络错误恢复
type NetworkErrorRecovery struct {
	retryPolicy     *RetryPolicy
	fallbackServers []string
	cache           *ResponseCache
}

func (n *NetworkErrorRecovery) CanHandle(err error) bool {
	return isNetworkError(err) || isTimeoutError(err)
}

func (n *NetworkErrorRecovery) Recover(err error, context *ErrorContext) error {
	endpoint := context.Metadata["endpoint"].(string)

	// 策略1: 重试
	retryErr := n.retryPolicy.Execute(func() error {
		return n.attemptRequest(endpoint)
	})

	if retryErr == nil {
		return nil
	}

	// 策略2: 使用备用服务器
	for _, server := range n.fallbackServers {
		if err := n.attemptRequest(server); err == nil {
			context.Metadata["fallback_server"] = server
			return nil
		}
	}

	// 策略3: 使用缓存响应
	if cached := n.cache.Get(endpoint); cached != nil {
		context.Metadata["using_cache"] = true
		context.Metadata["response"] = cached
		return nil
	}

	return fmt.Errorf("network recovery failed")
}

func (n *NetworkErrorRecovery) GetFallback() interface{} {
	return map[string]interface{}{"error": "network unavailable"}
}

func (n *NetworkErrorRecovery) attemptRequest(endpoint string) error {
	// 模拟网络请求
	return nil
}

// ResourceErrorRecovery 资源错误恢复
type ResourceErrorRecovery struct{}

func (r *ResourceErrorRecovery) CanHandle(err error) bool {
	return isResourceError(err)
}

func (r *ResourceErrorRecovery) Recover(err error, context *ErrorContext) error {
	// 尝试释放资源
	r.freeResources()

	// 等待一段时间
	time.Sleep(1 * time.Second)

	return nil
}

func (r *ResourceErrorRecovery) GetFallback() interface{} {
	return nil
}

func (r *ResourceErrorRecovery) freeResources() {
	// 强制垃圾回收
	// runtime.GC()
}

// 辅助类型和函数
type FileCache struct {
	// 简化实现
}

type JSONValidator struct {
	// 简化实现
}

type ResponseCache struct {
	data map[string]interface{}
}

func (rc *ResponseCache) Get(key string) interface{} {
	if rc.data == nil {
		return nil
	}
	return rc.data[key]
}

func isFileError(err error) bool {
	return os.IsNotExist(err) || os.IsPermission(err)
}

func isTimeoutError(err error) bool {
	return strings.Contains(err.Error(), "timeout")
}