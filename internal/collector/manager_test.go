package collector

import (
	"context"
	"testing"
	"time"
)

func TestNewCollectorManager(t *testing.T) {
	manager := NewCollectorManager()
	
	// 检查默认采集器是否已注册
	collectors := []string{"rss", "api", "html"}
	for _, collectorType := range collectors {
		collector, exists := manager.GetCollector(collectorType)
		if !exists {
			t.Errorf("Expected collector %s to be registered", collectorType)
		}
		if collector == nil {
			t.Errorf("Expected collector %s to be non-nil", collectorType)
		}
		if collector.GetSourceType() != collectorType {
			t.Errorf("Expected collector type %s, got %s", collectorType, collector.GetSourceType())
		}
	}
}

func TestCollectorManagerImpl_RegisterCollector(t *testing.T) {
	manager := NewCollectorManager()
	
	// 创建一个测试采集器
	testCollector := NewRSSCollector()
	
	// 注册新的采集器
	manager.RegisterCollector("test", testCollector)
	
	// 验证采集器已注册
	collector, exists := manager.GetCollector("test")
	if !exists {
		t.Error("Expected test collector to be registered")
	}
	if collector != testCollector {
		t.Error("Expected registered collector to be the same instance")
	}
}

func TestCollectorManagerImpl_GetCollector(t *testing.T) {
	manager := NewCollectorManager()
	
	// 测试获取存在的采集器
	collector, exists := manager.GetCollector("rss")
	if !exists {
		t.Error("Expected RSS collector to exist")
	}
	if collector == nil {
		t.Error("Expected RSS collector to be non-nil")
	}
	
	// 测试获取不存在的采集器
	collector, exists = manager.GetCollector("nonexistent")
	if exists {
		t.Error("Expected nonexistent collector to not exist")
	}
	if collector != nil {
		t.Error("Expected nonexistent collector to be nil")
	}
}

