package formatter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/models"
)

// TextFormatter implements the plain text output format for simple, readable output
type TextFormatter struct {
	config *Config
}

// NewTextFormatter creates a new text formatter with the given configuration
func NewTextFormatter(config *Config) *TextFormatter {
	if config == nil {
		config = DefaultConfig()
	}
	return &TextFormatter{config: config}
}

// FormatArticles formats a slice of articles as plain text
func (tf *TextFormatter) FormatArticles(articles []models.Article) (string, error) {
	if len(articles) == 0 {
		return "No articles found.\n", nil
	}

	var text strings.Builder

	// Header
	tf.writeHeader(&text, "ARTICLES", len(articles))

	// Sort articles according to configuration
	sortedArticles := tf.sortArticles(articles)

	// Format each article
	for i, article := range sortedArticles {
		if i > 0 {
			tf.writeSeparator(&text, tf.config.CompactOutput)
		}
		tf.formatSingleArticle(&text, article, i+1)
	}

	return text.String(), nil
}

// FormatRepositories formats a slice of repositories as plain text
func (tf *TextFormatter) FormatRepositories(repositories []models.Repository) (string, error) {
	if len(repositories) == 0 {
		return "No repositories found.\n", nil
	}

	var text strings.Builder

	// Header
	tf.writeHeader(&text, "REPOSITORIES", len(repositories))

	// Sort repositories according to configuration
	sortedRepos := tf.sortRepositories(repositories)

	// Format each repository
	for i, repo := range sortedRepos {
		if i > 0 {
			tf.writeSeparator(&text, tf.config.CompactOutput)
		}
		tf.formatSingleRepository(&text, repo, i+1)
	}

	return text.String(), nil
}

// FormatMixed formats both articles and repositories in a unified text output
func (tf *TextFormatter) FormatMixed(articles []models.Article, repositories []models.Repository) (string, error) {
	var text strings.Builder

	// Header
	tf.writeHeader(&text, "DEVELOPMENT CONTEXT REPORT", len(articles)+len(repositories))

	// Summary
	text.WriteString("SUMMARY:\n")
	text.WriteString(fmt.Sprintf("  Articles:     %d\n", len(articles)))
	text.WriteString(fmt.Sprintf("  Repositories: %d\n", len(repositories)))
	text.WriteString(fmt.Sprintf("  Total Items:  %d\n", len(articles)+len(repositories)))
	text.WriteString(fmt.Sprintf("  Generated:    %s\n", time.Now().Format(tf.config.DateFormat)))
	text.WriteString("\n")

	// Articles section
	if len(articles) > 0 {
		tf.writeSectionHeader(&text, "ARTICLES")

		sortedArticles := tf.sortArticles(articles)
		for i, article := range sortedArticles {
			if i > 0 {
				tf.writeSeparator(&text, tf.config.CompactOutput)
			}
			tf.formatSingleArticle(&text, article, i+1)
		}
	}

	// Repositories section
	if len(repositories) > 0 {
		if len(articles) > 0 {
			text.WriteString("\n")
		}
		tf.writeSectionHeader(&text, "REPOSITORIES")

		sortedRepos := tf.sortRepositories(repositories)
		for i, repo := range sortedRepos {
			if i > 0 {
				tf.writeSeparator(&text, tf.config.CompactOutput)
			}
			tf.formatSingleRepository(&text, repo, i+1)
		}
	}

	return text.String(), nil
}

// GetSupportedFormats returns the formats supported by this formatter
func (tf *TextFormatter) GetSupportedFormats() []OutputFormat {
	return []OutputFormat{FormatText}
}

