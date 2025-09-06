package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPICollector_GetSourceType(t *testing.T) {
	collector := NewAPICollector()
	if collector.GetSourceType() != "api" {
		t.Errorf("Expected source type 'api', got %s", collector.GetSourceType())
	}
}

func TestAPICollector_Validate(t *testing.T) {
	collector := NewAPICollector()

	tests := []struct {
		name      string
		config    CollectConfig
		wantError bool
	}{
		{
			name:      "empty URL",
			config:    CollectConfig{},
			wantError: true,
		},
		{
			name:      "invalid URL",
			config:    CollectConfig{URL: "not-a-url"},
			wantError: true,
		},
		{
			name:      "invalid scheme",
			config:    CollectConfig{URL: "ftp://example.com"},
			wantError: true,
		},
		{
			name:      "valid HTTP URL",
			config:    CollectConfig{URL: "http://example.com/api"},
			wantError: false,
		},
		{
			name:      "valid HTTPS URL",
			config:    CollectConfig{URL: "https://api.github.com/repos"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := collector.Validate(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestAPICollector_isGitHubAPI(t *testing.T) {
	collector := NewAPICollector()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "GitHub API URL",
			url:  "https://api.github.com/repos/owner/repo",
			want: true,
		},
		{
			name: "Non-GitHub URL",
			url:  "https://dev.to/api/articles",
			want: false,
		},
		{
			name: "Regular website",
			url:  "https://example.com",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.isGitHubAPI(tt.url)
			if result != tt.want {
				t.Errorf("isGitHubAPI() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestAPICollector_isDevToAPI(t *testing.T) {
	collector := NewAPICollector()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "Dev.to API URL",
			url:  "https://dev.to/api/articles",
			want: true,
		},
		{
			name: "GitHub API URL",
			url:  "https://api.github.com/repos",
			want: false,
		},
		{
			name: "Regular website",
			url:  "https://example.com",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.isDevToAPI(tt.url)
			if result != tt.want {
				t.Errorf("isDevToAPI() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestAPICollector_collectGitHubAPI_Repos(t *testing.T) {
	// 模拟GitHub仓库API响应
	githubReposResponse := `[
  {
    "id": 12345,
    "name": "test-repo",
    "full_name": "owner/test-repo",
    "description": "A test repository",
    "html_url": "https://github.com/owner/test-repo",
    "language": "Go",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-02T00:00:00Z",
    "pushed_at": "2024-01-02T12:00:00Z",
    "owner": {
      "login": "owner"
    },
    "topics": ["go", "api"]
  }
]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(githubReposResponse))
	}))
	defer server.Close()

	collector := NewAPICollector()
	
	// 测试GitHub API的转换功能
	var repos []GitHubRepo
	if err := json.Unmarshal([]byte(githubReposResponse), &repos); err != nil {
		t.Fatal(err)
	}
	
	config := CollectConfig{
		URL: "https://api.github.com/users/owner/repos",
		Headers: map[string]string{
			"Authorization": "token test-token",
		},
	}
	
	articles := collector.convertGitHubRepos(repos, config)

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}

	article := articles[0]
	if article.ID != "12345" {
		t.Errorf("Expected article ID '12345', got %s", article.ID)
	}
	if article.Title != "owner/test-repo" {
		t.Errorf("Expected title 'owner/test-repo', got %s", article.Title)
	}
	if article.Author != "owner" {
		t.Errorf("Expected author 'owner', got %s", article.Author)
	}
	if article.Language != "Go" {
		t.Errorf("Expected language 'Go', got %s", article.Language)
	}
	if len(article.Tags) < 2 {
		t.Errorf("Expected at least 2 tags, got %d", len(article.Tags))
	}
	if article.Metadata["github_repo"] != "true" {
		t.Errorf("Expected github_repo metadata to be 'true', got %s", article.Metadata["github_repo"])
	}
}

func TestAPICollector_collectGitHubAPI_Issues(t *testing.T) {
	githubIssuesResponse := `[
  {
    "id": 67890,
    "number": 1,
    "title": "Test Issue",
    "body": "This is a test issue description.",
    "state": "open",
    "html_url": "https://github.com/owner/repo/issues/1",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T12:00:00Z",
    "user": {
      "login": "issue-creator"
    },
    "labels": [
      {"name": "bug"},
      {"name": "help wanted"}
    ]
  }
]`

	collector := NewAPICollector()
	
	// 测试GitHub Issues的转换功能
	var issues []GitHubIssue
	if err := json.Unmarshal([]byte(githubIssuesResponse), &issues); err != nil {
		t.Fatal(err)
	}
	
	config := CollectConfig{
		URL: "https://api.github.com/repos/owner/repo/issues",
	}
	
	articles := collector.convertGitHubIssues(issues, config)

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}

	article := articles[0]
	if article.ID != "67890" {
		t.Errorf("Expected article ID '67890', got %s", article.ID)
	}
	if article.Title != "#1: Test Issue" {
		t.Errorf("Expected title '#1: Test Issue', got %s", article.Title)
	}
	if article.Author != "issue-creator" {
		t.Errorf("Expected author 'issue-creator', got %s", article.Author)
	}
	if article.Content != "This is a test issue description." {
		t.Errorf("Expected content 'This is a test issue description.', got %s", article.Content)
	}
	
	// 检查标签
	expectedTags := []string{"github-issue", "open", "bug", "help wanted"}
	if len(article.Tags) < len(expectedTags) {
		t.Errorf("Expected at least %d tags, got %d", len(expectedTags), len(article.Tags))
	}
	
	if article.Metadata["github_issue"] != "true" {
		t.Errorf("Expected github_issue metadata to be 'true', got %s", article.Metadata["github_issue"])
	}
	if article.Metadata["issue_number"] != "1" {
		t.Errorf("Expected issue_number metadata to be '1', got %s", article.Metadata["issue_number"])
	}
}

func TestAPICollector_collectDevToAPI(t *testing.T) {
	devtoResponse := `[
  {
    "id": 12345,
    "title": "How to Build APIs with Go",
    "description": "A comprehensive guide to building APIs",
    "body_markdown": "# How to Build APIs with Go\n\nThis is a comprehensive guide...",
    "url": "https://dev.to/author/how-to-build-apis-with-go",
    "published_at": "2024-01-01T00:00:00Z",
    "created_at": "2023-12-31T00:00:00Z",
    "tag_list": ["go", "api", "web development"],
    "user": {
      "username": "go_developer",
      "name": "Go Developer"
    },
    "organization": null,
    "reading_time_minutes": 5
  }
]`

	collector := NewAPICollector()
	
	// 测试Dev.to Articles的转换功能
	var devArticles []DevToArticle
	if err := json.Unmarshal([]byte(devtoResponse), &devArticles); err != nil {
		t.Fatal(err)
	}
	
	config := CollectConfig{
		URL: "https://dev.to/api/articles",
	}
	
	articles := collector.convertDevToArticles(devArticles, config)

	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}

	article := articles[0]
	if article.ID != "12345" {
		t.Errorf("Expected article ID '12345', got %s", article.ID)
	}
	if article.Title != "How to Build APIs with Go" {
		t.Errorf("Expected title 'How to Build APIs with Go', got %s", article.Title)
	}
	if article.Author != "Go Developer" {
		t.Errorf("Expected author 'Go Developer', got %s", article.Author)
	}
	if article.Summary != "A comprehensive guide to building APIs" {
		t.Errorf("Expected summary 'A comprehensive guide to building APIs', got %s", article.Summary)
	}
	
	// 检查标签
	expectedTags := []string{"go", "api", "web development"}
	if len(article.Tags) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d", len(expectedTags), len(article.Tags))
	}
	
	if article.Metadata["dev_to_article"] != "true" {
		t.Errorf("Expected dev_to_article metadata to be 'true', got %s", article.Metadata["dev_to_article"])
	}
	if article.Metadata["reading_time_minutes"] != "5" {
		t.Errorf("Expected reading_time_minutes metadata to be '5', got %s", article.Metadata["reading_time_minutes"])
	}
}

func TestAPICollector_collectGenericAPI(t *testing.T) {
	genericResponse := `{
  "message": "Hello from generic API",
  "data": {
    "items": ["item1", "item2"],
    "count": 2
  }
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(genericResponse))
	}))
	defer server.Close()

	collector := NewAPICollector()
	config := CollectConfig{
		URL: server.URL + "/generic-api",
	}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(result.Articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(result.Articles))
	}

	article := result.Articles[0]
	expectedTitle := "API Response from " + server.URL + "/generic-api"
	if article.Title != expectedTitle {
		t.Errorf("Expected title '%s', got %s", expectedTitle, article.Title)
	}
	if article.Author != "API" {
		t.Errorf("Expected author 'API', got %s", article.Author)
	}
	if article.Content != genericResponse {
		t.Errorf("Expected content to be the JSON response")
	}
	if len(article.Tags) == 0 || article.Tags[0] != "api" {
		t.Errorf("Expected first tag to be 'api', got %v", article.Tags)
	}
}

func TestAPICollector_fetchAPI_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	collector := NewAPICollector()
	config := CollectConfig{URL: server.URL}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err == nil {
		t.Error("Expected error for HTTP 401, got nil")
	}

	if len(result.Articles) != 0 {
		t.Errorf("Expected 0 articles on error, got %d", len(result.Articles))
	}
}

func TestAPICollector_fetchAPI_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	collector := NewAPICollector()
	config := CollectConfig{URL: server.URL + "/api/test"}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	// 对于通用API，即使JSON无效也应该成功，作为原始内容处理
	if err != nil {
		t.Fatalf("Generic API should handle invalid JSON, got error: %v", err)
	}

	if len(result.Articles) != 1 {
		t.Errorf("Expected 1 article for generic API, got %d", len(result.Articles))
	}
	
	article := result.Articles[0]
	if article.Content != "invalid json" {
		t.Errorf("Expected content to be 'invalid json', got %s", article.Content)
	}
}

func TestAPICollector_convertGitHubRepos(t *testing.T) {
	collector := NewAPICollector()
	
	repos := []GitHubRepo{
		{
			ID:          123,
			Name:        "test-repo",
			FullName:    "user/test-repo",
			Description: "Test repository",
			HTMLURL:     "https://github.com/user/test-repo",
			Language:    "Go",
			CreatedAt:   "2024-01-01T00:00:00Z",
			UpdatedAt:   "2024-01-01T12:00:00Z",
			Owner: struct {
				Login string `json:"login"`
			}{
				Login: "user",
			},
			Topics: []string{"go", "test"},
		},
	}
	
	config := CollectConfig{
		URL:  "https://api.github.com/users/user/repos",
		Tags: []string{"extra-tag"},
	}
	
	articles := collector.convertGitHubRepos(repos, config)
	
	if len(articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(articles))
	}
	
	article := articles[0]
	if article.ID != "123" {
		t.Errorf("Expected ID '123', got %s", article.ID)
	}
	if article.Title != "user/test-repo" {
		t.Errorf("Expected title 'user/test-repo', got %s", article.Title)
	}
	if article.Author != "user" {
		t.Errorf("Expected author 'user', got %s", article.Author)
	}
	if article.Language != "Go" {
		t.Errorf("Expected language 'Go', got %s", article.Language)
	}
	
	// 检查标签合并
	expectedTags := []string{"go", "test", "extra-tag"}
	if len(article.Tags) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d", len(expectedTags), len(article.Tags))
	}
}

func TestAPICollector_extractSummary(t *testing.T) {
	collector := NewAPICollector()

	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{
			name:     "short content",
			content:  "Short",
			maxLen:   50,
			expected: "Short",
		},
		{
			name:     "long content",
			content:  "This is a very long content that needs truncation",
			maxLen:   20,
			expected: "This is a very long...",
		},
		{
			name:     "exact length",
			content:  "Exact length",
			maxLen:   12,
			expected: "Exact length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.extractSummary(tt.content, tt.maxLen)
			if result != tt.expected {
				t.Errorf("extractSummary() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// 基准测试
func BenchmarkAPICollector_Collect_GitHub(b *testing.B) {
	githubResponse := `[{"id": 1, "name": "repo", "full_name": "user/repo", "description": "test", "html_url": "https://github.com/user/repo", "language": "Go", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "owner": {"login": "user"}, "topics": ["go"]}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(githubResponse))
	}))
	defer server.Close()

	collector := NewAPICollector()
	config := CollectConfig{URL: server.URL + "/users/user/repos"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := collector.Collect(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}