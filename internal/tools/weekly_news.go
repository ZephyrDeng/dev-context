package tools

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"frontend-news-mcp/internal/cache"
	"frontend-news-mcp/internal/collector"
	"frontend-news-mcp/internal/formatter"
	"frontend-news-mcp/internal/models"
	"frontend-news-mcp/internal/processor"
)

// WeeklyNewsParams 周报新闻参数
type WeeklyNewsParams struct {
	// StartDate 开始日期 (可选，默认为7天前)
	StartDate string `json:"startDate,omitempty"`
	
	// EndDate 结束日期 (可选，默认为今天)
	EndDate string `json:"endDate,omitempty"`
	
	// Category 新闻分类过滤 (可选: react, vue, angular, nodejs, typescript, etc.)
	Category string `json:"category,omitempty"`
	
	// MinQuality 最小质量分数 (0.0-1.0，默认0.5)
	MinQuality float64 `json:"minQuality,omitempty"`
	
	// MaxResults 最大返回结果数 (默认50，最大200)
	MaxResults int `json:"maxResults,omitempty"`
	
	// Format 输出格式 (json, markdown, text)
	Format string `json:"format,omitempty"`
	
	// IncludeContent 是否包含完整内容 (默认false，只返回摘要)
	IncludeContent bool `json:"includeContent,omitempty"`
	
	// SortBy 排序方式 (relevance, quality, date, title)
	SortBy string `json:"sortBy,omitempty"`
	
	// Sources 指定的数据源 (可选，多个用逗号分隔)
	Sources string `json:"sources,omitempty"`
}

// WeeklyNewsResult 周报新闻结果
type WeeklyNewsResult struct {
	Articles    []models.Article `json:"articles"`
	Summary     string          `json:"summary"`
	Period      Period          `json:"period"`
	TotalCount  int             `json:"totalCount"`
	FilterCount int             `json:"filterCount"`
	Sources     []SourceInfo    `json:"sources"`
}

// Period 时间范围信息
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Days  int       `json:"days"`
}

// SourceInfo 数据源信息
type SourceInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Type  string `json:"type"`
}

// WeeklyNewsService 周报新闻服务
type WeeklyNewsService struct {
	cacheManager    *cache.CacheManager
	collectorMgr    *collector.CollectorManager
	processor       *processor.Processor
	formatterFactory *formatter.FormatterFactory
	mu              sync.RWMutex
}

// NewWeeklyNewsService 创建周报新闻服务
func NewWeeklyNewsService(
	cacheManager *cache.CacheManager,
	collectorMgr *collector.CollectorManager,
	processor *processor.Processor,
	formatterFactory *formatter.FormatterFactory,
) *WeeklyNewsService {
	return &WeeklyNewsService{
		cacheManager:    cacheManager,
		collectorMgr:    collectorMgr,
		processor:       processor,
		formatterFactory: formatterFactory,
	}
}

// GetWeeklyFrontendNews 获取前端开发周报新闻
func (w *WeeklyNewsService) GetWeeklyFrontendNews(ctx context.Context, params WeeklyNewsParams) (*WeeklyNewsResult, error) {
	// 1. 参数验证和默认值设置
	if err := w.validateParams(&params); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}
	
	// 2. 解析时间范围
	period, err := w.parsePeriod(params.StartDate, params.EndDate)
	if err != nil {
		return nil, fmt.Errorf("时间范围解析失败: %w", err)
	}
	
	// 3. 生成缓存键
	cacheKey := w.generateCacheKey(params, period)
	
	// 4. 检查缓存
	if cached, found := w.cacheManager.Get(cacheKey); found {
		if result, ok := cached.(*WeeklyNewsResult); ok {
			log.Printf("从缓存返回周报新闻，期间: %s 到 %s", period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))
			return result, nil
		}
	}
	
	// 5. 并发收集数据
	articles, err := w.collectArticles(ctx, period, params)
	if err != nil {
		return nil, fmt.Errorf("数据收集失败: %w", err)
	}
	
	// 6. 处理和过滤数据
	filteredArticles, err := w.processAndFilter(articles, params, period)
	if err != nil {
		return nil, fmt.Errorf("数据处理失败: %w", err)
	}
	
	// 7. 构建结果
	result := &WeeklyNewsResult{
		Articles:    filteredArticles,
		Period:      *period,
		TotalCount:  len(articles),
		FilterCount: len(filteredArticles),
		Sources:     w.calculateSourceInfo(articles),
		Summary:     w.generateSummary(filteredArticles, period),
	}
	
	// 8. 缓存结果 (缓存1小时)
	w.cacheManager.SetWithTTL(cacheKey, result, time.Hour)
	
	log.Printf("成功获取周报新闻 %d 篇，期间: %s 到 %s", 
		len(filteredArticles), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))
	
	return result, nil
}

