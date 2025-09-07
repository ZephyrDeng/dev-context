package formatter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"frontend-news-mcp/internal/models"
)

// Test data helpers
func createTestArticle(id, title, url, source string) models.Article {
	article := models.NewArticle(title, url, source, "rss")
	article.ID = id
	article.SetSummary("This is a test summary for the article.")
	article.AddTags("go", "testing", "development")
	article.Relevance = 0.85
	article.Quality = 0.75
	article.Content = "This is the full content of the test article. It contains more detailed information."
	article.SetMetadata("test_field", "test_value")
	article.SetMetadata("author", "Test Author")
	return *article
}

func createTestRepository(id, name, fullName, url string) models.Repository {
	repo := models.NewRepository(name, fullName, url)
	repo.ID = id
	repo.SetDescription("This is a test repository for unit testing.")
	repo.SetLanguage("Go")
	repo.UpdateStats(1250, 85, time.Now().Add(-24*time.Hour))
	return *repo
}

func createTestArticles() []models.Article {
	return []models.Article{
		createTestArticle("1", "Test Article 1", "https://example.com/article1", "Test Source 1"),
		createTestArticle("2", "Test Article 2", "https://example.com/article2", "Test Source 2"),
		createTestArticle("3", "Very Long Article Title That Should Be Handled Properly in All Formatters", "https://example.com/article3", "Test Source 3"),
	}
}

func createTestRepositories() []models.Repository {
	return []models.Repository{
		createTestRepository("1", "test-repo-1", "testuser/test-repo-1", "https://github.com/testuser/test-repo-1"),
		createTestRepository("2", "awesome-project", "testorg/awesome-project", "https://github.com/testorg/awesome-project"),
		createTestRepository("3", "utility-lib", "developer/utility-lib", "https://github.com/developer/utility-lib"),
	}
}

// Test Configuration
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.Format != FormatJSON {
		t.Errorf("Expected default format to be JSON, got %s", config.Format)
	}
	
	if config.Indent != "  " {
		t.Errorf("Expected default indent to be '  ', got '%s'", config.Indent)
	}
	
	if config.MaxSummaryLength != 150 {
		t.Errorf("Expected default max summary length to be 150, got %d", config.MaxSummaryLength)
	}
	
	if !config.EnableLinks {
		t.Error("Expected default EnableLinks to be true")
	}
}

func TestConfigCustomization(t *testing.T) {
	config := DefaultConfig()
	config.Format = FormatMarkdown
	config.CompactOutput = true
	config.MaxSummaryLength = 100
	config.SortBy = "title"
	config.SortOrder = "asc"
	
	if config.Format != FormatMarkdown {
		t.Errorf("Expected format to be Markdown, got %s", config.Format)
	}
	
	if !config.CompactOutput {
		t.Error("Expected CompactOutput to be true")
	}
	
	if config.MaxSummaryLength != 100 {
		t.Errorf("Expected max summary length to be 100, got %d", config.MaxSummaryLength)
	}
}

// Test Factory
func TestFormatterFactory(t *testing.T) {
	config := DefaultConfig()
	factory := NewFormatterFactory(config)
	
	// Test JSON formatter creation
	config.Format = FormatJSON
	factory.SetFormat(FormatJSON)
	formatter, err := factory.CreateFormatter()
	if err != nil {
		t.Fatalf("Failed to create JSON formatter: %v", err)
	}
	
	if _, ok := formatter.(*JSONFormatter); !ok {
		t.Error("Expected JSONFormatter instance")
	}
	
	// Test Markdown formatter creation
	config.Format = FormatMarkdown
	factory.SetFormat(FormatMarkdown)
	formatter, err = factory.CreateFormatter()
	if err != nil {
		t.Fatalf("Failed to create Markdown formatter: %v", err)
	}
	
	if _, ok := formatter.(*MarkdownFormatter); !ok {
		t.Error("Expected MarkdownFormatter instance")
	}
	
	// Test Text formatter creation
	config.Format = FormatText
	factory.SetFormat(FormatText)
	formatter, err = factory.CreateFormatter()
	if err != nil {
		t.Fatalf("Failed to create Text formatter: %v", err)
	}
	
	if _, ok := formatter.(*TextFormatter); !ok {
		t.Error("Expected TextFormatter instance")
	}
}

