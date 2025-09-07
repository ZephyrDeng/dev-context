package tools

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/cache"
	"github.com/ZephyrDeng/dev-context/internal/collector"
	"github.com/ZephyrDeng/dev-context/internal/formatter"
	"github.com/ZephyrDeng/dev-context/internal/models"
	"github.com/ZephyrDeng/dev-context/internal/processor"
)

// TrendingReposParams 热门仓库参数
type TrendingReposParams struct {
	// Language 编程语言过滤 (可选: javascript, typescript, python, etc.)
	Language string `json:"language,omitempty"`

	// TimeRange 时间范围 (daily, weekly, monthly)
	TimeRange string `json:"timeRange,omitempty"`

	// MinStars 最小星标数 (默认10)
	MinStars int `json:"minStars,omitempty"`

	// MaxResults 最大返回结果数 (默认30，最大100)
	MaxResults int `json:"maxResults,omitempty"`

	// Category 仓库分类 (可选: framework, library, tool, example)
	Category string `json:"category,omitempty"`

	// IncludeForks 是否包含Fork仓库 (默认false)
	IncludeForks bool `json:"includeForks,omitempty"`

	// SortBy 排序方式 (stars, forks, updated, trending)
	SortBy string `json:"sortBy,omitempty"`

	// Format 输出格式 (json, markdown, text)
	Format string `json:"format,omitempty"`

	// IncludeDescription 是否包含详细描述 (默认true)
	IncludeDescription bool `json:"includeDescription,omitempty"`

	// FrontendOnly 是否只返回前端相关仓库 (默认true)
	FrontendOnly bool `json:"frontendOnly,omitempty"`
}

// TrendingReposResult 热门仓库结果
type TrendingReposResult struct {
	Repositories []models.Repository `json:"repositories"`
	Summary      RepoSummary         `json:"summary"`
	TimeRange    string              `json:"timeRange"`
	Language     string              `json:"language,omitempty"`
	TotalCount   int                 `json:"totalCount"`
	FilterCount  int                 `json:"filterCount"`
	UpdatedAt    time.Time           `json:"updatedAt"`
	Sources      []RepoSource        `json:"sources"`
}

// RepoSummary 仓库摘要
type RepoSummary struct {
	TopLanguages     []LanguageInfo `json:"topLanguages"`
	CategoryStats    map[string]int `json:"categoryStats"`
	StarDistribution map[string]int `json:"starDistribution"`
	ActivityLevel    map[string]int `json:"activityLevel"`
	TrendingTopics   []string       `json:"trendingTopics"`
	AverageStars     float64        `json:"averageStars"`
	TotalStars       int            `json:"totalStars"`
}

// LanguageInfo 编程语言信息
type LanguageInfo struct {
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	TotalStars int     `json:"totalStars"`
}

// RepoSource 仓库来源信息
type RepoSource struct {
	Name     string    `json:"name"`
	Count    int       `json:"count"`
	Quality  float64   `json:"quality"`
	LastSync time.Time `json:"lastSync"`
}

// TrendingReposService 热门仓库服务
type TrendingReposService struct {
	cacheManager     *cache.CacheManager
	collectorMgr     *collector.CollectorManager
	processor        *processor.Processor
	formatterFactory *formatter.FormatterFactory
	mu               sync.RWMutex
}

// NewTrendingReposService 创建热门仓库服务
func NewTrendingReposService(
	cacheManager *cache.CacheManager,
	collectorMgr *collector.CollectorManager,
	processor *processor.Processor,
	formatterFactory *formatter.FormatterFactory,
) *TrendingReposService {
	return &TrendingReposService{
		cacheManager:     cacheManager,
		collectorMgr:     collectorMgr,
		processor:        processor,
		formatterFactory: formatterFactory,
	}
}

