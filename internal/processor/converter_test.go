package processor

import (
	"sync"
	"testing"
	"time"

	"frontend-news-mcp/internal/collector"
)

func TestNewConverter(t *testing.T) {
	config := DefaultConverterConfig()
	converter := NewConverter(config)
	
	if converter == nil {
		t.Fatal("NewConverter returned nil")
	}
	
	if converter.config.MaxSummaryLength != config.MaxSummaryLength {
		t.Errorf("Expected MaxSummaryLength %d, got %d", config.MaxSummaryLength, converter.config.MaxSummaryLength)
	}
}

func TestNewDefaultConverter(t *testing.T) {
	converter := NewDefaultConverter()
	
	if converter == nil {
		t.Fatal("NewDefaultConverter returned nil")
	}
	
	expectedConfig := DefaultConverterConfig()
	if converter.config.MaxSummaryLength != expectedConfig.MaxSummaryLength {
		t.Errorf("Expected default MaxSummaryLength %d, got %d", expectedConfig.MaxSummaryLength, converter.config.MaxSummaryLength)
	}
}

func TestConvertToArticle_BasicConversion(t *testing.T) {
	converter := NewDefaultConverter()
	
	collectorArticle := collector.Article{
		ID:          "test-123",
		Title:       "Test Article Title",
		Content:     "This is the test content with some <b>HTML tags</b> and special chars: ©",
		Summary:     "Test summary",
		Author:      "Test Author",
		URL:         "https://example.com/article",
		PublishedAt: time.Now(),
		Tags:        []string{"tech", "go", "programming"},
		Source:      "example.com",
		SourceType:  "rss",
		Language:    "en",
		Metadata:    map[string]string{"category": "technology"},
	}
	
	article, err := converter.ConvertToArticle(collectorArticle)
	if err != nil {
		t.Fatalf("ConvertToArticle failed: %v", err)
	}
	
	if article == nil {
		t.Fatal("ConvertToArticle returned nil article")
	}
	
	// Test basic fields
	if article.Title != collectorArticle.Title {
		t.Errorf("Expected title %q, got %q", collectorArticle.Title, article.Title)
	}
	
	if article.URL != collectorArticle.URL {
		t.Errorf("Expected URL %q, got %q", collectorArticle.URL, article.URL)
	}
	
	if article.Source != collectorArticle.Source {
		t.Errorf("Expected source %q, got %q", collectorArticle.Source, article.Source)
	}
	
	if article.SourceType != collectorArticle.SourceType {
		t.Errorf("Expected sourceType %q, got %q", collectorArticle.SourceType, article.SourceType)
	}
	
	// Test that HTML tags are removed from content
	if article.Content == collectorArticle.Content {
		t.Error("Expected HTML tags to be removed from content")
	}
	
	// Test tags are properly converted
	if len(article.Tags) != len(collectorArticle.Tags) {
		t.Errorf("Expected %d tags, got %d", len(collectorArticle.Tags), len(article.Tags))
	}
	
	// Test metadata is copied
	author, exists := article.GetMetadata("author")
	if !exists || author != collectorArticle.Author {
		t.Errorf("Expected author metadata %q, got %q (exists: %v)", collectorArticle.Author, author, exists)
	}
}

func TestConvertToArticle_EmptyRequiredFields(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		name    string
		article collector.Article
	}{
		{
			name: "empty title",
			article: collector.Article{
				Title:      "",
				URL:        "https://example.com",
				Source:     "example.com",
				SourceType: "rss",
			},
		},
		{
			name: "empty URL",
			article: collector.Article{
				Title:      "Test Title",
				URL:        "",
				Source:     "example.com",
				SourceType: "rss",
			},
		},
		{
			name: "empty source",
			article: collector.Article{
				Title:      "Test Title",
				URL:        "https://example.com",
				Source:     "",
				SourceType: "rss",
			},
		},
		{
			name: "empty sourceType",
			article: collector.Article{
				Title:      "Test Title",
				URL:        "https://example.com",
				Source:     "example.com",
				SourceType: "",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := converter.ConvertToArticle(tc.article)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
		})
	}
}

