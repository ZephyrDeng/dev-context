package processor

import (
	"crypto/md5"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ZephyrDeng/dev-context/internal/collector"
	"github.com/ZephyrDeng/dev-context/internal/models"
)

// Converter handles conversion from raw collector data to unified Article/Repository models
// It provides thread-safe operations for concurrent processing and comprehensive data normalization
type Converter struct {
	// mutex for thread-safe operations
	mu sync.RWMutex

	// Configuration for data processing
	config ConverterConfig
}

// ConverterConfig contains configuration options for data conversion
type ConverterConfig struct {
	// Maximum length for summary text (default: 1000)
	MaxSummaryLength int

	// Maximum length for title text (default: 500)
	MaxTitleLength int

	// Maximum length for content text (default: 50000)
	MaxContentLength int

	// Default quality score for articles without explicit quality indicators
	DefaultQuality float64

	// Default relevance score for articles without relevance calculation
	DefaultRelevance float64

	// Enable aggressive HTML cleaning (removes more tags and attributes)
	AggressiveHTMLCleaning bool

	// Normalize URLs to canonical form (removes tracking parameters, etc.)
	NormalizeURLs bool

	// Time zone for date normalization (default: UTC)
	TimeZone *time.Location
}

// DefaultConverterConfig returns a configuration with sensible defaults
func DefaultConverterConfig() ConverterConfig {
	return ConverterConfig{
		MaxSummaryLength:       1000,
		MaxTitleLength:         500,
		MaxContentLength:       50000,
		DefaultQuality:         0.5,
		DefaultRelevance:       0.0,
		AggressiveHTMLCleaning: true,
		NormalizeURLs:          true,
		TimeZone:               time.UTC,
	}
}

// NewConverter creates a new converter with the given configuration
func NewConverter(config ConverterConfig) *Converter {
	return &Converter{
		config: config,
	}
}

// NewDefaultConverter creates a new converter with default configuration
func NewDefaultConverter() *Converter {
	return NewConverter(DefaultConverterConfig())
}

// ConvertToArticle converts a collector.Article to models.Article with full data normalization
func (c *Converter) ConvertToArticle(collectorArticle collector.Article) (*models.Article, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.validateCollectorArticle(collectorArticle); err != nil {
		return nil, fmt.Errorf("invalid collector article: %w", err)
	}

	// Apply source-specific processing based on sourceType
	processedArticle := c.applySourceSpecificProcessing(collectorArticle)

	// Create new article with basic required fields
	article := models.NewArticle(
		c.normalizeTitle(processedArticle.Title),
		c.normalizeURL(processedArticle.URL),
		c.normalizeSource(processedArticle.Source),
		c.normalizeSourceType(processedArticle.SourceType),
	)

	// Set publication time
	if !processedArticle.PublishedAt.IsZero() {
		article.PublishedAt = c.normalizeTime(processedArticle.PublishedAt)
	}

	// Process and set content
	if processedArticle.Content != "" {
		article.Content = c.cleanAndNormalizeContent(processedArticle.Content)
	}

	// Process and set summary
	summary := c.generateSummary(processedArticle.Summary, processedArticle.Content)
	if err := article.SetSummary(summary); err != nil {
		// If summary is invalid, try to generate a shorter one
		summary = c.truncateText(summary, c.config.MaxSummaryLength/2)
		if err := article.SetSummary(summary); err != nil {
			return nil, fmt.Errorf("failed to set summary: %w", err)
		}
	}

	// Process tags
	if len(processedArticle.Tags) > 0 {
		normalizedTags := c.normalizeTags(processedArticle.Tags)
		article.AddTags(normalizedTags...)
	}

	// Set author in metadata if available
	if processedArticle.Author != "" {
		article.SetMetadata("author", c.normalizeText(processedArticle.Author))
	}

	// Set language in metadata if available
	if processedArticle.Language != "" {
		article.SetMetadata("language", strings.ToLower(strings.TrimSpace(processedArticle.Language)))
	}

	// Copy original metadata
	for key, value := range processedArticle.Metadata {
		article.SetMetadata(key, value)
	}

	// Set default quality and relevance scores
	article.Quality = c.config.DefaultQuality
	article.Relevance = c.config.DefaultRelevance

	// Calculate and update quality score based on content
	article.UpdateQuality()

	// Validate the final article
	if err := article.Validate(); err != nil {
		return nil, fmt.Errorf("converted article failed validation: %w", err)
	}

	return article, nil
}

