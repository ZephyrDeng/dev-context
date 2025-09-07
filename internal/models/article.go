package models

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Article represents a unified data structure for articles from various sources
// This structure standardizes data from RSS feeds, APIs, and HTML scrapers
type Article struct {
	// ID is a unique identifier for the article, typically generated from URL or content hash
	ID string `json:"id" validate:"required"`
	
	// Title is the article headline or title
	Title string `json:"title" validate:"required,min=1,max=500"`
	
	// URL is the original article URL
	URL string `json:"url" validate:"required,url"`
	
	// Source identifies the publication or website name
	Source string `json:"source" validate:"required,min=1,max=100"`
	
	// SourceType indicates the collection method: rss, api, or html
	SourceType string `json:"sourceType" validate:"required,oneof=rss api html"`
	
	// PublishedAt is the article publication timestamp
	PublishedAt time.Time `json:"publishedAt" validate:"required"`
	
	// Summary is the article summary or excerpt (50-150 characters recommended)
	Summary string `json:"summary" validate:"min=10,max=1000"`
	
	// Content is the full article text (optional, excluded from JSON by default for size)
	Content string `json:"content,omitempty"`
	
	// Tags are relevant keywords or categories associated with the article
	Tags []string `json:"tags"`
	
	// Relevance score (0.0-1.0) calculated based on content matching
	Relevance float64 `json:"relevance" validate:"min=0,max=1"`
	
	// Quality score (0.0-1.0) based on content completeness and source credibility
	Quality float64 `json:"quality" validate:"min=0,max=1"`
	
	// Metadata contains additional source-specific information
	Metadata map[string]interface{} `json:"metadata"`
}

// NewArticle creates a new Article instance with required fields and generates ID
func NewArticle(title, url, source, sourceType string) *Article {
	article := &Article{
		Title:       strings.TrimSpace(title),
		URL:         strings.TrimSpace(url),
		Source:      strings.TrimSpace(source),
		SourceType:  strings.ToLower(strings.TrimSpace(sourceType)),
		PublishedAt: time.Now(),
		Tags:        make([]string, 0),
		Relevance:   0.0,
		Quality:     0.0,
		Metadata:    make(map[string]interface{}),
	}
	article.ID = article.GenerateID()
	return article
}

// Validate performs basic validation on the Article fields
func (a *Article) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("ID is required")
	}
	
	if strings.TrimSpace(a.Title) == "" {
		return fmt.Errorf("Title is required")
	}
	
	if len(a.Title) > 500 {
		return fmt.Errorf("Title must not exceed 500 characters")
	}
	
	if strings.TrimSpace(a.URL) == "" {
		return fmt.Errorf("URL is required")
	}
	
	if strings.TrimSpace(a.Source) == "" {
		return fmt.Errorf("Source is required")
	}
	
	if len(a.Source) > 100 {
		return fmt.Errorf("Source must not exceed 100 characters")
	}
	
	validSourceTypes := []string{"rss", "api", "html"}
	isValidType := false
	for _, validType := range validSourceTypes {
		if a.SourceType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("SourceType must be one of: %v", validSourceTypes)
	}
	
	if a.PublishedAt.IsZero() {
		return fmt.Errorf("PublishedAt is required")
	}
	
	if a.Summary != "" && len(a.Summary) < 10 {
		return fmt.Errorf("Summary must be at least 10 characters")
	}
	
	if len(a.Summary) > 1000 {
		return fmt.Errorf("Summary must not exceed 1000 characters")
	}
	
	if a.Relevance < 0.0 || a.Relevance > 1.0 {
		return fmt.Errorf("Relevance must be between 0.0 and 1.0")
	}
	
	if a.Quality < 0.0 || a.Quality > 1.0 {
		return fmt.Errorf("Quality must be between 0.0 and 1.0")
	}
	
	return nil
}

// GenerateID creates a unique identifier for the article based on URL and title
func (a *Article) GenerateID() string {
	if a.URL == "" && a.Title == "" {
		return ""
	}
	
	content := a.URL + "|" + a.Title
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// SetSummary sets the article summary with length validation and optimization
func (a *Article) SetSummary(summary string) error {
	summary = strings.TrimSpace(summary)
	
	// Ensure summary is within recommended length (50-150 chars for optimal display)
	if len(summary) < 10 {
		return fmt.Errorf("summary too short (minimum 10 characters)")
	}
	
	if len(summary) > 1000 {
		return fmt.Errorf("summary too long (maximum 1000 characters)")
	}
	
	a.Summary = summary
	return nil
}

// AddTag adds a tag to the article, avoiding duplicates
func (a *Article) AddTag(tag string) {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return
	}
	
	// Check for duplicates
	for _, existingTag := range a.Tags {
		if existingTag == tag {
			return
		}
	}
	
	a.Tags = append(a.Tags, tag)
}

// AddTags adds multiple tags to the article
func (a *Article) AddTags(tags ...string) {
	for _, tag := range tags {
		a.AddTag(tag)
	}
}

// SetMetadata sets a metadata key-value pair
func (a *Article) SetMetadata(key string, value interface{}) {
	if a.Metadata == nil {
		a.Metadata = make(map[string]interface{})
	}
	a.Metadata[key] = value
}

// GetMetadata retrieves a metadata value by key
func (a *Article) GetMetadata(key string) (interface{}, bool) {
	if a.Metadata == nil {
		return nil, false
	}
	value, exists := a.Metadata[key]
	return value, exists
}

// TitleLength returns the UTF-8 character count of the title
func (a *Article) TitleLength() int {
	return utf8.RuneCountInString(a.Title)
}

// SummaryLength returns the UTF-8 character count of the summary
func (a *Article) SummaryLength() int {
	return utf8.RuneCountInString(a.Summary)
}

// IsRecent checks if the article was published within the specified duration
func (a *Article) IsRecent(within time.Duration) bool {
	return time.Since(a.PublishedAt) <= within
}

// HasTag checks if the article has a specific tag
func (a *Article) HasTag(tag string) bool {
	tag = strings.TrimSpace(strings.ToLower(tag))
	for _, t := range a.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// CalculateHash generates a hash for deduplication purposes
func (a *Article) CalculateHash() string {
	// Use title + URL for basic deduplication
	// This can be enhanced with content similarity algorithms if needed
	content := strings.ToLower(a.Title) + "|" + a.URL
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// UpdateQuality recalculates the quality score based on multiple factors
func (a *Article) UpdateQuality() {
	score := 0.0
	
	// Base score for having required fields
	if a.Title != "" && a.URL != "" && a.Source != "" {
		score += 0.3
	}
	
	// Score for having summary
	if a.Summary != "" {
		score += 0.2
		// Bonus for optimal summary length
		if len(a.Summary) >= 50 && len(a.Summary) <= 150 {
			score += 0.1
		}
	}
	
	// Score for having tags
	if len(a.Tags) > 0 {
		score += 0.1
		// Bonus for multiple relevant tags
		if len(a.Tags) >= 3 {
			score += 0.1
		}
	}
	
	// Score for recent publication (articles published within last 30 days get bonus)
	if a.IsRecent(30 * 24 * time.Hour) {
		score += 0.1
	}
	
	// Score for having content
	if a.Content != "" {
		score += 0.1
	}
	
	// Ensure score is within valid range
	if score > 1.0 {
		score = 1.0
	}
	
	a.Quality = score
}

// String returns a string representation of the article
func (a *Article) String() string {
	return fmt.Sprintf("Article{ID: %s, Title: %s, Source: %s, Quality: %.2f}", 
		a.ID, a.Title, a.Source, a.Quality)
}