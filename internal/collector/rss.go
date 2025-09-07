package collector

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// RSS feed 结构定义
type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Author      string `xml:"author"`
	Category    string `xml:"category"`
}

// Atom feed 结构定义
type AtomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Link    []AtomLink  `xml:"link"`
	Entries []AtomEntry `xml:"entry"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type AtomEntry struct {
	Title     string     `xml:"title"`
	Content   AtomContent `xml:"content"`
	Summary   string     `xml:"summary"`
	Link      []AtomLink `xml:"link"`
	ID        string     `xml:"id"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Author    AtomAuthor `xml:"author"`
	Category  []AtomCategory `xml:"category"`
}

type AtomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type AtomAuthor struct {
	Name  string `xml:"name"`
	Email string `xml:"email"`
}

type AtomCategory struct {
	Term string `xml:"term,attr"`
}

// RSSCollector RSS数据采集器
type RSSCollector struct {
	client *http.Client
}

// NewRSSCollector 创建RSS采集器
func NewRSSCollector() *RSSCollector {
	return &RSSCollector{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetSourceType 返回采集器类型
func (r *RSSCollector) GetSourceType() string {
	return "rss"
}

// Validate 验证配置
func (r *RSSCollector) Validate(config CollectConfig) error {
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

// Collect 采集RSS数据
func (r *RSSCollector) Collect(ctx context.Context, config CollectConfig) (CollectResult, error) {
	if err := r.Validate(config); err != nil {
		return CollectResult{}, fmt.Errorf("validation failed: %w", err)
	}

	// 设置超时
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	// 发起HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
	if err != nil {
		return CollectResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置头部
	req.Header.Set("User-Agent", "RSS Collector/1.0")
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return CollectResult{}, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CollectResult{}, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CollectResult{}, fmt.Errorf("failed to read response body: %w", err)
	}

	// 尝试解析RSS或Atom
	articles, err := r.parseFeed(body, config)
	if err != nil {
		return CollectResult{}, fmt.Errorf("failed to parse feed: %w", err)
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

// parseFeed 解析RSS或Atom feed
func (r *RSSCollector) parseFeed(data []byte, config CollectConfig) ([]Article, error) {
	// 先尝试解析RSS
	var rssFeed RSSFeed
	if err := xml.Unmarshal(data, &rssFeed); err == nil && len(rssFeed.Channel.Items) > 0 {
		return r.parseRSSItems(rssFeed.Channel.Items, config)
	}

	// 再尝试解析Atom
	var atomFeed AtomFeed
	if err := xml.Unmarshal(data, &atomFeed); err == nil && len(atomFeed.Entries) > 0 {
		return r.parseAtomEntries(atomFeed.Entries, config)
	}

	return nil, fmt.Errorf("unable to parse as RSS or Atom feed")
}

// parseRSSItems 解析RSS条目
func (r *RSSCollector) parseRSSItems(items []Item, config CollectConfig) ([]Article, error) {
	articles := make([]Article, 0, len(items))

	for _, item := range items {
		article := Article{
			Title:      html.UnescapeString(strings.TrimSpace(item.Title)),
			Content:    html.UnescapeString(r.cleanHTML(item.Description)),
			Summary:    html.UnescapeString(r.extractSummary(item.Description, 200)),
			Author:     html.UnescapeString(strings.TrimSpace(item.Author)),
			URL:        strings.TrimSpace(item.Link),
			Source:     config.URL,
			SourceType: r.GetSourceType(),
			Language:   config.Language,
			Metadata:   make(map[string]string),
		}

		// 生成ID
		if item.GUID != "" {
			article.ID = item.GUID
		} else {
			article.ID = r.generateID(item.Link + item.Title)
		}

		// 解析发布时间
		if item.PubDate != "" {
			if pubTime, err := r.parseTime(item.PubDate); err == nil {
				article.PublishedAt = pubTime
			}
		}

		// 解析标签
		if item.Category != "" {
			article.Tags = []string{strings.TrimSpace(item.Category)}
		}
		if len(config.Tags) > 0 {
			article.Tags = append(article.Tags, config.Tags...)
		}

		// 添加元数据
		if config.Metadata != nil {
			for k, v := range config.Metadata {
				article.Metadata[k] = v
			}
		}

		articles = append(articles, article)
	}

	return articles, nil
}

// parseAtomEntries 解析Atom条目
func (r *RSSCollector) parseAtomEntries(entries []AtomEntry, config CollectConfig) ([]Article, error) {
	articles := make([]Article, 0, len(entries))

	for _, entry := range entries {
		article := Article{
			ID:         entry.ID,
			Title:      html.UnescapeString(strings.TrimSpace(entry.Title)),
			Content:    html.UnescapeString(r.cleanHTML(entry.Content.Value)),
			Summary:    html.UnescapeString(r.extractSummary(entry.Summary, 200)),
			Author:     html.UnescapeString(strings.TrimSpace(entry.Author.Name)),
			Source:     config.URL,
			SourceType: r.GetSourceType(),
			Language:   config.Language,
			Metadata:   make(map[string]string),
		}

		// 查找链接
		for _, link := range entry.Link {
			if link.Rel == "alternate" || link.Rel == "" {
				article.URL = link.Href
				break
			}
		}

		// 解析发布时间
		timeStr := entry.Published
		if timeStr == "" {
			timeStr = entry.Updated
		}
		if timeStr != "" {
			if pubTime, err := r.parseTime(timeStr); err == nil {
				article.PublishedAt = pubTime
			}
		}

		// 解析标签
		tags := make([]string, 0, len(entry.Category))
		for _, cat := range entry.Category {
			if cat.Term != "" {
				tags = append(tags, cat.Term)
			}
		}
		article.Tags = tags
		if len(config.Tags) > 0 {
			article.Tags = append(article.Tags, config.Tags...)
		}

		// 添加元数据
		if config.Metadata != nil {
			for k, v := range config.Metadata {
				article.Metadata[k] = v
			}
		}

		articles = append(articles, article)
	}

	return articles, nil
}

// parseTime 解析时间字符串
func (r *RSSCollector) parseTime(timeStr string) (time.Time, error) {
	// 常见的时间格式
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC822Z,
		time.RFC822,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, strings.TrimSpace(timeStr)); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// cleanHTML 清理HTML标签
func (r *RSSCollector) cleanHTML(content string) string {
	// 简单的HTML标签清理
	re := regexp.MustCompile(`<[^>]*>`)
	cleaned := re.ReplaceAllString(content, " ")
	
	// 清理多余的空白字符
	re = regexp.MustCompile(`\s+`)
	cleaned = re.ReplaceAllString(cleaned, " ")
	
	return strings.TrimSpace(cleaned)
}

// extractSummary 提取摘要
func (r *RSSCollector) extractSummary(content string, maxLen int) string {
	cleaned := r.cleanHTML(content)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	
	// 在单词边界处截断
	if idx := strings.LastIndex(cleaned[:maxLen], " "); idx > 0 {
		return cleaned[:idx] + "..."
	}
	
	return cleaned[:maxLen] + "..."
}

// generateID 生成文章ID
func (r *RSSCollector) generateID(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash[:8])
}