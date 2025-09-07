package tools

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
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
		params.MinScore = 0.15  // 调整为0.15，配合新的评分系统
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
	
	// 获取搜索配置
	searchConfigs := t.getSearchConfigs(params)
	
	// 并发搜索各个平台，但限制并发数避免资源争抢
	semaphore := make(chan struct{}, 2) // 最大并发数为2
	
	// 为每个平台启动goroutine，使用信号量控制并发
	for platform, config := range searchConfigs {
		wg.Add(1)
		go func(platformName string, cfg collector.CollectConfig) {
			defer wg.Done()
			
			// 获取信号量
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			
			// 为每个平台设置独立的超时上下文
			platformCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			
			t.searchSinglePlatform(platformCtx, platformName, cfg, params, results)
		}(platform, config)
	}
	
	wg.Wait()
	
	log.Printf("搜索完成，结果统计 - 文章: %d, 仓库: %d, 讨论: %d", 
		len(results.Articles), len(results.Repositories), len(results.Discussions))
	
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
				Timeout: 25 * time.Second, // 增加超时时间
			}
		}
	}
	
	// Dev.to搜索配置
	if params.Platform == "" || params.Platform == "dev.to" {
		if params.SearchType == "all" || params.SearchType == "articles" {
			// Dev.to API不支持query参数，所以使用相关标签
			tag := params.Query
			if tag == "react" {
				tag = "react"
			} else if tag == "vue" {
				tag = "vue"
			} else if tag == "angular" {
				tag = "angular"
			} else {
				tag = "javascript" // 默认使用javascript标签
			}
			
			configs["devto"] = collector.CollectConfig{
				URL:        fmt.Sprintf("https://dev.to/api/articles?tag=%s&per_page=30", tag),
				Headers: map[string]string{
					"User-Agent": "FrontendNews-MCP/1.0",
				},
				Timeout: 25 * time.Second, // 增加超时时间
			}
		}
	}
	
	return configs
}

// searchSinglePlatform 搜索单个平台
func (t *TopicSearchService) searchSinglePlatform(ctx context.Context, platform string, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	// 使用defer处理panic，确保其他平台不受影响
	defer func() {
		if r := recover(); r != nil {
			log.Printf("平台 %s 搜索发生panic: %v", platform, r)
		}
	}()
	
	// 根据平台类型处理不同的响应
	switch {
	case strings.Contains(platform, "github_repos"):
		t.handleGitHubRepos(ctx, config, params, results)
	case strings.Contains(platform, "devto"):
		t.handleDevTo(ctx, config, params, results)
	default:
		log.Printf("平台 %s 暂未实现，跳过", platform)
	}
}

// handleGitHubRepos 处理GitHub仓库搜索
func (t *TopicSearchService) handleGitHubRepos(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	// 使用collector进行API调用
	resultList := (*t.collectorMgr).CollectAll(ctx, []collector.CollectConfig{config})
	if len(resultList) == 0 {
		log.Printf("GitHub仓库搜索失败: 没有返回结果")
		return
	}
	
	result := resultList[0]
	if result.Error != nil {
		log.Printf("GitHub仓库搜索失败: %v", result.Error)
		return
	}
	
	// 将Articles转换为Repositories（GitHub API返回的是仓库信息）
	for _, collectorArticle := range result.Articles {
		repo := convertArticleToRepository(collectorArticle)
		results.addRepository(repo)
	}
	
	log.Printf("GitHub仓库搜索完成，找到 %d 个仓库", len(result.Articles))
}


