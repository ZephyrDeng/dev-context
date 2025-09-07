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

// TopicSearchParams 主题搜索参数
type TopicSearchParams struct {
	// Query 搜索关键词 (必需)
	Query string `json:"query" validate:"required"`
	
	// Language 编程语言过滤 (可选: javascript, typescript, etc.)
	Language string `json:"language,omitempty"`
	
	// Platform 平台过滤 (可选: github, stackoverflow, reddit, etc.)
	Platform string `json:"platform,omitempty"`
	
	// SortBy 排序方式 (relevance, date, popularity, stars)
	SortBy string `json:"sortBy,omitempty"`
	
	// TimeRange 时间范围 (可选: day, week, month, year, all)
	TimeRange string `json:"timeRange,omitempty"`
	
	// MaxResults 最大返回结果数 (默认30，最大100)
	MaxResults int `json:"maxResults,omitempty"`
	
	// Format 输出格式 (json, markdown, text)
	Format string `json:"format,omitempty"`
	
	// IncludeCode 是否包含代码片段 (默认true)
	IncludeCode bool `json:"includeCode,omitempty"`
	
	// MinScore 最小相关性分数 (0.0-1.0，默认0.3)
	MinScore float64 `json:"minScore,omitempty"`
	
	// SearchType 搜索类型 (discussions, repositories, articles, all)
	SearchType string `json:"searchType,omitempty"`
}

// TopicSearchResult 主题搜索结果
type TopicSearchResult struct {
	Query        string                `json:"query"`
	Articles     []models.Article     `json:"articles,omitempty"`
	Repositories []models.Repository  `json:"repositories,omitempty"`
	Discussions  []Discussion         `json:"discussions,omitempty"`
	Summary      SearchSummary        `json:"summary"`
	SearchTime   time.Time           `json:"searchTime"`
	TotalResults int                 `json:"totalResults"`
	Sources      []PlatformInfo      `json:"sources"`
}