// ConvertToRepository converts API data to models.Repository for repository-type content
func (c *Converter) ConvertToRepository(name, fullName, url string, metadata map[string]string) (*models.Repository, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("repository name is required")
	}
	if strings.TrimSpace(fullName) == "" {
		return nil, fmt.Errorf("repository fullName is required")
	}
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("repository URL is required")
	}

	// Create new repository
	repo := models.NewRepository(
		c.normalizeText(name),
		c.normalizeText(fullName),
		c.normalizeURL(url),
	)

	// Set description if available
	if desc, exists := metadata["description"]; exists && desc != "" {
		if err := repo.SetDescription(c.cleanAndNormalizeContent(desc)); err != nil {
			// Truncate if too long
			truncated := c.truncateText(desc, 1000)
			repo.SetDescription(c.cleanAndNormalizeContent(truncated))
		}
	}

	// Set language if available
	if lang, exists := metadata["language"]; exists && lang != "" {
		repo.SetLanguage(c.normalizeText(lang))
	}

	// Set stars if available
	if starsStr, exists := metadata["stars"]; exists && starsStr != "" {
		if stars, err := strconv.Atoi(starsStr); err == nil && stars >= 0 {
			repo.Stars = stars
		}
	}

	// Set forks if available
	if forksStr, exists := metadata["forks"]; exists && forksStr != "" {
		if forks, err := strconv.Atoi(forksStr); err == nil && forks >= 0 {
			repo.Forks = forks
		}
	}

	// Set update time if available
	if updatedStr, exists := metadata["updated_at"]; exists && updatedStr != "" {
		if updatedTime, err := c.parseTime(updatedStr); err == nil {
			repo.UpdatedAt = c.normalizeTime(updatedTime)
		}
	}

	// Calculate trend score
	repo.CalculateTrendScore()

	// Validate the final repository
	if err := repo.Validate(); err != nil {
		return nil, fmt.Errorf("converted repository failed validation: %w", err)
	}

	return repo, nil
}

// BatchConvertArticles converts multiple collector articles concurrently
func (c *Converter) BatchConvertArticles(collectorArticles []collector.Article) ([]*models.Article, []error) {
	if len(collectorArticles) == 0 {
		return []*models.Article{}, []error{}
	}

	articles := make([]*models.Article, len(collectorArticles))
	errors := make([]error, len(collectorArticles))

	// Use a wait group for concurrent processing
	var wg sync.WaitGroup

	for i, collectorArticle := range collectorArticles {
		wg.Add(1)
		go func(index int, ca collector.Article) {
			defer wg.Done()

			article, err := c.ConvertToArticle(ca)
			articles[index] = article
			errors[index] = err
		}(i, collectorArticle)
	}

	wg.Wait()

	return articles, errors
}

// Data normalization and cleaning functions

// normalizeTitle cleans and normalizes article titles
func (c *Converter) normalizeTitle(title string) string {
	if title == "" {
		return ""
	}

	// Decode HTML entities
	title = html.UnescapeString(title)

	// Remove HTML tags
	title = c.removeHTMLTags(title)

	// Normalize whitespace
	title = c.normalizeWhitespace(title)

	// Truncate if too long
	title = c.truncateText(title, c.config.MaxTitleLength)

	return strings.TrimSpace(title)
}

// normalizeURL cleans and normalizes URLs
func (c *Converter) normalizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// Parse URL
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL // Return original if parsing fails
	}

	// Convert to lowercase for scheme and host
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	if c.config.NormalizeURLs {
		// Remove common tracking parameters
		query := parsedURL.Query()
		trackingParams := []string{
			"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
			"fbclid", "gclid", "ref", "source", "campaign_id", "ad_id",
		}

		for _, param := range trackingParams {
			query.Del(param)
		}
		parsedURL.RawQuery = query.Encode()
	}

	return parsedURL.String()
}

// normalizeSource cleans source names
func (c *Converter) normalizeSource(source string) string {
	if source == "" {
		return ""
	}

	// Extract domain from URL if source looks like a URL
	if strings.HasPrefix(source, "http") {
		if parsedURL, err := url.Parse(source); err == nil {
			source = parsedURL.Host
		}
	}

	// Clean up source name
	source = strings.TrimSpace(source)
	source = strings.ToLower(source)
	source = strings.TrimPrefix(source, "www.")

	return source
}

