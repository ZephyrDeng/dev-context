package formatter

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/models"
)

// MarkdownFormatter implements the Markdown output format with readable layout and links
type MarkdownFormatter struct {
	config *Config
}

// NewMarkdownFormatter creates a new Markdown formatter with the given configuration
func NewMarkdownFormatter(config *Config) *MarkdownFormatter {
	if config == nil {
		config = DefaultConfig()
	}
	return &MarkdownFormatter{config: config}
}

// FormatArticles formats a slice of articles as Markdown
func (mf *MarkdownFormatter) FormatArticles(articles []models.Article) (string, error) {
	if len(articles) == 0 {
		return "# No Articles Found\n\nNo articles to display.", nil
	}

	var md strings.Builder

	// Header
	md.WriteString("# Articles\n\n")
	md.WriteString(fmt.Sprintf("*Generated on %s*\n\n", time.Now().Format(mf.config.DateFormat)))
	md.WriteString(fmt.Sprintf("**Total Articles:** %d\n\n", len(articles)))
	md.WriteString("---\n\n")

	// Sort articles according to configuration
	sortedArticles := mf.sortArticles(articles)

	// Format each article
	for i, article := range sortedArticles {
		if i > 0 {
			md.WriteString("\n---\n\n")
		}
		mf.formatSingleArticle(&md, article, i+1)
	}

	return md.String(), nil
}

// FormatRepositories formats a slice of repositories as Markdown
func (mf *MarkdownFormatter) FormatRepositories(repositories []models.Repository) (string, error) {
	if len(repositories) == 0 {
		return "# No Repositories Found\n\nNo repositories to display.", nil
	}

	var md strings.Builder

	// Header
	md.WriteString("# Repositories\n\n")
	md.WriteString(fmt.Sprintf("*Generated on %s*\n\n", time.Now().Format(mf.config.DateFormat)))
	md.WriteString(fmt.Sprintf("**Total Repositories:** %d\n\n", len(repositories)))
	md.WriteString("---\n\n")

	// Sort repositories according to configuration
	sortedRepos := mf.sortRepositories(repositories)

	// Format each repository
	for i, repo := range sortedRepos {
		if i > 0 {
			md.WriteString("\n---\n\n")
		}
		mf.formatSingleRepository(&md, repo, i+1)
	}

	return md.String(), nil
}

// FormatMixed formats both articles and repositories in a unified Markdown output
func (mf *MarkdownFormatter) FormatMixed(articles []models.Article, repositories []models.Repository) (string, error) {
	var md strings.Builder

	// Header
	md.WriteString("# Development Context Report\n\n")
	md.WriteString(fmt.Sprintf("*Generated on %s*\n\n", time.Now().Format(mf.config.DateFormat)))

	// Summary
	md.WriteString("## Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Articles:** %d\n", len(articles)))
	md.WriteString(fmt.Sprintf("- **Repositories:** %d\n", len(repositories)))
	md.WriteString(fmt.Sprintf("- **Total Items:** %d\n\n", len(articles)+len(repositories)))

	// Table of Contents
	md.WriteString("## Table of Contents\n\n")
	if len(articles) > 0 {
		md.WriteString("- [Articles](#articles)\n")
	}
	if len(repositories) > 0 {
		md.WriteString("- [Repositories](#repositories)\n")
	}
	md.WriteString("\n")

	// Articles section
	if len(articles) > 0 {
		md.WriteString("## Articles\n\n")
		sortedArticles := mf.sortArticles(articles)

		for i, article := range sortedArticles {
			if i > 0 {
				md.WriteString("\n---\n\n")
			}
			mf.formatSingleArticle(&md, article, i+1)
		}
	}

	// Repositories section
	if len(repositories) > 0 {
		if len(articles) > 0 {
			md.WriteString("\n\n")
		}
		md.WriteString("## Repositories\n\n")
		sortedRepos := mf.sortRepositories(repositories)

		for i, repo := range sortedRepos {
			if i > 0 {
				md.WriteString("\n---\n\n")
			}
			mf.formatSingleRepository(&md, repo, i+1)
		}
	}

	return md.String(), nil
}

// GetSupportedFormats returns the formats supported by this formatter
func (mf *MarkdownFormatter) GetSupportedFormats() []OutputFormat {
	return []OutputFormat{FormatMarkdown}
}