// Discussion 讨论信息
type Discussion struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	URL         string            `json:"url"`
	Platform    string            `json:"platform"`
	Author      string            `json:"author"`
	Score       int               `json:"score"`
	Replies     int               `json:"replies"`
	CreatedAt   time.Time         `json:"createdAt"`
	Tags        []string          `json:"tags"`
	Content     string            `json:"content,omitempty"`
	CodeBlocks  []CodeBlock       `json:"codeBlocks,omitempty"`
	Relevance   float64           `json:"relevance"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CodeBlock 代码块
type CodeBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Context  string `json:"context,omitempty"`
}

// SearchSummary 搜索摘要
type SearchSummary struct {
	TopicKeywords    []string          `json:"topicKeywords"`
	PopularLanguages []string          `json:"popularLanguages"`
	TrendingTopics   []string          `json:"trendingTopics"`
	AverageScore     float64           `json:"averageScore"`
	SearchStats      map[string]int    `json:"searchStats"`
}

// PlatformInfo 平台信息
type PlatformInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Count   int    `json:"count"`
	Quality float64 `json:"quality"`
}

// TopicSearchService 主题搜索服务
type TopicSearchService struct {
	cacheManager     *cache.CacheManager
	collectorMgr     *collector.CollectorManager
	processor        *processor.Processor
	formatterFactory *formatter.FormatterFactory
	mu               sync.RWMutex
}

// NewTopicSearchService 创建主题搜索服务
func NewTopicSearchService(
	cacheManager *cache.CacheManager,
	collectorMgr *collector.CollectorManager,
	processor *processor.Processor,
	formatterFactory *formatter.FormatterFactory,
) *TopicSearchService {
	return &TopicSearchService{
		cacheManager:     cacheManager,
		collectorMgr:     collectorMgr,
		processor:        processor,
		formatterFactory: formatterFactory,
	}
}

// SearchFrontendTopic 搜索前端相关主题和讨论
func (t *TopicSearchService) SearchFrontendTopic(ctx context.Context, params TopicSearchParams) (*TopicSearchResult, error) {
	// 1. 参数验证和默认值设置
	if err := t.validateParams(&params); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}
	
	// 2. 生成缓存键
	cacheKey := t.generateCacheKey(params)
	
	// 3. 检查缓存
	if cached, found := t.cacheManager.Get(cacheKey); found {
		if result, ok := cached.(*TopicSearchResult); ok {
			log.Printf("从缓存返回主题搜索结果: %s", params.Query)
			return result, nil
		}
	}
	
	// 4. 并发搜索多个平台
	searchResults, err := t.searchAcrossPlatforms(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("跨平台搜索失败: %w", err)
	}
	
	// 5. 处理和排序结果
	result, err := t.processSearchResults(searchResults, params)
	if err != nil {
		return nil, fmt.Errorf("结果处理失败: %w", err)
	}
	
	// 6. 缓存结果 (缓存30分钟)
	t.cacheManager.SetWithTTL(cacheKey, result, 30*time.Minute)
	
	log.Printf("成功搜索主题 '%s'，找到 %d 个结果", params.Query, result.TotalResults)
	
	return result, nil
}

// validateParams 验证参数并设置默认值
func (t *TopicSearchService) validateParams(params *TopicSearchParams) error {
	if strings.TrimSpace(params.Query) == "" {
		return fmt.Errorf("query 参数是必需的")
	}
	
	// 设置默认值
	if params.MaxResults == 0 {
		params.MaxResults = 30
	}
	if params.Format == "" {
		params.Format = "json"
	}
	if params.SortBy == "" {
		params.SortBy = "relevance"
	}
	if params.TimeRange == "" {
		params.TimeRange = "month"
	}
	if params.MinScore == 0 {
		params.MinScore = 0.3
	}
	if params.SearchType == "" {
		params.SearchType = "all"
	}
	params.IncludeCode = true // 默认包含代码
	
	// 验证范围
	if params.MaxResults < 1 || params.MaxResults > 100 {
		return fmt.Errorf("maxResults 必须在 1-100 之间")
	}
	if params.MinScore < 0 || params.MinScore > 1 {
		return fmt.Errorf("minScore 必须在 0.0-1.0 之间")
	}
	
	// 验证枚举值
	validFormats := []string{"json", "markdown", "text"}
	if !contains(validFormats, params.Format) {
		return fmt.Errorf("format 必须是: %v 中的一个", validFormats)
	}
	
	validSortBy := []string{"relevance", "date", "popularity", "stars"}
	if !contains(validSortBy, params.SortBy) {
		return fmt.Errorf("sortBy 必须是: %v 中的一个", validSortBy)
	}
	
	validTimeRanges := []string{"day", "week", "month", "year", "all"}
	if !contains(validTimeRanges, params.TimeRange) {
		return fmt.Errorf("timeRange 必须是: %v 中的一个", validTimeRanges)
	}
	
	validSearchTypes := []string{"discussions", "repositories", "articles", "all"}
	if !contains(validSearchTypes, params.SearchType) {
		return fmt.Errorf("searchType 必须是: %v 中的一个", validSearchTypes)
	}
	
	return nil
}

// generateCacheKey 生成缓存键
func (t *TopicSearchService) generateCacheKey(params TopicSearchParams) string {
	return fmt.Sprintf("topic_search:%s:%s:%s:%s:%s:%d:%.1f",
		params.Query,
		params.Language,
		params.Platform,
		params.SortBy,
		params.TimeRange,
		params.MaxResults,
		params.MinScore,
	)
}

// searchAcrossPlatforms 跨平台搜索
func (t *TopicSearchService) searchAcrossPlatforms(ctx context.Context, params TopicSearchParams) (*multiPlatformResults, error) {
	var wg sync.WaitGroup
	results := &multiPlatformResults{}
	
	// 定义搜索通道
	type searchJob struct {
		platform string
		config   collector.CollectConfig
	}
	
	jobs := make(chan searchJob, 10)
	
	// 启动worker goroutines
	numWorkers := 3
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				t.searchSinglePlatform(ctx, job.platform, job.config, params, results)
			}
		}()
	}
	
	// 发送搜索任务
	go func() {
		defer close(jobs)
		searchConfigs := t.getSearchConfigs(params)
		for platform, config := range searchConfigs {
			select {
			case jobs <- searchJob{platform: platform, config: config}:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	wg.Wait()
	return results, nil
}

// multiPlatformResults 多平台搜索结果
type multiPlatformResults struct {
	Articles     []models.Article    `json:"articles"`
	Repositories []models.Repository `json:"repositories"`
	Discussions  []Discussion        `json:"discussions"`
	mu           sync.Mutex
}

// addArticle 线程安全添加文章
func (m *multiPlatformResults) addArticle(article models.Article) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Articles = append(m.Articles, article)
}

// addRepository 线程安全添加仓库
func (m *multiPlatformResults) addRepository(repo models.Repository) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Repositories = append(m.Repositories, repo)
}

// addDiscussion 线程安全添加讨论
func (m *multiPlatformResults) addDiscussion(discussion Discussion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Discussions = append(m.Discussions, discussion)
}

// getSearchConfigs 获取搜索配置
func (t *TopicSearchService) getSearchConfigs(params TopicSearchParams) map[string]collector.CollectConfig {
	configs := make(map[string]collector.CollectConfig)
	
	// GitHub搜索配置
	if params.Platform == "" || params.Platform == "github" {
		if params.SearchType == "all" || params.SearchType == "repositories" {
			configs["github_repos"] = collector.CollectConfig{
				URL:        fmt.Sprintf("https://api.github.com/search/repositories?q=%s+language:%s&sort=stars&order=desc&per_page=30",
					params.Query, getLanguageParam(params.Language)),
				Headers: map[string]string{
					"Accept":     "application/vnd.github.v3+json",
					"User-Agent": "FrontendNews-MCP/1.0",
				},
			}
		}
		
		if params.SearchType == "all" || params.SearchType == "discussions" {
			configs["github_issues"] = collector.CollectConfig{
				URL:        fmt.Sprintf("https://api.github.com/search/issues?q=%s+is:issue&sort=updated&order=desc&per_page=20",
					params.Query),
				Headers: map[string]string{
					"Accept":     "application/vnd.github.v3+json",
					"User-Agent": "FrontendNews-MCP/1.0",
				},
			}
		}
	}
	
	// Stack Overflow搜索配置
	if params.Platform == "" || params.Platform == "stackoverflow" {
		if params.SearchType == "all" || params.SearchType == "discussions" {
			configs["stackoverflow"] = collector.CollectConfig{
				URL:        fmt.Sprintf("https://api.stackexchange.com/2.3/search/advanced?order=desc&sort=relevance&q=%s&site=stackoverflow&pagesize=20",
					params.Query),
				Headers: map[string]string{
					"User-Agent": "FrontendNews-MCP/1.0",
				},
			}
		}
	}
	
	// Reddit搜索配置
	if params.Platform == "" || params.Platform == "reddit" {
		if params.SearchType == "all" || params.SearchType == "discussions" {
			subreddits := "javascript+reactjs+vuejs+angular+frontend+webdev"
			configs["reddit"] = collector.CollectConfig{
				URL:        fmt.Sprintf("https://www.reddit.com/r/%s/search.json?q=%s&restrict_sr=1&sort=relevance&limit=20",
					subreddits, params.Query),
				Headers: map[string]string{
					"User-Agent": "FrontendNews-MCP/1.0",
				},
			}
		}
	}
	
	// Dev.to搜索配置
	if params.Platform == "" || params.Platform == "dev.to" {
		if params.SearchType == "all" || params.SearchType == "articles" {
			configs["devto"] = collector.CollectConfig{
				URL:        fmt.Sprintf("https://dev.to/api/articles?tag=frontend&per_page=30&query=%s",
					params.Query),
				Headers: map[string]string{
					"User-Agent": "FrontendNews-MCP/1.0",
				},
			}
		}
	}
	
	return configs
}

// searchSinglePlatform 搜索单个平台
func (t *TopicSearchService) searchSinglePlatform(ctx context.Context, platform string, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	log.Printf("搜索平台 %s: %s", platform, params.Query)
	
	// 使用collector收集数据
	collector := t.getCollectorForPlatform(platform)
	if collector == nil {
		log.Printf("未找到平台 %s 的采集器", platform)
		return
	}
	
	// 根据平台类型处理不同的响应
	switch {
	case strings.Contains(platform, "github_repos"):
		t.handleGitHubRepos(ctx, config, params, results)
	case strings.Contains(platform, "github_issues"):
		t.handleGitHubIssues(ctx, config, params, results)
	case strings.Contains(platform, "stackoverflow"):
		t.handleStackOverflow(ctx, config, params, results)
	case strings.Contains(platform, "reddit"):
		t.handleReddit(ctx, config, params, results)
	case strings.Contains(platform, "devto"):
		t.handleDevTo(ctx, config, params, results)
	default:
		log.Printf("未知平台类型: %s", platform)
	}
}

// handleGitHubRepos 处理GitHub仓库搜索
func (t *TopicSearchService) handleGitHubRepos(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	// 这里应该调用collector获取数据，然后解析为Repository对象
	// 由于collector的具体实现细节，这里提供结构化的处理逻辑
	
	// 模拟数据收集过程
	log.Printf("处理GitHub仓库搜索: %s", params.Query)
	
	// TODO: 实际实现需要调用collector并解析GitHub API响应
	// repositories := collector.CollectRepositories(ctx, config)
	// for _, repo := range repositories {
	//     results.addRepository(repo)
	// }
}

// handleGitHubIssues 处理GitHub Issues搜索
func (t *TopicSearchService) handleGitHubIssues(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	log.Printf("处理GitHub Issues搜索: %s", params.Query)
	// TODO: 实现GitHub Issues搜索逻辑
}

// handleStackOverflow 处理Stack Overflow搜索
func (t *TopicSearchService) handleStackOverflow(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	log.Printf("处理Stack Overflow搜索: %s", params.Query)
	// TODO: 实现Stack Overflow搜索逻辑
}

// handleReddit 处理Reddit搜索
func (t *TopicSearchService) handleReddit(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	log.Printf("处理Reddit搜索: %s", params.Query)
	// TODO: 实现Reddit搜索逻辑
}

// handleDevTo 处理Dev.to搜索
func (t *TopicSearchService) handleDevTo(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	log.Printf("处理Dev.to搜索: %s", params.Query)
	// TODO: 实现Dev.to搜索逻辑
}

// getCollectorForPlatform 获取平台对应的collector
func (t *TopicSearchService) getCollectorForPlatform(platform string) *collector.CollectorManager {
	// TODO: 根据平台返回相应的collector实例
	return nil
}

// processSearchResults 处理搜索结果
func (t *TopicSearchService) processSearchResults(searchResults *multiPlatformResults, params TopicSearchParams) (*TopicSearchResult, error) {
	result := &TopicSearchResult{
		Query:       params.Query,
		SearchTime:  time.Now(),
		Articles:    searchResults.Articles,
		Repositories: searchResults.Repositories,
		Discussions: searchResults.Discussions,
	}
	
	// 计算相关性分数
	t.calculateRelevanceScores(result, params)
	
	// 过滤低分结果
	t.filterByScore(result, params.MinScore)
	
	// 排序结果
	t.sortResults(result, params.SortBy)
	
	// 限制结果数量
	t.limitResults(result, params.MaxResults)
	
	// 生成统计摘要
	result.Summary = t.generateSearchSummary(result)
	result.Sources = t.calculatePlatformInfo(result)
	result.TotalResults = len(result.Articles) + len(result.Repositories) + len(result.Discussions)
	
	return result, nil
}

// calculateRelevanceScores 计算相关性分数
func (t *TopicSearchService) calculateRelevanceScores(result *TopicSearchResult, params TopicSearchParams) {
	keywords := strings.Fields(strings.ToLower(params.Query))
	
	// 计算文章相关性
	for i := range result.Articles {
		result.Articles[i].Relevance = t.calculateTextRelevance(
			result.Articles[i].Title+" "+result.Articles[i].Summary,
			keywords,
		)
	}
	
	// 计算仓库相关性
	for i := range result.Repositories {
		result.Repositories[i].TrendScore = t.calculateTextRelevance(
			result.Repositories[i].Name+" "+result.Repositories[i].Description,
			keywords,
		)
	}
	
	// 计算讨论相关性
	for i := range result.Discussions {
		result.Discussions[i].Relevance = t.calculateTextRelevance(
			result.Discussions[i].Title+" "+result.Discussions[i].Content,
			keywords,
		)
	}
}

// calculateTextRelevance 计算文本相关性
func (t *TopicSearchService) calculateTextRelevance(text string, keywords []string) float64 {
	text = strings.ToLower(text)
	score := 0.0
	totalKeywords := len(keywords)
	
	if totalKeywords == 0 {
		return 0.0
	}
	
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			score += 1.0
		}
	}
	
	return score / float64(totalKeywords)
}

// filterByScore 按分数过滤结果
func (t *TopicSearchService) filterByScore(result *TopicSearchResult, minScore float64) {
	var filteredArticles []models.Article
	for _, article := range result.Articles {
		if article.Relevance >= minScore {
			filteredArticles = append(filteredArticles, article)
		}
	}
	result.Articles = filteredArticles
	
	var filteredRepos []models.Repository
	for _, repo := range result.Repositories {
		if repo.TrendScore >= minScore {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	result.Repositories = filteredRepos
	
	var filteredDiscussions []Discussion
	for _, discussion := range result.Discussions {
		if discussion.Relevance >= minScore {
			filteredDiscussions = append(filteredDiscussions, discussion)
		}
	}
	result.Discussions = filteredDiscussions
}

// sortResults 排序结果
func (t *TopicSearchService) sortResults(result *TopicSearchResult, sortBy string) {
	switch sortBy {
	case "relevance":
		sort.Slice(result.Articles, func(i, j int) bool {
			return result.Articles[i].Relevance > result.Articles[j].Relevance
		})
		sort.Slice(result.Repositories, func(i, j int) bool {
			return result.Repositories[i].TrendScore > result.Repositories[j].TrendScore
		})
		sort.Slice(result.Discussions, func(i, j int) bool {
			return result.Discussions[i].Relevance > result.Discussions[j].Relevance
		})
	case "date":
		sort.Slice(result.Articles, func(i, j int) bool {
			return result.Articles[i].PublishedAt.After(result.Articles[j].PublishedAt)
		})
		sort.Slice(result.Repositories, func(i, j int) bool {
			return result.Repositories[i].UpdatedAt.After(result.Repositories[j].UpdatedAt)
		})
		sort.Slice(result.Discussions, func(i, j int) bool {
			return result.Discussions[i].CreatedAt.After(result.Discussions[j].CreatedAt)
		})
	case "popularity", "stars":
		sort.Slice(result.Repositories, func(i, j int) bool {
			return result.Repositories[i].Stars > result.Repositories[j].Stars
		})
		sort.Slice(result.Discussions, func(i, j int) bool {
			return result.Discussions[i].Score > result.Discussions[j].Score
		})
	}
}

// limitResults 限制结果数量
func (t *TopicSearchService) limitResults(result *TopicSearchResult, maxResults int) {
	// 按比例分配各类型结果数量
	articlesLimit := maxResults / 3
	reposLimit := maxResults / 3
	discussionsLimit := maxResults - articlesLimit - reposLimit
	
	if len(result.Articles) > articlesLimit {
		result.Articles = result.Articles[:articlesLimit]
	}
	if len(result.Repositories) > reposLimit {
		result.Repositories = result.Repositories[:reposLimit]
	}
	if len(result.Discussions) > discussionsLimit {
		result.Discussions = result.Discussions[:discussionsLimit]
	}
}

// generateSearchSummary 生成搜索摘要
func (t *TopicSearchService) generateSearchSummary(result *TopicSearchResult) SearchSummary {
	summary := SearchSummary{
		SearchStats: make(map[string]int),
	}
	
	// 统计各类型结果数量
	summary.SearchStats["articles"] = len(result.Articles)
	summary.SearchStats["repositories"] = len(result.Repositories)
	summary.SearchStats["discussions"] = len(result.Discussions)
	
	// 提取热门关键词
	keywordCount := make(map[string]int)
	for _, article := range result.Articles {
		for _, tag := range article.Tags {
			keywordCount[tag]++
		}
	}
	
	// 排序并获取前5个关键词
	type kv struct {
		Key   string
		Value int
	}
	var kvs []kv
	for k, v := range keywordCount {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Value > kvs[j].Value
	})
	
	for i := 0; i < len(kvs) && i < 5; i++ {
		summary.TopicKeywords = append(summary.TopicKeywords, kvs[i].Key)
	}
	
	// 计算平均分数
	var totalScore float64
	var count int
	for _, article := range result.Articles {
		totalScore += article.Relevance
		count++
	}
	for _, repo := range result.Repositories {
		totalScore += repo.TrendScore
		count++
	}
	for _, discussion := range result.Discussions {
		totalScore += discussion.Relevance
		count++
	}
	
	if count > 0 {
		summary.AverageScore = totalScore / float64(count)
	}
	
	return summary
}

// calculatePlatformInfo 计算平台信息
func (t *TopicSearchService) calculatePlatformInfo(result *TopicSearchResult) []PlatformInfo {
	platformCount := make(map[string]int)
	platformQuality := make(map[string][]float64)
	
	// 统计文章来源
	for _, article := range result.Articles {
		platformCount[article.Source]++
		platformQuality[article.Source] = append(platformQuality[article.Source], article.Quality)
	}
	
	// 统计讨论来源
	for _, discussion := range result.Discussions {
		platformCount[discussion.Platform]++
		platformQuality[discussion.Platform] = append(platformQuality[discussion.Platform], discussion.Relevance)
	}
	
	var platforms []PlatformInfo
	for platform, count := range platformCount {
		avgQuality := 0.0
		if qualities := platformQuality[platform]; len(qualities) > 0 {
			sum := 0.0
			for _, q := range qualities {
				sum += q
			}
			avgQuality = sum / float64(len(qualities))
		}
		
		platforms = append(platforms, PlatformInfo{
			Name:    platform,
			Type:    "mixed",
			Count:   count,
			Quality: avgQuality,
		})
	}
	
	// 按数量排序
	sort.Slice(platforms, func(i, j int) bool {
		return platforms[i].Count > platforms[j].Count
	})
	
	return platforms
}

// FormatResult 格式化结果输出
func (t *TopicSearchService) FormatResult(result *TopicSearchResult, format string) (string, error) {
	// 设置格式化配置
	config := formatter.DefaultConfig()
	config.Format = formatter.OutputFormat(format)
	config.IncludeMetadata = true
	config.MaxSummaryLength = 150
	
	t.formatterFactory.UpdateConfig(config)
	
	// 创建格式化器
	fmt, err := t.formatterFactory.CreateFormatter()
	if err != nil {
		return "", err
	}
	
	// 格式化混合结果
	return fmt.FormatMixed(result.Articles, result.Repositories)
}

// 辅助函数

// getLanguageParam 获取语言参数
func getLanguageParam(language string) string {
	if language == "" {
		return "javascript" // 默认JavaScript
	}
	return language
}