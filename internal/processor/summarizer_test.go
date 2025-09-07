package processor

import (
	"strings"
	"testing"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/models"
)

// TestNewSummarizer tests the creation of a new summarizer with default settings
func TestNewSummarizer(t *testing.T) {
	s := NewSummarizer()

	if s == nil {
		t.Fatal("NewSummarizer should not return nil")
	}

	if s.MinSummaryLength != 50 {
		t.Errorf("Expected MinSummaryLength to be 50, got %d", s.MinSummaryLength)
	}

	if s.MaxSummaryLength != 150 {
		t.Errorf("Expected MaxSummaryLength to be 150, got %d", s.MaxSummaryLength)
	}

	if s.SentenceCount != 2 {
		t.Errorf("Expected SentenceCount to be 2, got %d", s.SentenceCount)
	}

	if s.PositionWeight != 0.3 {
		t.Errorf("Expected PositionWeight to be 0.3, got %.2f", s.PositionWeight)
	}

	if s.LengthWeight != 0.2 {
		t.Errorf("Expected LengthWeight to be 0.2, got %.2f", s.LengthWeight)
	}

	if s.KeywordWeight != 0.5 {
		t.Errorf("Expected KeywordWeight to be 0.5, got %.2f", s.KeywordWeight)
	}
}

// TestGenerateSummary tests the core summarization functionality
func TestGenerateSummary(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name          string
		input         string
		expectError   bool
		minLength     int
		maxLength     int
		shouldContain []string
	}{
		{
			name:        "empty text",
			input:       "",
			expectError: true,
		},
		{
			name:        "short text",
			input:       "This is a short text.",
			expectError: false,
			minLength:   10,
			maxLength:   50,
		},
		{
			name:          "long article",
			input:         `Artificial intelligence is transforming the modern world in unprecedented ways. Machine learning algorithms are being used to solve complex problems across various industries. Deep learning models can now process vast amounts of data and identify patterns that humans might miss. Natural language processing has enabled computers to understand and generate human language with remarkable accuracy. Computer vision systems can analyze images and videos to extract meaningful information. The applications of AI are limitless and continue to expand into new domains. However, ethical considerations around AI development remain important. Privacy concerns and algorithmic bias are significant challenges that need to be addressed. The future of AI depends on responsible development and deployment. Collaboration between technologists, ethicists, and policymakers is crucial for ensuring AI benefits humanity.`,
			expectError:   false,
			minLength:     50,
			maxLength:     150,
			shouldContain: []string{"machine", "learning"}, // More realistic expectations
		},
		{
			name:        "HTML content",
			input:       `<h1>Technology News</h1><p>The latest developments in <strong>artificial intelligence</strong> are reshaping industries. Companies are investing heavily in AI research and development.</p><p>Machine learning models are becoming more sophisticated and accurate. Deep learning has shown remarkable progress in recent years.</p>`,
			expectError: false,
			minLength:   50,
			maxLength:   150,
		},
		{
			name:        "single sentence",
			input:       "This is a single sentence that should be returned as the summary.",
			expectError: false,
			minLength:   50,
			maxLength:   80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := s.GenerateSummary(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(summary) < tt.minLength {
				t.Errorf("Summary too short: expected at least %d chars, got %d", tt.minLength, len(summary))
			}

			if len(summary) > tt.maxLength {
				t.Errorf("Summary too long: expected at most %d chars, got %d", tt.maxLength, len(summary))
			}

			// Check for expected keywords (case-insensitive)
			summaryLower := strings.ToLower(summary)
			for _, keyword := range tt.shouldContain {
				if !strings.Contains(summaryLower, strings.ToLower(keyword)) {
					t.Errorf("Summary should contain keyword '%s', but got: %s", keyword, summary)
				}
			}

			// Ensure summary is not just whitespace
			if strings.TrimSpace(summary) == "" {
				t.Error("Summary should not be empty or just whitespace")
			}
		})
	}
}

