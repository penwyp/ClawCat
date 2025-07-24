# Claude Code Usage Monitor - 核心业务逻辑

本文档描述了 Claude Code Usage Monitor 从数据解析到 Dashboard 展示的完整业务逻辑流程。

## 1. 总体架构

```
JSONL 文件 → 数据解析 → 会话分析 → 统计计算 → Dashboard 展示
     ↑                                           ↓
     └──────────── 文件监控（实时更新）←──────────┘
```

## 2. 数据处理流程

### 2.1 JSONL 文件解析

**输入**: `~/.claude/projects/*/conversation.jsonl`

**处理步骤**:
1. **文件发现**
   ```go
   // 扫描标准路径
   paths := []string{
       "~/.claude/projects",
       "~/.config/claude/projects",
   }
   // 递归查找所有 *.jsonl 文件
   ```

2. **逐行解析**
   ```go
   // 每行是一个独立的 JSON 对象
   for scanner.Scan() {
       var entry RawEntry
       json.Unmarshal(scanner.Bytes(), &entry)
   }
   ```

3. **数据过滤**
   - 只处理 `type` 为 "message" 或 "assistant" 的条目
   - 必须包含有效的 token 使用数据
   - 去重：基于 `message_id` + `request_id` 组合

4. **数据转换**
   ```go
   // 原始 JSON → UsageEntry 模型
   UsageEntry {
       Timestamp: 解析 ISO 8601 时间戳
       Model: 提取模型名称
       InputTokens: usage.input_tokens
       OutputTokens: usage.output_tokens
       CacheCreationTokens: usage.cache_creation_tokens
       CacheReadTokens: usage.cache_read_tokens
       TotalTokens: 计算总和
       CostUSD: 根据模型定价计算
   }
   ```

### 2.2 成本计算逻辑

```go
// 每个模型有不同的定价（每百万 token）
cost = (inputTokens / 1M * inputPrice) +
       (outputTokens / 1M * outputPrice) +
       (cacheCreationTokens / 1M * cacheCreationPrice) +
       (cacheReadTokens / 1M * cacheReadPrice)
```

## 3. 会话管理逻辑

### 3.1 会话定义
- **会话时长**: 严格的 5 小时窗口
- **会话开始**: 第一条消息的时间戳，向下取整到小时
- **会话结束**: 开始时间 + 5 小时
- **多会话支持**: 可以有多个重叠的活跃会话

### 3.2 会话检测算法

```go
func DetectActiveSessions(entries []UsageEntry) []Session {
    sessions := make(map[time.Time][]UsageEntry)
    now := time.Now()
    
    for _, entry := range entries {
        // 计算该条目属于的会话开始时间（向下取整到小时）
        sessionStart := RoundDownToHour(entry.Timestamp)
        sessionEnd := sessionStart.Add(5 * time.Hour)
        
        // 检查会话是否仍然活跃
        if now.After(sessionStart) && now.Before(sessionEnd) {
            sessions[sessionStart] = append(sessions[sessionStart], entry)
        }
    }
    
    return ConvertToSessions(sessions)
}
```

### 3.3 会话聚合

```go
type SessionBlock struct {
    StartTime   time.Time
    EndTime     time.Time
    IsActive    bool        // 当前时间是否在会话窗口内
    IsGap       bool        // 是否为空闲时段
    TotalTokens int
    TotalCost   float64
    ModelStats  map[string]ModelStat
}

// 聚合规则：
// 1. 按 5 小时窗口分组
// 2. 检测空闲时段（>= 5 小时无活动）
// 3. 合并相邻的活跃时段
```

## 4. 统计计算

### 4.1 实时指标计算

```go
type RealtimeMetrics struct {
    // 基础统计
    CurrentTokens      int       // 当前会话已使用 tokens
    CurrentCost        float64   // 当前会话已产生成本
    SessionProgress    float64   // 会话进度 (0-100%)
    TimeRemaining      Duration  // 会话剩余时间
    
    // 速率计算
    TokensPerMinute    float64   // 平均 token 使用速率
    CostPerHour        float64   // 平均成本速率
    BurnRate           float64   // 燃烧率（最近一小时）
    
    // 预测
    ProjectedTokens    int       // 预计总 token 使用
    ProjectedCost      float64   // 预计总成本
    PredictedEndTime   time.Time // 预计资源耗尽时间
}
```

### 4.2 计算公式

**进度计算**:
```go
elapsed := time.Since(sessionStart)
progress := (elapsed / 5*time.Hour) * 100
```

**燃烧率**:
```go
// 最近一小时的 token 使用量
tokensLastHour := SumTokensInTimeWindow(entries, 1*time.Hour)
burnRate := tokensLastHour / 60.0  // tokens per minute
```

**预测算法**:
```go
// 基于当前速率预测
remainingTime := sessionEnd.Sub(now)
projectedAdditionalTokens := tokensPerMinute * remainingTime.Minutes()
projectedTotal := currentTokens + projectedAdditionalTokens

// 资源耗尽时间
if costPerMinute > 0 {
    minutesToDepletion := (costLimit - currentCost) / costPerMinute
    predictedEndTime := now.Add(minutesToDepletion * time.Minute)
}
```

