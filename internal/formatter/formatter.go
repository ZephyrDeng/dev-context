package formatter

import (
	"strings"
	"time"

	"frontend-news-mcp/internal/models"
)

// OutputFormat represents the supported output formats
type OutputFormat string

const (
	FormatJSON     OutputFormat = "json"
	FormatMarkdown OutputFormat = "markdown"
	FormatText     OutputFormat = "text"
)

// Config represents configuration options for formatting
type Config struct {
	// Format specifies the output format (json, markdown, text)
	Format OutputFormat `json:"format"`
	
	// Indent specifies indentation for structured formats (JSON)
	Indent string `json:"indent"`
	
	// DateFormat specifies the date format string
	DateFormat string `json:"dateFormat"`
	
	// IncludeMetadata determines whether to include metadata in output
	IncludeMetadata bool `json:"includeMetadata"`
	
	// IncludeContent determines whether to include full content (can be large)
	IncludeContent bool `json:"includeContent"`
	
	// MaxSummaryLength limits the length of summaries in output
	MaxSummaryLength int `json:"maxSummaryLength"`
	
	// SortBy specifies how to sort results (relevance, quality, date, title)
	SortBy string `json:"sortBy"`
	
	// SortOrder specifies sort direction (asc, desc)
	SortOrder string `json:"sortOrder"`
	
	// EnableLinks determines whether to make URLs clickable in supported formats
	EnableLinks bool `json:"enableLinks"`
	
	// CompactOutput reduces whitespace for smaller output
	CompactOutput bool `json:"compactOutput"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Format:           FormatJSON,
		Indent:          "  ",
		DateFormat:      "2006-01-02 15:04:05",
		IncludeMetadata: false,
		IncludeContent:  false,
		MaxSummaryLength: 150,
		SortBy:          "relevance",
		SortOrder:       "desc",
		EnableLinks:     true,
		CompactOutput:   false,
	}
}

// Formatter defines the interface for all formatters
type Formatter interface {
	// FormatArticles formats a slice of articles according to the configuration
	FormatArticles(articles []models.Article) (string, error)
	
	// FormatRepositories formats a slice of repositories according to the configuration
	FormatRepositories(repositories []models.Repository) (string, error)
	
	// FormatMixed formats both articles and repositories in a unified output
	FormatMixed(articles []models.Article, repositories []models.Repository) (string, error)
	
	// GetSupportedFormats returns the formats supported by this formatter
	GetSupportedFormats() []OutputFormat
}

// FormatterFactory creates formatters based on configuration
type FormatterFactory struct {
	config *Config
}

// NewFormatterFactory creates a new formatter factory with the given configuration
func NewFormatterFactory(config *Config) *FormatterFactory {
	if config == nil {
		config = DefaultConfig()
	}
	return &FormatterFactory{config: config}
}

// CreateFormatter creates a formatter based on the configured format
func (ff *FormatterFactory) CreateFormatter() (Formatter, error) {
	switch ff.config.Format {
	case FormatJSON:
		return NewJSONFormatter(ff.config), nil
	case FormatMarkdown:
		return NewMarkdownFormatter(ff.config), nil
	case FormatText:
		return NewTextFormatter(ff.config), nil
	default:
		return NewJSONFormatter(ff.config), nil
	}
}

// SetFormat updates the output format in the configuration
func (ff *FormatterFactory) SetFormat(format OutputFormat) {
	ff.config.Format = format
}

// GetConfig returns a copy of the current configuration
func (ff *FormatterFactory) GetConfig() Config {
	return *ff.config
}

// UpdateConfig updates the factory configuration
func (ff *FormatterFactory) UpdateConfig(config *Config) {
	if config != nil {
		ff.config = config
	}
}

// BatchFormatter handles formatting of large datasets with performance optimizations
type BatchFormatter struct {
	formatter Formatter
	config    *Config
	batchSize int
}

// NewBatchFormatter creates a new batch formatter
func NewBatchFormatter(formatter Formatter, config *Config) *BatchFormatter {
	if config == nil {
		config = DefaultConfig()
	}
	
	batchSize := 100 // Default batch size for performance
	if config.CompactOutput {
		batchSize = 500 // Larger batches for compact output
	}
	
	return &BatchFormatter{
		formatter: formatter,
		config:    config,
		batchSize: batchSize,
	}
}

// FormatArticlesBatch formats articles in batches for better performance
func (bf *BatchFormatter) FormatArticlesBatch(articles []models.Article) (string, error) {
	if len(articles) <= bf.batchSize {
		return bf.formatter.FormatArticles(articles)
	}
	
	var results []string
	for i := 0; i < len(articles); i += bf.batchSize {
		end := i + bf.batchSize
		if end > len(articles) {
			end = len(articles)
		}
		
		batch := articles[i:end]
		result, err := bf.formatter.FormatArticles(batch)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}
	
	// Combine batch results based on format
	return bf.combineBatchResults(results)
}

// FormatRepositoriesBatch formats repositories in batches for better performance
func (bf *BatchFormatter) FormatRepositoriesBatch(repositories []models.Repository) (string, error) {
	if len(repositories) <= bf.batchSize {
		return bf.formatter.FormatRepositories(repositories)
	}
	
	var results []string
	for i := 0; i < len(repositories); i += bf.batchSize {
		end := i + bf.batchSize
		if end > len(repositories) {
			end = len(repositories)
		}
		
		batch := repositories[i:end]
		result, err := bf.formatter.FormatRepositories(batch)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}
	
	// Combine batch results based on format
	return bf.combineBatchResults(results)
}

// combineBatchResults combines multiple batch results into a single output
func (bf *BatchFormatter) combineBatchResults(results []string) (string, error) {
	switch bf.config.Format {
	case FormatJSON:
		return bf.combineJSONResults(results)
	case FormatMarkdown:
		return bf.combineMarkdownResults(results)
	case FormatText:
		return bf.combineTextResults(results)
	default:
		return bf.combineJSONResults(results)
	}
}

// combineJSONResults combines JSON batch results
func (bf *BatchFormatter) combineJSONResults(results []string) (string, error) {
	if len(results) == 0 {
		return "[]", nil
	}
	if len(results) == 1 {
		return results[0], nil
	}
	
	// For JSON, we need to merge arrays properly
	combined := "["
	for i, result := range results {
		if i > 0 {
			combined += ","
		}
		// Remove outer brackets and add content
		if len(result) > 2 {
			combined += result[1 : len(result)-1]
		}
	}
	combined += "]"
	
	return combined, nil
}

// combineMarkdownResults combines Markdown batch results
func (bf *BatchFormatter) combineMarkdownResults(results []string) (string, error) {
	combined := ""
	for i, result := range results {
		if i > 0 {
			combined += "\n\n---\n\n"
		}
		combined += result
	}
	return combined, nil
}

// combineTextResults combines plain text batch results
func (bf *BatchFormatter) combineTextResults(results []string) (string, error) {
	combined := ""
	for i, result := range results {
		if i > 0 {
			combined += "\n" + strings.Repeat("-", 80) + "\n"
		}
		combined += result
	}
	return combined, nil
}

// formatTimestamp formats a timestamp according to the configured date format
func formatTimestamp(t time.Time, format string) string {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	return t.Format(format)
}

// truncateText truncates text to the specified length while preserving word boundaries
func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	
	if maxLength <= 0 {
		return ""
	}
	
	// Find the last space within the limit to avoid cutting words
	truncated := text[:maxLength]
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	
	return truncated + "..."
}