// normalizeSourceType ensures source type is valid
func (c *Converter) normalizeSourceType(sourceType string) string {
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))

	validTypes := []string{"rss", "api", "html"}
	for _, validType := range validTypes {
		if sourceType == validType {
			return sourceType
		}
	}

	// Default to 'api' if unknown
	return "api"
}

// normalizeTime ensures time is in configured timezone
func (c *Converter) normalizeTime(t time.Time) time.Time {
	if c.config.TimeZone != nil {
		return t.In(c.config.TimeZone)
	}
	return t.UTC()
}

// normalizeText performs general text normalization
func (c *Converter) normalizeText(text string) string {
	if text == "" {
		return ""
	}

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Normalize Unicode (NFC normalization)
	text = c.normalizeUnicode(text)

	// Normalize whitespace
	text = c.normalizeWhitespace(text)

	return strings.TrimSpace(text)
}

// cleanAndNormalizeContent performs comprehensive content cleaning
func (c *Converter) cleanAndNormalizeContent(content string) string {
	if content == "" {
		return ""
	}

	// Remove HTML tags first (before decoding entities)
	content = c.removeHTMLTags(content)

	// Decode HTML entities after removing tags
	content = html.UnescapeString(content)

	// Remove special characters that might cause issues
	content = c.removeSpecialCharacters(content)

	// Normalize whitespace
	content = c.normalizeWhitespace(content)

	// Truncate if too long
	content = c.truncateText(content, c.config.MaxContentLength)

	return strings.TrimSpace(content)
}

// generateSummary creates a summary from available text
func (c *Converter) generateSummary(providedSummary, fullContent string) string {
	// Use provided summary if available and valid
	if providedSummary != "" {
		summary := c.cleanAndNormalizeContent(providedSummary)
		if len(summary) >= 10 && len(summary) <= c.config.MaxSummaryLength {
			return summary
		}
	}

	// Generate summary from content
	if fullContent != "" {
		content := c.cleanAndNormalizeContent(fullContent)
		return c.extractSummary(content, c.config.MaxSummaryLength)
	}

	return ""
}

// normalizeTags cleans and normalizes tag arrays
func (c *Converter) normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}

	normalizedTags := make([]string, 0, len(tags))
	seenTags := make(map[string]bool)

	for _, tag := range tags {
		// Clean and normalize tag
		tag = c.normalizeText(tag)
		tag = strings.ToLower(tag)

		// Skip empty or duplicate tags
		if tag == "" || len(tag) > 50 || seenTags[tag] {
			continue
		}

		seenTags[tag] = true
		normalizedTags = append(normalizedTags, tag)
	}

	return normalizedTags
}

// HTML and content cleaning utilities

// removeHTMLTags strips HTML tags from text
func (c *Converter) removeHTMLTags(text string) string {
	if text == "" {
		return ""
	}

	if c.config.AggressiveHTMLCleaning {
		// Remove script and style tags with their content
		scriptRegex := regexp.MustCompile(`<script[^>]*>.*?</script>`)
		text = scriptRegex.ReplaceAllString(text, "")

		styleRegex := regexp.MustCompile(`<style[^>]*>.*?</style>`)
		text = styleRegex.ReplaceAllString(text, "")
	}

	// Remove all HTML tags
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	text = tagRegex.ReplaceAllString(text, " ")

	return text
}

// removeSpecialCharacters removes problematic special characters
func (c *Converter) removeSpecialCharacters(text string) string {
	if text == "" {
		return ""
	}

	// Remove control characters except tab, newline, and carriage return
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}

// normalizeWhitespace normalizes whitespace characters
func (c *Converter) normalizeWhitespace(text string) string {
	if text == "" {
		return ""
	}

	// Replace multiple whitespace with single space
	whitespaceRegex := regexp.MustCompile(`\s+`)
	return whitespaceRegex.ReplaceAllString(text, " ")
}

// normalizeUnicode performs Unicode normalization
func (c *Converter) normalizeUnicode(text string) string {
	// Basic Unicode validation and cleaning
	if !utf8.ValidString(text) {
		// Convert invalid UTF-8 to valid string
		return strings.ToValidUTF8(text, "")
	}
	return text
}