// GetTrendingRepositories 获取GitHub热门前端仓库
func (t *TrendingReposService) GetTrendingRepositories(ctx context.Context, params TrendingReposParams) (*TrendingReposResult, error) {
	// 1. 参数验证和默认值设置
	if err := t.validateParams(&params); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}

	// 2. 生成缓存键
	cacheKey := t.generateCacheKey(params)

	// 3. 检查缓存
	if cached, found := t.cacheManager.Get(cacheKey); found {
		if result, ok := cached.(*TrendingReposResult); ok {
			log.Printf("从缓存返回热门仓库，语言: %s，时间: %s", params.Language, params.TimeRange)
			return result, nil
		}
	}

	// 4. 并发收集多个源的数据
	repositories, err := t.collectTrendingRepos(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("收集热门仓库失败: %w", err)
	}

	// 5. 处理和过滤数据
	filteredRepos, err := t.processAndFilterRepos(repositories, params)
	if err != nil {
		return nil, fmt.Errorf("处理仓库数据失败: %w", err)
	}

	// 6. 构建结果
	result := &TrendingReposResult{
		Repositories: filteredRepos,
		TimeRange:    params.TimeRange,
		Language:     params.Language,
		TotalCount:   len(filteredRepos),
		FilterCount:  len(filteredRepos),
		UpdatedAt:    time.Now(),
		Summary:      t.generateRepoSummary(filteredRepos),
		Sources:      t.calculateRepoSources(filteredRepos),
	}

	// 7. 缓存结果 (缓存15分钟，热门仓库变化较快)
	t.cacheManager.SetWithTTL(cacheKey, result, 15*time.Minute)

	log.Printf("成功获取热门仓库 %d 个，语言: %s，时间范围: %s",
		len(filteredRepos), params.Language, params.TimeRange)

	return result, nil
}

// validateParams 验证参数并设置默认值
func (t *TrendingReposService) validateParams(params *TrendingReposParams) error {
	// 设置默认值
	if params.TimeRange == "" {
		params.TimeRange = "weekly"
	}
	if params.MinStars == 0 {
		params.MinStars = 5 // 设置为5，过滤掉低质量仓库
	}
	if params.MaxResults == 0 {
		params.MaxResults = 30
	}
	if params.SortBy == "" {
		params.SortBy = "trending"
	}
	if params.Format == "" {
		params.Format = "json"
	}
	params.IncludeDescription = true // 默认包含描述
	// 不默认启用FrontendOnly，让用户明确指定

	// 验证范围
	if params.MinStars < 0 {
		return fmt.Errorf("minStars 不能为负数")
	}
	if params.MaxResults < 1 || params.MaxResults > 100 {
		return fmt.Errorf("maxResults 必须在 1-100 之间")
	}

	// 验证枚举值
	validTimeRanges := []string{"daily", "weekly", "monthly"}
	if !contains(validTimeRanges, params.TimeRange) {
		return fmt.Errorf("timeRange 必须是: %v 中的一个", validTimeRanges)
	}

	validSortBy := []string{"stars", "forks", "updated", "trending"}
	if !contains(validSortBy, params.SortBy) {
		return fmt.Errorf("sortBy 必须是: %v 中的一个", validSortBy)
	}

	validFormats := []string{"json", "markdown", "text"}
	if !contains(validFormats, params.Format) {
		return fmt.Errorf("format 必须是: %v 中的一个", validFormats)
	}

	return nil
}

// generateCacheKey 生成缓存键
func (t *TrendingReposService) generateCacheKey(params TrendingReposParams) string {
	return fmt.Sprintf("trending_repos:%s:%s:%d:%d:%s:%t:%t",
		params.Language,
		params.TimeRange,
		params.MinStars,
		params.MaxResults,
		params.Category,
		params.IncludeForks,
		params.FrontendOnly,
	)
}