// formatSingleArticle formats a single article as Markdown
func (mf *MarkdownFormatter) formatSingleArticle(md *strings.Builder, article models.Article, index int) {
	// Title with link
	title := mf.escapeMarkdown(article.Title)
	if mf.config.EnableLinks && article.URL != "" {
		md.WriteString(fmt.Sprintf("### %d. [%s](%s)\n\n", index, title, article.URL))
	} else {
		md.WriteString(fmt.Sprintf("### %d. %s\n\n", index, title))
	}

	// Metadata table
	md.WriteString("| Field | Value |\n")
	md.WriteString("|-------|-------|\n")
	md.WriteString(fmt.Sprintf("| **Source** | %s |\n", mf.escapeMarkdown(article.Source)))
	md.WriteString(fmt.Sprintf("| **Type** | %s |\n", article.SourceType))
	md.WriteString(fmt.Sprintf("| **Published** | %s |\n", formatTimestamp(article.PublishedAt, mf.config.DateFormat)))

	if article.Relevance > 0 {
		md.WriteString(fmt.Sprintf("| **Relevance** | %.1f%% |\n", article.Relevance*100))
	}
	if article.Quality > 0 {
		md.WriteString(fmt.Sprintf("| **Quality** | %.1f%% |\n", article.Quality*100))
	}

	if !mf.config.EnableLinks && article.URL != "" {
		md.WriteString(fmt.Sprintf("| **URL** | `%s` |\n", article.URL))
	}

	md.WriteString("\n")

	// Summary
	if article.Summary != "" {
		summary := article.Summary
		if mf.config.MaxSummaryLength > 0 && len(summary) > mf.config.MaxSummaryLength {
			summary = truncateText(summary, mf.config.MaxSummaryLength)
		}
		md.WriteString("**Summary:**\n")
		md.WriteString(fmt.Sprintf("%s\n\n", mf.escapeMarkdown(summary)))
	}

	// Tags
	if len(article.Tags) > 0 {
		md.WriteString("**Tags:** ")
		for i, tag := range article.Tags {
			if i > 0 {
				md.WriteString(", ")
			}
			md.WriteString(fmt.Sprintf("`%s`", mf.escapeMarkdown(tag)))
		}
		md.WriteString("\n\n")
	}

	// Content (if requested)
	if mf.config.IncludeContent && article.Content != "" {
		md.WriteString("**Content:**\n\n")
		content := mf.escapeMarkdown(article.Content)
		if !mf.config.CompactOutput {
			md.WriteString("```\n")
			md.WriteString(content)
			md.WriteString("\n```\n\n")
		} else {
			md.WriteString(fmt.Sprintf("%s\n\n", content))
		}
	}

	// Metadata (if requested)
	if mf.config.IncludeMetadata && len(article.Metadata) > 0 {
		md.WriteString("**Additional Metadata:**\n\n")
		for key, value := range article.Metadata {
			md.WriteString(fmt.Sprintf("- **%s:** %v\n", mf.escapeMarkdown(key), value))
		}
		md.WriteString("\n")
	}
}

// formatSingleRepository formats a single repository as Markdown
func (mf *MarkdownFormatter) formatSingleRepository(md *strings.Builder, repo models.Repository, index int) {
	// Title with link
	title := mf.escapeMarkdown(repo.FullName)
	if mf.config.EnableLinks && repo.URL != "" {
		md.WriteString(fmt.Sprintf("### %d. [%s](%s)\n\n", index, title, repo.URL))
	} else {
		md.WriteString(fmt.Sprintf("### %d. %s\n\n", index, title))
	}

	// Metadata table
	md.WriteString("| Field | Value |\n")
	md.WriteString("|-------|-------|\n")

	if repo.Language != "" {
		md.WriteString(fmt.Sprintf("| **Language** | %s |\n", mf.escapeMarkdown(repo.Language)))
	}

	md.WriteString(fmt.Sprintf("| **Stars** | ⭐ %d |\n", repo.Stars))
	md.WriteString(fmt.Sprintf("| **Forks** | 🍴 %d |\n", repo.Forks))
	md.WriteString(fmt.Sprintf("| **Trend Score** | %.1f%% |\n", repo.TrendScore*100))
	md.WriteString(fmt.Sprintf("| **Updated** | %s |\n", formatTimestamp(repo.UpdatedAt, mf.config.DateFormat)))

	if !mf.config.EnableLinks && repo.URL != "" {
		md.WriteString(fmt.Sprintf("| **URL** | `%s` |\n", repo.URL))
	}

	md.WriteString("\n")

	// Description
	if repo.Description != "" {
		description := repo.Description
		if mf.config.MaxSummaryLength > 0 && len(description) > mf.config.MaxSummaryLength {
			description = truncateText(description, mf.config.MaxSummaryLength)
		}
		md.WriteString("**Description:**\n")
		md.WriteString(fmt.Sprintf("%s\n\n", mf.escapeMarkdown(description)))
	}

	// Repository stats (visual indicators)
	md.WriteString("**Stats:**\n")

	// Popularity indicator
	popularityTier := repo.GetPopularityTier()
	switch popularityTier {
	case "viral":
		md.WriteString("🔥 **Viral** repository with massive community engagement\n")
	case "very_popular":
		md.WriteString("🌟 **Very Popular** repository with strong community support\n")
	case "popular":
		md.WriteString("⭐ **Popular** repository with good traction\n")
	case "gaining_traction":
		md.WriteString("📈 **Gaining Traction** - emerging repository\n")
	default:
		md.WriteString("🌱 **New or Niche** repository\n")
	}

	// Activity indicator
	activityLevel := repo.GetActivityLevel()
	switch activityLevel {
	case "very_active":
		md.WriteString("🚀 **Very Active** - updated within the last week\n")
	case "active":
		md.WriteString("✅ **Active** - updated within the last month\n")
	case "moderate":
		md.WriteString("🔄 **Moderate** activity - updated within 3 months\n")
	default:
		md.WriteString("💤 **Inactive** - not recently updated\n")
	}

	md.WriteString("\n")
}

