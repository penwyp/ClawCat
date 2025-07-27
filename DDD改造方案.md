# claudecat DDD 改造方案

## 项目概述

claudecat 是一个用于监控 Claude AI token 使用情况和成本的 Go 应用程序。当前项目采用传统的分包方式组织代码，本方案将基于领域驱动设计（DDD）原则对项目进行重构。

## 一、领域分析

### 1.1 核心领域

通过分析现有代码，识别出以下核心领域：

1. **使用监控领域（Usage Monitoring）** - 核心领域
   - Token 使用追踪
   - 会话管理
   - 实时监控

2. **成本计算领域（Cost Calculation）** - 核心领域
   - 价格计算
   - 预算管理
   - 成本分析

3. **数据采集领域（Data Collection）** - 支撑领域
   - 文件读取
   - 数据解析
   - 缓存管理

4. **报告展示领域（Reporting）** - 支撑领域
   - 数据格式化
   - 导出功能
   - UI 展示

### 1.2 领域模型设计

#### 核心实体（Entities）

1. **UsageSession（使用会话）**
   - ID: SessionID
   - StartTime: time.Time
   - EndTime: time.Time
   - Entries: []UsageEntry
   - Status: SessionStatus

2. **UsageEntry（使用条目）**
   - ID: EntryID
   - Timestamp: time.Time
   - Model: Model
   - TokenUsage: TokenUsage
   - Cost: Money

#### 值对象（Value Objects）

1. **TokenUsage（Token 使用量）**
   ```go
   type TokenUsage struct {
       InputTokens         int
       OutputTokens        int
       CacheCreationTokens int
       CacheReadTokens     int
   }
   ```

2. **Money（金额）**
   ```go
   type Money struct {
       Amount   decimal.Decimal
       Currency string
   }
   ```

3. **Model（模型）**
   ```go
   type Model struct {
       Name    string
       Version string
       Tier    ModelTier
   }
   ```

4. **TimeWindow（时间窗口）**
   ```go
   type TimeWindow struct {
       Start time.Time
       End   time.Time
   }
   ```

#### 聚合根（Aggregate Roots）

1. **MonitoringSession（监控会话聚合）**
   - 聚合根：UsageSession
   - 包含：多个 UsageEntry
   - 不变量：会话内的条目时间必须在会话时间范围内

2. **CostAnalysis（成本分析聚合）**
   - 聚合根：CostReport
   - 包含：PricingStrategy, BudgetLimit
   - 不变量：成本计算必须基于有效的定价策略

#### 领域服务（Domain Services）

1. **TokenLimitCalculator** - 计算 token 限制
2. **CostCalculator** - 计算使用成本
3. **SessionDetector** - 检测会话边界
4. **BurnRateCalculator** - 计算消耗率

## 二、分层架构设计

### 2.1 架构层次

```
├── cmd/                      # 应用程序入口
├── internal/                 # 内部包（不对外暴露）
│   ├── domain/              # 领域层
│   ├── application/         # 应用层
│   ├── infrastructure/      # 基础设施层
│   └── interfaces/          # 接口层
└── pkg/                     # 公共包（可对外暴露）
```

### 2.2 各层职责

#### 领域层（Domain Layer）
- 包含所有业务逻辑和规则
- 定义实体、值对象、聚合和领域服务
- 不依赖任何外部框架或技术

#### 应用层（Application Layer）
- 协调领域对象完成用例
- 事务管理
- 安全性检查
- 向其他层发送事件通知

#### 基础设施层（Infrastructure Layer）
- 技术相关的实现
- 数据持久化
- 外部服务集成
- 框架和工具

#### 接口层（Interface Layer）
- 用户界面（CLI/TUI）
- REST API
- 事件处理器

## 三、目标项目结构