// Test JSON Formatter
func TestJSONFormatterArticles(t *testing.T) {
	config := DefaultConfig()
	config.Format = FormatJSON
	config.IncludeMetadata = true
	formatter := NewJSONFormatter(config)
	
	articles := createTestArticles()
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format articles as JSON: %v", err)
	}
	
	// Verify valid JSON
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}
	
	// Verify content
	if len(parsed) != len(articles) {
		t.Errorf("Expected %d articles in JSON, got %d", len(articles), len(parsed))
	}
	
	// Check first article
	firstArticle := parsed[0]
	if firstArticle["id"].(string) != "1" {
		t.Errorf("Expected first article ID to be '1', got '%s'", firstArticle["id"])
	}
	
	if firstArticle["title"].(string) != "Test Article 1" {
		t.Errorf("Expected first article title to be 'Test Article 1', got '%s'", firstArticle["title"])
	}
	
	// Check metadata inclusion
	if _, exists := firstArticle["metadata"]; !exists {
		t.Error("Expected metadata to be included in JSON output")
	}
}

func TestJSONFormatterRepositories(t *testing.T) {
	config := DefaultConfig()
	formatter := NewJSONFormatter(config)
	
	repositories := createTestRepositories()
	result, err := formatter.FormatRepositories(repositories)
	if err != nil {
		t.Fatalf("Failed to format repositories as JSON: %v", err)
	}
	
	// Verify valid JSON
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}
	
	// Verify content
	if len(parsed) != len(repositories) {
		t.Errorf("Expected %d repositories in JSON, got %d", len(repositories), len(parsed))
	}
	
	// Check repository fields
	firstRepo := parsed[0]
	if firstRepo["name"].(string) != "test-repo-1" {
		t.Errorf("Expected first repo name to be 'test-repo-1', got '%s'", firstRepo["name"])
	}
	
	if firstRepo["language"].(string) != "Go" {
		t.Errorf("Expected first repo language to be 'Go', got '%s'", firstRepo["language"])
	}
}

func TestJSONFormatterMixed(t *testing.T) {
	config := DefaultConfig()
	formatter := NewJSONFormatter(config)
	
	articles := createTestArticles()
	repositories := createTestRepositories()
	
	result, err := formatter.FormatMixed(articles, repositories)
	if err != nil {
		t.Fatalf("Failed to format mixed content as JSON: %v", err)
	}
	
	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}
	
	// Check structure
	if _, exists := parsed["articles"]; !exists {
		t.Error("Expected 'articles' field in mixed JSON output")
	}
	
	if _, exists := parsed["repositories"]; !exists {
		t.Error("Expected 'repositories' field in mixed JSON output")
	}
	
	if _, exists := parsed["summary"]; !exists {
		t.Error("Expected 'summary' field in mixed JSON output")
	}
}

func TestJSONFormatterCompactOutput(t *testing.T) {
	config := DefaultConfig()
	config.CompactOutput = true
	formatter := NewJSONFormatter(config)
	
	articles := createTestArticles()
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format articles as compact JSON: %v", err)
	}
	
	// Compact JSON should not contain newlines (except in string content)
	lines := strings.Split(result, "\n")
	if len(lines) > 1 {
		// Allow for single trailing newline
		if len(lines) == 2 && lines[1] == "" {
			// This is acceptable
		} else {
			t.Error("Compact JSON should not contain multiple lines")
		}
	}
}

// Test Markdown Formatter
func TestMarkdownFormatterArticles(t *testing.T) {
	config := DefaultConfig()
	config.EnableLinks = true
	formatter := NewMarkdownFormatter(config)
	
	articles := createTestArticles()
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format articles as Markdown: %v", err)
	}
	
	// Check for Markdown headers
	if !strings.Contains(result, "# Articles") {
		t.Error("Expected main header '# Articles' in Markdown output")
	}
	
	// Check for links
	if !strings.Contains(result, "[Test Article 1](https://example.com/article1)") {
		t.Error("Expected linked article title in Markdown output")
	}
	
	// Check for metadata table
	if !strings.Contains(result, "| Field | Value |") {
		t.Error("Expected metadata table in Markdown output")
	}
	
	// Check for tags
	if !strings.Contains(result, "`go`") {
		t.Error("Expected formatted tags in Markdown output")
	}
}

