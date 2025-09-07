// Package processor provides unified data processing capabilities
package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/models"
)

// Processor provides unified data processing functionality
type Processor struct {
	config     *Config
	summarizer *Summarizer
	sorter     *ArticleSorter
	converter  *Converter
	mu         sync.RWMutex
}

// Config holds processor configuration
type Config struct {
	EnableSummarization bool          `json:"enableSummarization"`
	EnableSorting       bool          `json:"enableSorting"`
	MaxSummaryLength    int           `json:"maxSummaryLength"`
	ProcessingTimeout   time.Duration `json:"processingTimeout"`
	MaxConcurrency      int           `json:"maxConcurrency"`
}

// DefaultConfig returns default processor configuration
func DefaultConfig() *Config {
	return &Config{
		EnableSummarization: true,
		EnableSorting:       true,
		MaxSummaryLength:    200,
		ProcessingTimeout:   30 * time.Second,
		MaxConcurrency:      10,
	}
}

// NewProcessor creates a new processor instance
func NewProcessor(config *Config) *Processor {
	if config == nil {
		config = DefaultConfig()
	}

	return &Processor{
		config:     config,
		summarizer: NewSummarizer(),
		sorter:     NewArticleSorter(nil), // No relevance scorer needed for basic sorting
		converter: NewConverter(ConverterConfig{
			MaxSummaryLength: 1000,
			MaxTitleLength:   500,
			MaxContentLength: 50000,
		}),
	}
}

// ProcessArticles processes a slice of articles with various enhancements
func (p *Processor) ProcessArticles(ctx context.Context, articles []models.Article, options ProcessOptions) ([]models.Article, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(articles) == 0 {
		return articles, nil
	}

	// Apply processing timeout
	ctx, cancel := context.WithTimeout(ctx, p.config.ProcessingTimeout)
	defer cancel()

	var processed []models.Article
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process articles in batches for better performance
	batchSize := min(p.config.MaxConcurrency, len(articles))
	semaphore := make(chan struct{}, batchSize)

	for _, article := range articles {
		wg.Add(1)
		go func(a models.Article) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			select {
			case <-ctx.Done():
				return
			default:
				// Process single article
				processedArticle := p.processSingleArticle(ctx, a, options)

				mu.Lock()
				processed = append(processed, processedArticle)
				mu.Unlock()
			}
		}(article)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return processed, fmt.Errorf("processing timeout: %v", ctx.Err())
	}

	// Apply sorting if enabled and requested
	if p.config.EnableSorting && options.SortBy != "" {
		// Convert []models.Article to []*models.Article for sorter interface
		articlePtrs := make([]*models.Article, len(processed))
		for i := range processed {
			articlePtrs[i] = &processed[i]
		}

		sortedPtrs := p.sorter.SortArticles(articlePtrs)

		// Convert back to []models.Article
		processed = make([]models.Article, len(sortedPtrs))
		for i, ptr := range sortedPtrs {
			processed[i] = *ptr
		}
	}

	// Apply limit if specified
	if options.Limit > 0 && len(processed) > options.Limit {
		processed = processed[:options.Limit]
	}

	return processed, nil
}

// ProcessRepositories processes a slice of repositories
func (p *Processor) ProcessRepositories(ctx context.Context, repos []models.Repository, options ProcessOptions) ([]models.Repository, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(repos) == 0 {
		return repos, nil
	}

	// Calculate trend scores for each repository
	for i := range repos {
		repos[i].CalculateTrendScore()
	}

	// Sort if requested
	if options.SortBy != "" {
		// Note: Repository sorting would be implemented in a separate sorter
		// For now, we'll sort by trend score as default
		repos = p.sortRepositoriesByTrendScore(repos, options.SortOrder == "desc")
	}

	// Apply limit
	if options.Limit > 0 && len(repos) > options.Limit {
		repos = repos[:options.Limit]
	}

	return repos, nil
}

// processSingleArticle processes a single article
func (p *Processor) processSingleArticle(ctx context.Context, article models.Article, options ProcessOptions) models.Article {
	// Generate summary if enabled and not already present
	if p.config.EnableSummarization && article.Summary == "" && article.Content != "" {
		if summary, err := p.summarizer.GenerateSummary(article.Content); err == nil {
			article.Summary = summary
		}
	}

	// Calculate relevance score
	if options.Query != "" {
		article.Relevance = p.CalculateFrontendRelevance(article, options.Query)
	}

	// Extract keywords if not present
	if len(article.Tags) == 0 && article.Content != "" {
		keywords := p.summarizer.extractKeywords(article.Content)
		// Convert map[string]int to []string
		var tags []string
		for keyword := range keywords {
			tags = append(tags, keyword)
		}
		// Limit to top 5 keywords
		if len(tags) > 5 {
			tags = tags[:5]
		}
		article.Tags = tags
	}

	return article
}