// validateParams 验证参数并设置默认值
func (w *WeeklyNewsService) validateParams(params *WeeklyNewsParams) error {
	// 设置默认值
	if params.MinQuality == 0 {
		params.MinQuality = 0.5
	}
	if params.MaxResults == 0 {
		params.MaxResults = 50
	}
	if params.Format == "" {
		params.Format = "json"
	}
	if params.SortBy == "" {
		params.SortBy = "relevance"
	}
	
	// 验证范围
	if params.MinQuality < 0 || params.MinQuality > 1 {
		return fmt.Errorf("minQuality 必须在 0.0-1.0 之间")
	}
	if params.MaxResults < 1 || params.MaxResults > 200 {
		return fmt.Errorf("maxResults 必须在 1-200 之间")
	}
	
	// 验证格式
	validFormats := []string{"json", "markdown", "text"}
	if !contains(validFormats, params.Format) {
		return fmt.Errorf("format 必须是: %v 中的一个", validFormats)
	}
	
	// 验证排序方式
	validSortBy := []string{"relevance", "quality", "date", "title"}
	if !contains(validSortBy, params.SortBy) {
		return fmt.Errorf("sortBy 必须是: %v 中的一个", validSortBy)
	}
	
	return nil
}

// parsePeriod 解析时间范围
func (w *WeeklyNewsService) parsePeriod(startDate, endDate string) (*Period, error) {
	var start, end time.Time
	var err error
	
	// 解析结束日期
	if endDate == "" {
		end = time.Now()
	} else {
		end, err = time.Parse("2006-01-02", endDate)
		if err != nil {
			return nil, fmt.Errorf("endDate 格式错误，应为 YYYY-MM-DD: %w", err)
		}
	}
	
	// 解析开始日期
	if startDate == "" {
		start = end.AddDate(0, 0, -7) // 默认7天前
	} else {
		start, err = time.Parse("2006-01-02", startDate)
		if err != nil {
			return nil, fmt.Errorf("startDate 格式错误，应为 YYYY-MM-DD: %w", err)
		}
	}
	
	// 验证日期范围
	if start.After(end) {
		return nil, fmt.Errorf("startDate 不能晚于 endDate")
	}
	
	days := int(end.Sub(start).Hours() / 24)
	if days > 30 {
		return nil, fmt.Errorf("时间范围不能超过30天")
	}
	
	return &Period{
		Start: start,
		End:   end,
		Days:  days,
	}, nil
}

// generateCacheKey 生成缓存键
func (w *WeeklyNewsService) generateCacheKey(params WeeklyNewsParams, period *Period) string {
	return fmt.Sprintf("weekly_news:%s:%s:%s:%.1f:%d:%s:%s",
		period.Start.Format("2006-01-02"),
		period.End.Format("2006-01-02"),
		params.Category,
		params.MinQuality,
		params.MaxResults,
		params.SortBy,
		params.Sources,
	)
}