func TestMarkdownFormatterRepositories(t *testing.T) {
	config := DefaultConfig()
	formatter := NewMarkdownFormatter(config)
	
	repositories := createTestRepositories()
	result, err := formatter.FormatRepositories(repositories)
	if err != nil {
		t.Fatalf("Failed to format repositories as Markdown: %v", err)
	}
	
	// Check for Markdown headers
	if !strings.Contains(result, "# Repositories") {
		t.Error("Expected main header '# Repositories' in Markdown output")
	}
	
	// Check for star/fork indicators
	if !strings.Contains(result, "⭐") {
		t.Error("Expected star emoji in repository output")
	}
	
	if !strings.Contains(result, "🍴") {
		t.Error("Expected fork emoji in repository output")
	}
}

func TestMarkdownFormatterEscaping(t *testing.T) {
	config := DefaultConfig()
	formatter := NewMarkdownFormatter(config)
	
	// Create article with special characters
	article := createTestArticle("test", "Test *Article* with _Special_ Characters", "https://example.com", "Test Source")
	articles := []models.Article{article}
	
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format article with special characters: %v", err)
	}
	
	// Check that special characters are escaped
	if !strings.Contains(result, "Test \\*Article\\* with \\_Special\\_ Characters") {
		t.Error("Expected special Markdown characters to be escaped")
	}
}

// Test Text Formatter
func TestTextFormatterArticles(t *testing.T) {
	config := DefaultConfig()
	formatter := NewTextFormatter(config)
	
	articles := createTestArticles()
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format articles as text: %v", err)
	}
	
	// Check for text formatting
	if !strings.Contains(result, "ARTICLES") {
		t.Error("Expected 'ARTICLES' header in text output")
	}
	
	// Check for article numbering
	if !strings.Contains(result, "[1]") {
		t.Error("Expected article numbering in text output")
	}
	
	// Check for metadata
	if !strings.Contains(result, "Source:") {
		t.Error("Expected source information in text output")
	}
}

func TestTextFormatterCompact(t *testing.T) {
	config := DefaultConfig()
	config.CompactOutput = true
	formatter := NewTextFormatter(config)
	
	articles := createTestArticles()
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format articles as compact text: %v", err)
	}
	
	// Compact output should be shorter
	lines := strings.Split(result, "\n")
	
	// Should have fewer lines than full output
	fullConfig := DefaultConfig()
	fullFormatter := NewTextFormatter(fullConfig)
	fullResult, _ := fullFormatter.FormatArticles(articles)
	fullLines := strings.Split(fullResult, "\n")
	
	if len(lines) >= len(fullLines) {
		t.Error("Compact output should have fewer lines than full output")
	}
	
	// Should contain basic info on single lines
	foundArticle := false
	for _, line := range lines {
		if strings.Contains(line, "Test Article 1") && strings.Contains(line, "(https://example.com/article1)") {
			foundArticle = true
			break
		}
	}
	
	if !foundArticle {
		t.Error("Expected compact article line with title and URL")
	}
}

// Test Batch Formatter
func TestBatchFormatter(t *testing.T) {
	config := DefaultConfig()
	config.Format = FormatJSON
	
	factory := NewFormatterFactory(config)
	baseFormatter, _ := factory.CreateFormatter()
	batchFormatter := NewBatchFormatter(baseFormatter, config)
	
	// Create large dataset
	articles := make([]models.Article, 250) // More than default batch size
	for i := 0; i < 250; i++ {
		articles[i] = createTestArticle(
			string(rune(i)),
			"Test Article "+string(rune(i)),
			"https://example.com/article"+string(rune(i)),
			"Test Source",
		)
	}
	
	result, err := batchFormatter.FormatArticlesBatch(articles)
	if err != nil {
		t.Fatalf("Failed to format articles in batches: %v", err)
	}
	
	// Verify valid JSON output
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Batch formatted JSON is invalid: %v", err)
	}
	
	if len(parsed) != 250 {
		t.Errorf("Expected 250 articles in batch output, got %d", len(parsed))
	}
}

