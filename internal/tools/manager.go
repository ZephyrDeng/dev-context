package tools

import (
	"context"
	"sync"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/cache"
	"github.com/ZephyrDeng/dev-context/internal/collector"
	"github.com/ZephyrDeng/dev-context/internal/formatter"
	"github.com/ZephyrDeng/dev-context/internal/processor"
)

// ToolsManager 工具管理器，提供并发请求处理和缓存优化
type ToolsManager struct {
	handler     *Handler
	concurrency *ConcurrencyManager
	cache       *cache.CacheManager
	mu          sync.RWMutex
}

// ConcurrencyManager 并发管理器
type ConcurrencyManager struct {
	maxConcurrent int
	semaphore     chan struct{}
	activeJobs    map[string]context.CancelFunc
	mu            sync.RWMutex
}

// NewToolsManager 创建工具管理器
func NewToolsManager(
	cacheManager *cache.CacheManager,
	collectorMgr *collector.CollectorManager,
	processor *processor.Processor,
	formatterFactory *formatter.FormatterFactory,
	maxConcurrent int,
) *ToolsManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	handler := NewHandler(cacheManager, collectorMgr, processor, formatterFactory)
	concurrency := &ConcurrencyManager{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		activeJobs:    make(map[string]context.CancelFunc),
	}

	return &ToolsManager{
		handler:     handler,
		concurrency: concurrency,
		cache:       cacheManager,
	}
}

// GetHandler 获取MCP工具处理器
func (tm *ToolsManager) GetHandler() *Handler {
	return tm.handler
}

// ExecuteWithConcurrency 并发执行工具调用
func (tm *ToolsManager) ExecuteWithConcurrency(ctx context.Context, jobID string, fn func() error) error {
	// 获取并发信号量
	select {
	case tm.concurrency.semaphore <- struct{}{}:
		defer func() { <-tm.concurrency.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// 创建可取消的上下文
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	// 注册活跃任务
	tm.concurrency.mu.Lock()
	tm.concurrency.activeJobs[jobID] = cancel
	tm.concurrency.mu.Unlock()

	// 清理注册
	defer func() {
		tm.concurrency.mu.Lock()
		delete(tm.concurrency.activeJobs, jobID)
		tm.concurrency.mu.Unlock()
	}()

	// 执行任务
	return fn()
}

// CancelJob 取消指定任务
func (tm *ToolsManager) CancelJob(jobID string) bool {
	tm.concurrency.mu.Lock()
	defer tm.concurrency.mu.Unlock()

	if cancel, exists := tm.concurrency.activeJobs[jobID]; exists {
		cancel()
		delete(tm.concurrency.activeJobs, jobID)
		return true
	}

	return false
}

// GetActiveJobs 获取活跃任务列表
func (tm *ToolsManager) GetActiveJobs() []string {
	tm.concurrency.mu.RLock()
	defer tm.concurrency.mu.RUnlock()

	jobs := make([]string, 0, len(tm.concurrency.activeJobs))
	for jobID := range tm.concurrency.activeJobs {
		jobs = append(jobs, jobID)
	}

	return jobs
}

// GetStats 获取工具管理器统计信息
func (tm *ToolsManager) GetStats() ManagerStats {
	tm.concurrency.mu.RLock()
	activeJobs := len(tm.concurrency.activeJobs)
	tm.concurrency.mu.RUnlock()

	return ManagerStats{
		MaxConcurrency: tm.concurrency.maxConcurrent,
		ActiveJobs:     activeJobs,
		AvailableSlots: tm.concurrency.maxConcurrent - activeJobs,
		CacheSize:      tm.cache.Size(),
		CacheHitRate:   0.0, // TODO: 实现缓存命中率统计
		UpdatedAt:      time.Now(),
	}
}

// ManagerStats 管理器统计信息
type ManagerStats struct {
	MaxConcurrency int       `json:"maxConcurrency"`
	ActiveJobs     int       `json:"activeJobs"`
	AvailableSlots int       `json:"availableSlots"`
	CacheSize      int       `json:"cacheSize"`
	CacheHitRate   float64   `json:"cacheHitRate"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// WarmupCache 预热缓存，为常见查询预加载数据
func (tm *ToolsManager) WarmupCache(ctx context.Context) error {
	// 预热周报新闻缓存
	go func() {
		params := WeeklyNewsParams{
			MaxResults: 30,
			MinQuality: 0.5,
			Format:     "json",
		}
		_, _ = tm.handler.weeklyNewsService.GetWeeklyFrontendNews(ctx, params)
	}()

	// 预热热门仓库缓存
	go func() {
		params := TrendingReposParams{
			TimeRange:  "weekly",
			MaxResults: 30,
			MinStars:   10,
			Format:     "json",
		}
		_, _ = tm.handler.trendingReposService.GetTrendingRepositories(ctx, params)
	}()

	return nil
}

// HealthCheck 健康检查
func (tm *ToolsManager) HealthCheck(ctx context.Context) map[string]interface{} {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
	}

	// 检查缓存状态
	health["cache"] = map[string]interface{}{
		"status": "healthy",
		"size":   tm.cache.Size(),
	}

	// 检查并发管理状态
	stats := tm.GetStats()
	health["concurrency"] = map[string]interface{}{
		"activeJobs":     stats.ActiveJobs,
		"availableSlots": stats.AvailableSlots,
		"utilization":    float64(stats.ActiveJobs) / float64(stats.MaxConcurrency),
	}

	// 检查工具可用性
	toolsHealth := map[string]bool{
		"get_weekly_frontend_news":  tm.handler.weeklyNewsService != nil,
		"search_frontend_topic":     tm.handler.topicSearchService != nil,
		"get_trending_repositories": tm.handler.trendingReposService != nil,
	}
	health["tools"] = toolsHealth

	return health
}