func TestCollectorManagerImpl_determineSourceType(t *testing.T) {
	manager := NewCollectorManager().(*CollectorManagerImpl)
	
	tests := []struct {
		name     string
		config   CollectConfig
		expected string
	}{
		{
			name:     "RSS URL with .xml",
			config:   CollectConfig{URL: "https://example.com/feed.xml"},
			expected: "rss",
		},
		{
			name:     "RSS URL with /rss",
			config:   CollectConfig{URL: "https://example.com/rss"},
			expected: "rss",
		},
		{
			name:     "RSS URL with /feed",
			config:   CollectConfig{URL: "https://example.com/feed"},
			expected: "rss",
		},
		{
			name:     "RSS URL with /atom",
			config:   CollectConfig{URL: "https://example.com/atom"},
			expected: "rss",
		},
		{
			name:     "GitHub API URL",
			config:   CollectConfig{URL: "https://api.github.com/repos/owner/repo"},
			expected: "api",
		},
		{
			name:     "Dev.to API URL",
			config:   CollectConfig{URL: "https://dev.to/api/articles"},
			expected: "api",
		},
		{
			name:     "Generic API URL",
			config:   CollectConfig{URL: "https://example.com/api/data"},
			expected: "api",
		},
		{
			name:     "HTML page",
			config:   CollectConfig{URL: "https://example.com/article"},
			expected: "html",
		},
		{
			name:     "Explicit source type",
			config:   CollectConfig{
				URL: "https://example.com/data",
				Metadata: map[string]string{
					"source_type": "rss",
				},
			},
			expected: "rss",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.determineSourceType(tt.config)
			if result != tt.expected {
				t.Errorf("determineSourceType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCollectorManagerImpl_shouldRetry(t *testing.T) {
	manager := NewCollectorManager().(*CollectorManagerImpl)
	
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "validation error",
			err:      &ValidationError{message: "validation failed"},
			expected: false,
		},
		{
			name:     "404 error",
			err:      &HTTPError{statusCode: 404},
			expected: false,
		},
		{
			name:     "401 error",
			err:      &HTTPError{statusCode: 401},
			expected: false,
		},
		{
			name:     "403 error",
			err:      &HTTPError{statusCode: 403},
			expected: false,
		},
		{
			name:     "500 error",
			err:      &HTTPError{statusCode: 500},
			expected: true,
		},
		{
			name:     "502 error",
			err:      &HTTPError{statusCode: 502},
			expected: true,
		},
		{
			name:     "timeout error",
			err:      &TimeoutError{},
			expected: true,
		},
		{
			name:     "network error",
			err:      &NetworkError{},
			expected: true,
		},
		{
			name:     "unknown error",
			err:      &UnknownError{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.shouldRetry(tt.err)
			if result != tt.expected {
				t.Errorf("shouldRetry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCollectorManagerImpl_CollectAll(t *testing.T) {
	// 创建测试采集器，总是返回成功
	testCollector := &TestCollector{}
	
	manager := NewCollectorManager()
	manager.RegisterCollector("test", testCollector)
	
	configs := []CollectConfig{
		{
			URL: "test://example1.com",
			Metadata: map[string]string{"source_type": "test"},
		},
		{
			URL: "test://example2.com",
			Metadata: map[string]string{"source_type": "test"},
		},
	}
	
	ctx := context.Background()
	results := manager.CollectAll(ctx, configs)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	
	for i, result := range results {
		if result.Error != nil {
			t.Errorf("Result %d should not have error: %v", i, result.Error)
		}
		if len(result.Articles) != 1 {
			t.Errorf("Result %d should have 1 article, got %d", i, len(result.Articles))
		}
	}
}

func TestCollectorManagerImpl_CollectWithRetry(t *testing.T) {
	// 创建失败的采集器，然后成功
	failingCollector := &FailingTestCollector{failTimes: 2}
	
	manager := NewCollectorManager()
	manager.RegisterCollector("failing", failingCollector)
	
	config := CollectConfig{
		URL: "test://example.com",
		Metadata: map[string]string{"source_type": "failing"},
	}
	
	retryConfig := RetryConfig{
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	}
	
	ctx := context.Background()
	result, err := manager.CollectWithRetry(ctx, config, retryConfig)
	
	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}
	if len(result.Articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(result.Articles))
	}
	if failingCollector.attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", failingCollector.attempts)
	}
}

func TestCollectorManagerImpl_CollectWithRetry_ExceedsMaxRetries(t *testing.T) {
	// 创建总是失败的采集器
	failingCollector := &FailingTestCollector{failTimes: 10}
	
	manager := NewCollectorManager()
	manager.RegisterCollector("failing", failingCollector)
	
	config := CollectConfig{
		URL: "test://example.com",
		Metadata: map[string]string{"source_type": "failing"},
	}
	
	retryConfig := RetryConfig{
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
	}
	
	ctx := context.Background()
	result, err := manager.CollectWithRetry(ctx, config, retryConfig)
	
	if err == nil {
		t.Error("Expected error after exceeding max retries")
	}
	if len(result.Articles) != 0 {
		t.Errorf("Expected 0 articles on failure, got %d", len(result.Articles))
	}
}

func TestNewBatchCollector(t *testing.T) {
	manager := NewCollectorManager()
	batchCollector := NewBatchCollector(manager, 5, 30*time.Second)
	
	if batchCollector.manager != manager {
		t.Error("Expected batch collector to have the same manager")
	}
	if batchCollector.maxConcurrent != 5 {
		t.Errorf("Expected maxConcurrent 5, got %d", batchCollector.maxConcurrent)
	}
	if batchCollector.timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", batchCollector.timeout)
	}
}

func TestBatchCollector_CollectBatch(t *testing.T) {
	testCollector := &TestCollector{}
	manager := NewCollectorManager()
	manager.RegisterCollector("test", testCollector)
	
	batchCollector := NewBatchCollector(manager, 2, 10*time.Second)
	
	configs := []CollectConfig{
		{URL: "test://example1.com", Metadata: map[string]string{"source_type": "test"}},
		{URL: "test://example2.com", Metadata: map[string]string{"source_type": "test"}},
		{URL: "test://example3.com", Metadata: map[string]string{"source_type": "test"}},
	}
	
	ctx := context.Background()
	results := batchCollector.CollectBatch(ctx, configs)
	
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
	
	for i, result := range results {
		if result.Error != nil {
			t.Errorf("Result %d should not have error: %v", i, result.Error)
		}
	}
}

func TestAggregateResults(t *testing.T) {
	results := []CollectResult{
		{
			Articles: []Article{
				{ID: "1", Title: "Article 1"},
				{ID: "2", Title: "Article 2"},
			},
			Source: "source1",
		},
		{
			Articles: []Article{
				{ID: "3", Title: "Article 3"},
			},
			Source: "source2",
		},
		{
			Articles: nil,
			Source:   "source3",
			Error:    &TestError{message: "test error"},
		},
	}
	
	articles, errors := AggregateResults(results)
	
	if len(articles) != 3 {
		t.Errorf("Expected 3 articles, got %d", len(articles))
	}
	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}
	
	// 验证文章内容
	expectedTitles := []string{"Article 1", "Article 2", "Article 3"}
	for i, article := range articles {
		if article.Title != expectedTitles[i] {
			t.Errorf("Expected title '%s', got '%s'", expectedTitles[i], article.Title)
		}
	}
}

func TestArticleFilter_FilterArticles(t *testing.T) {
	articles := []Article{
		{
			ID:          "1",
			Title:       "Go Programming",
			Content:     "This is a comprehensive guide to Go programming language",
			Author:      "John Doe",
			Tags:        []string{"go", "programming"},
			Language:    "en",
			PublishedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:          "2",
			Title:       "Python Basics",
			Content:     "Learn Python programming",
			Author:      "Jane Smith",
			Tags:        []string{"python", "basics"},
			Language:    "en",
			PublishedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:          "3",
			Title:       "JavaScript Tips",
			Content:     "Short tips for JavaScript",
			Author:      "Bob Wilson",
			Tags:        []string{"javascript", "tips"},
			Language:    "en",
			PublishedAt: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
		},
	}
	
	// 测试关键词过滤
	filter := &ArticleFilter{Keywords: []string{"Go", "programming"}}
	filtered := filter.FilterArticles(articles)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 articles with keywords filter, got %d", len(filtered))
	}
	
	// 测试作者过滤
	filter = &ArticleFilter{Authors: []string{"John Doe"}}
	filtered = filter.FilterArticles(articles)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 article with author filter, got %d", len(filtered))
	}
	
	// 测试标签过滤
	filter = &ArticleFilter{Tags: []string{"python"}}
	filtered = filter.FilterArticles(articles)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 article with tag filter, got %d", len(filtered))
	}
	
	// 测试日期过滤
	filter = &ArticleFilter{
		DateFrom: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	filtered = filter.FilterArticles(articles)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 articles with date filter, got %d", len(filtered))
	}
	
	// 测试长度过滤
	filter = &ArticleFilter{MinLength: 30}
	filtered = filter.FilterArticles(articles)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 article with length filter, got %d", len(filtered))
	}
}