### 4.3 模型分布分析

```go
// 计算每个模型的使用占比
modelDistribution := map[string]float64{}
for model, stats := range sessionBlock.ModelStats {
    percentage := float64(stats.TotalTokens) / float64(totalTokens) * 100
    modelDistribution[model] = percentage
}
```

## 5. Dashboard 生成

### 5.1 组件结构

```
┌─────────────────────────────────────────────────────┐
│                    Header Panel                      │
│  Plan: Pro | Active Sessions: 2 | Theme: Dark       │
├─────────────────────────────────────────────────────┤
│                  Progress Section                    │
│  Token Usage:  [████████░░░░░░░] 42.5% (8,075)     │
│  Cost Usage:   [██████░░░░░░░░░] 35.2% ($6.34)     │
│  Time Elapsed: [███████████░░░░] 68.0% (3h 24m)    │
├─────────────────────────────────────────────────────┤
│                 Statistics Table                     │
│  ┌─────────────────┬─────────────┬────────────┐    │
│  │ Metric          │ Current     │ Projected  │    │
│  ├─────────────────┼─────────────┼────────────┤    │
│  │ Tokens          │ 8,075       │ 11,875     │    │
│  │ Cost            │ $6.34       │ $9.32      │    │
│  │ Burn Rate       │ 39.6 tok/min│ -          │    │
│  └─────────────────┴─────────────┴────────────┘    │
├─────────────────────────────────────────────────────┤
│                  Model Distribution                  │
│  claude-3-opus:    65.3% [████████████░░░]         │
│  claude-3-sonnet:  34.7% [██████░░░░░░░░░]         │
└─────────────────────────────────────────────────────┘
```

### 5.2 进度条组件

```go
type ProgressBar struct {
    Label       string
    Current     float64
    Max         float64
    Percentage  float64
    Color       string    // 基于使用率的动态颜色
    DisplayText string    // 右侧显示文本
}

// 颜色逻辑
func GetProgressColor(percentage float64) string {
    switch {
    case percentage < 50:
        return "green"
    case percentage < 75:
        return "yellow"
    case percentage < 90:
        return "orange"
    default:
        return "red"
    }
}
```

### 5.3 实时更新机制

```go
// 更新流程
type UpdatePipeline struct {
    FileWatcher   *FileWatcher      // 监控文件变化
    DataManager   *DataManager      // 管理数据缓存
    Calculator    *Calculator       // 计算统计指标
    UIController  *UIController     // 更新界面
}

// 更新触发
1. 文件变化事件 → 
2. 读取新数据 → 
3. 重新计算统计 → 
4. 更新 Dashboard 组件 → 
5. 刷新终端显示

// 刷新策略
- 数据刷新: 每 10 秒（可配置）
- UI 刷新: 每 0.75Hz（约 1.33 秒）
- 批量更新: 避免频繁重绘
```

## 6. 日/月聚合视图

### 6.1 聚合逻辑

```go
// 按天聚合
dailyStats := map[string]DayStat{}
for _, entry := range entries {
    day := entry.Timestamp.Format("2006-01-02")
    stats := dailyStats[day]
    stats.AddEntry(entry)
    dailyStats[day] = stats
}

// 生成表格
┌────────────┬──────────┬──────────┬────────┬──────────┐
│ Date       │ Messages │ Tokens   │ Cost   │ Avg/Msg  │
├────────────┼──────────┼──────────┼────────┼──────────┤
│ 2024-01-15 │ 142      │ 285,420  │ $42.81 │ 2,010    │
│ 2024-01-14 │ 98       │ 196,000  │ $29.40 │ 2,000    │
└────────────┴──────────┴──────────┴────────┴──────────┘
```

## 7. 关键业务规则

1. **会话窗口**: 严格 5 小时，不可延长
2. **多会话并发**: 支持同时追踪多个活跃会话
3. **成本限制**: 根据订阅计划设置上限
   - Pro: $18.00
   - Max5: $35.00
   - Max20: $140.00
   - Custom: P90 自动检测
4. **P90 计算**: 基于历史数据的 90 百分位数
5. **实时性**: 10 秒内响应文件变化
6. **精度要求**:
   - Token 计数: 精确匹配
   - 成本计算: 保留 4 位小数
   - 时间显示: 秒级精度

## 8. 错误处理

1. **文件访问错误**: 跳过无法读取的文件，继续处理
2. **JSON 解析错误**: 跳过损坏的行，记录日志
3. **数据不完整**: 使用默认值或跳过该条目
4. **计算溢出**: 使用安全的数学运算，避免除零
5. **UI 渲染错误**: 降级到简单文本输出

## 9. 性能优化

1. **流式处理**: 逐行读取 JSONL，避免全文件加载
2. **增量更新**: 只处理新增数据
3. **缓存策略**: 
   - 内存缓存解析结果
   - LRU 淘汰旧数据
4. **并发处理**: 
   - 文件读取与 UI 更新分离
   - 使用 goroutine 处理多文件
5. **批量渲染**: 累积多个更新后统一刷新

这个完整的业务逻辑流程确保了从原始 JSONL 数据到最终 Dashboard 展示的每个环节都有明确的处理规则和实现方式。