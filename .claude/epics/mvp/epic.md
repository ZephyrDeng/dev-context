---
name: mvp
status: completed
created: 2025-09-06T14:06:00Z
progress: 100%
prd: .claude/prds/mvp.md
github: https://github.com/ZephyrDeng/dev-context/issues/1
completed: 2025-09-07T07:25:30Z
---

# Epic: MVP - 实时查询 MCP 系统

## Overview

基于 Golang 的 MCP 服务器，为 AI 助手提供实时前端资讯查询能力。采用无数据库架构，通过实时爬取和智能缓存提供高性能的信息检索服务。

## Architecture Decisions

### 核心技术选择
- **Golang**: 高并发性能，单文件部署，资源占用低
- **官方 MCP Go SDK**: 使用 `github.com/modelcontextprotocol/go-sdk` 确保协议标准兼容
- **无数据库架构**: 零维护成本，实时数据保证，简化部署
- **Colly 爬虫框架**: 成熟的 Go 爬虫库，支持并发和错误处理

### 设计模式
- **工厂模式**: 数据源采集器的统一接口
- **策略模式**: 不同类型数据源的采集策略
- **装饰器模式**: 缓存层包装数据采集逻辑
- **观察者模式**: 缓存过期和清理通知

## Technical Approach

### MCP 服务层
**官方 MCP Go SDK 集成**:
```go
import "github.com/modelcontextprotocol/go-sdk/mcp"

server := mcp.NewServer(&mcp.Implementation{
    Name: "github.com/ZephyrDeng/dev-context",
    Version: "v0.1.0",
}, nil)
```

- SDK 原生工具注册和发现机制
- 标准化请求验证和错误处理  
- 内置会话管理和中间件支持
- 多传输方式支持 (HTTP/WebSocket/SSE)

**核心 MCP 工具实现**:
```go
// 工具注册更加标准化
server.AddTool("get_weekly_frontend_news", &mcp.Tool{
    Description: "获取指定时间范围内的前端开发资讯",
    InputSchema: newsToolSchema,
    Handler:     handleWeeklyNews,
})
```

- `get_weekly_frontend_news`: 时间范围新闻查询
- `search_frontend_topic`: 主题搜索功能
- `get_trending_repositories`: GitHub 热门仓库

### 数据采集系统
**多源采集器架构**:
```go
type DataCollector interface {
    FetchData(source DataSource) ([]Article, error)
    GetSourceType() string
}

type RSSCollector struct{}      // RSS feeds
type APICollector struct{}      // REST APIs
type HTMLCollector struct{}     // Web scraping
```

**并发采集机制**:
- Goroutines 并行处理多个数据源
- Channel 协调结果聚合
- 超时和错误恢复机制

### 缓存管理系统
**内存缓存实现**:
```go
type CacheManager struct {
    cache map[string]CachedResult
    mutex sync.RWMutex
    ttl   time.Duration
}
```

**缓存策略**:
- 基于查询参数的精确匹配
- 15分钟 TTL 平衡实时性和性能
- 自动过期清理机制
- 并发查询合并防重复请求

### 数据处理管道
**统一数据模型**:
```go
type Article struct {
    Title       string    `json:"title"`
    URL         string    `json:"url"`
    Source      string    `json:"source"`
    PublishedAt time.Time `json:"publishedAt"`
    Summary     string    `json:"summary"`
    Tags        []string  `json:"tags"`
    Relevance   float64   `json:"relevance"`
}
```

**处理流程**:
1. 原始数据采集
2. 统一格式转换
3. 内容摘要提取
4. 相关度评分
5. 结果排序和限制

## Implementation Strategy

### 开发阶段
**Phase 1: 核心基础设施** (2-3 天)
- MCP JSON-RPC 服务器搭建
- 基础数据结构定义
- 简单内存缓存实现

**Phase 2: 数据采集能力** (3-4 天)
- RSS 采集器实现
- GitHub API 集成
- Dev.to API 集成
- 并发采集机制

**Phase 3: 工具集成和优化** (2-3 天)
- 三个核心 MCP 工具实现
- 缓存优化和错误处理
- 性能测试和调优