// collectArticles 并发收集文章数据
func (w *WeeklyNewsService) collectArticles(ctx context.Context, period *Period, params WeeklyNewsParams) ([]models.Article, error) {
	// 定义前端开发相关的数据源配置
	configs := w.getFrontendCollectConfigs(period, params.Sources)
	
	// 并发收集
	// TODO: 实现CollectConcurrently方法或使用其他收集方式
	log.Printf("开始收集前端新闻数据，配置数量: %d", len(configs))
	
	// 暂时返回空的文章列表，后续实现具体的数据收集逻辑
	var articles []models.Article
	
	// 去重
	uniqueArticles := w.deduplicateArticles(articles)
	
	log.Printf("收集到 %d 篇文章，去重后 %d 篇", len(articles), len(uniqueArticles))
	return uniqueArticles, nil
}

// getFrontendCollectConfigs 获取前端相关数据源配置
func (w *WeeklyNewsService) getFrontendCollectConfigs(period *Period, sources string) []collector.CollectConfig {
	var configs []collector.CollectConfig
	
	// 基础前端新闻源
	frontendSources := map[string]collector.CollectConfig{
		"dev.to": {
			URL:        "https://dev.to/api/articles?tag=frontend&per_page=50",
			Headers: map[string]string{
				"User-Agent": "FrontendNews-MCP/1.0",
			},
		},
		"hackernews": {
			URL:        "https://hacker-news.firebaseio.com/v0/topstories.json",
			Headers: map[string]string{
				"User-Agent": "FrontendNews-MCP/1.0",
			},
		},
		"reddit": {
			URL:        "https://www.reddit.com/r/javascript+reactjs+vuejs+angular+frontend/.json?limit=50",
			Headers: map[string]string{
				"User-Agent": "FrontendNews-MCP/1.0",
			},
		},
		"medium": {
			URL:        "https://medium.com/feed/tag/frontend",
		},
	}
	
	// 如果指定了特定数据源
	if sources != "" {
		sourceList := splitAndTrim(sources, ",")
		for _, sourceName := range sourceList {
			if config, exists := frontendSources[sourceName]; exists {
				configs = append(configs, config)
			}
		}
	} else {
		// 使用所有数据源
		for _, config := range frontendSources {
			configs = append(configs, config)
		}
	}
	
	return configs
}

// processAndFilter 处理和过滤数据
func (w *WeeklyNewsService) processAndFilter(articles []models.Article, params WeeklyNewsParams, period *Period) ([]models.Article, error) {
	var filtered []models.Article
	
	for _, article := range articles {
		// 时间范围过滤
		if !article.PublishedAt.After(period.Start.Add(-time.Hour)) || !article.PublishedAt.Before(period.End.Add(time.Hour)) {
			continue
		}
		
		// 质量分数过滤
		if article.Quality < params.MinQuality {
			continue
		}
		
		// 分类过滤
		if params.Category != "" && !w.matchesCategory(article, params.Category) {
			continue
		}
		
		// 计算相关性分数
		if w.processor != nil {
			article.Relevance = w.processor.CalculateFrontendRelevance(article, params.Category)
		}
		
		// 更新质量分数
		article.UpdateQuality()
		
		filtered = append(filtered, article)
	}
	
	// 排序
	w.sortArticles(filtered, params.SortBy)
	
	// 限制数量
	if len(filtered) > params.MaxResults {
		filtered = filtered[:params.MaxResults]
	}
	
	return filtered, nil
}

// matchesCategory 检查文章是否匹配指定分类
func (w *WeeklyNewsService) matchesCategory(article models.Article, category string) bool {
	// 检查标签
	for _, tag := range article.Tags {
		if tag == category {
			return true
		}
	}
	
	// 检查标题和摘要中的关键词
	categoryKeywords := w.getCategoryKeywords(category)
	content := article.Title + " " + article.Summary
	
	for _, keyword := range categoryKeywords {
		if contains([]string{content}, keyword) {
			return true
		}
	}
	
	return false
}