// formatSingleArticle formats a single article as plain text
func (tf *TextFormatter) formatSingleArticle(text *strings.Builder, article models.Article, index int) {
	title := tf.cleanText(article.Title)

	if tf.config.CompactOutput {
		// Compact format - single line per article
		text.WriteString(fmt.Sprintf("%d. %s", index, title))
		if article.URL != "" {
			text.WriteString(fmt.Sprintf(" (%s)", article.URL))
		}
		text.WriteString(fmt.Sprintf(" [%s]", article.Source))
		if article.Relevance > 0 || article.Quality > 0 {
			text.WriteString(fmt.Sprintf(" R:%.1f Q:%.1f", article.Relevance*100, article.Quality*100))
		}
		text.WriteString("\n")

		if article.Summary != "" && !tf.config.CompactOutput {
			summary := article.Summary
			if tf.config.MaxSummaryLength > 0 && len(summary) > tf.config.MaxSummaryLength {
				summary = truncateText(summary, tf.config.MaxSummaryLength)
			}
			text.WriteString(fmt.Sprintf("   %s\n", tf.wrapText(tf.cleanText(summary), 77, "   ")))
		}
	} else {
		// Full format
		text.WriteString(fmt.Sprintf("[%d] %s\n", index, title))

		// Metadata
		text.WriteString(fmt.Sprintf("    Source:    %s (%s)\n", article.Source, article.SourceType))
		text.WriteString(fmt.Sprintf("    Published: %s\n", formatTimestamp(article.PublishedAt, tf.config.DateFormat)))

		if article.URL != "" {
			text.WriteString(fmt.Sprintf("    URL:       %s\n", article.URL))
		}

		if article.Relevance > 0 {
			text.WriteString(fmt.Sprintf("    Relevance: %.1f%%\n", article.Relevance*100))
		}

		if article.Quality > 0 {
			text.WriteString(fmt.Sprintf("    Quality:   %.1f%%\n", article.Quality*100))
		}

		// Summary
		if article.Summary != "" {
			summary := article.Summary
			if tf.config.MaxSummaryLength > 0 && len(summary) > tf.config.MaxSummaryLength {
				summary = truncateText(summary, tf.config.MaxSummaryLength)
			}
			text.WriteString("\n    Summary:\n")
			text.WriteString(fmt.Sprintf("    %s\n", tf.wrapText(tf.cleanText(summary), 76, "    ")))
		}

		// Tags
		if len(article.Tags) > 0 {
			text.WriteString(fmt.Sprintf("\n    Tags: %s\n", strings.Join(article.Tags, ", ")))
		}

		// Content (if requested)
		if tf.config.IncludeContent && article.Content != "" {
			text.WriteString("\n    Content:\n")
			content := tf.cleanText(article.Content)
			text.WriteString(fmt.Sprintf("    %s\n", tf.wrapText(content, 76, "    ")))
		}

		// Metadata (if requested)
		if tf.config.IncludeMetadata && len(article.Metadata) > 0 {
			text.WriteString("\n    Additional Info:\n")
			for key, value := range article.Metadata {
				text.WriteString(fmt.Sprintf("      %s: %v\n", key, value))
			}
		}
	}

	text.WriteString("\n")
}

// formatSingleRepository formats a single repository as plain text
func (tf *TextFormatter) formatSingleRepository(text *strings.Builder, repo models.Repository, index int) {
	name := tf.cleanText(repo.FullName)

	if tf.config.CompactOutput {
		// Compact format - single line per repository
		text.WriteString(fmt.Sprintf("%d. %s", index, name))
		if repo.Language != "" {
			text.WriteString(fmt.Sprintf(" (%s)", repo.Language))
		}
		text.WriteString(fmt.Sprintf(" ⭐%d 🍴%d", repo.Stars, repo.Forks))
		if repo.TrendScore > 0 {
			text.WriteString(fmt.Sprintf(" T:%.1f", repo.TrendScore*100))
		}
		text.WriteString("\n")

		if repo.Description != "" && len(repo.Description) < 100 {
			text.WriteString(fmt.Sprintf("   %s\n", tf.cleanText(repo.Description)))
		}
	} else {
		// Full format
		text.WriteString(fmt.Sprintf("[%d] %s\n", index, name))

		// Metadata
		if repo.Language != "" {
			text.WriteString(fmt.Sprintf("    Language:    %s\n", repo.Language))
		}
		text.WriteString(fmt.Sprintf("    Stars:       %d\n", repo.Stars))
		text.WriteString(fmt.Sprintf("    Forks:       %d\n", repo.Forks))
		text.WriteString(fmt.Sprintf("    Trend Score: %.1f%%\n", repo.TrendScore*100))
		text.WriteString(fmt.Sprintf("    Updated:     %s\n", formatTimestamp(repo.UpdatedAt, tf.config.DateFormat)))

		if repo.URL != "" {
			text.WriteString(fmt.Sprintf("    URL:         %s\n", repo.URL))
		}

		// Description
		if repo.Description != "" {
			description := repo.Description
			if tf.config.MaxSummaryLength > 0 && len(description) > tf.config.MaxSummaryLength {
				description = truncateText(description, tf.config.MaxSummaryLength)
			}
			text.WriteString("\n    Description:\n")
			text.WriteString(fmt.Sprintf("    %s\n", tf.wrapText(tf.cleanText(description), 76, "    ")))
		}

		// Stats
		popularityTier := repo.GetPopularityTier()
		activityLevel := repo.GetActivityLevel()

		text.WriteString(fmt.Sprintf("\n    Status: %s, %s\n",
			tf.formatPopularityTier(popularityTier),
			tf.formatActivityLevel(activityLevel)))
	}

	text.WriteString("\n")
}

// writeHeader writes a formatted header
func (tf *TextFormatter) writeHeader(text *strings.Builder, title string, count int) {
	if tf.config.CompactOutput {
		text.WriteString(fmt.Sprintf("%s (%d items)\n", title, count))
		text.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(tf.config.DateFormat)))
	} else {
		separator := strings.Repeat("=", len(title)+20)
		text.WriteString(fmt.Sprintf("%s\n", separator))
		text.WriteString(fmt.Sprintf("  %s (%d items)\n", title, count))
		text.WriteString(fmt.Sprintf("  Generated: %s\n", time.Now().Format(tf.config.DateFormat)))
		text.WriteString(fmt.Sprintf("%s\n\n", separator))
	}
}