// escapeMarkdown escapes special Markdown characters
func (mf *MarkdownFormatter) escapeMarkdown(text string) string {
	// Escape HTML first to prevent XSS
	text = html.EscapeString(text)

	// Escape common Markdown special characters
	replacements := []struct {
		old, new string
	}{
		{"\\", "\\\\"}, // Backslash (must be first)
		{"*", "\\*"},   // Asterisk
		{"_", "\\_"},   // Underscore
		{"`", "\\`"},   // Backtick
		{"[", "\\["},   // Square bracket
		{"]", "\\]"},   // Square bracket
		{"(", "\\("},   // Parenthesis
		{")", "\\)"},   // Parenthesis
		{"{", "\\{"},   // Curly brace
		{"}", "\\}"},   // Curly brace
		{"#", "\\#"},   // Hash
		{"+", "\\+"},   // Plus
		{"-", "\\-"},   // Minus
		{".", "\\."},   // Dot
		{"!", "\\!"},   // Exclamation
		{"|", "\\|"},   // Pipe
	}

	for _, r := range replacements {
		text = strings.ReplaceAll(text, r.old, r.new)
	}

	return text
}

// sortArticles sorts articles according to the configuration
func (mf *MarkdownFormatter) sortArticles(articles []models.Article) []models.Article {
	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Article, len(articles))
	copy(sorted, articles)

	switch mf.config.SortBy {
	case "relevance":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].Relevance < sorted[j].Relevance
			}
			return sorted[i].Relevance > sorted[j].Relevance
		})
	case "quality":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].Quality < sorted[j].Quality
			}
			return sorted[i].Quality > sorted[j].Quality
		})
	case "date":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].PublishedAt.Before(sorted[j].PublishedAt)
			}
			return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return strings.ToLower(sorted[i].Title) < strings.ToLower(sorted[j].Title)
			}
			return strings.ToLower(sorted[i].Title) > strings.ToLower(sorted[j].Title)
		})
	default:
		// Default to relevance sorting
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].Relevance < sorted[j].Relevance
			}
			return sorted[i].Relevance > sorted[j].Relevance
		})
	}

	return sorted
}

// sortRepositories sorts repositories according to the configuration
func (mf *MarkdownFormatter) sortRepositories(repositories []models.Repository) []models.Repository {
	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Repository, len(repositories))
	copy(sorted, repositories)

	switch mf.config.SortBy {
	case "stars":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].Stars < sorted[j].Stars
			}
			return sorted[i].Stars > sorted[j].Stars
		})
	case "forks":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].Forks < sorted[j].Forks
			}
			return sorted[i].Forks > sorted[j].Forks
		})
	case "date":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].UpdatedAt.Before(sorted[j].UpdatedAt)
			}
			return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
			}
			return strings.ToLower(sorted[i].Name) > strings.ToLower(sorted[j].Name)
		})
	case "trendScore":
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].TrendScore < sorted[j].TrendScore
			}
			return sorted[i].TrendScore > sorted[j].TrendScore
		})
	default:
		// Default to trend score sorting
		sort.Slice(sorted, func(i, j int) bool {
			if mf.config.SortOrder == "asc" {
				return sorted[i].TrendScore < sorted[j].TrendScore
			}
			return sorted[i].TrendScore > sorted[j].TrendScore
		})
	}

	return sorted
}