// truncateText safely truncates text to specified length at word boundaries
func (c *Converter) truncateText(text string, maxLength int) string {
	if text == "" || len(text) <= maxLength {
		return text
	}

	// Find last word boundary before max length
	truncated := text[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 {
		return truncated[:lastSpace] + "..."
	}

	return truncated + "..."
}

// extractSummary creates a summary by taking first sentences up to maxLength
func (c *Converter) extractSummary(content string, maxLength int) string {
	if content == "" {
		return ""
	}

	// Split into sentences (simple approach)
	sentences := strings.FieldsFunc(content, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	if len(sentences) == 0 {
		return c.truncateText(content, maxLength)
	}

	var summary strings.Builder
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		// Check if adding this sentence would exceed max length
		if summary.Len()+len(sentence)+2 > maxLength {
			break
		}

		if summary.Len() > 0 {
			summary.WriteString(". ")
		}
		summary.WriteString(sentence)
	}

	result := summary.String()
	if result == "" {
		return c.truncateText(content, maxLength)
	}

	return result + "."
}

// parseTime attempts to parse time strings in various formats
func (c *Converter) parseTime(timeStr string) (time.Time, error) {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Common time formats used in APIs and feeds
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon Jan 02 15:04:05 -0700 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time string: %s", timeStr)
}

// Validation functions

// validateCollectorArticle validates input from collector
func (c *Converter) validateCollectorArticle(article collector.Article) error {
	if strings.TrimSpace(article.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if strings.TrimSpace(article.URL) == "" {
		return fmt.Errorf("URL is required")
	}

	if strings.TrimSpace(article.Source) == "" {
		return fmt.Errorf("source is required")
	}

	if strings.TrimSpace(article.SourceType) == "" {
		return fmt.Errorf("sourceType is required")
	}

	return nil
}

// GetConfig returns a copy of the current configuration
func (c *Converter) GetConfig() ConverterConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// UpdateConfig updates the converter configuration in a thread-safe manner
func (c *Converter) UpdateConfig(config ConverterConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GenerateHash creates a hash for deduplication purposes
func (c *Converter) GenerateHash(title, url string) string {
	content := strings.ToLower(strings.TrimSpace(title)) + "|" + strings.TrimSpace(url)
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// Source-specific processing methods

// applySourceSpecificProcessing applies processing logic based on the data source type
func (c *Converter) applySourceSpecificProcessing(article collector.Article) collector.Article {
	switch strings.ToLower(article.SourceType) {
	case "rss":
		return c.processRSSArticle(article)
	case "api":
		return c.processAPIArticle(article)
	case "html":
		return c.processHTMLArticle(article)
	default:
		return article
	}
}

// processRSSArticle applies RSS-specific processing logic
func (c *Converter) processRSSArticle(article collector.Article) collector.Article {
	processed := article // Make a copy

	// RSS-specific title cleaning (often has extra prefixes/suffixes)
	if processed.Title != "" {
		// Remove common RSS feed prefixes like "RSS:" or "[RSS]"
		rssPatterns := []string{`^RSS:\s*`, `^\[RSS\]\s*`, `^\[.*?\]\s*`}
		for _, pattern := range rssPatterns {
			if re := regexp.MustCompile(pattern); re != nil {
				processed.Title = re.ReplaceAllString(processed.Title, "")
			}
		}
	}

	// RSS often has better structured content, prioritize RSS summary
	if processed.Summary == "" && processed.Content != "" {
		// Generate summary from first paragraph of RSS content
		processed.Summary = c.extractFirstParagraph(processed.Content)
	}

	// RSS feeds often have reliable publication dates, but normalize timezone
	if !processed.PublishedAt.IsZero() {
		// RSS dates are usually already well-formatted, just normalize to UTC
		processed.PublishedAt = processed.PublishedAt.UTC()
	}

	return processed
}

// processAPIArticle applies API-specific processing logic
func (c *Converter) processAPIArticle(article collector.Article) collector.Article {
	processed := article // Make a copy

	// API data is usually well-structured but might have JSON escaping issues
	if processed.Title != "" {
		// Unescape JSON strings that might have been double-encoded
		processed.Title = strings.ReplaceAll(processed.Title, `\"`, `"`)
		processed.Title = strings.ReplaceAll(processed.Title, `\\`, `\`)
	}

	// API content might have JSON structure artifacts
	if processed.Content != "" {
		processed.Content = strings.ReplaceAll(processed.Content, `\"`, `"`)
		processed.Content = strings.ReplaceAll(processed.Content, `\\n`, "\n")
		processed.Content = strings.ReplaceAll(processed.Content, `\\t`, "\t")
	}

	// API sources often provide metadata that should be preserved
	if processed.Metadata == nil {
		processed.Metadata = make(map[string]string)
	}
	processed.Metadata["api_processed"] = "true"

	return processed
}

// processHTMLArticle applies HTML-specific processing logic
func (c *Converter) processHTMLArticle(article collector.Article) collector.Article {
	processed := article // Make a copy

	// HTML content requires more aggressive cleaning
	if processed.Content != "" {
		// Remove navigation elements, ads, and other non-content HTML
		processed.Content = c.cleanHTMLContent(processed.Content)
	}

	// HTML titles often contain site names, try to extract just the article title
	if processed.Title != "" {
		processed.Title = c.extractArticleTitleFromHTML(processed.Title)
	}

	// HTML scraping often has encoding issues
	processed.Title = html.UnescapeString(processed.Title)
	processed.Content = html.UnescapeString(processed.Content)
	if processed.Summary != "" {
		processed.Summary = html.UnescapeString(processed.Summary)
	}

	// HTML sources might not have reliable publication dates
	if processed.PublishedAt.IsZero() {
		// Use current time as fallback for HTML scraped content
		processed.PublishedAt = time.Now()
		if processed.Metadata == nil {
			processed.Metadata = make(map[string]string)
		}
		processed.Metadata["date_fallback"] = "true"
	}

	return processed
}

// Helper methods for source-specific processing

// extractFirstParagraph extracts the first meaningful paragraph from content
func (c *Converter) extractFirstParagraph(content string) string {
	if content == "" {
		return ""
	}

	// Split by paragraph breaks
	paragraphs := strings.Split(content, "\n\n")

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		// Skip empty paragraphs and very short ones
		if len(para) > 50 {
			// Clean and return first meaningful paragraph
			cleaned := c.cleanAndNormalizeContent(para)
			if len(cleaned) > 30 && len(cleaned) <= 500 {
				return cleaned
			}
		}
	}

	// If no good paragraph found, try first paragraph regardless of length
	if len(paragraphs) > 0 && strings.TrimSpace(paragraphs[0]) != "" {
		firstPara := strings.TrimSpace(paragraphs[0])
		cleaned := c.cleanAndNormalizeContent(firstPara)
		if len(cleaned) > 0 {
			return c.truncateText(cleaned, 200)
		}
	}

	// Final fallback to truncated content
	return c.truncateText(content, 200)
}

// cleanHTMLContent performs aggressive HTML content cleaning for scraped content
func (c *Converter) cleanHTMLContent(content string) string {
	if content == "" {
		return ""
	}

	// Remove common non-content elements
	unwantedPatterns := []string{
		`<nav[^>]*>.*?</nav>`,                               // Navigation
		`<header[^>]*>.*?</header>`,                         // Headers
		`<footer[^>]*>.*?</footer>`,                         // Footers
		`<aside[^>]*>.*?</aside>`,                           // Sidebars
		`<div[^>]*class="[^"]*ad[^"]*"[^>]*>.*?</div>`,      // Ad containers
		`<div[^>]*class="[^"]*comment[^"]*"[^>]*>.*?</div>`, // Comments
		`<script[^>]*>.*?</script>`,                         // Scripts
		`<style[^>]*>.*?</style>`,                           // Styles
		`<!--.*?-->`,                                        // Comments
	}

	for _, pattern := range unwantedPatterns {
		if re := regexp.MustCompile(`(?is)` + pattern); re != nil {
			content = re.ReplaceAllString(content, "")
		}
	}

	return content
}

// extractArticleTitleFromHTML tries to extract clean article title from HTML title
func (c *Converter) extractArticleTitleFromHTML(title string) string {
	if title == "" {
		return ""
	}

	// Common patterns in HTML titles: "Article Title | Site Name" or "Article Title - Site Name"
	separators := []string{" | ", " - ", " :: ", " — "}

	// Find the first separator that appears in the title
	bestIndex := len(title)
	bestSep := ""

	for _, sep := range separators {
		if index := strings.Index(title, sep); index != -1 && index < bestIndex {
			bestIndex = index
			bestSep = sep
		}
	}

	// If we found a separator, extract the part before it
	if bestSep != "" {
		parts := strings.Split(title, bestSep)
		if len(parts) > 0 && len(strings.TrimSpace(parts[0])) >= 5 {
			return strings.TrimSpace(parts[0])
		}
	}

	return title
}