// CalculateFrontendRelevance calculates how relevant an article is to frontend development
func (p *Processor) CalculateFrontendRelevance(article models.Article, query string) float64 {
	score := 0.0

	// Frontend-specific keywords and their weights
	frontendKeywords := map[string]float64{
		"react":      0.15,
		"vue":        0.15,
		"angular":    0.15,
		"javascript": 0.12,
		"typescript": 0.12,
		"css":        0.10,
		"html":       0.08,
		"frontend":   0.10,
		"ui":         0.08,
		"ux":         0.06,
		"responsive": 0.07,
		"webpack":    0.05,
		"babel":      0.04,
		"npm":        0.04,
		"yarn":       0.04,
		"nextjs":     0.08,
		"nuxt":       0.08,
		"svelte":     0.10,
		"tailwind":   0.06,
		"bootstrap":  0.05,
	}

	title := article.Title
	content := article.Content
	tags := article.Tags

	// Check title (higher weight)
	for keyword, weight := range frontendKeywords {
		if containsIgnoreCase(title, keyword) {
			score += weight * 1.5 // Title matches get bonus
		}
	}

	// Check content
	for keyword, weight := range frontendKeywords {
		if containsIgnoreCase(content, keyword) {
			score += weight
		}
	}

	// Check tags
	for _, tag := range tags {
		if weight, exists := frontendKeywords[tag]; exists {
			score += weight * 1.2 // Tag matches get slight bonus
		}
	}

	// Query-specific relevance
	if query != "" {
		if containsIgnoreCase(title, query) {
			score += 0.3
		}
		if containsIgnoreCase(content, query) {
			score += 0.2
		}
	}

	// Source bonus (trusted frontend sources)
	frontendSources := map[string]float64{
		"css-tricks.com":   0.1,
		"dev.to":           0.08,
		"medium.com":       0.06,
		"hackernews":       0.05,
		"smashingmagazine": 0.1,
		"a11yproject":      0.08,
	}

	for source, bonus := range frontendSources {
		if containsIgnoreCase(article.Source, source) {
			score += bonus
			break
		}
	}

	// Normalize score to 0-1 range
	return minFloat64(score, 1.0)
}

// sortRepositoriesByTrendScore sorts repositories by their trend score
func (p *Processor) sortRepositoriesByTrendScore(repos []models.Repository, descending bool) []models.Repository {
	sorted := make([]models.Repository, len(repos))
	copy(sorted, repos)

	if descending {
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].TrendScore < sorted[j].TrendScore {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	} else {
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].TrendScore > sorted[j].TrendScore {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	}

	return sorted
}

// ProcessOptions defines options for processing
type ProcessOptions struct {
	Query     string `json:"query"`
	SortBy    string `json:"sortBy"`
	SortOrder string `json:"sortOrder"`
	Limit     int    `json:"limit"`
}

// GetStats returns processor statistics
func (p *Processor) GetStats() ProcessorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProcessorStats{
		ProcessedArticles:     0, // Would be tracked in implementation
		ProcessedRepositories: 0, // Would be tracked in implementation
		AverageProcessingTime: 0, // Would be tracked in implementation
		CacheHitRate:          0, // Would be tracked if caching is implemented
	}
}

// ProcessorStats holds processor performance statistics
type ProcessorStats struct {
	ProcessedArticles     int           `json:"processedArticles"`
	ProcessedRepositories int           `json:"processedRepositories"`
	AverageProcessingTime time.Duration `json:"averageProcessingTime"`
	CacheHitRate          float64       `json:"cacheHitRate"`
}

// Helper functions
func containsIgnoreCase(text, substr string) bool {
	// Simple case-insensitive contains check
	return len(text) > 0 && len(substr) > 0 &&
		findSubstring(toLower(text), toLower(substr))
}

func findSubstring(text, substr string) bool {
	if len(substr) > len(text) {
		return false
	}

	for i := 0; i <= len(text)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if text[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + 32
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// min function for Go versions that don't have it built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