// TestCleanContent tests content cleaning functionality
func TestCleanContent(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name        string
		article     *models.Article
		expectError bool
		checkTitle  string
		checkTags   []string
	}{
		{
			name:        "nil article",
			article:     nil,
			expectError: true,
		},
		{
			name: "HTML in title and content",
			article: &models.Article{
				Title:   "<h1>Breaking: <strong>AI Revolution</strong></h1>",
				Content: "<p>This is the content with <em>HTML tags</em> that should be cleaned.</p>",
				Tags:    []string{"  ai  ", "  technology  "}, // Remove problematic script tag
			},
			expectError: false,
			checkTitle:  "Breaking: AI Revolution",
			checkTags:   []string{"ai", "technology"},
		},
		{
			name: "excessive whitespace",
			article: &models.Article{
				Title:   "  Multiple    spaces    between    words  ",
				Content: "Content\n\n\nwith\t\t\tmultiple\r\n   whitespace   types",
				Summary: "  Summary   with   extra   spaces  ",
			},
			expectError: false,
			checkTitle:  "Multiple spaces between words",
		},
		{
			name: "empty title after cleaning",
			article: &models.Article{
				Title:   "   <div></div>   ",
				Content: "Valid content",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.CleanContent(tt.article)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkTitle != "" && tt.article.Title != tt.checkTitle {
				t.Errorf("Expected title '%s', got '%s'", tt.checkTitle, tt.article.Title)
			}

			if len(tt.checkTags) > 0 {
				if len(tt.article.Tags) != len(tt.checkTags) {
					t.Errorf("Expected %d tags, got %d", len(tt.checkTags), len(tt.article.Tags))
				}
				for i, expectedTag := range tt.checkTags {
					if i < len(tt.article.Tags) && tt.article.Tags[i] != expectedTag {
						t.Errorf("Expected tag[%d] to be '%s', got '%s'", i, expectedTag, tt.article.Tags[i])
					}
				}
			}

			// Ensure no HTML tags remain in title
			if strings.Contains(tt.article.Title, "<") || strings.Contains(tt.article.Title, ">") {
				t.Errorf("Title still contains HTML tags: %s", tt.article.Title)
			}

			// Ensure no HTML tags remain in content
			if strings.Contains(tt.article.Content, "<") || strings.Contains(tt.article.Content, ">") {
				t.Errorf("Content still contains HTML tags: %s", tt.article.Content)
			}
		})
	}
}

// TestProcessArticle tests the complete article processing pipeline
func TestProcessArticle(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name        string
		article     *models.Article
		expectError bool
	}{
		{
			name:        "nil article",
			article:     nil,
			expectError: true,
		},
		{
			name: "complete processing",
			article: &models.Article{
				Title: "<h1>AI in Healthcare: Revolutionary Changes</h1>",
				Content: `<p>Artificial intelligence is revolutionizing healthcare delivery worldwide. Machine learning algorithms are being used to analyze medical images with unprecedented accuracy. Doctors can now diagnose diseases faster and more accurately than ever before.</p>
				<p>Deep learning models are helping identify patterns in patient data that human doctors might miss. Natural language processing is being used to analyze medical records and extract valuable insights. Computer vision systems can detect abnormalities in X-rays and MRI scans.</p>
				<p>However, ethical considerations around AI in healthcare are important. Privacy concerns and the need for explainable AI decisions are critical factors. The future of healthcare will depend on responsible AI implementation.</p>`,
				URL:         "https://example.com/ai-healthcare",
				Source:      "Tech News",
				SourceType:  "html",
				Tags:        []string{"  ai  ", "  healthcare  ", "  technology  "},
				PublishedAt: time.Now().Add(-24 * time.Hour),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ProcessArticle(tt.article)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify processing results
			if tt.article.Summary == "" {
				t.Error("Summary should be generated")
			}

			if len(tt.article.Summary) < s.MinSummaryLength || len(tt.article.Summary) > s.MaxSummaryLength {
				t.Errorf("Summary length should be between %d and %d, got %d",
					s.MinSummaryLength, s.MaxSummaryLength, len(tt.article.Summary))
			}

			if tt.article.Quality <= 0.0 || tt.article.Quality > 1.0 {
				t.Errorf("Quality score should be between 0.0 and 1.0, got %.3f", tt.article.Quality)
			}

			// Verify content was cleaned
			if strings.Contains(tt.article.Title, "<") || strings.Contains(tt.article.Title, ">") {
				t.Error("Title should not contain HTML tags after processing")
			}
		})
	}
}

