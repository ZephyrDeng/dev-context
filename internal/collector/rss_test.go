package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRSSCollector_GetSourceType(t *testing.T) {
	collector := NewRSSCollector()
	if collector.GetSourceType() != "rss" {
		t.Errorf("Expected source type 'rss', got %s", collector.GetSourceType())
	}
}

func TestRSSCollector_Validate(t *testing.T) {
	collector := NewRSSCollector()

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
			config:    CollectConfig{URL: "http://example.com"},
			wantError: false,
		},
		{
			name:      "valid HTTPS URL",
			config:    CollectConfig{URL: "https://example.com"},
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

func TestRSSCollector_Collect_RSS(t *testing.T) {
	// 创建测试RSS feed
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS Feed</title>
    <description>A test RSS feed</description>
    <link>http://example.com</link>
    <item>
      <title>Test Article 1</title>
      <description>This is the first test article content.</description>
      <link>http://example.com/article1</link>
      <guid>article1</guid>
      <pubDate>Mon, 01 Jan 2024 12:00:00 GMT</pubDate>
      <author>Test Author</author>
      <category>Technology</category>
    </item>
    <item>
      <title>Test Article 2</title>
      <description>This is the second test article content.</description>
      <link>http://example.com/article2</link>
      <guid>article2</guid>
      <pubDate>Tue, 02 Jan 2024 12:00:00 GMT</pubDate>
      <author>Test Author 2</author>
      <category>Science</category>
    </item>
  </channel>
</rss>`

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{
		URL:         server.URL,
		MaxArticles: 10,
		Language:    "en",
		Tags:        []string{"test"},
	}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(result.Articles) != 2 {
		t.Errorf("Expected 2 articles, got %d", len(result.Articles))
	}

	// 检查第一篇文章
	article := result.Articles[0]
	if article.ID != "article1" {
		t.Errorf("Expected article ID 'article1', got %s", article.ID)
	}
	if article.Title != "Test Article 1" {
		t.Errorf("Expected title 'Test Article 1', got %s", article.Title)
	}
	if article.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %s", article.Author)
	}
	if article.URL != "http://example.com/article1" {
		t.Errorf("Expected URL 'http://example.com/article1', got %s", article.URL)
	}
	if article.SourceType != "rss" {
		t.Errorf("Expected source type 'rss', got %s", article.SourceType)
	}
	if len(article.Tags) == 0 || article.Tags[0] != "Technology" {
		t.Errorf("Expected first tag 'Technology', got %v", article.Tags)
	}
}

func TestRSSCollector_Collect_Atom(t *testing.T) {
	atomContent := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom Feed</title>
  <link href="http://example.com" />
  <entry>
    <id>http://example.com/entry1</id>
    <title>Test Entry 1</title>
    <link href="http://example.com/entry1" />
    <published>2024-01-01T12:00:00Z</published>
    <author>
      <name>Atom Author</name>
    </author>
    <content type="html">This is the first atom entry content.</content>
    <summary>First entry summary</summary>
    <category term="tech" />
  </entry>
</feed>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(atomContent))
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{URL: server.URL}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(result.Articles) != 1 {
		t.Errorf("Expected 1 article, got %d", len(result.Articles))
	}

	article := result.Articles[0]
	if article.ID != "http://example.com/entry1" {
		t.Errorf("Expected article ID 'http://example.com/entry1', got %s", article.ID)
	}
	if article.Title != "Test Entry 1" {
		t.Errorf("Expected title 'Test Entry 1', got %s", article.Title)
	}
	if article.Author != "Atom Author" {
		t.Errorf("Expected author 'Atom Author', got %s", article.Author)
	}
}

func TestRSSCollector_Collect_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{URL: server.URL}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err == nil {
		t.Error("Expected error for HTTP 404, got nil")
	}

	if len(result.Articles) != 0 {
		t.Errorf("Expected 0 articles on error, got %d", len(result.Articles))
	}
}

func TestRSSCollector_Collect_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid xml content"))
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{URL: server.URL}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err == nil {
		t.Error("Expected error for invalid XML, got nil")
	}

	if len(result.Articles) != 0 {
		t.Errorf("Expected 0 articles on error, got %d", len(result.Articles))
	}
}

func TestRSSCollector_Collect_MaxArticles(t *testing.T) {
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS Feed</title>
    <item>
      <title>Article 1</title>
      <link>http://example.com/1</link>
      <guid>1</guid>
    </item>
    <item>
      <title>Article 2</title>
      <link>http://example.com/2</link>
      <guid>2</guid>
    </item>
    <item>
      <title>Article 3</title>
      <link>http://example.com/3</link>
      <guid>3</guid>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{
		URL:         server.URL,
		MaxArticles: 2,
	}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(result.Articles) != 2 {
		t.Errorf("Expected 2 articles (limited by MaxArticles), got %d", len(result.Articles))
	}
}

func TestRSSCollector_Collect_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟慢响应
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<?xml version='1.0'?><rss version='2.0'><channel></channel></rss>"))
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{
		URL:     server.URL,
		Timeout: 100 * time.Millisecond, // 超时时间比服务器响应时间短
	}

	ctx := context.Background()
	result, err := collector.Collect(ctx, config)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if len(result.Articles) != 0 {
		t.Errorf("Expected 0 articles on timeout, got %d", len(result.Articles))
	}
}

func TestRSSCollector_parseTime(t *testing.T) {
	collector := NewRSSCollector()

	tests := []struct {
		name      string
		timeStr   string
		wantError bool
	}{
		{
			name:      "RFC1123Z format",
			timeStr:   "Mon, 02 Jan 2006 15:04:05 -0700",
			wantError: false,
		},
		{
			name:      "RFC3339 format",
			timeStr:   "2006-01-02T15:04:05Z",
			wantError: false,
		},
		{
			name:      "Invalid format",
			timeStr:   "invalid-time",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := collector.parseTime(tt.timeStr)
			if (err != nil) != tt.wantError {
				t.Errorf("parseTime() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestRSSCollector_cleanHTML(t *testing.T) {
	collector := NewRSSCollector()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple HTML tags",
			input:    "<p>Hello <strong>world</strong>!</p>",
			expected: "Hello world !",
		},
		{
			name:     "multiple spaces",
			input:    "<div>Multiple    spaces   here</div>",
			expected: "Multiple spaces here",
		},
		{
			name:     "empty content",
			input:    "",
			expected: "",
		},
		{
			name:     "no HTML tags",
			input:    "Plain text content",
			expected: "Plain text content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.cleanHTML(tt.input)
			if result != tt.expected {
				t.Errorf("cleanHTML() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRSSCollector_extractSummary(t *testing.T) {
	collector := NewRSSCollector()

	tests := []struct {
		name     string
		content  string
		maxLen   int
		expected string
	}{
		{
			name:     "short content",
			content:  "Short content",
			maxLen:   50,
			expected: "Short content",
		},
		{
			name:     "long content with word boundary",
			content:  "This is a very long content that should be truncated at word boundary",
			maxLen:   30,
			expected: "This is a very long content...",
		},
		{
			name:     "long content without word boundary",
			content:  "Thisisaverylongcontentwithoutspaces",
			maxLen:   10,
			expected: "Thisisaver...",
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

func TestRSSCollector_generateID(t *testing.T) {
	collector := NewRSSCollector()

	// 测试相同内容生成相同ID
	content := "test content"
	id1 := collector.generateID(content)
	id2 := collector.generateID(content)

	if id1 != id2 {
		t.Errorf("generateID() should return same ID for same content, got %v and %v", id1, id2)
	}

	// 测试不同内容生成不同ID
	content2 := "different content"
	id3 := collector.generateID(content2)

	if id1 == id3 {
		t.Errorf("generateID() should return different ID for different content")
	}

	// 测试ID格式
	if len(id1) != 16 { // SHA256前8字节的十六进制表示
		t.Errorf("generateID() should return 16 character hex string, got %d characters", len(id1))
	}
}

// 基准测试
func BenchmarkRSSCollector_Collect(b *testing.B) {
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Benchmark RSS Feed</title>
    <item>
      <title>Benchmark Article</title>
      <description>Benchmark article content</description>
      <link>http://example.com/benchmark</link>
      <guid>benchmark</guid>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	collector := NewRSSCollector()
	config := CollectConfig{URL: server.URL}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := collector.Collect(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}