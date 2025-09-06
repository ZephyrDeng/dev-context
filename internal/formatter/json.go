package formatter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"frontend-news-mcp/internal/models"
)

// JSONFormatter implements the JSON output format with proper structure and indentation
type JSONFormatter struct {
	config *Config
}

// NewJSONFormatter creates a new JSON formatter with the given configuration
func NewJSONFormatter(config *Config) *JSONFormatter {
	if config == nil {
		config = DefaultConfig()
	}
	return &JSONFormatter{config: config}
}

// FormatArticles formats a slice of articles as JSON
func (jf *JSONFormatter) FormatArticles(articles []models.Article) (string, error) {
	if len(articles) == 0 {
		return "[]", nil
	}

	// Sort articles according to configuration
	sortedArticles := jf.sortArticles(articles)

	// Convert to JSON format
	jsonArticles := jf.convertArticlesToJSON(sortedArticles)

	// Marshal to JSON with proper indentation
	var data []byte
	var err error
	
	if jf.config.CompactOutput {
		data, err = json.Marshal(jsonArticles)
	} else {
		data, err = json.MarshalIndent(jsonArticles, "", jf.config.Indent)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to marshal articles to JSON: %w", err)
	}

	return string(data), nil
}

// FormatRepositories formats a slice of repositories as JSON
func (jf *JSONFormatter) FormatRepositories(repositories []models.Repository) (string, error) {
	if len(repositories) == 0 {
		return "[]", nil
	}

	// Sort repositories according to configuration
	sortedRepos := jf.sortRepositories(repositories)

	// Convert to JSON format
	jsonRepos := jf.convertRepositoriesToJSON(sortedRepos)

	// Marshal to JSON with proper indentation
	var data []byte
	var err error
	
	if jf.config.CompactOutput {
		data, err = json.Marshal(jsonRepos)
	} else {
		data, err = json.MarshalIndent(jsonRepos, "", jf.config.Indent)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to marshal repositories to JSON: %w", err)
	}

	return string(data), nil
}

// FormatMixed formats both articles and repositories in a unified JSON output
func (jf *JSONFormatter) FormatMixed(articles []models.Article, repositories []models.Repository) (string, error) {
	result := map[string]interface{}{
		"articles":     jf.convertArticlesToJSON(jf.sortArticles(articles)),
		"repositories": jf.convertRepositoriesToJSON(jf.sortRepositories(repositories)),
		"summary": map[string]interface{}{
			"total_articles":     len(articles),
			"total_repositories": len(repositories),
			"timestamp":          time.Now().Format(jf.config.DateFormat),
		},
	}

	// Marshal to JSON with proper indentation
	var data []byte
	var err error
	
	if jf.config.CompactOutput {
		data, err = json.Marshal(result)
	} else {
		data, err = json.MarshalIndent(result, "", jf.config.Indent)
	}
	
	if err != nil {
		return "", fmt.Errorf("failed to marshal mixed content to JSON: %w", err)
	}

	return string(data), nil
}

// GetSupportedFormats returns the formats supported by this formatter
func (jf *JSONFormatter) GetSupportedFormats() []OutputFormat {
	return []OutputFormat{FormatJSON}
}

// convertArticlesToJSON converts article models to JSON-serializable format
func (jf *JSONFormatter) convertArticlesToJSON(articles []models.Article) []map[string]interface{} {
	jsonArticles := make([]map[string]interface{}, len(articles))

	for i, article := range articles {
		jsonArticle := map[string]interface{}{
			"id":          article.ID,
			"title":       article.Title,
			"url":         article.URL,
			"source":      article.Source,
			"sourceType":  article.SourceType,
			"publishedAt": formatTimestamp(article.PublishedAt, jf.config.DateFormat),
			"tags":        article.Tags,
			"relevance":   article.Relevance,
			"quality":     article.Quality,
		}

		// Add summary with length limit
		summary := article.Summary
		if jf.config.MaxSummaryLength > 0 && len(summary) > jf.config.MaxSummaryLength {
			summary = truncateText(summary, jf.config.MaxSummaryLength)
		}
		jsonArticle["summary"] = summary

		// Add content if requested
		if jf.config.IncludeContent {
			jsonArticle["content"] = article.Content
		}

		// Add metadata if requested
		if jf.config.IncludeMetadata && len(article.Metadata) > 0 {
			jsonArticle["metadata"] = article.Metadata
		}

		jsonArticles[i] = jsonArticle
	}

	return jsonArticles
}