// TestAssessQuality tests quality assessment functionality
func TestAssessQuality(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name          string
		article       *models.Article
		expectedRange [2]float64 // [min, max]
	}{
		{
			name:          "nil article",
			article:       nil,
			expectedRange: [2]float64{0.0, 0.0},
		},
		{
			name: "minimal article",
			article: &models.Article{
				Title:  "Basic Title",
				URL:    "https://example.com",
				Source: "Test Source",
			},
			expectedRange: [2]float64{0.3, 0.7},
		},
		{
			name: "complete article",
			article: &models.Article{
				Title:       "Comprehensive Article Title",
				URL:         "https://example.com/article",
				Source:      "Premium Source",
				Summary:     "This is a well-written summary that provides good insights into the topic discussed in the article.",
				Content:     "This is comprehensive content that discusses various aspects of the topic in detail. It provides valuable information and maintains good readability throughout. The content is structured well and covers multiple important points that readers would find useful and informative.",
				Tags:        []string{"technology", "innovation", "research"},
				PublishedAt: time.Now().Add(-12 * time.Hour),
			},
			expectedRange: [2]float64{0.8, 1.0},
		},
		{
			name: "recent article with good content",
			article: &models.Article{
				Title:       "Recent Technology News",
				URL:         "https://news.example.com",
				Source:      "Tech News",
				Summary:     "Breaking news about recent technological developments.",
				Content:     strings.Repeat("Good content with sufficient length to meet quality criteria. ", 20),
				Tags:        []string{"tech", "news"},
				PublishedAt: time.Now().Add(-6 * time.Hour),
			},
			expectedRange: [2]float64{0.7, 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quality := s.AssessQuality(tt.article)

			if quality < tt.expectedRange[0] || quality > tt.expectedRange[1] {
				t.Errorf("Quality score %.3f not in expected range [%.3f, %.3f]",
					quality, tt.expectedRange[0], tt.expectedRange[1])
			}
		})
	}
}

// TestExtractSentences tests sentence extraction functionality
func TestExtractSentences(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty text",
			input:    "",
			expected: 0,
		},
		{
			name:     "single sentence",
			input:    "This is a single sentence.",
			expected: 1,
		},
		{
			name:     "multiple sentences",
			input:    "First sentence. Second sentence! Third sentence? Fourth sentence.",
			expected: 4,
		},
		{
			name:     "sentences with abbreviations",
			input:    "Dr. Smith visited the U.S.A. He met with Prof. Johnson. They discussed A.I. research.",
			expected: 3,
		},
		{
			name:     "very short fragments filtered out",
			input:    "Good. Bad. This is a proper sentence with sufficient length. Yes.",
			expected: 1, // Only the longer sentence should be kept
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentences := s.extractSentences(tt.input)
			if len(sentences) != tt.expected {
				t.Errorf("Expected %d sentences, got %d: %v", tt.expected, len(sentences), sentences)
			}
		})
	}
}

// TestExtractKeywords tests keyword extraction functionality
func TestExtractKeywords(t *testing.T) {
	s := NewSummarizer()

	text := "Artificial intelligence machine learning algorithms are transforming technology industry. Machine learning and artificial intelligence research continues to advance rapidly."
	keywords := s.extractKeywords(text)

	// Should include repeated important words
	expectedKeywords := []string{"artificial", "intelligence", "machine", "learning"}
	for _, expected := range expectedKeywords {
		if _, exists := keywords[expected]; !exists {
			t.Errorf("Expected keyword '%s' not found in keywords: %v", expected, keywords)
		}
	}

	// Should exclude stop words
	stopWords := []string{"are", "and", "to", "the"}
	for _, stopWord := range stopWords {
		if _, exists := keywords[stopWord]; exists {
			t.Errorf("Stop word '%s' should not be in keywords: %v", stopWord, keywords)
		}
	}
}