```
claudecat/
├── cmd/
│   └── claudecat/
│       └── main.go              # 应用入口
├── internal/
│   ├── domain/                  # 领域层
│   │   ├── monitoring/          # 监控领域
│   │   │   ├── session.go       # 会话实体
│   │   │   ├── entry.go         # 使用条目实体
│   │   │   ├── repository.go    # 仓储接口
│   │   │   └── service.go       # 领域服务
│   │   ├── costing/             # 成本计算领域
│   │   │   ├── calculator.go    # 成本计算器
│   │   │   ├── pricing.go       # 定价策略
│   │   │   ├── budget.go        # 预算管理
│   │   │   └── repository.go    # 仓储接口
│   │   └── shared/              # 共享内核
│   │       ├── types.go         # 共享类型
│   │       └── events.go        # 领域事件
│   ├── application/             # 应用层
│   │   ├── monitoring/          # 监控用例
│   │   │   ├── service.go       # 应用服务
│   │   │   ├── commands.go      # 命令处理
│   │   │   └── queries.go       # 查询处理
│   │   ├── analysis/            # 分析用例
│   │   │   ├── service.go
│   │   │   ├── commands.go
│   │   │   └── queries.go
│   │   └── dto/                 # 数据传输对象
│   │       └── types.go
│   ├── infrastructure/          # 基础设施层
│   │   ├── persistence/         # 持久化
│   │   │   ├── file/            # 文件存储
│   │   │   │   ├── reader.go
│   │   │   │   └── repository.go
│   │   │   ├── cache/           # 缓存实现
│   │   │   │   ├── memory.go
│   │   │   │   └── disk.go
│   │   │   └── memory/          # 内存存储
│   │   │       └── repository.go
│   │   ├── monitoring/          # 监控实现
│   │   │   ├── orchestrator.go
│   │   │   └── watcher.go
│   │   ├── pricing/             # 定价服务
│   │   │   ├── provider.go
│   │   │   └── litellm.go
│   │   └── config/              # 配置管理
│   │       ├── loader.go
│   │       └── validator.go
│   └── interfaces/              # 接口层
│       ├── cli/                 # CLI 接口
│       │   ├── commands/
│       │   │   ├── root.go
│       │   │   ├── analyze.go
│       │   │   └── monitor.go
│       │   └── handlers/
│       │       └── handler.go
│       └── tui/                 # TUI 接口
│           ├── app.go
│           ├── views/
│           └── formatter.go
├── pkg/                         # 公共包
│   ├── errors/                  # 错误处理
│   │   └── types.go
│   └── logging/                 # 日志
│       └── logger.go
├── config/                      # 配置文件
│   └── default.yaml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 四、模块职责定义

### 4.1 领域层模块

#### monitoring（监控领域）
- **职责**：管理使用会话和条目的生命周期
- **核心功能**：
  - 会话创建和管理
  - 使用条目聚合
  - 会话状态转换
  - 数据验证

#### costing（成本计算领域）
- **职责**：处理所有与成本相关的计算和分析
- **核心功能**：
  - 成本计算
  - 定价策略管理
  - 预算控制
  - 成本预测

### 4.2 应用层模块

#### monitoring service（监控服务）
- **职责**：协调监控相关的用例
- **核心功能**：
  - 启动/停止监控
  - 处理数据更新
  - 会话管理
  - 事件发布

#### analysis service（分析服务）
- **职责**：提供数据分析功能
- **核心功能**：
  - 统计分析
  - 报告生成
  - 数据导出
  - 趋势分析

### 4.3 基础设施层模块

#### persistence（持久化）
- **职责**：实现数据存储和检索
- **子模块**：
  - file: JSONL 文件读写
  - cache: 缓存管理
  - memory: 内存存储

#### monitoring infrastructure（监控基础设施）
- **职责**：提供监控相关的技术实现
- **核心功能**：
  - 文件监视
  - 数据编排
  - 定时任务

### 4.4 接口层模块

#### CLI（命令行接口）
- **职责**：提供命令行交互
- **核心功能**：
  - 命令解析
  - 参数验证
  - 结果输出

#### TUI（终端用户界面）
- **职责**：提供实时监控界面
- **核心功能**：
  - 实时数据展示
  - 交互式操作
  - 格式化输出

## 五、渐进式迁移计划

### 第一阶段：建立基础架构（可编译）

1. **创建新的目录结构**
   ```bash
   mkdir -p internal/{domain,application,infrastructure,interfaces}
   mkdir -p internal/domain/{monitoring,costing,shared}
   mkdir -p internal/application/{monitoring,analysis,dto}
   mkdir -p internal/infrastructure/{persistence,monitoring,pricing,config}
   mkdir -p internal/interfaces/{cli,tui}
   ```

2. **定义核心接口和类型**
   - 创建领域模型接口
   - 定义仓储接口
   - 建立基础值对象

3. **保持现有功能**
   - 在新结构中创建适配器
   - 保持原有 API 不变
   - 确保所有测试通过

### 第二阶段：迁移领域模型（可测试）

1. **迁移核心实体**
   - 将 models.UsageEntry 重构为领域实体
   - 将 models.SessionBlock 重构为聚合
   - 创建值对象替代原有结构

2. **实现领域服务**
   - 迁移 calculations 包到领域服务
   - 将业务逻辑从基础设施层提取

3. **单元测试**
   - 为每个领域对象编写测试
   - 确保业务规则正确实现

### 第三阶段：重构应用层（可验证）

1. **实现应用服务**
   - 将 orchestrator 逻辑迁移到应用服务
   - 实现命令和查询分离（CQRS）

2. **事务管理**
   - 实现工作单元模式
   - 处理跨聚合事务

3. **集成测试**
   - 测试完整用例流程
   - 验证服务协作

### 第四阶段：基础设施层重构（可运行）

1. **实现仓储**
   - 将 fileio 包重构为仓储实现
   - 迁移缓存逻辑

2. **外部服务集成**
   - 重构定价服务集成
   - 实现配置管理

3. **性能测试**
   - 确保性能不降低
   - 优化关键路径

### 第五阶段：接口层优化（可交付）

1. **CLI 重构**
   - 使用应用服务替代直接调用
   - 实现命令模式

2. **TUI 优化**
   - 分离展示逻辑
   - 实现观察者模式

3. **端到端测试**
   - 完整功能测试
   - 用户验收测试

## 六、迁移步骤详细说明

### 步骤 1：创建领域模型基础（第 1 周）

```go
// internal/domain/monitoring/types.go
package monitoring