// collectTrendingRepos 收集热门仓库数据
func (t *TrendingReposService) collectTrendingRepos(ctx context.Context, params TrendingReposParams) ([]models.Repository, error) {
	// 获取数据源配置
	configs := t.getTrendingConfigs(params)

	var allRepos []models.Repository
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 并发收集各个数据源
	for source, config := range configs {
		wg.Add(1)
		go func(sourceName string, cfg collector.CollectConfig) {
			defer wg.Done()

			repos, err := t.collectFromSource(ctx, sourceName, cfg, params)
			if err != nil {
				log.Printf("收集 %s 失败: %v", sourceName, err)
				return
			}

			mu.Lock()
			allRepos = append(allRepos, repos...)
			mu.Unlock()
		}(source, config)
	}

	wg.Wait()

	// 去重
	uniqueRepos := t.deduplicateRepos(allRepos)

	return uniqueRepos, nil
}

// getTrendingConfigs 获取热门仓库数据源配置
func (t *TrendingReposService) getTrendingConfigs(params TrendingReposParams) map[string]collector.CollectConfig {
	configs := make(map[string]collector.CollectConfig)

	// GitHub Trending API
	var githubURL string
	if params.Language != "" {
		githubURL = fmt.Sprintf("https://api.github.com/search/repositories?q=language:%s+created:>%s&sort=stars&order=desc&per_page=20",
			params.Language, t.getDateForTimeRange(params.TimeRange))
	} else {
		// 使用JavaScript作为默认前端语言
		githubURL = fmt.Sprintf("https://api.github.com/search/repositories?q=language:javascript+created:>%s&sort=stars&order=desc&per_page=20",
			t.getDateForTimeRange(params.TimeRange))
	}

	configs["github_trending"] = collector.CollectConfig{
		URL: githubURL,
		Headers: map[string]string{
			"Accept":     "application/vnd.github.v3+json",
			"User-Agent": "FrontendNews-MCP/1.0",
		},
		Timeout: 15 * time.Second,
	}

	// GitHub Topics API (获取特定主题的仓库)
	frontendTopics := []string{"frontend", "react", "vue", "angular", "javascript", "typescript"}
	for _, topic := range frontendTopics {
		if params.Category != "" && topic != params.Category {
			continue
		}

		// 限制topics数量避免过多并发请求
		if len(configs) >= 4 {
			break
		}

		configs[fmt.Sprintf("github_topic_%s", topic)] = collector.CollectConfig{
			URL: fmt.Sprintf("https://api.github.com/search/repositories?q=topic:%s+created:>%s&sort=stars&order=desc&per_page=15",
				topic, t.getDateForTimeRange(params.TimeRange)),
			Headers: map[string]string{
				"Accept":     "application/vnd.github.v3+json",
				"User-Agent": "FrontendNews-MCP/1.0",
			},
			Timeout: 15 * time.Second,
		}
	}

	return configs
}

// getDateForTimeRange 根据时间范围获取日期字符串
func (t *TrendingReposService) getDateForTimeRange(timeRange string) string {
	now := time.Now()
	var since time.Time

	switch timeRange {
	case "daily":
		since = now.AddDate(0, 0, -1)
	case "weekly":
		since = now.AddDate(0, 0, -7)
	case "monthly":
		since = now.AddDate(0, -1, 0)
	default:
		since = now.AddDate(0, 0, -7)
	}

	return since.Format("2006-01-02")
}

// collectFromSource 从单个数据源收集仓库
func (t *TrendingReposService) collectFromSource(ctx context.Context, sourceName string, config collector.CollectConfig, params TrendingReposParams) ([]models.Repository, error) {
	// 使用collector进行API调用
	resultList := (*t.collectorMgr).CollectAll(ctx, []collector.CollectConfig{config})

	if len(resultList) == 0 {
		return nil, fmt.Errorf("收集 %s 数据失败: 没有返回结果", sourceName)
	}

	result := resultList[0]
	if result.Error != nil {
		return nil, fmt.Errorf("收集 %s 数据失败: %w", sourceName, result.Error)
	}

	var repositories []models.Repository

	// 将collector.Article转换为models.Repository
	for _, article := range result.Articles {
		repo := t.convertArticleToRepository(article)
		repositories = append(repositories, repo)
	}

	return repositories, nil
}