// handleDevTo 处理Dev.to搜索
func (t *TopicSearchService) handleDevTo(ctx context.Context, config collector.CollectConfig, params TopicSearchParams, results *multiPlatformResults) {
	// 使用collector进行API调用
	resultList := (*t.collectorMgr).CollectAll(ctx, []collector.CollectConfig{config})
	
	if len(resultList) == 0 {
		log.Printf("Dev.to搜索失败: 没有返回结果")
		return
	}
	
	result := resultList[0]
	if result.Error != nil {
		log.Printf("Dev.to搜索失败: %v", result.Error)
		return
	}
	
	// 将collector.Article转换为models.Article并添加到结果中
	for _, collectorArticle := range result.Articles {
		modelArticle := convertCollectorToModelArticle(collectorArticle)
		results.addArticle(modelArticle)
	}
	
	log.Printf("Dev.to搜索完成，找到 %d 篇文章", len(result.Articles))
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
		article := &result.Articles[i]
		
		// 基础文本相关性
		textScore := t.calculateTextRelevance(
			article.Title+" "+article.Summary,
			keywords,
		)
		
		// 标签相关性加成
		tagScore := t.calculateTagRelevance(article.Tags, keywords)
		
		// 技术相关性加成（针对前端技术）
		techScore := t.calculateTechRelevance(article, params.Query)
		
		// 综合评分
		finalScore := (textScore * 0.5) + (tagScore * 0.3) + (techScore * 0.2)
		
		// 确保分数在合理范围
		if finalScore > 1.0 {
			finalScore = 1.0
		} else if finalScore < 0.1 {
			finalScore = 0.1
		}
		
		article.Relevance = finalScore
	}
	
	// 计算仓库相关性
	for i := range result.Repositories {
		repo := &result.Repositories[i]
		
		textScore := t.calculateTextRelevance(
			repo.Name+" "+repo.Description,
			keywords,
		)
		
		// 仓库语言匹配加成
		langScore := t.calculateLanguageRelevance(repo.Language, params.Query)
		
		// 星标数影响（受欢迎程度）
		starScore := t.calculatePopularityScore(repo.Stars)
		
		// 综合评分
		finalScore := (textScore * 0.6) + (langScore * 0.3) + (starScore * 0.1)
		
		if finalScore > 1.0 {
			finalScore = 1.0
		} else if finalScore < 0.1 {
			finalScore = 0.1
		}
		
		repo.TrendScore = finalScore
	}
	
	// 计算讨论相关性
	for i := range result.Discussions {
		discussion := &result.Discussions[i]
		
		discussion.Relevance = t.calculateTextRelevance(
			discussion.Title+" "+discussion.Content,
			keywords,
		)
	}
}

// calculateTagRelevance 计算标签相关性
func (t *TopicSearchService) calculateTagRelevance(tags []string, keywords []string) float64 {
	if len(tags) == 0 || len(keywords) == 0 {
		return 0.0
	}
	
	score := 0.0
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		for _, keyword := range keywords {
			keywordLower := strings.ToLower(keyword)
			
			// 完全匹配
			if tagLower == keywordLower {
				score += 1.0
			} else if strings.Contains(tagLower, keywordLower) {
				score += 0.7
			} else if strings.Contains(keywordLower, tagLower) {
				score += 0.5
			}
		}
	}
	
	// 标准化分数
	normalizedScore := score / float64(len(keywords))
	if normalizedScore > 1.0 {
		normalizedScore = 1.0
	}
	
	return normalizedScore
}

// calculateTechRelevance 计算技术相关性
func (t *TopicSearchService) calculateTechRelevance(article *models.Article, query string) float64 {
	queryLower := strings.ToLower(query)
	
	// 前端技术关键词权重映射
	techKeywords := map[string]float64{
		"react":      1.0,
		"vue":        1.0,
		"angular":    1.0,
		"javascript": 0.9,
		"typescript": 0.9,
		"nodejs":     0.8,
		"css":        0.7,
		"html":       0.7,
		"webpack":    0.8,
		"vite":       0.8,
		"nextjs":     0.9,
		"nuxt":       0.9,
		"svelte":     0.8,
		"frontend":   0.7,
	}
	
	maxScore := 0.0
	
	// 检查文章标题和内容中的技术关键词
	content := strings.ToLower(article.Title + " " + article.Summary)
	for tech, weight := range techKeywords {
		if strings.Contains(queryLower, tech) && strings.Contains(content, tech) {
			if weight > maxScore {
				maxScore = weight
			}
		}
	}
	
	return maxScore
}

// calculateLanguageRelevance 计算编程语言相关性
func (t *TopicSearchService) calculateLanguageRelevance(language, query string) float64 {
	if language == "" {
		return 0.0
	}
	
	languageLower := strings.ToLower(language)
	queryLower := strings.ToLower(query)
	
	// 直接语言匹配
	if strings.Contains(queryLower, languageLower) {
		return 1.0
	}
	
	// 语言别名映射
	aliases := map[string][]string{
		"javascript": {"js", "node", "nodejs"},
		"typescript": {"ts"},
		"python":     {"py"},
		"go":         {"golang"},
	}
	
	for lang, aliasList := range aliases {
		if languageLower == lang {
			for _, alias := range aliasList {
				if strings.Contains(queryLower, alias) {
					return 0.9
				}
			}
		}
	}
	
	return 0.0
}

// calculatePopularityScore 根据星标数计算受欢迎程度分数
func (t *TopicSearchService) calculatePopularityScore(stars int) float64 {
	// 使用对数函数避免高星标项目过度影响
	if stars <= 0 {
		return 0.0
	}
	
	// 使用对数缩放，1000星为基准点0.5分
	score := math.Log10(float64(stars)) / math.Log10(1000.0) * 0.5
	
	if score > 1.0 {
		score = 1.0
	} else if score < 0.0 {
		score = 0.0
	}
	
	return score
}