**Phase 4: 测试和文档** (1-2 天)
- 单元测试和集成测试
- 部署文档和使用指南
- 错误场景覆盖测试

### 风险缓解
- **数据源不稳定**: 实现多源冗余和优雅降级
- **性能瓶颈**: 并发采集和智能缓存
- **内存泄漏**: 定期缓存清理和内存监控
- **SDK 版本变更**: 官方 SDK 还在 pre-1.0，需要跟进 API 变化

## Task Breakdown Preview

高级任务分类，将创建为具体实现任务:

- [ ] **MCP SDK 集成设置**: 官方 SDK 集成、服务器初始化、工具注册框架
- [ ] **数据采集系统**: RSS/API/HTML 采集器、并发调度、错误处理  
- [ ] **缓存管理系统**: 内存缓存、TTL 管理、并发安全
- [ ] **核心 MCP 工具业务逻辑**: 三个主要工具的数据处理实现
- [ ] **数据处理和格式化**: 统一数据模型、摘要生成、相关度评分
- [ ] **测试套件**: 单元测试、SDK 集成测试、协议兼容性验证
- [ ] **部署和文档**: 构建配置、部署指南、使用文档

## Dependencies

### 外部服务依赖
**关键数据源**:
- GitHub API (api.github.com): 热门仓库和趋势数据
- Dev.to API (dev.to/api): 开发者社区文章
- CSS-Tricks RSS (css-tricks.com/feed): 前端技术文章
- Hacker News API: 技术新闻和讨论

**备用方案**:
- HTML 解析器作为 API 失效时的备选
- 多数据源冗余减少单点故障
- 缓存机制提供离线降级能力

### 开发工具依赖
- **Go 1.21+ 开发环境**
- **github.com/modelcontextprotocol/go-sdk** (官方 MCP SDK)
- **gocolly/colly v2** (网页爬取)
- **testify** (测试框架)

**注意**: 官方 MCP Go SDK 目前为 pre-1.0 版本，预计2025年7月发布稳定版

## Success Criteria (Technical)

### 性能基准
- **冷启动响应**: < 5 秒完成首次查询
- **缓存命中响应**: < 500ms 返回结果
- **并发处理**: 支持 50+ 并发连接
- **内存使用**: 峰值 < 1GB RAM

### 质量标准
- **测试覆盖率**: > 80% 单元测试覆盖
- **错误处理**: 覆盖所有外部依赖故障场景
- **协议兼容**: 100% MCP 规范兼容
- **数据准确性**: 95% 查询返回有效数据

### 验收条件
- [ ] 所有 3 个 MCP 工具正常工作
- [ ] 支持 RSS、API、HTML 三种数据源类型
- [ ] 缓存系统有效提升重复查询性能
- [ ] 系统 7*24 小时稳定运行
- [ ] 完整的部署和使用文档

## Estimated Effort

### 总体时间估算
- **开发时间**: 8-12 工作日
- **测试时间**: 2-3 工作日
- **文档时间**: 1-2 工作日
- **总计**: 11-17 工作日 (约 2-3 周)

### 资源需求
- **开发人员**: 1 名 Golang 开发工程师
- **测试环境**: 有互联网访问的开发机器
- **部署环境**: 512MB RAM, 1 CPU Core 最小配置

### 关键路径
1. MCP 服务器基础设施 (阻塞所有后续开发)
2. 数据采集系统 (阻塞工具实现)
3. 核心 MCP 工具实现 (系统核心价值)
4. 测试和部署 (发布前必需)

这个 Epic 将交付一个功能完整、性能优秀的 MCP 服务器，为 AI 助手提供实时前端资讯查询能力，显著提升开发者的 AI 辅助开发体验。

## Tasks Created
- [ ] #2 - MCP SDK 集成设置 (parallel: false)
- [ ] #3 - 数据采集系统 (parallel: true)
- [ ] #4 - 缓存管理系统 (parallel: true)
- [ ] #5 - 核心 MCP 工具业务逻辑 (parallel: false)
- [ ] #6 - 数据处理和格式化 (parallel: true)
- [ ] #7 - 测试套件 (parallel: false)
- [ ] #8 - 部署和文档 (parallel: false)

Total tasks:        7
Parallel tasks:        3
Sequential tasks: 4