func TestConvertToArticle_HTMLCleaning(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic HTML tags",
			input:    "This has <b>bold</b> and <i>italic</i> text",
			expected: "This has bold and italic text",
		},
		{
			name:     "complex HTML",
			input:    "<div class='content'><p>Paragraph with <a href='#'>link</a></p></div>",
			expected: "Paragraph with link",
		},
		{
			name:     "script tags",
			input:    "Content with <script>alert('hack')</script> script",
			expected: "Content with script",
		},
		{
			name:     "HTML entities",
			input:    "Text with &lt;entities&gt; and &amp; symbols",
			expected: "Text with <entities> and & symbols",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collectorArticle := collector.Article{
				Title:      "Test Title",
				Content:    tc.input,
				URL:        "https://example.com",
				Source:     "example.com",
				SourceType: "rss",
			}
			
			article, err := converter.ConvertToArticle(collectorArticle)
			if err != nil {
				t.Fatalf("ConvertToArticle failed: %v", err)
			}
			
			if article.Content != tc.expected {
				t.Errorf("Expected content %q, got %q", tc.expected, article.Content)
			}
		})
	}
}

func TestConvertToArticle_URLNormalization(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tracking parameters",
			input:    "https://example.com/article?utm_source=twitter&utm_campaign=test&content=real",
			expected: "https://example.com/article?content=real",
		},
		{
			name:     "case normalization",
			input:    "HTTPS://EXAMPLE.COM/Article",
			expected: "https://example.com/Article",
		},
		{
			name:     "facebook click tracking",
			input:    "https://example.com/article?fbclid=abc123&ref=facebook",
			expected: "https://example.com/article",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collectorArticle := collector.Article{
				Title:      "Test Title",
				URL:        tc.input,
				Source:     "example.com",
				SourceType: "rss",
			}
			
			article, err := converter.ConvertToArticle(collectorArticle)
			if err != nil {
				t.Fatalf("ConvertToArticle failed: %v", err)
			}
			
			if article.URL != tc.expected {
				t.Errorf("Expected URL %q, got %q", tc.expected, article.URL)
			}
		})
	}
}

func TestConvertToArticle_SummaryGeneration(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		name            string
		providedSummary string
		content         string
		expectGenerated bool
	}{
		{
			name:            "use provided summary",
			providedSummary: "This is a good summary that meets length requirements",
			content:         "Much longer content that could be used for summary generation if needed",
			expectGenerated: false,
		},
		{
			name:            "generate from content when summary too short",
			providedSummary: "Too short",
			content:         "This is longer content that should be used to generate a proper summary because the provided one is too short",
			expectGenerated: true,
		},
		{
			name:            "generate from content when no summary",
			providedSummary: "",
			content:         "This is the content that should be used to generate a summary when none is provided",
			expectGenerated: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collectorArticle := collector.Article{
				Title:      "Test Title",
				Summary:    tc.providedSummary,
				Content:    tc.content,
				URL:        "https://example.com",
				Source:     "example.com",
				SourceType: "rss",
			}
			
			article, err := converter.ConvertToArticle(collectorArticle)
			if err != nil {
				t.Fatalf("ConvertToArticle failed: %v", err)
			}
			
			if tc.expectGenerated {
				if article.Summary == tc.providedSummary {
					t.Error("Expected summary to be generated from content, but got provided summary")
				}
			} else {
				if article.Summary != tc.providedSummary {
					t.Errorf("Expected to use provided summary %q, got %q", tc.providedSummary, article.Summary)
				}
			}
			
			// Ensure summary meets length requirements
			if len(article.Summary) < 10 {
				t.Errorf("Summary too short: %q", article.Summary)
			}
			if len(article.Summary) > 1000 {
				t.Errorf("Summary too long: length %d", len(article.Summary))
			}
		})
	}
}

