package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// CollectorManagerImpl 采集器管理器实现
type CollectorManagerImpl struct {
	collectors map[string]DataCollector
	mutex      sync.RWMutex
}

// NewCollectorManager 创建采集器管理器
func NewCollectorManager() CollectorManager {
	manager := &CollectorManagerImpl{
		collectors: make(map[string]DataCollector),
	}

	// 注册默认采集器
	manager.RegisterCollector("rss", NewRSSCollector())
	manager.RegisterCollector("api", NewAPICollector())
	manager.RegisterCollector("html", NewHTMLCollector())

	return manager
}

// RegisterCollector 注册采集器
func (cm *CollectorManagerImpl) RegisterCollector(sourceType string, collector DataCollector) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.collectors[sourceType] = collector
}

// GetCollector 根据类型获取采集器
func (cm *CollectorManagerImpl) GetCollector(sourceType string) (DataCollector, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	collector, exists := cm.collectors[sourceType]
	return collector, exists
}

// CollectAll 并发采集多个数据源
func (cm *CollectorManagerImpl) CollectAll(ctx context.Context, configs []CollectConfig) []CollectResult {
	if len(configs) == 0 {
		return nil
	}

	// 创建结果通道
	resultChan := make(chan CollectResult, len(configs))
	var wg sync.WaitGroup

	// 为每个配置启动一个goroutine
	for _, config := range configs {
		wg.Add(1)
		go func(cfg CollectConfig) {
			defer wg.Done()
			
			result := cm.collectSingle(ctx, cfg)
			resultChan <- result
		}(config)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	var results []CollectResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// collectSingle 采集单个数据源
func (cm *CollectorManagerImpl) collectSingle(ctx context.Context, config CollectConfig) CollectResult {
	// 确定采集器类型
	sourceType := cm.determineSourceType(config)
	
	collector, exists := cm.GetCollector(sourceType)
	if !exists {
		return CollectResult{
			Source: config.URL,
			Error:  fmt.Errorf("no collector found for source type: %s", sourceType),
		}
	}

	// 执行采集
	result, err := collector.Collect(ctx, config)
	if err != nil {
		return CollectResult{
			Source: config.URL,
			Error:  fmt.Errorf("collection failed: %w", err),
		}
	}

	return result
}

// CollectWithRetry 带重试机制的采集
func (cm *CollectorManagerImpl) CollectWithRetry(ctx context.Context, config CollectConfig, retryConfig RetryConfig) (CollectResult, error) {
	var lastErr error
	
	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// 等待重试延迟
			select {
			case <-ctx.Done():
				return CollectResult{}, ctx.Err()
			case <-time.After(retryConfig.RetryDelay):
			}
			
			log.Printf("Retrying collection attempt %d/%d for %s", attempt, retryConfig.MaxRetries, config.URL)
		}

		result := cm.collectSingle(ctx, config)
		if result.Error == nil {
			return result, nil
		}

		lastErr = result.Error
		
		// 检查是否应该重试
		if !cm.shouldRetry(result.Error) {
			break
		}
	}

	return CollectResult{}, fmt.Errorf("collection failed after %d retries: %w", retryConfig.MaxRetries, lastErr)
}

// shouldRetry 判断是否应该重试
func (cm *CollectorManagerImpl) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	
	// 不重试的错误类型
	nonRetryableErrors := []string{
		"validation failed",
		"invalid URL",
		"no collector found",
		"404",
		"401", 
		"403",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if contains(errStr, nonRetryable) {
			return false
		}
	}

	// 可重试的错误类型
	retryableErrors := []string{
		"timeout",
		"connection",
		"network",
		"500",
		"502",
		"503",
		"504",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	// 默认重试
	return true
}

// determineSourceType 确定数据源类型
func (cm *CollectorManagerImpl) determineSourceType(config CollectConfig) string {
	url := config.URL
	
	// 检查是否指定了源类型
	if sourceType, exists := config.Metadata["source_type"]; exists {
		return sourceType
	}

	// 根据URL特征自动判断
	if contains(url, ".xml") || contains(url, "/rss") || contains(url, "/feed") || contains(url, "/atom") {
		return "rss"
	}

	if contains(url, "api.github.com") || contains(url, "dev.to/api") || contains(url, "/api/") {
		return "api"
	}

	// 默认使用HTML采集器
	return "html"
}

// BatchCollector 批量采集器
type BatchCollector struct {
	manager       CollectorManager
	maxConcurrent int
	timeout       time.Duration
}

// NewBatchCollector 创建批量采集器
func NewBatchCollector(manager CollectorManager, maxConcurrent int, timeout time.Duration) *BatchCollector {
	return &BatchCollector{
		manager:       manager,
		maxConcurrent: maxConcurrent,
		timeout:       timeout,
	}
}

// CollectBatch 批量采集，支持并发控制
func (bc *BatchCollector) CollectBatch(ctx context.Context, configs []CollectConfig) []CollectResult {
	if len(configs) == 0 {
		return nil
	}

	// 创建信号量控制并发
	semaphore := make(chan struct{}, bc.maxConcurrent)
	resultChan := make(chan CollectResult, len(configs))
	var wg sync.WaitGroup

	// 设置全局超时
	if bc.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, bc.timeout)
		defer cancel()
	}

	for _, config := range configs {
		wg.Add(1)
		go func(cfg CollectConfig) {
			defer wg.Done()
			
			// 获取信号量
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				resultChan <- CollectResult{
					Source: cfg.URL,
					Error:  ctx.Err(),
				}
				return
			}

			// 执行采集
			result := bc.manager.(*CollectorManagerImpl).collectSingle(ctx, cfg)
			resultChan <- result
		}(config)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	var results []CollectResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// AggregateResults 聚合多个采集结果