import "time"

// SessionID 会话标识符
type SessionID string

// Session 会话聚合根
type Session struct {
    id        SessionID
    startTime time.Time
    endTime   time.Time
    entries   []Entry
    status    SessionStatus
}

// Entry 使用条目
type Entry struct {
    timestamp time.Time
    model     Model
    usage     TokenUsage
    cost      Money
}
```

**验证方法**：
```bash
go build ./internal/domain/...
go test ./internal/domain/...
```

### 步骤 2：实现仓储接口（第 1-2 周）

```go
// internal/domain/monitoring/repository.go
package monitoring

import "context"

type SessionRepository interface {
    Save(ctx context.Context, session *Session) error
    FindByID(ctx context.Context, id SessionID) (*Session, error)
    FindActive(ctx context.Context) ([]*Session, error)
}

type EntryRepository interface {
    Save(ctx context.Context, entry *Entry) error
    FindByTimeRange(ctx context.Context, start, end time.Time) ([]*Entry, error)
}
```

**验证方法**：
- 实现内存版本的仓储
- 编写仓储接口测试

### 步骤 3：迁移计算逻辑（第 2-3 周）

将现有的计算逻辑迁移到领域服务：

```go
// internal/domain/costing/calculator.go
package costing

type CostCalculator struct {
    pricingStrategy PricingStrategy
}

func (c *CostCalculator) Calculate(usage TokenUsage, model Model) Money {
    // 迁移自 calculations 包
}
```

**验证方法**：
- 对比新旧实现的计算结果
- 运行基准测试确保性能

### 步骤 4：实现应用服务（第 3-4 周）

```go
// internal/application/monitoring/service.go
package monitoring

type MonitoringService struct {
    sessionRepo domain.SessionRepository
    entryRepo   domain.EntryRepository
    calculator  *domain.CostCalculator
}

func (s *MonitoringService) StartMonitoring(ctx context.Context, path string) error {
    // 实现监控逻辑
}
```

**验证方法**：
- 集成测试覆盖主要用例
- 确保与现有功能一致

### 步骤 5：适配现有接口（第 4-5 周）

创建适配器保持向后兼容：

```go
// internal/interfaces/cli/adapter.go
package cli

type LegacyAdapter struct {
    monitoringService *application.MonitoringService
    analysisService   *application.AnalysisService
}

func (a *LegacyAdapter) Run() error {
    // 适配现有 CLI 接口
}
```

**验证方法**：
- 运行现有的端到端测试
- 手动测试所有命令

## 七、风险管理

### 技术风险

1. **性能降低**
   - 缓解：持续进行性能测试
   - 监控：建立性能基准线

2. **功能回归**
   - 缓解：保持高测试覆盖率
   - 监控：自动化回归测试

3. **迁移中断**
   - 缓解：小步迭代，频繁集成
   - 监控：每日构建验证

### 业务风险

1. **开发周期延长**
   - 缓解：明确迁移优先级
   - 监控：周进度评审

2. **团队学习成本**
   - 缓解：DDD 培训和文档
   - 监控：代码评审质量

## 八、成功标准

1. **代码质量**
   - 测试覆盖率 > 80%
   - 圈复杂度 < 10
   - 明确的模块边界

2. **性能指标**
   - 启动时间 < 2s
   - 内存使用不增加 > 20%
   - 响应时间保持现有水平

3. **可维护性**
   - 新功能开发时间减少 30%
   - Bug 修复时间减少 50%
   - 代码理解成本降低

## 九、总结

本 DDD 改造方案通过清晰的领域边界、分层架构和渐进式迁移策略，将帮助 claudecat 项目：

1. 提高代码的可维护性和可扩展性
2. 建立清晰的业务模型
3. 降低系统复杂度
4. 提升团队开发效率

整个迁移过程预计需要 5-6 周，每个阶段都确保系统可编译、可测试、可运行，最大限度降低迁移风险。