func TestConvertToRepository(t *testing.T) {
	converter := NewDefaultConverter()
	
	metadata := map[string]string{
		"description": "A test repository for testing purposes",
		"language":    "Go",
		"stars":       "42",
		"forks":       "7",
		"updated_at":  "2023-01-15T10:30:00Z",
	}
	
	repo, err := converter.ConvertToRepository(
		"test-repo",
		"user/test-repo",
		"https://github.com/user/test-repo",
		metadata,
	)
	
	if err != nil {
		t.Fatalf("ConvertToRepository failed: %v", err)
	}
	
	if repo == nil {
		t.Fatal("ConvertToRepository returned nil repository")
	}
	
	// Test basic fields
	if repo.Name != "test-repo" {
		t.Errorf("Expected name %q, got %q", "test-repo", repo.Name)
	}
	
	if repo.FullName != "user/test-repo" {
		t.Errorf("Expected fullName %q, got %q", "user/test-repo", repo.FullName)
	}
	
	if repo.URL != "https://github.com/user/test-repo" {
		t.Errorf("Expected URL %q, got %q", "https://github.com/user/test-repo", repo.URL)
	}
	
	// Test metadata conversion
	if repo.Description != metadata["description"] {
		t.Errorf("Expected description %q, got %q", metadata["description"], repo.Description)
	}
	
	if repo.Language != metadata["language"] {
		t.Errorf("Expected language %q, got %q", metadata["language"], repo.Language)
	}
	
	if repo.Stars != 42 {
		t.Errorf("Expected stars %d, got %d", 42, repo.Stars)
	}
	
	if repo.Forks != 7 {
		t.Errorf("Expected forks %d, got %d", 7, repo.Forks)
	}
	
	// Test that trend score was calculated
	if repo.TrendScore == 0.0 {
		t.Error("Expected trend score to be calculated")
	}
}

func TestConvertToRepository_EmptyRequiredFields(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		name     string
		repoName string
		fullName string
		url      string
	}{
		{
			name:     "empty name",
			repoName: "",
			fullName: "user/repo",
			url:      "https://github.com/user/repo",
		},
		{
			name:     "empty fullName",
			repoName: "repo",
			fullName: "",
			url:      "https://github.com/user/repo",
		},
		{
			name:     "empty URL",
			repoName: "repo",
			fullName: "user/repo",
			url:      "",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := converter.ConvertToRepository(tc.repoName, tc.fullName, tc.url, map[string]string{})
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
		})
	}
}

func TestBatchConvertArticles(t *testing.T) {
	converter := NewDefaultConverter()
	
	collectorArticles := []collector.Article{
		{
			Title:      "Article 1",
			URL:        "https://example.com/1",
			Source:     "example.com",
			SourceType: "rss",
		},
		{
			Title:      "Article 2",
			URL:        "https://example.com/2",
			Source:     "example.com",
			SourceType: "api",
		},
		{
			// This one should fail due to missing title
			Title:      "",
			URL:        "https://example.com/3",
			Source:     "example.com",
			SourceType: "html",
		},
	}
	
	articles, errors := converter.BatchConvertArticles(collectorArticles)
	
	if len(articles) != len(collectorArticles) {
		t.Errorf("Expected %d articles, got %d", len(collectorArticles), len(articles))
	}
	
	if len(errors) != len(collectorArticles) {
		t.Errorf("Expected %d errors, got %d", len(collectorArticles), len(errors))
	}
	
	// First two should succeed
	if errors[0] != nil {
		t.Errorf("Expected first conversion to succeed, got error: %v", errors[0])
	}
	if articles[0] == nil {
		t.Error("Expected first article to be converted")
	}
	
	if errors[1] != nil {
		t.Errorf("Expected second conversion to succeed, got error: %v", errors[1])
	}
	if articles[1] == nil {
		t.Error("Expected second article to be converted")
	}
	
	// Third should fail
	if errors[2] == nil {
		t.Error("Expected third conversion to fail due to empty title")
	}
	if articles[2] != nil {
		t.Error("Expected third article to be nil due to conversion failure")
	}
}