func TestDeduplicateArticles(t *testing.T) {
	articles := []Article{
		{ID: "1", URL: "http://example.com/1", Title: "Article 1"},
		{ID: "2", URL: "http://example.com/2", Title: "Article 2"},
		{ID: "3", URL: "http://example.com/1", Title: "Duplicate Article 1"}, // 相同URL
		{ID: "1", URL: "http://example.com/3", Title: "Article 1 Different URL"}, // 不同URL但相同ID
	}
	
	deduplicated := DeduplicateArticles(articles)
	
	if len(deduplicated) != 3 {
		t.Errorf("Expected 3 unique articles, got %d", len(deduplicated))
	}
	
	// 验证去重逻辑：应该保留第一个出现的文章
	expectedTitles := []string{"Article 1", "Article 2", "Article 1 Different URL"}
	for i, article := range deduplicated {
		if article.Title != expectedTitles[i] {
			t.Errorf("Expected title '%s', got '%s'", expectedTitles[i], article.Title)
		}
	}
}

func TestSortArticles(t *testing.T) {
	articles := []Article{
		{
			Title:       "Z Article",
			Author:      "Alice",
			PublishedAt: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			Title:       "A Article",
			Author:      "Bob",
			PublishedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Title:       "M Article",
			Author:      "Charlie",
			PublishedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}
	
	// 测试按日期升序排序
	sorted := SortArticles(articles, SortByDate, true)
	expectedDates := []time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
	}
	for i, article := range sorted {
		if !article.PublishedAt.Equal(expectedDates[i]) {
			t.Errorf("Expected date %v, got %v", expectedDates[i], article.PublishedAt)
		}
	}
	
	// 测试按标题降序排序
	sorted = SortArticles(articles, SortByTitle, false)
	expectedTitles := []string{"Z Article", "M Article", "A Article"}
	for i, article := range sorted {
		if article.Title != expectedTitles[i] {
			t.Errorf("Expected title '%s', got '%s'", expectedTitles[i], article.Title)
		}
	}
	
	// 测试按作者升序排序
	sorted = SortArticles(articles, SortByAuthor, true)
	expectedAuthors := []string{"Alice", "Bob", "Charlie"}
	for i, article := range sorted {
		if article.Author != expectedAuthors[i] {
			t.Errorf("Expected author '%s', got '%s'", expectedAuthors[i], article.Author)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		substr   string
		expected bool
	}{
		{
			name:     "substring at beginning",
			str:      "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring at end",
			str:      "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "substring in middle",
			str:      "hello world",
			substr:   "lo wo",
			expected: true,
		},
		{
			name:     "exact match",
			str:      "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "not found",
			str:      "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "empty substring",
			str:      "hello world",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			str:      "",
			substr:   "hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.str, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.str, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		substr   string
		expected int
	}{
		{
			name:     "found at beginning",
			str:      "hello world",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "found in middle",
			str:      "hello world",
			substr:   "lo",
			expected: 3,
		},
		{
			name:     "not found",
			str:      "hello world",
			substr:   "foo",
			expected: -1,
		},
		{
			name:     "empty substring",
			str:      "hello world",
			substr:   "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOf(tt.str, tt.substr)
			if result != tt.expected {
				t.Errorf("indexOf(%q, %q) = %v, want %v", tt.str, tt.substr, result, tt.expected)
			}
		})
	}
}

// 测试采集器实现
type TestCollector struct{}

func (tc *TestCollector) GetSourceType() string {
	return "test"
}

func (tc *TestCollector) Validate(config CollectConfig) error {
	return nil
}

func (tc *TestCollector) Collect(ctx context.Context, config CollectConfig) (CollectResult, error) {
	return CollectResult{
		Articles: []Article{
			{
				ID:     "test-1",
				Title:  "Test Article",
				URL:    config.URL,
				Source: config.URL,
			},
		},
		Source: config.URL,
	}, nil
}

// 失败的测试采集器
type FailingTestCollector struct {
	failTimes int
	attempts  int
}

func (ftc *FailingTestCollector) GetSourceType() string {
	return "failing"
}

func (ftc *FailingTestCollector) Validate(config CollectConfig) error {
	return nil
}

func (ftc *FailingTestCollector) Collect(ctx context.Context, config CollectConfig) (CollectResult, error) {
	ftc.attempts++
	if ftc.attempts <= ftc.failTimes {
		return CollectResult{}, &TestError{message: "intentional test failure"}
	}
	return CollectResult{
		Articles: []Article{
			{
				ID:     "test-1",
				Title:  "Test Article",
				URL:    config.URL,
				Source: config.URL,
			},
		},
		Source: config.URL,
	}, nil
}

// 测试错误类型
type TestError struct {
	message string
}

func (e *TestError) Error() string {
	return e.message
}

type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return "validation failed: " + e.message
}

type HTTPError struct {
	statusCode int
}

func (e *HTTPError) Error() string {
	if e.statusCode == 404 {
		return "HTTP error 404: Not Found"
	}
	if e.statusCode == 401 {
		return "HTTP error 401: Unauthorized"
	}
	if e.statusCode == 403 {
		return "HTTP error 403: Forbidden"
	}
	if e.statusCode >= 500 {
		return "HTTP error 500: Internal Server Error"
	}
	return "HTTP error"
}

type TimeoutError struct{}

func (e *TimeoutError) Error() string {
	return "timeout error"
}

type NetworkError struct{}

func (e *NetworkError) Error() string {
	return "network connection error"
}

type UnknownError struct{}

func (e *UnknownError) Error() string {
	return "unknown error"
}