// Test sorting functionality
func TestSortingByRelevance(t *testing.T) {
	config := DefaultConfig()
	config.SortBy = "relevance"
	config.SortOrder = "desc"
	formatter := NewJSONFormatter(config)
	
	articles := createTestArticles()
	// Set different relevance scores
	articles[0].Relevance = 0.5
	articles[1].Relevance = 0.9
	articles[2].Relevance = 0.7
	
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format sorted articles: %v", err)
	}
	
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}
	
	// Check order (should be 0.9, 0.7, 0.5)
	if parsed[0]["relevance"].(float64) != 0.9 {
		t.Errorf("Expected first article relevance to be 0.9, got %f", parsed[0]["relevance"])
	}
	
	if parsed[1]["relevance"].(float64) != 0.7 {
		t.Errorf("Expected second article relevance to be 0.7, got %f", parsed[1]["relevance"])
	}
}

// Test utility functions
func TestTruncateText(t *testing.T) {
	longText := "This is a very long text that should be truncated at a reasonable word boundary to maintain readability."
	
	truncated := truncateText(longText, 50)
	
	if len(truncated) > 53 { // 50 + "..." = 53
		t.Errorf("Truncated text is too long: %d characters", len(truncated))
	}
	
	if !strings.HasSuffix(truncated, "...") {
		t.Error("Expected truncated text to end with '...'")
	}
	
	// Test with text shorter than limit
	shortText := "Short text"
	truncated = truncateText(shortText, 50)
	
	if truncated != shortText {
		t.Errorf("Expected short text to remain unchanged, got '%s'", truncated)
	}
}

func TestFormatTimestamp(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	
	formatted := formatTimestamp(testTime, "2006-01-02 15:04:05")
	expected := "2024-01-15 14:30:00"
	
	if formatted != expected {
		t.Errorf("Expected timestamp '%s', got '%s'", expected, formatted)
	}
	
	// Test with different format
	formatted = formatTimestamp(testTime, "Jan 2, 2006")
	expected = "Jan 15, 2024"
	
	if formatted != expected {
		t.Errorf("Expected timestamp '%s', got '%s'", expected, formatted)
	}
}

// Test edge cases
func TestEmptyInputs(t *testing.T) {
	config := DefaultConfig()
	
	// Test JSON formatter with empty inputs
	jsonFormatter := NewJSONFormatter(config)
	
	result, err := jsonFormatter.FormatArticles([]models.Article{})
	if err != nil {
		t.Errorf("Failed to format empty articles: %v", err)
	}
	if result != "[]" {
		t.Errorf("Expected '[]' for empty articles, got '%s'", result)
	}
	
	result, err = jsonFormatter.FormatRepositories([]models.Repository{})
	if err != nil {
		t.Errorf("Failed to format empty repositories: %v", err)
	}
	if result != "[]" {
		t.Errorf("Expected '[]' for empty repositories, got '%s'", result)
	}
	
	// Test Markdown formatter with empty inputs
	mdFormatter := NewMarkdownFormatter(config)
	
	result, err = mdFormatter.FormatArticles([]models.Article{})
	if err != nil {
		t.Errorf("Failed to format empty articles as Markdown: %v", err)
	}
	if !strings.Contains(result, "No Articles Found") {
		t.Error("Expected 'No Articles Found' message for empty Markdown articles")
	}
	
	// Test Text formatter with empty inputs
	textFormatter := NewTextFormatter(config)
	
	result, err = textFormatter.FormatArticles([]models.Article{})
	if err != nil {
		t.Errorf("Failed to format empty articles as text: %v", err)
	}
	if !strings.Contains(result, "No articles found") {
		t.Error("Expected 'No articles found' message for empty text articles")
	}
}

func TestMaxSummaryLength(t *testing.T) {
	config := DefaultConfig()
	config.MaxSummaryLength = 20
	formatter := NewJSONFormatter(config)
	
	article := createTestArticle("1", "Test", "https://example.com", "Test Source")
	article.SetSummary("This is a very long summary that should be truncated to the maximum length specified in the configuration.")
	articles := []models.Article{article}
	
	result, err := formatter.FormatArticles(articles)
	if err != nil {
		t.Fatalf("Failed to format articles with summary truncation: %v", err)
	}
	
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}
	
	summary := parsed[0]["summary"].(string)
	if len(summary) > 23 { // 20 + "..." = 23
		t.Errorf("Summary was not properly truncated: length %d", len(summary))
	}
}