func AggregateResults(results []CollectResult) ([]Article, []error) {
	var allArticles []Article
	var errors []error
	
	for _, result := range results {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("source %s: %w", result.Source, result.Error))
		} else {
			allArticles = append(allArticles, result.Articles...)
		}
	}

	return allArticles, errors
}

// FilterArticles 根据条件过滤文章
type ArticleFilter struct {
	Keywords    []string  // 关键词过滤
	Authors     []string  // 作者过滤
	Tags        []string  // 标签过滤
	Languages   []string  // 语言过滤
	DateFrom    time.Time // 起始日期
	DateTo      time.Time // 结束日期
	MinLength   int       // 最小内容长度
	MaxLength   int       // 最大内容长度
}

// FilterArticles 过滤文章
func (af *ArticleFilter) FilterArticles(articles []Article) []Article {
	var filtered []Article

	for _, article := range articles {
		if af.matchesFilter(article) {
			filtered = append(filtered, article)
		}
	}

	return filtered
}

// matchesFilter 检查文章是否匹配过滤条件
func (af *ArticleFilter) matchesFilter(article Article) bool {
	// 关键词过滤
	if len(af.Keywords) > 0 {
		found := false
		for _, keyword := range af.Keywords {
			if contains(article.Title, keyword) || contains(article.Content, keyword) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 作者过滤
	if len(af.Authors) > 0 {
		found := false
		for _, author := range af.Authors {
			if contains(article.Author, author) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 标签过滤
	if len(af.Tags) > 0 {
		found := false
		for _, filterTag := range af.Tags {
			for _, articleTag := range article.Tags {
				if contains(articleTag, filterTag) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// 语言过滤
	if len(af.Languages) > 0 {
		found := false
		for _, lang := range af.Languages {
			if article.Language == lang {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 日期过滤
	if !af.DateFrom.IsZero() && article.PublishedAt.Before(af.DateFrom) {
		return false
	}
	if !af.DateTo.IsZero() && article.PublishedAt.After(af.DateTo) {
		return false
	}

	// 长度过滤
	contentLen := len(article.Content)
	if af.MinLength > 0 && contentLen < af.MinLength {
		return false
	}
	if af.MaxLength > 0 && contentLen > af.MaxLength {
		return false
	}

	return true
}

// DeduplicateArticles 去重文章
func DeduplicateArticles(articles []Article) []Article {
	seen := make(map[string]bool)
	var deduplicated []Article

	for _, article := range articles {
		// 使用URL或ID作为去重键
		key := article.URL
		if key == "" {
			key = article.ID
		}

		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, article)
		}
	}

	return deduplicated
}

// SortArticles 排序文章
type SortBy string

const (
	SortByDate   SortBy = "date"
	SortByTitle  SortBy = "title"
	SortByAuthor SortBy = "author"
)

func SortArticles(articles []Article, sortBy SortBy, ascending bool) []Article {
	sorted := make([]Article, len(articles))
	copy(sorted, articles)

	switch sortBy {
	case SortByDate:
		if ascending {
			// 简单的冒泡排序 - 升序
			for i := 0; i < len(sorted)-1; i++ {
				for j := 0; j < len(sorted)-i-1; j++ {
					if sorted[j].PublishedAt.After(sorted[j+1].PublishedAt) {
						sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
					}
				}
			}
		} else {
			// 简单的冒泡排序 - 降序
			for i := 0; i < len(sorted)-1; i++ {
				for j := 0; j < len(sorted)-i-1; j++ {
					if sorted[j].PublishedAt.Before(sorted[j+1].PublishedAt) {
						sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
					}
				}
			}
		}
	case SortByTitle:
		if ascending {
			for i := 0; i < len(sorted)-1; i++ {
				for j := 0; j < len(sorted)-i-1; j++ {
					if sorted[j].Title > sorted[j+1].Title {
						sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
					}
				}
			}
		} else {
			for i := 0; i < len(sorted)-1; i++ {
				for j := 0; j < len(sorted)-i-1; j++ {
					if sorted[j].Title < sorted[j+1].Title {
						sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
					}
				}
			}
		}
	case SortByAuthor:
		if ascending {
			for i := 0; i < len(sorted)-1; i++ {
				for j := 0; j < len(sorted)-i-1; j++ {
					if sorted[j].Author > sorted[j+1].Author {
						sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
					}
				}
			}
		} else {
			for i := 0; i < len(sorted)-1; i++ {
				for j := 0; j < len(sorted)-i-1; j++ {
					if sorted[j].Author < sorted[j+1].Author {
						sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
					}
				}
			}
		}
	}

	return sorted
}

// 辅助函数
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || 
		(len(str) > len(substr) && 
			(str[:len(substr)] == substr || 
				str[len(str)-len(substr):] == substr ||
				indexOf(str, substr) >= 0)))
}

func indexOf(str, substr string) int {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}