// calculateTextRelevance 计算文本相关性
func (t *TopicSearchService) calculateTextRelevance(text string, keywords []string) float64 {
	text = strings.ToLower(text)
	
	if len(keywords) == 0 {
		return 0.5 // 没有关键词时给予中等分数
	}
	
	score := 0.0
	matchCount := 0
	
	for _, keyword := range keywords {
		keyword = strings.ToLower(keyword)
		
		// 完全匹配权重更高
		if strings.Contains(text, keyword) {
			// 根据匹配位置给不同权重
			if strings.HasPrefix(text, keyword) {
				score += 1.0 // 标题开头匹配
			} else if strings.Contains(text[:min(len(text), 100)], keyword) {
				score += 0.8 // 前100字符内匹配
			} else {
				score += 0.6 // 其他位置匹配
			}
			matchCount++
		}
		
		// 部分匹配检查（如果关键词长度>3）
		if len(keyword) > 3 {
			// 检查是否包含关键词的子字符串
			if containsPartialMatch(text, keyword) {
				score += 0.3
			}
		}
	}
	
	// 计算最终相关性分数
	baseScore := score / float64(len(keywords))
	
	// 匹配比例奖励
	matchRatio := float64(matchCount) / float64(len(keywords))
	bonusScore := matchRatio * 0.2
	
	finalScore := baseScore + bonusScore
	
	// 确保分数在合理范围内
	if finalScore > 1.0 {
		finalScore = 1.0
	} else if finalScore < 0.1 {
		finalScore = 0.1
	}
	
	return finalScore
}

// containsPartialMatch 检查部分匹配
func containsPartialMatch(text, keyword string) bool {
	// 检查关键词的前后缀是否在文本中
	if len(keyword) <= 4 {
		return false
	}
	
	prefix := keyword[:len(keyword)/2]
	suffix := keyword[len(keyword)/2:]
	
	return strings.Contains(text, prefix) || strings.Contains(text, suffix)
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// convertCollectorToModelArticle 转换collector.Article到models.Article
func convertCollectorToModelArticle(collectorArticle collector.Article) models.Article {
	modelArticle := models.Article{
		ID:          collectorArticle.ID,
		Title:       collectorArticle.Title,
		URL:         collectorArticle.URL,
		Source:      collectorArticle.Source,
		SourceType:  collectorArticle.SourceType,
		PublishedAt: collectorArticle.PublishedAt,
		Summary:     collectorArticle.Summary,
		Content:     collectorArticle.Content,
		Tags:        collectorArticle.Tags,
		Quality:     0.5,
		Relevance:   0.5,
		Metadata:    convertStringMapToInterface(collectorArticle.Metadata),
	}
	
	// 将Author和Language信息存储到Metadata中
	if collectorArticle.Author != "" {
		modelArticle.SetMetadata("author", collectorArticle.Author)
	}
	if collectorArticle.Language != "" {
		modelArticle.SetMetadata("language", collectorArticle.Language)
	}
	
	// 更新质量分数
	modelArticle.UpdateQuality()
	
	return modelArticle
}

// convertArticleToRepository 转换Article到Repository（用于GitHub仓库数据）
func convertArticleToRepository(article collector.Article) models.Repository {
	repo := models.Repository{
		ID:          article.ID,
		Name:        article.Title,
		FullName:    article.Title,
		Description: article.Summary,
		URL:         article.URL,
		Language:    article.Language,
		Stars:       0,
		Forks:       0,
		TrendScore:  0.5,
		UpdatedAt:   article.PublishedAt,
	}
	
	// 从metadata中获取GitHub特定信息
	if starsStr, exists := article.Metadata["stars"]; exists {
		if stars, err := parseIntFromString(starsStr); err == nil {
			repo.Stars = stars
		}
	}
	if forksStr, exists := article.Metadata["forks"]; exists {
		if forks, err := parseIntFromString(forksStr); err == nil {
			repo.Forks = forks
		}
	}
	
	// 重新计算trend score
	repo.CalculateTrendScore()
	
	return repo
}

// convertStringMapToInterface 转换map[string]string到map[string]interface{}
func convertStringMapToInterface(stringMap map[string]string) map[string]interface{} {
	interfaceMap := make(map[string]interface{})
	for k, v := range stringMap {
		interfaceMap[k] = v
	}
	return interfaceMap
}

// parseIntFromString 从字符串解析整数
func parseIntFromString(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	// 使用strconv.Atoi进行实际的字符串到整数转换
	return strconv.Atoi(s)
}

// contains 检查slice是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}