func TestBatchConvertArticles_EmptySlice(t *testing.T) {
	converter := NewDefaultConverter()
	
	articles, errors := converter.BatchConvertArticles([]collector.Article{})
	
	if len(articles) != 0 {
		t.Errorf("Expected empty articles slice, got %d articles", len(articles))
	}
	
	if len(errors) != 0 {
		t.Errorf("Expected empty errors slice, got %d errors", len(errors))
	}
}

func TestConcurrentConversion(t *testing.T) {
	converter := NewDefaultConverter()
	
	// Test concurrent access to converter
	const numGoroutines = 10
	const articlesPerGoroutine = 5
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalArticles int
	var totalErrors int
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			collectorArticles := make([]collector.Article, articlesPerGoroutine)
			for j := 0; j < articlesPerGoroutine; j++ {
				collectorArticles[j] = collector.Article{
					Title:      "Article " + string(rune(goroutineID*articlesPerGoroutine+j)),
					URL:        "https://example.com/" + string(rune(goroutineID*articlesPerGoroutine+j)),
					Source:     "example.com",
					SourceType: "rss",
				}
			}
			
			articles, errors := converter.BatchConvertArticles(collectorArticles)
			
			mu.Lock()
			defer mu.Unlock()
			
			for _, article := range articles {
				if article != nil {
					totalArticles++
				}
			}
			
			for _, err := range errors {
				if err != nil {
					totalErrors++
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedArticles := numGoroutines * articlesPerGoroutine
	if totalArticles != expectedArticles {
		t.Errorf("Expected %d successful conversions, got %d", expectedArticles, totalArticles)
	}
	
	if totalErrors != 0 {
		t.Errorf("Expected no conversion errors, got %d", totalErrors)
	}
}

func TestTextNormalizationFunctions(t *testing.T) {
	converter := NewDefaultConverter()
	
	t.Run("removeHTMLTags", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{
				input:    "<p>Hello <b>world</b>!</p>",
				expected: " Hello  world ! ",
			},
			{
				input:    "No tags here",
				expected: "No tags here",
			},
			{
				input:    "",
				expected: "",
			},
		}
		
		for _, tc := range testCases {
			result := converter.removeHTMLTags(tc.input)
			if result != tc.expected {
				t.Errorf("removeHTMLTags(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		}
	})
	
	t.Run("normalizeWhitespace", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{
				input:    "Multiple   spaces    here",
				expected: "Multiple spaces here",
			},
			{
				input:    "Tabs\t\tand\nnewlines\r\n",
				expected: "Tabs and newlines ",
			},
			{
				input:    "",
				expected: "",
			},
		}
		
		for _, tc := range testCases {
			result := converter.normalizeWhitespace(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeWhitespace(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		}
	})
	
	t.Run("truncateText", func(t *testing.T) {
		testCases := []struct {
			input     string
			maxLength int
			expected  string
		}{
			{
				input:     "This is a long sentence that should be truncated",
				maxLength: 20,
				expected:  "This is a long...",
			},
			{
				input:     "Short text",
				maxLength: 20,
				expected:  "Short text",
			},
			{
				input:     "",
				maxLength: 10,
				expected:  "",
			},
		}
		
		for _, tc := range testCases {
			result := converter.truncateText(tc.input, tc.maxLength)
			if result != tc.expected {
				t.Errorf("truncateText(%q, %d) = %q, expected %q", tc.input, tc.maxLength, result, tc.expected)
			}
		}
	})
}

func TestTimeNormalization(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		{
			name:      "RFC3339 format",
			input:     "2023-01-15T10:30:00Z",
			shouldErr: false,
		},
		{
			name:      "RFC1123 format",
			input:     "Sun, 15 Jan 2023 10:30:00 GMT",
			shouldErr: false,
		},
		{
			name:      "Simple date",
			input:     "2023-01-15",
			shouldErr: false,
		},
		{
			name:      "Invalid format",
			input:     "not a date",
			shouldErr: true,
		},
		{
			name:      "Empty string",
			input:     "",
			shouldErr: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := converter.parseTime(tc.input)
			if tc.shouldErr && err == nil {
				t.Errorf("Expected error for input %q, but got none", tc.input)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Expected no error for input %q, but got: %v", tc.input, err)
			}
		})
	}
}

func TestConfigUpdateThreadSafety(t *testing.T) {
	converter := NewDefaultConverter()
	
	var wg sync.WaitGroup
	const numGoroutines = 10
	
	// Test concurrent config updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			config := DefaultConverterConfig()
			config.MaxSummaryLength = 500 + id*10
			
			converter.UpdateConfig(config)
			
			retrievedConfig := converter.GetConfig()
			if retrievedConfig.MaxSummaryLength != config.MaxSummaryLength {
				t.Errorf("Config update failed for goroutine %d", id)
			}
		}(i)
	}
	
	wg.Wait()
}

