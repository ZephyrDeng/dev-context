package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GitHub API 响应结构
type GitHubRepo struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	Description    string `json:"description"`
	HTMLURL        string `json:"html_url"`
	Language       string `json:"language"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	PushedAt       string `json:"pushed_at"`
	StargazersCount int   `json:"stargazers_count"`
	ForksCount     int    `json:"forks_count"`
	WatchersCount  int    `json:"watchers_count"`
	OpenIssuesCount int   `json:"open_issues_count"`
	Fork           bool   `json:"fork"`
	Owner          struct {
		Login string `json:"login"`
	} `json:"owner"`
	Topics []string `json:"topics"`
}

type GitHubIssue struct {
	ID        int    `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// Dev.to API 响应结构
type DevToArticle struct {
	ID                 int      `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	BodyMarkdown       string   `json:"body_markdown"`
	URL                string   `json:"url"`
	PublishedAt        string   `json:"published_at"`
	CreatedAt          string   `json:"created_at"`
	TagList            []string `json:"tag_list"`
	User               DevToUser `json:"user"`
	Organization       *DevToOrg `json:"organization"`
	ReadingTimeMinutes int      `json:"reading_time_minutes"`
}

type DevToUser struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

type DevToOrg struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

// APICollector API数据采集器
type APICollector struct {
	client *http.Client
}

// NewAPICollector 创建API采集器
func NewAPICollector() *APICollector {
	return &APICollector{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetSourceType 返回采集器类型
func (a *APICollector) GetSourceType() string {
	return "api"
}

// Validate 验证配置
func (a *APICollector) Validate(config CollectConfig) error {
	if config.URL == "" {
		return fmt.Errorf("URL is required")
	}
	
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	
	return nil
}

// Collect 采集API数据
func (a *APICollector) Collect(ctx context.Context, config CollectConfig) (CollectResult, error) {
	if err := a.Validate(config); err != nil {
		return CollectResult{}, fmt.Errorf("validation failed: %w", err)
	}

	// 设置超时
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// 根据URL判断API类型并采集
	if a.isGitHubAPI(config.URL) {
		return a.collectGitHubAPI(ctx, config)
	} else if a.isDevToAPI(config.URL) {
		return a.collectDevToAPI(ctx, config)
	} else {
		return a.collectGenericAPI(ctx, config)
	}
}

// isGitHubAPI 判断是否为GitHub API
func (a *APICollector) isGitHubAPI(apiURL string) bool {
	return strings.Contains(apiURL, "api.github.com")
}

// isDevToAPI 判断是否为Dev.to API
func (a *APICollector) isDevToAPI(apiURL string) bool {
	return strings.Contains(apiURL, "dev.to/api")
}

// collectGitHubAPI 采集GitHub API数据
func (a *APICollector) collectGitHubAPI(ctx context.Context, config CollectConfig) (CollectResult, error) {
	data, err := a.fetchAPI(ctx, config)
	if err != nil {
		return CollectResult{}, err
	}

	var articles []Article

	// 根据URL类型解析不同的GitHub API响应
	if strings.Contains(config.URL, "/repos") && strings.Contains(config.URL, "/issues") {
		// GitHub Issues API
		var issues []GitHubIssue
		if err := json.Unmarshal(data, &issues); err != nil {
			return CollectResult{}, fmt.Errorf("failed to parse GitHub issues: %w", err)
		}
		articles = a.convertGitHubIssues(issues, config)
	} else if strings.Contains(config.URL, "/search/repositories") || strings.Contains(config.URL, "/users/") && strings.Contains(config.URL, "/repos") {
		// GitHub Repositories API
		if strings.Contains(config.URL, "/search/repositories") {
			// 搜索结果格式
			var searchResult struct {
				Items []GitHubRepo `json:"items"`
			}
			if err := json.Unmarshal(data, &searchResult); err != nil {
				return CollectResult{}, fmt.Errorf("failed to parse GitHub search results: %w", err)
			}
			articles = a.convertGitHubRepos(searchResult.Items, config)
		} else {
			// 直接仓库列表
			var repos []GitHubRepo
			if err := json.Unmarshal(data, &repos); err != nil {
				return CollectResult{}, fmt.Errorf("failed to parse GitHub repos: %w", err)
			}
			articles = a.convertGitHubRepos(repos, config)
		}
	}

	// 限制文章数量
	if config.MaxArticles > 0 && len(articles) > config.MaxArticles {
		articles = articles[:config.MaxArticles]
	}

	return CollectResult{
		Articles: articles,
		Source:   config.URL,
	}, nil
}

// collectDevToAPI 采集Dev.to API数据
func (a *APICollector) collectDevToAPI(ctx context.Context, config CollectConfig) (CollectResult, error) {
	data, err := a.fetchAPI(ctx, config)
	if err != nil {
		return CollectResult{}, err
	}

	var devArticles []DevToArticle
	if err := json.Unmarshal(data, &devArticles); err != nil {
		return CollectResult{}, fmt.Errorf("failed to parse Dev.to articles: %w", err)
	}

	articles := a.convertDevToArticles(devArticles, config)

	// 限制文章数量
	if config.MaxArticles > 0 && len(articles) > config.MaxArticles {
		articles = articles[:config.MaxArticles]
	}

	return CollectResult{
		Articles: articles,
		Source:   config.URL,
	}, nil
}

// collectGenericAPI 采集通用API数据
func (a *APICollector) collectGenericAPI(ctx context.Context, config CollectConfig) (CollectResult, error) {
	data, err := a.fetchAPI(ctx, config)
	if err != nil {
		return CollectResult{}, err
	}

	// 创建一个通用文章表示API响应
	article := Article{
		ID:         a.generateID(config.URL),
		Title:      fmt.Sprintf("API Response from %s", config.URL),
		Content:    string(data),
		Summary:    fmt.Sprintf("API response containing %d bytes of data", len(data)),
		Author:     "API",
		URL:        config.URL,
		Source:     config.URL,
		SourceType: a.GetSourceType(),
		Language:   config.Language,
		PublishedAt: time.Now(),
		Tags:       append([]string{"api"}, config.Tags...),
		Metadata:   make(map[string]string),
	}

	if config.Metadata != nil {
		for k, v := range config.Metadata {
			article.Metadata[k] = v
		}
	}

	return CollectResult{
		Articles: []Article{article},
		Source:   config.URL,
	}, nil
}

// fetchAPI 获取API数据
func (a *APICollector) fetchAPI(ctx context.Context, config CollectConfig) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认头部
	req.Header.Set("User-Agent", "API Collector/1.0")
	req.Header.Set("Accept", "application/json")

	// 设置自定义头部
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// convertGitHubRepos 转换GitHub仓库为文章
func (a *APICollector) convertGitHubRepos(repos []GitHubRepo, config CollectConfig) []Article {
	articles := make([]Article, 0, len(repos))

	for _, repo := range repos {
		article := Article{
			ID:         strconv.Itoa(repo.ID),
			Title:      repo.FullName,
			Content:    repo.Description,
			Summary:    repo.Description,
			Author:     repo.Owner.Login,
			URL:        repo.HTMLURL,
			Source:     config.URL,
			SourceType: a.GetSourceType(),
			Language:   repo.Language,
			Tags:       append(repo.Topics, config.Tags...),
			Metadata:   make(map[string]string),
		}

		// 解析创建时间
		if createdAt, err := time.Parse(time.RFC3339, repo.CreatedAt); err == nil {
			article.PublishedAt = createdAt
		}

		// 添加元数据
		article.Metadata["github_repo"] = "true"
		article.Metadata["repo_language"] = repo.Language
		article.Metadata["created_at"] = repo.CreatedAt
		article.Metadata["updated_at"] = repo.UpdatedAt
		article.Metadata["stars"] = strconv.Itoa(repo.StargazersCount)
		article.Metadata["forks"] = strconv.Itoa(repo.ForksCount)
		article.Metadata["watchers"] = strconv.Itoa(repo.WatchersCount)
		article.Metadata["open_issues"] = strconv.Itoa(repo.OpenIssuesCount)
		article.Metadata["is_fork"] = strconv.FormatBool(repo.Fork)

		if config.Metadata != nil {
			for k, v := range config.Metadata {
				article.Metadata[k] = v
			}
		}

		articles = append(articles, article)
	}

	return articles
}

// convertGitHubIssues 转换GitHub Issue为文章
func (a *APICollector) convertGitHubIssues(issues []GitHubIssue, config CollectConfig) []Article {
	articles := make([]Article, 0, len(issues))

	for _, issue := range issues {
		tags := []string{"github-issue", issue.State}
		for _, label := range issue.Labels {
			tags = append(tags, label.Name)
		}
		tags = append(tags, config.Tags...)

		article := Article{
			ID:         strconv.Itoa(issue.ID),
			Title:      fmt.Sprintf("#%d: %s", issue.Number, issue.Title),
			Content:    issue.Body,
			Summary:    a.extractSummary(issue.Body, 200),
			Author:     issue.User.Login,
			URL:        issue.HTMLURL,
			Source:     config.URL,
			SourceType: a.GetSourceType(),
			Language:   config.Language,
			Tags:       tags,
			Metadata:   make(map[string]string),
		}

		// 解析创建时间
		if createdAt, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			article.PublishedAt = createdAt
		}

		// 添加元数据
		article.Metadata["github_issue"] = "true"
		article.Metadata["issue_number"] = strconv.Itoa(issue.Number)
		article.Metadata["issue_state"] = issue.State
		article.Metadata["created_at"] = issue.CreatedAt
		article.Metadata["updated_at"] = issue.UpdatedAt

		if config.Metadata != nil {
			for k, v := range config.Metadata {
				article.Metadata[k] = v
			}
		}

		articles = append(articles, article)
	}

	return articles
}

// convertDevToArticles 转换Dev.to文章
func (a *APICollector) convertDevToArticles(devArticles []DevToArticle, config CollectConfig) []Article {
	articles := make([]Article, 0, len(devArticles))

	for _, devArticle := range devArticles {
		author := devArticle.User.Name
		if author == "" {
			author = devArticle.User.Username
		}
		if devArticle.Organization != nil {
			author = devArticle.Organization.Name
		}

		article := Article{
			ID:         strconv.Itoa(devArticle.ID),
			Title:      devArticle.Title,
			Content:    devArticle.BodyMarkdown,
			Summary:    devArticle.Description,
			Author:     author,
			URL:        devArticle.URL,
			Source:     config.URL,
			SourceType: a.GetSourceType(),
			Language:   config.Language,
			Tags:       append(devArticle.TagList, config.Tags...),
			Metadata:   make(map[string]string),
		}

		// 解析发布时间
		if publishedAt, err := time.Parse(time.RFC3339, devArticle.PublishedAt); err == nil {
			article.PublishedAt = publishedAt
		}

		// 添加元数据
		article.Metadata["dev_to_article"] = "true"
		article.Metadata["reading_time_minutes"] = strconv.Itoa(devArticle.ReadingTimeMinutes)
		article.Metadata["published_at"] = devArticle.PublishedAt
		article.Metadata["created_at"] = devArticle.CreatedAt

		if config.Metadata != nil {
			for k, v := range config.Metadata {
				article.Metadata[k] = v
			}
		}

		articles = append(articles, article)
	}

	return articles
}

// 辅助方法
func (a *APICollector) extractSummary(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	
	// 在单词边界处截断
	if idx := strings.LastIndex(content[:maxLen], " "); idx > 0 {
		return content[:idx] + "..."
	}
	
	return content[:maxLen] + "..."
}

func (a *APICollector) generateID(content string) string {
	// 重用RSS采集器的ID生成方法
	rss := &RSSCollector{}
	return rss.generateID(content)
}