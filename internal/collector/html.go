package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HTMLSelector 定义HTML选择器配置
type HTMLSelector struct {
	Title       string `json:"title"`        // 标题选择器
	Content     string `json:"content"`      // 内容选择器
	Summary     string `json:"summary"`      // 摘要选择器
	Author      string `json:"author"`       // 作者选择器
	PublishedAt string `json:"published_at"` // 发布时间选择器
	Tags        string `json:"tags"`         // 标签选择器
	Links       string `json:"links"`        // 链接选择器
}

// HTMLCollector HTML网页采集器
type HTMLCollector struct {
	client *http.Client
}

// NewHTMLCollector 创建HTML采集器
func NewHTMLCollector() *HTMLCollector {
	return &HTMLCollector{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetSourceType 返回采集器类型
func (h *HTMLCollector) GetSourceType() string {
	return "html"
}

// Validate 验证配置
func (h *HTMLCollector) Validate(config CollectConfig) error {
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

// Collect 采集HTML页面数据
func (h *HTMLCollector) Collect(ctx context.Context, config CollectConfig) (CollectResult, error) {
	if err := h.Validate(config); err != nil {
		return CollectResult{}, fmt.Errorf("validation failed: %w", err)
	}

	// 设置超时
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// 获取HTML内容
	htmlContent, err := h.fetchHTML(ctx, config)
	if err != nil {
		return CollectResult{}, err
	}

	// 解析HTML内容
	articles, err := h.parseHTML(htmlContent, config)
	if err != nil {
		return CollectResult{}, fmt.Errorf("failed to parse HTML: %w", err)
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

// fetchHTML 获取HTML内容
func (h *HTMLCollector) fetchHTML(ctx context.Context, config CollectConfig) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认头部
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; HTMLCollector/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	// 设置自定义头部
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch HTML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// parseHTML 解析HTML内容
func (h *HTMLCollector) parseHTML(htmlContent string, config CollectConfig) ([]Article, error) {
	// 检查是否是文章列表页面还是单篇文章页面
	if h.isArticleListPage(htmlContent) {
		return h.parseArticleList(htmlContent, config)
	}
	
	return h.parseSingleArticle(htmlContent, config)
}

// isArticleListPage 判断是否为文章列表页面
func (h *HTMLCollector) isArticleListPage(htmlContent string) bool {
	// 简单的启发式判断：查找常见的列表结构
	listPatterns := []string{
		`<article[^>]*>.*?<article[^>]*>`,
		`<div[^>]*class="[^"]*post[^"]*"[^>]*>.*?<div[^>]*class="[^"]*post[^"]*"[^>]*>`,
		`<div[^>]*class="[^"]*item[^"]*"[^>]*>.*?<div[^>]*class="[^"]*item[^"]*"[^>]*>`,
		`<h2[^>]*>.*?<h2[^>]*>`,
		`<h3[^>]*>.*?<h3[^>]*>`,
	}

	for _, pattern := range listPatterns {
		if matched, _ := regexp.MatchString(pattern, htmlContent); matched {
			return true
		}
	}

	return false
}

// parseArticleList 解析文章列表页面
func (h *HTMLCollector) parseArticleList(htmlContent string, config CollectConfig) ([]Article, error) {
	var articles []Article

	// 尝试提取文章列表项
	articleBlocks := h.extractArticleBlocks(htmlContent)
	
	for i, block := range articleBlocks {
		article := h.parseArticleBlock(block, config, i)
		if article.Title != "" || article.Content != "" {
			articles = append(articles, article)
		}
	}

	// 如果没有找到文章块，尝试提取链接作为文章
	if len(articles) == 0 {
		links := h.extractLinks(htmlContent, config.URL)
		for i, link := range links {
			if i >= 50 { // 限制链接数量
				break
			}
			article := Article{
				ID:         h.generateID(link.URL + link.Title),
				Title:      link.Title,
				Content:    link.Title,
				Summary:    link.Title,
				Author:     "Unknown",
				URL:        link.URL,
				Source:     config.URL,
				SourceType: h.GetSourceType(),
				Language:   config.Language,
				PublishedAt: time.Now(),
				Tags:       append([]string{"link"}, config.Tags...),
				Metadata:   make(map[string]string),
			}

			if config.Metadata != nil {
				for k, v := range config.Metadata {
					article.Metadata[k] = v
				}
			}

			articles = append(articles, article)
		}
	}

	return articles, nil
}

// parseSingleArticle 解析单篇文章页面
func (h *HTMLCollector) parseSingleArticle(htmlContent string, config CollectConfig) ([]Article, error) {
	article := Article{
		ID:         h.generateID(config.URL),
		Source:     config.URL,
		SourceType: h.GetSourceType(),
		Language:   config.Language,
		URL:        config.URL,
		PublishedAt: time.Now(),
		Tags:       config.Tags,
		Metadata:   make(map[string]string),
	}

	// 提取标题
	article.Title = h.extractTitle(htmlContent)

	// 提取内容
	article.Content = h.extractContent(htmlContent)

	// 提取摘要
	article.Summary = h.extractSummary(article.Content, 200)

	// 提取作者
	article.Author = h.extractAuthor(htmlContent)

	// 提取发布时间
	if pubTime := h.extractPublishedAt(htmlContent); !pubTime.IsZero() {
		article.PublishedAt = pubTime
	}

	// 提取标签
	extractedTags := h.extractTags(htmlContent)
	article.Tags = append(article.Tags, extractedTags...)

	// 添加元数据
	if config.Metadata != nil {
		for k, v := range config.Metadata {
			article.Metadata[k] = v
		}
	}

	// 添加页面元数据
	article.Metadata["page_type"] = "single_article"
	article.Metadata["extraction_time"] = time.Now().Format(time.RFC3339)

	return []Article{article}, nil
}

// extractArticleBlocks 提取文章块
func (h *HTMLCollector) extractArticleBlocks(htmlContent string) []string {
	var blocks []string

	// 尝试不同的文章块选择器
	selectors := []string{
		`<article[^>]*>(.*?)</article>`,
		`<div[^>]*class="[^"]*post[^"]*"[^>]*>(.*?)</div>`,
		`<div[^>]*class="[^"]*item[^"]*"[^>]*>(.*?)</div>`,
		`<div[^>]*class="[^"]*entry[^"]*"[^>]*>(.*?)</div>`,
	}

	for _, selector := range selectors {
		re := regexp.MustCompile(selector)
		matches := re.FindAllStringSubmatch(htmlContent, -1)
		if len(matches) > 0 {
			for _, match := range matches {
				if len(match) > 1 {
					blocks = append(blocks, match[1])
				}
			}
			break
		}
	}

	return blocks
}

// parseArticleBlock 解析单个文章块
func (h *HTMLCollector) parseArticleBlock(block string, config CollectConfig, index int) Article {
	article := Article{
		Source:     config.URL,
		SourceType: h.GetSourceType(),
		Language:   config.Language,
		PublishedAt: time.Now(),
		Tags:       config.Tags,
		Metadata:   make(map[string]string),
	}

	// 提取标题
	article.Title = h.extractTitleFromBlock(block)

	// 提取内容
	article.Content = h.cleanHTML(block)

	// 提取摘要
	article.Summary = h.extractSummary(article.Content, 200)

	// 提取URL
	if url := h.extractLinkFromBlock(block, config.URL); url != "" {
		article.URL = url
		article.ID = h.generateID(url + article.Title)
	} else {
		article.URL = config.URL
		article.ID = h.generateID(config.URL + article.Title + strconv.Itoa(index))
	}

	// 添加元数据
	if config.Metadata != nil {
		for k, v := range config.Metadata {
			article.Metadata[k] = v
		}
	}
	article.Metadata["block_index"] = strconv.Itoa(index)

	return article
}

// 提取方法
func (h *HTMLCollector) extractTitle(htmlContent string) string {
	// 尝试多种标题选择器
	titleSelectors := []string{
		`<title[^>]*>(.*?)</title>`,
		`<h1[^>]*>(.*?)</h1>`,
		`<h2[^>]*>(.*?)</h2>`,
		`<meta[^>]*property="og:title"[^>]*content="([^"]*)"`,
		`<meta[^>]*name="title"[^>]*content="([^"]*)"`,
	}

	for _, selector := range titleSelectors {
		re := regexp.MustCompile(selector)
		if match := re.FindStringSubmatch(htmlContent); len(match) > 1 {
			return h.cleanHTML(match[1])
		}
	}

	return "Untitled"
}

func (h *HTMLCollector) extractTitleFromBlock(block string) string {
	// 从块中提取标题
	titleSelectors := []string{
		`<h[1-6][^>]*>(.*?)</h[1-6]>`,
		`<a[^>]*class="[^"]*title[^"]*"[^>]*>(.*?)</a>`,
		`<div[^>]*class="[^"]*title[^"]*"[^>]*>(.*?)</div>`,
		`<span[^>]*class="[^"]*title[^"]*"[^>]*>(.*?)</span>`,
	}

	for _, selector := range titleSelectors {
		re := regexp.MustCompile(selector)
		if match := re.FindStringSubmatch(block); len(match) > 1 {
			return h.cleanHTML(match[1])
		}
	}

	// 如果没有找到专门的标题，尝试提取第一个链接文本
	linkRe := regexp.MustCompile(`<a[^>]*>(.*?)</a>`)
	if match := linkRe.FindStringSubmatch(block); len(match) > 1 {
		return h.cleanHTML(match[1])
	}

	return ""
}

func (h *HTMLCollector) extractContent(htmlContent string) string {
	// 尝试多种内容选择器
	contentSelectors := []string{
		`<main[^>]*>(.*?)</main>`,
		`<article[^>]*>(.*?)</article>`,
		`<div[^>]*class="[^"]*content[^"]*"[^>]*>(.*?)</div>`,
		`<div[^>]*class="[^"]*post[^"]*"[^>]*>(.*?)</div>`,
		`<div[^>]*id="content"[^>]*>(.*?)</div>`,
	}

	for _, selector := range contentSelectors {
		re := regexp.MustCompile(`(?s)` + selector)
		if match := re.FindStringSubmatch(htmlContent); len(match) > 1 {
			return h.cleanHTML(match[1])
		}
	}

	// 如果没有找到特定的内容区域，提取body内容
	if match := regexp.MustCompile(`(?s)<body[^>]*>(.*?)</body>`).FindStringSubmatch(htmlContent); len(match) > 1 {
		return h.cleanHTML(match[1])
	}

	return h.cleanHTML(htmlContent)
}

func (h *HTMLCollector) extractAuthor(htmlContent string) string {
	authorSelectors := []string{
		`<meta[^>]*name="author"[^>]*content="([^"]*)"`,
		`<meta[^>]*property="article:author"[^>]*content="([^"]*)"`,
		`<span[^>]*class="[^"]*author[^"]*"[^>]*>(.*?)</span>`,
		`<div[^>]*class="[^"]*author[^"]*"[^>]*>(.*?)</div>`,
		`<a[^>]*class="[^"]*author[^"]*"[^>]*>(.*?)</a>`,
	}

	for _, selector := range authorSelectors {
		re := regexp.MustCompile(selector)
		if match := re.FindStringSubmatch(htmlContent); len(match) > 1 {
			return h.cleanHTML(match[1])
		}
	}

	return "Unknown"
}

func (h *HTMLCollector) extractPublishedAt(htmlContent string) time.Time {
	timeSelectors := []string{
		`<meta[^>]*property="article:published_time"[^>]*content="([^"]*)"`,
		`<time[^>]*datetime="([^"]*)"`,
		`<span[^>]*class="[^"]*date[^"]*"[^>]*>(.*?)</span>`,
		`<div[^>]*class="[^"]*date[^"]*"[^>]*>(.*?)</div>`,
	}

	for _, selector := range timeSelectors {
		re := regexp.MustCompile(selector)
		if match := re.FindStringSubmatch(htmlContent); len(match) > 1 {
			timeStr := h.cleanHTML(match[1])
			if parsedTime, err := h.parseTime(timeStr); err == nil {
				return parsedTime
			}
		}
	}

	return time.Time{}
}

func (h *HTMLCollector) extractTags(htmlContent string) []string {
	var tags []string

	tagSelectors := []string{
		`<meta[^>]*name="keywords"[^>]*content="([^"]*)"`,
		`<meta[^>]*property="article:tag"[^>]*content="([^"]*)"`,
		`<span[^>]*class="[^"]*tag[^"]*"[^>]*>(.*?)</span>`,
		`<a[^>]*class="[^"]*tag[^"]*"[^>]*>(.*?)</a>`,
	}

	for _, selector := range tagSelectors {
		re := regexp.MustCompile(selector)
		matches := re.FindAllStringSubmatch(htmlContent, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tag := h.cleanHTML(match[1])
				if strings.Contains(tag, ",") {
					// 分割逗号分隔的标签
					splitTags := strings.Split(tag, ",")
					for _, t := range splitTags {
						if cleaned := strings.TrimSpace(t); cleaned != "" {
							tags = append(tags, cleaned)
						}
					}
				} else if tag != "" {
					tags = append(tags, tag)
				}
			}
		}
	}

	return tags
}

func (h *HTMLCollector) extractLinkFromBlock(block, baseURL string) string {
	re := regexp.MustCompile(`<a[^>]*href="([^"]*)"`)
	if match := re.FindStringSubmatch(block); len(match) > 1 {
		link := match[1]
		return h.resolveURL(link, baseURL)
	}
	return ""
}

type Link struct {
	URL   string
	Title string
}

func (h *HTMLCollector) extractLinks(htmlContent, baseURL string) []Link {
	var links []Link
	re := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)

	for _, match := range matches {
		if len(match) > 2 {
			url := h.resolveURL(match[1], baseURL)
			title := h.cleanHTML(match[2])
			if title != "" && url != "" {
				links = append(links, Link{URL: url, Title: title})
			}
		}
	}

	return links
}

func (h *HTMLCollector) resolveURL(href, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}

	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	return base.ResolveReference(ref).String()
}

// 辅助方法
func (h *HTMLCollector) cleanHTML(content string) string {
	// 移除HTML标签
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(content, " ")
	
	// 解码HTML实体
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&#39;", "'")
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	
	// 清理多余的空白字符
	re = regexp.MustCompile(`\s+`)
	cleaned = re.ReplaceAllString(cleaned, " ")
	
	return strings.TrimSpace(cleaned)
}

func (h *HTMLCollector) extractSummary(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	
	// 在单词边界处截断
	if idx := strings.LastIndex(content[:maxLen], " "); idx > 0 {
		return content[:idx] + "..."
	}
	
	return content[:maxLen] + "..."
}

func (h *HTMLCollector) parseTime(timeStr string) (time.Time, error) {
	// 常见的时间格式
	formats := []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"Jan 2, 2006",
		"January 2, 2006",
		"02 Jan 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, strings.TrimSpace(timeStr)); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

func (h *HTMLCollector) generateID(content string) string {
	// 重用RSS采集器的ID生成方法
	rss := &RSSCollector{}
	return rss.generateID(content)
}