func TestGenerateHash(t *testing.T) {
	converter := NewDefaultConverter()
	
	testCases := []struct {
		title1, url1 string
		title2, url2 string
		shouldMatch  bool
	}{
		{
			title1:      "Same Title",
			url1:        "https://example.com",
			title2:      "Same Title",
			url2:        "https://example.com",
			shouldMatch: true,
		},
		{
			title1:      "Different Title",
			url1:        "https://example.com",
			title2:      "Same Title",
			url2:        "https://example.com",
			shouldMatch: false,
		},
		{
			title1:      "Same Title",
			url1:        "https://example.com/1",
			title2:      "Same Title",
			url2:        "https://example.com/2",
			shouldMatch: false,
		},
	}
	
	for _, tc := range testCases {
		hash1 := converter.GenerateHash(tc.title1, tc.url1)
		hash2 := converter.GenerateHash(tc.title2, tc.url2)
		
		if tc.shouldMatch && hash1 != hash2 {
			t.Errorf("Expected hashes to match for %q/%q and %q/%q", tc.title1, tc.url1, tc.title2, tc.url2)
		}
		
		if !tc.shouldMatch && hash1 == hash2 {
			t.Errorf("Expected hashes to differ for %q/%q and %q/%q", tc.title1, tc.url1, tc.title2, tc.url2)
		}
	}
}

// Benchmark tests
func BenchmarkConvertToArticle(b *testing.B) {
	converter := NewDefaultConverter()
	
	collectorArticle := collector.Article{
		Title:      "Benchmark Article Title",
		Content:    "This is benchmark content with some <b>HTML tags</b> and longer text that might need processing",
		Summary:    "Benchmark summary",
		Author:     "Benchmark Author",
		URL:        "https://example.com/benchmark",
		PublishedAt: time.Now(),
		Tags:       []string{"benchmark", "performance", "test"},
		Source:     "example.com",
		SourceType: "rss",
		Language:   "en",
		Metadata:   map[string]string{"category": "performance"},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := converter.ConvertToArticle(collectorArticle)
		if err != nil {
			b.Fatalf("ConvertToArticle failed: %v", err)
		}
	}
}

func BenchmarkBatchConvertArticles(b *testing.B) {
	converter := NewDefaultConverter()
	
	collectorArticles := make([]collector.Article, 100)
	for i := 0; i < 100; i++ {
		collectorArticles[i] = collector.Article{
			Title:      "Benchmark Article " + string(rune(i)),
			Content:    "Benchmark content for article " + string(rune(i)),
			URL:        "https://example.com/" + string(rune(i)),
			Source:     "example.com",
			SourceType: "rss",
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = converter.BatchConvertArticles(collectorArticles)
	}
}