// writeSectionHeader writes a section header
func (tf *TextFormatter) writeSectionHeader(text *strings.Builder, title string) {
	if tf.config.CompactOutput {
		text.WriteString(fmt.Sprintf("\n%s:\n", title))
	} else {
		separator := strings.Repeat("-", len(title)+4)
		text.WriteString(fmt.Sprintf("\n%s\n", separator))
		text.WriteString(fmt.Sprintf("  %s\n", title))
		text.WriteString(fmt.Sprintf("%s\n\n", separator))
	}
}

// writeSeparator writes a separator between items
func (tf *TextFormatter) writeSeparator(text *strings.Builder, compact bool) {
	if compact {
		// No separator for compact output
	} else {
		text.WriteString(strings.Repeat("-", 80) + "\n\n")
	}
}

// cleanText removes HTML tags and normalizes whitespace
func (tf *TextFormatter) cleanText(input string) string {
	// Remove HTML tags (simple approach)
	text := strings.ReplaceAll(input, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	// Normalize whitespace
	words := strings.Fields(text)
	return strings.Join(words, " ")
}

// wrapText wraps text at the specified width with given prefix
func (tf *TextFormatter) wrapText(text string, width int, prefix string) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentLength := 0

	for _, word := range words {
		wordLen := len(word)

		// If adding this word would exceed width, start a new line
		if currentLength > 0 && currentLength+1+wordLen > width {
			result.WriteString(currentLine.String())
			result.WriteString("\n")
			result.WriteString(prefix)
			currentLine.Reset()
			currentLength = 0
		}

		// Add word to current line
		if currentLength > 0 {
			currentLine.WriteString(" ")
			currentLength++
		}
		currentLine.WriteString(word)
		currentLength += wordLen
	}

	// Add the last line
	if currentLength > 0 {
		result.WriteString(currentLine.String())
	}

	return result.String()
}

// formatPopularityTier converts popularity tier to readable text
func (tf *TextFormatter) formatPopularityTier(tier string) string {
	switch tier {
	case "viral":
		return "Viral"
	case "very_popular":
		return "Very Popular"
	case "popular":
		return "Popular"
	case "gaining_traction":
		return "Gaining Traction"
	default:
		return "New/Niche"
	}
}

// formatActivityLevel converts activity level to readable text
func (tf *TextFormatter) formatActivityLevel(level string) string {
	switch level {
	case "very_active":
		return "Very Active"
	case "active":
		return "Active"
	case "moderate":
		return "Moderate Activity"
	default:
		return "Inactive"
	}
}

// sortArticles sorts articles according to the configuration
func (tf *TextFormatter) sortArticles(articles []models.Article) []models.Article {
	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Article, len(articles))
	copy(sorted, articles)

	switch tf.config.SortBy {
	case "relevance":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].Relevance < sorted[j].Relevance
			}
			return sorted[i].Relevance > sorted[j].Relevance
		})
	case "quality":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].Quality < sorted[j].Quality
			}
			return sorted[i].Quality > sorted[j].Quality
		})
	case "date":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].PublishedAt.Before(sorted[j].PublishedAt)
			}
			return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return strings.ToLower(sorted[i].Title) < strings.ToLower(sorted[j].Title)
			}
			return strings.ToLower(sorted[i].Title) > strings.ToLower(sorted[j].Title)
		})
	default:
		// Default to relevance sorting
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].Relevance < sorted[j].Relevance
			}
			return sorted[i].Relevance > sorted[j].Relevance
		})
	}

	return sorted
}

// sortRepositories sorts repositories according to the configuration
func (tf *TextFormatter) sortRepositories(repositories []models.Repository) []models.Repository {
	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Repository, len(repositories))
	copy(sorted, repositories)

	switch tf.config.SortBy {
	case "stars":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].Stars < sorted[j].Stars
			}
			return sorted[i].Stars > sorted[j].Stars
		})
	case "forks":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].Forks < sorted[j].Forks
			}
			return sorted[i].Forks > sorted[j].Forks
		})
	case "date":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].UpdatedAt.Before(sorted[j].UpdatedAt)
			}
			return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
			}
			return strings.ToLower(sorted[i].Name) > strings.ToLower(sorted[j].Name)
		})
	case "trendScore":
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].TrendScore < sorted[j].TrendScore
			}
			return sorted[i].TrendScore > sorted[j].TrendScore
		})
	default:
		// Default to trend score sorting
		sort.Slice(sorted, func(i, j int) bool {
			if tf.config.SortOrder == "asc" {
				return sorted[i].TrendScore < sorted[j].TrendScore
			}
			return sorted[i].TrendScore > sorted[j].TrendScore
		})
	}

	return sorted
}