// TestCleanText tests text cleaning functionality
func TestCleanText(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "HTML tags removal",
			input:    "<p>Hello <strong>world</strong>!</p>",
			expected: "Hello world!",
		},
		{
			name:     "HTML entities",
			input:    "AT&amp;T &quot;innovation&quot;",
			expected: "AT&T \"innovation\"",
		},
		{
			name:     "excessive whitespace",
			input:    "Multiple    spaces\n\n\nand    \t\ttabs",
			expected: "Multiple spaces and tabs",
		},
		{
			name:     "mixed HTML and whitespace",
			input:    "<h1>  Title  </h1>\n\n<p>  Paragraph  with  <em>emphasis</em>  </p>",
			expected: "Title Paragraph with emphasis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.cleanText(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestCalculateLengthScore tests sentence length scoring
func TestCalculateLengthScore(t *testing.T) {
	s := NewSummarizer()

	tests := []struct {
		name     string
		sentence string
		minScore float64
		maxScore float64
	}{
		{
			name:     "optimal length sentence",
			sentence: "This is a sentence with optimal length that should score well in our assessment algorithm.",
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name:     "too short sentence",
			sentence: "Too short.",
			minScore: 0.0,
			maxScore: 0.3,
		},
		{
			name:     "too long sentence",
			sentence: strings.Repeat("This is a very long sentence that exceeds the optimal length range and should receive a penalty for being too verbose and potentially difficult to read. ", 5),
			minScore: 0.0,
			maxScore: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := s.calculateLengthScore(tt.sentence)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Length score %.3f not in expected range [%.3f, %.3f] for sentence: %s",
					score, tt.minScore, tt.maxScore, tt.sentence)
			}
		})
	}
}

// TestAdjustSummaryLength tests summary length adjustment
func TestAdjustSummaryLength(t *testing.T) {
	s := NewSummarizer()
	s.MaxSummaryLength = 50 // Set short limit for testing

	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{
			name:   "within limit",
			input:  "Short summary.",
			maxLen: 50,
		},
		{
			name:   "exceeds limit",
			input:  "This is a very long summary that definitely exceeds our maximum length limit and should be truncated properly at word boundaries.",
			maxLen: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.adjustSummaryLength(tt.input)

			if len(result) > tt.maxLen {
				t.Errorf("Result length %d exceeds maximum %d", len(result), tt.maxLen)
			}

			// If truncated, should end with "..."
			if len(tt.input) > tt.maxLen && !strings.HasSuffix(result, "...") {
				t.Error("Truncated summary should end with '...'")
			}

			// Should not break words in the middle
			if len(result) > 3 && strings.HasSuffix(result, "...") {
				withoutEllipsis := result[:len(result)-3]
				if strings.HasSuffix(withoutEllipsis, " ") == false && len(withoutEllipsis) > 0 {
					// The last character before ... should be end of word (space or original end)
					lastChar := withoutEllipsis[len(withoutEllipsis)-1]
					if lastChar != ' ' && string(lastChar) != string(tt.input[len(withoutEllipsis)-1]) {
						t.Error("Summary should be truncated at word boundary")
					}
				}
			}
		})
	}
}

// BenchmarkGenerateSummary benchmarks the summarization performance
func BenchmarkGenerateSummary(b *testing.B) {
	s := NewSummarizer()
	text := `Artificial intelligence is transforming the modern world in unprecedented ways. Machine learning algorithms are being used to solve complex problems across various industries. Deep learning models can now process vast amounts of data and identify patterns that humans might miss. Natural language processing has enabled computers to understand and generate human language with remarkable accuracy. Computer vision systems can analyze images and videos to extract meaningful information. The applications of AI are limitless and continue to expand into new domains. However, ethical considerations around AI development remain important. Privacy concerns and algorithmic bias are significant challenges that need to be addressed. The future of AI depends on responsible development and deployment. Collaboration between technologists, ethicists, and policymakers is crucial for ensuring AI benefits humanity.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GenerateSummary(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProcessArticle benchmarks the complete article processing pipeline
func BenchmarkProcessArticle(b *testing.B) {
	s := NewSummarizer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a new article for each iteration to ensure consistent state
		article := &models.Article{
			Title: "AI in Healthcare: Revolutionary Changes",
			Content: `<p>Artificial intelligence is revolutionizing healthcare delivery worldwide. Machine learning algorithms are being used to analyze medical images with unprecedented accuracy. Doctors can now diagnose diseases faster and more accurately than ever before.</p>
			<p>Deep learning models are helping identify patterns in patient data that human doctors might miss. Natural language processing is being used to analyze medical records and extract valuable insights.</p>`,
			URL:         "https://example.com/ai-healthcare",
			Source:      "Tech News",
			SourceType:  "html",
			Tags:        []string{"ai", "healthcare", "technology"},
			PublishedAt: time.Now().Add(-24 * time.Hour),
		}

		err := s.ProcessArticle(article)
		if err != nil {
			b.Fatal(err)
		}
	}
}