// convertArticleToRepository 转换Article到Repository（用于GitHub仓库数据）
func (t *TrendingReposService) convertArticleToRepository(article collector.Article) models.Repository {
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
		Metadata:    make(map[string]interface{}),
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

	// 将 collector.Article 的 metadata 复制到 Repository
	for k, v := range article.Metadata {
		repo.Metadata[k] = v
	}

	// 重新计算trend score
	repo.CalculateTrendScore()

	return repo
}

// handleGitHubTrendingAPI 处理GitHub Trending API
func (t *TrendingReposService) handleGitHubTrendingAPI(ctx context.Context, config collector.CollectConfig, params TrendingReposParams) []models.Repository {
	// TODO: 实际实现需要调用collector并解析GitHub API响应
	log.Printf("处理GitHub Trending API")

	// 这里应该:
	// 1. 调用collector获取数据
	// 2. 解析JSON响应为Repository对象
	// 3. 设置trend score等字段

	return []models.Repository{} // 暂时返回空数组
}

// handleGitHubTopicAPI 处理GitHub Topic API
func (t *TrendingReposService) handleGitHubTopicAPI(ctx context.Context, config collector.CollectConfig, params TrendingReposParams) []models.Repository {
	// TODO: 实际实现需要调用collector并解析GitHub API响应
	log.Printf("处理GitHub Topic API")

	return []models.Repository{} // 暂时返回空数组
}

// processAndFilterRepos 处理和过滤仓库数据
func (t *TrendingReposService) processAndFilterRepos(repositories []models.Repository, params TrendingReposParams) ([]models.Repository, error) {
	var filtered []models.Repository

	for _, repo := range repositories {
		// 星标数过滤
		if repo.Stars < params.MinStars {
			continue
		}

		// Fork过滤
		if !params.IncludeForks && t.isForkRepo(repo) {
			continue
		}

		// 前端相关过滤
		if params.FrontendOnly && !t.isFrontendRelated(repo) {
			continue
		}

		// 分类过滤
		if params.Category != "" && !t.matchesCategory(repo, params.Category) {
			continue
		}

		// 计算趋势分数
		repo.CalculateTrendScore()

		// 更新仓库活跃度信息
		t.updateRepoActivityInfo(&repo)

		filtered = append(filtered, repo)
	}

	// 排序
	t.sortRepositories(filtered, params.SortBy)

	// 限制数量
	if len(filtered) > params.MaxResults {
		filtered = filtered[:params.MaxResults]
	}

	return filtered, nil
}

// isForkRepo 判断是否为Fork仓库
func (t *TrendingReposService) isForkRepo(repo models.Repository) bool {
	// 检查metadata中的is_fork字段
	if repo.Metadata != nil {
		if isForkInterface, exists := repo.Metadata["is_fork"]; exists {
			if isForkStr, ok := isForkInterface.(string); ok {
				if isFork, err := strconv.ParseBool(isForkStr); err == nil {
					return isFork
				}
			}
		}
	}
	return false
}