// getCategoryKeywords 获取分类关键词
func (w *WeeklyNewsService) getCategoryKeywords(category string) []string {
	keywords := map[string][]string{
		"react":      {"react", "jsx", "hooks", "redux", "next.js", "nextjs"},
		"vue":        {"vue", "vuejs", "vue.js", "nuxt", "vuex", "pinia"},
		"angular":    {"angular", "typescript", "rxjs", "ngrx", "ionic"},
		"nodejs":     {"node.js", "nodejs", "express", "npm", "backend"},
		"typescript": {"typescript", "ts", "type", "interface"},
		"javascript": {"javascript", "js", "es6", "es2021", "es2022"},
		"css":        {"css", "sass", "scss", "less", "styled-components"},
		"testing":    {"test", "testing", "jest", "cypress", "playwright"},
		"webpack":    {"webpack", "vite", "rollup", "bundler", "build"},
	}
	
	if words, exists := keywords[category]; exists {
		return words
	}
	
	return []string{category}
}

// sortArticles 排序文章
func (w *WeeklyNewsService) sortArticles(articles []models.Article, sortBy string) {
	sort.Slice(articles, func(i, j int) bool {
		switch sortBy {
		case "relevance":
			return articles[i].Relevance > articles[j].Relevance
		case "quality":
			return articles[i].Quality > articles[j].Quality
		case "date":
			return articles[i].PublishedAt.After(articles[j].PublishedAt)
		case "title":
			return articles[i].Title < articles[j].Title
		default:
			return articles[i].Relevance > articles[j].Relevance
		}
	})
}

// deduplicateArticles 文章去重
func (w *WeeklyNewsService) deduplicateArticles(articles []models.Article) []models.Article {
	seen := make(map[string]bool)
	var unique []models.Article
	
	for _, article := range articles {
		hash := article.CalculateHash()
		if !seen[hash] {
			seen[hash] = true
			unique = append(unique, article)
		}
	}
	
	return unique
}

// calculateSourceInfo 计算数据源信息
func (w *WeeklyNewsService) calculateSourceInfo(articles []models.Article) []SourceInfo {
	sourceCount := make(map[string]int)
	sourceType := make(map[string]string)
	
	for _, article := range articles {
		sourceCount[article.Source]++
		sourceType[article.Source] = article.SourceType
	}
	
	var sources []SourceInfo
	for source, count := range sourceCount {
		sources = append(sources, SourceInfo{
			Name:  source,
			Count: count,
			Type:  sourceType[source],
		})
	}
	
	// 按文章数量排序
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Count > sources[j].Count
	})
	
	return sources
}

// generateSummary 生成摘要
func (w *WeeklyNewsService) generateSummary(articles []models.Article, period *Period) string {
	if len(articles) == 0 {
		return fmt.Sprintf("在 %s 到 %s 期间未找到符合条件的前端开发新闻。",
			period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))
	}
	
	// 分析主要话题
	topicCount := make(map[string]int)
	for _, article := range articles {
		for _, tag := range article.Tags {
			topicCount[tag]++
		}
	}
	
	var topTopics []string
	for topic, count := range topicCount {
		if count >= 2 && len(topTopics) < 5 {
			topTopics = append(topTopics, topic)
		}
	}
	
	summary := fmt.Sprintf("在 %s 到 %s 期间，共收集到 %d 篇高质量前端开发新闻。",
		period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"), len(articles))
	
	if len(topTopics) > 0 {
		summary += fmt.Sprintf(" 主要涉及话题：%s。", joinStrings(topTopics, "、"))
	}
	
	return summary
}

// FormatResult 格式化结果输出
func (w *WeeklyNewsService) FormatResult(result *WeeklyNewsResult, format string) (string, error) {
	// 设置格式化配置
	config := formatter.DefaultConfig()
	config.Format = formatter.OutputFormat(format)
	config.IncludeMetadata = true
	config.MaxSummaryLength = 200
	
	w.formatterFactory.UpdateConfig(config)
	
	// 创建格式化器
	fmt, err := w.formatterFactory.CreateFormatter()
	if err != nil {
		return "", err
	}
	
	// 格式化文章
	return fmt.FormatArticles(result.Articles)
}

// 辅助函数
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var result []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func joinStrings(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += sep + slice[i]
	}
	return result
}