// convertRepositoriesToJSON converts repository models to JSON-serializable format
func (jf *JSONFormatter) convertRepositoriesToJSON(repositories []models.Repository) []map[string]interface{} {
	jsonRepos := make([]map[string]interface{}, len(repositories))

	for i, repo := range repositories {
		jsonRepo := map[string]interface{}{
			"id":         repo.ID,
			"name":       repo.Name,
			"fullName":   repo.FullName,
			"url":        repo.URL,
			"language":   repo.Language,
			"stars":      repo.Stars,
			"forks":      repo.Forks,
			"trendScore": repo.TrendScore,
			"updatedAt":  formatTimestamp(repo.UpdatedAt, jf.config.DateFormat),
		}

		// Add description with length limit
		description := repo.Description
		if jf.config.MaxSummaryLength > 0 && len(description) > jf.config.MaxSummaryLength {
			description = truncateText(description, jf.config.MaxSummaryLength)
		}
		jsonRepo["description"] = description

		jsonRepos[i] = jsonRepo
	}

	return jsonRepos
}

// sortArticles sorts articles according to the configuration
func (jf *JSONFormatter) sortArticles(articles []models.Article) []models.Article {
	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Article, len(articles))
	copy(sorted, articles)

	switch jf.config.SortBy {
	case "relevance":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].Relevance < sorted[j].Relevance
			}
			return sorted[i].Relevance > sorted[j].Relevance
		})
	case "quality":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].Quality < sorted[j].Quality
			}
			return sorted[i].Quality > sorted[j].Quality
		})
	case "date":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].PublishedAt.Before(sorted[j].PublishedAt)
			}
			return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return strings.ToLower(sorted[i].Title) < strings.ToLower(sorted[j].Title)
			}
			return strings.ToLower(sorted[i].Title) > strings.ToLower(sorted[j].Title)
		})
	default:
		// Default to relevance sorting
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].Relevance < sorted[j].Relevance
			}
			return sorted[i].Relevance > sorted[j].Relevance
		})
	}

	return sorted
}

// sortRepositories sorts repositories according to the configuration
func (jf *JSONFormatter) sortRepositories(repositories []models.Repository) []models.Repository {
	// Create a copy to avoid modifying the original slice
	sorted := make([]models.Repository, len(repositories))
	copy(sorted, repositories)

	switch jf.config.SortBy {
	case "stars":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].Stars < sorted[j].Stars
			}
			return sorted[i].Stars > sorted[j].Stars
		})
	case "forks":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].Forks < sorted[j].Forks
			}
			return sorted[i].Forks > sorted[j].Forks
		})
	case "date":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].UpdatedAt.Before(sorted[j].UpdatedAt)
			}
			return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
		})
	case "title":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
			}
			return strings.ToLower(sorted[i].Name) > strings.ToLower(sorted[j].Name)
		})
	case "trendScore":
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].TrendScore < sorted[j].TrendScore
			}
			return sorted[i].TrendScore > sorted[j].TrendScore
		})
	default:
		// Default to trend score sorting
		sort.Slice(sorted, func(i, j int) bool {
			if jf.config.SortOrder == "asc" {
				return sorted[i].TrendScore < sorted[j].TrendScore
			}
			return sorted[i].TrendScore > sorted[j].TrendScore
		})
	}

	return sorted
}