// isFrontendRelated 判断是否与前端开发相关
func (t *TrendingReposService) isFrontendRelated(repo models.Repository) bool {
	frontendKeywords := []string{
		"javascript", "typescript", "react", "vue", "angular", "frontend",
		"css", "html", "sass", "scss", "webpack", "vite", "next.js", "nuxt",
		"ui", "component", "framework", "library", "web", "browser",
	}

	// 检查编程语言
	frontendLangs := []string{"JavaScript", "TypeScript", "CSS", "HTML", "Vue", "Sass", "Less"}
	for _, lang := range frontendLangs {
		if strings.EqualFold(repo.Language, lang) {
			return true
		}
	}

	// 检查仓库名称和描述
	content := strings.ToLower(repo.Name + " " + repo.Description + " " + repo.FullName)
	for _, keyword := range frontendKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

// matchesCategory 检查是否匹配指定分类
func (t *TrendingReposService) matchesCategory(repo models.Repository, category string) bool {
	categoryKeywords := map[string][]string{
		"framework": {"framework", "react", "vue", "angular", "next.js", "nuxt", "svelte"},
		"library":   {"library", "util", "helper", "component", "ui"},
		"tool":      {"tool", "cli", "build", "webpack", "vite", "rollup", "babel"},
		"example":   {"example", "demo", "sample", "template", "boilerplate"},
	}

	keywords, exists := categoryKeywords[category]
	if !exists {
		return true // 未知分类，不过滤
	}

	content := strings.ToLower(repo.Name + " " + repo.Description)
	for _, keyword := range keywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

// updateRepoActivityInfo 更新仓库活跃度信息
func (t *TrendingReposService) updateRepoActivityInfo(repo *models.Repository) {
	// 基于最近更新时间、星标数和Fork数计算活跃度
	// 这里可以根据需要添加更复杂的活跃度计算逻辑
}

// sortRepositories 排序仓库
func (t *TrendingReposService) sortRepositories(repositories []models.Repository, sortBy string) {
	sort.Slice(repositories, func(i, j int) bool {
		switch sortBy {
		case "stars":
			return repositories[i].Stars > repositories[j].Stars
		case "forks":
			return repositories[i].Forks > repositories[j].Forks
		case "updated":
			return repositories[i].UpdatedAt.After(repositories[j].UpdatedAt)
		case "trending":
			return repositories[i].TrendScore > repositories[j].TrendScore
		default:
			return repositories[i].TrendScore > repositories[j].TrendScore
		}
	})
}

// deduplicateRepos 仓库去重
func (t *TrendingReposService) deduplicateRepos(repositories []models.Repository) []models.Repository {
	seen := make(map[string]bool)
	var unique []models.Repository

	for _, repo := range repositories {
		// 使用FullName作为去重标识
		key := strings.ToLower(repo.FullName)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, repo)
		}
	}

	return unique
}

// generateRepoSummary 生成仓库摘要统计
func (t *TrendingReposService) generateRepoSummary(repositories []models.Repository) RepoSummary {
	summary := RepoSummary{
		CategoryStats:    make(map[string]int),
		StarDistribution: make(map[string]int),
		ActivityLevel:    make(map[string]int),
	}

	// 统计编程语言
	languageCount := make(map[string]int)
	languageStars := make(map[string]int)
	totalStars := 0

	for _, repo := range repositories {
		// 统计语言
		if repo.Language != "" {
			languageCount[repo.Language]++
			languageStars[repo.Language] += repo.Stars
		}

		// 统计星标分布
		if repo.Stars >= 10000 {
			summary.StarDistribution["10k+"]++
		} else if repo.Stars >= 1000 {
			summary.StarDistribution["1k-10k"]++
		} else if repo.Stars >= 100 {
			summary.StarDistribution["100-1k"]++
		} else {
			summary.StarDistribution["<100"]++
		}

		// 统计活跃度
		activityLevel := repo.GetActivityLevel()
		summary.ActivityLevel[activityLevel]++

		totalStars += repo.Stars
	}

	// 生成语言统计
	for lang, count := range languageCount {
		percentage := float64(count) / float64(len(repositories)) * 100
		summary.TopLanguages = append(summary.TopLanguages, LanguageInfo{
			Name:       lang,
			Count:      count,
			Percentage: percentage,
			TotalStars: languageStars[lang],
		})
	}

	// 按数量排序语言
	sort.Slice(summary.TopLanguages, func(i, j int) bool {
		return summary.TopLanguages[i].Count > summary.TopLanguages[j].Count
	})

	// 限制前10种语言
	if len(summary.TopLanguages) > 10 {
		summary.TopLanguages = summary.TopLanguages[:10]
	}

	// 计算平均星标数
	if len(repositories) > 0 {
		summary.AverageStars = float64(totalStars) / float64(len(repositories))
	}
	summary.TotalStars = totalStars

	// 提取热门主题
	summary.TrendingTopics = t.extractTrendingTopics(repositories)

	return summary
}

// extractTrendingTopics 提取热门主题
func (t *TrendingReposService) extractTrendingTopics(repositories []models.Repository) []string {
	topicCount := make(map[string]int)

	// 从仓库名称和描述中提取关键词
	for _, repo := range repositories {
		words := strings.Fields(strings.ToLower(repo.Name + " " + repo.Description))
		for _, word := range words {
			// 过滤常见词汇，只保留有意义的技术词汇
			if len(word) > 2 && t.isTechKeyword(word) {
				topicCount[word]++
			}
		}
	}

	// 转换为排序切片
	type kv struct {
		Key   string
		Value int
	}
	var kvs []kv
	for k, v := range topicCount {
		if v >= 2 { // 至少出现2次
			kvs = append(kvs, kv{k, v})
		}
	}

	// 按频率排序
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Value > kvs[j].Value
	})

	// 返回前10个热门主题
	var topics []string
	for i := 0; i < len(kvs) && i < 10; i++ {
		topics = append(topics, kvs[i].Key)
	}

	return topics
}

// isTechKeyword 判断是否为技术关键词
func (t *TrendingReposService) isTechKeyword(word string) bool {
	techKeywords := []string{
		"react", "vue", "angular", "javascript", "typescript", "css", "html",
		"node", "express", "webpack", "vite", "babel", "eslint", "prettier",
		"tailwind", "bootstrap", "sass", "less", "redux", "mobx", "graphql",
		"rest", "api", "ui", "component", "library", "framework", "tool",
	}

	for _, keyword := range techKeywords {
		if strings.Contains(word, keyword) {
			return true
		}
	}

	return false
}

// calculateRepoSources 计算仓库来源信息
func (t *TrendingReposService) calculateRepoSources(repositories []models.Repository) []RepoSource {
	sourceCount := make(map[string]int)
	sourceQuality := make(map[string][]float64)

	for _, repo := range repositories {
		// 根据URL判断来源
		source := "GitHub" // 默认GitHub
		if strings.Contains(repo.URL, "gitlab") {
			source = "GitLab"
		} else if strings.Contains(repo.URL, "bitbucket") {
			source = "Bitbucket"
		}

		sourceCount[source]++
		sourceQuality[source] = append(sourceQuality[source], repo.TrendScore)
	}

	var sources []RepoSource
	for source, count := range sourceCount {
		avgQuality := 0.0
		if qualities := sourceQuality[source]; len(qualities) > 0 {
			sum := 0.0
			for _, q := range qualities {
				sum += q
			}
			avgQuality = sum / float64(len(qualities))
		}

		sources = append(sources, RepoSource{
			Name:     source,
			Count:    count,
			Quality:  avgQuality,
			LastSync: time.Now(),
		})
	}

	// 按数量排序
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Count > sources[j].Count
	})

	return sources
}

// FormatResult 格式化结果输出
func (t *TrendingReposService) FormatResult(result *TrendingReposResult, format string) (string, error) {
	// 设置格式化配置
	config := formatter.DefaultConfig()
	config.Format = formatter.OutputFormat(format)
	config.IncludeMetadata = true

	t.formatterFactory.UpdateConfig(config)

	// 创建格式化器
	fmt, err := t.formatterFactory.CreateFormatter()
	if err != nil {
		return "", err
	}

	// 格式化仓库
	return fmt.FormatRepositories(result.Repositories)
}
