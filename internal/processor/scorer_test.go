package processor

import (
	"strings"
	"testing"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/models"
)

// TestData for consistent testing
var testArticles = []*models.Article{
	{
		ID:          "article1",
		Title:       "Go Programming Language Tutorial",
		Summary:     "Learn Go programming with practical examples",
		Content:     "Go is a programming language developed by Google. It's fast, simple, and efficient for building scalable applications.",
		Tags:        []string{"go", "programming", "tutorial"},
		Source:      "tech-blog",
		SourceType:  "rss",
		PublishedAt: time.Now().Add(-24 * time.Hour),
		Relevance:   0.0,
		Quality:     0.8,
	},
	{
		ID:          "article2",
		Title:       "Advanced JavaScript Frameworks",
		Summary:     "Exploring modern JavaScript frameworks for web development",
		Content:     "JavaScript frameworks like React, Vue, and Angular have revolutionized web development. Each offers unique advantages.",
		Tags:        []string{"javascript", "react", "vue", "angular"},
		Source:      "web-dev",
		SourceType:  "api",
		PublishedAt: time.Now().Add(-48 * time.Hour),
		Relevance:   0.0,
		Quality:     0.9,
	},
	{
		ID:          "article3",
		Title:       "Machine Learning with Python",
		Summary:     "Introduction to ML algorithms using Python libraries",
		Content:     "Python is the most popular language for machine learning. Libraries like TensorFlow and PyTorch make ML accessible.",
		Tags:        []string{"python", "machine-learning", "tensorflow", "pytorch"},
		Source:      "ai-news",
		SourceType:  "html",
		PublishedAt: time.Now().Add(-12 * time.Hour),
		Relevance:   0.0,
		Quality:     0.95,
	},
	{
		ID:          "article4",
		Title:       "Database Optimization Techniques",
		Summary:     "Best practices for optimizing database performance",
		Content:     "Database optimization is crucial for application performance. Indexing, query optimization, and proper schema design are key.",
		Tags:        []string{"database", "optimization", "sql", "performance"},
		Source:      "db-weekly",
		SourceType:  "rss",
		PublishedAt: time.Now().Add(-72 * time.Hour),
		Relevance:   0.0,
		Quality:     0.85,
	},
}

func TestNewRelevanceScorer(t *testing.T) {
	keywords := []string{"go", "programming", "tutorial"}
	scorer := NewRelevanceScorer(keywords)

	if scorer == nil {
		t.Fatal("NewRelevanceScorer returned nil")
	}

	if len(scorer.keywordMatcher.Keywords) != len(keywords) {
		t.Errorf("Expected %d keywords, got %d", len(keywords), len(scorer.keywordMatcher.Keywords))
	}

	// Verify default weight configuration
	if scorer.keywordMatcher.WeightConfig.TitleWeight != 3.0 {
		t.Errorf("Expected TitleWeight 3.0, got %f", scorer.keywordMatcher.WeightConfig.TitleWeight)
	}

	if scorer.keywordMatcher.WeightConfig.SummaryWeight != 2.0 {
		t.Errorf("Expected SummaryWeight 2.0, got %f", scorer.keywordMatcher.WeightConfig.SummaryWeight)
	}
}

func TestSimpleStemmer(t *testing.T) {
	stemmer := NewSimpleStemmer()

	testCases := []struct {
		word     string
		expected string
	}{
		{"running", "runn"},
		{"programmed", "programm"},
		{"faster", "fast"},
		{"beautiful", "beauti"},
		{"development", "develop"},
		{"go", "go"}, // short word unchanged
		{"", ""},     // empty string
	}

	for _, tc := range testCases {
		result := stemmer.Stem(tc.word)
		if result != tc.expected {
			t.Errorf("Stem(%q) = %q, expected %q", tc.word, result, tc.expected)
		}
	}
}

func TestKeywordMatcherScoreKeywordMatch(t *testing.T) {
	keywords := []string{"go", "programming"}
	scorer := NewRelevanceScorer(keywords)

	article := testArticles[0] // Go Programming Language Tutorial
	score := scorer.keywordMatcher.ScoreKeywordMatch(article)

	if score <= 0.0 {
		t.Error("Expected positive score for matching keywords")
	}

	if score > 1.0 {
		t.Errorf("Score should not exceed 1.0, got %f", score)
	}

	// Test with non-matching keywords
	scorer.UpdateKeywords([]string{"python", "machine-learning"})
	score = scorer.keywordMatcher.ScoreKeywordMatch(article)

	if score > 0.2 { // Allow some tolerance for partial matching
		t.Errorf("Expected low score for non-matching keywords, got %f", score)
	}
}

func TestTFIDFScoring(t *testing.T) {
	keywords := []string{"go", "programming", "javascript"}
	scorer := NewRelevanceScorer(keywords)
	scorer.SetCorpus(testArticles)

	// Test scoring with corpus
	for _, article := range testArticles {
		score := scorer.ScoreRelevance(article)

		if score < 0.0 || score > 1.0 {
			t.Errorf("Score should be between 0 and 1, got %f for article %s", score, article.ID)
		}
	}

	// Go article should score higher for Go-related keywords
	goScore := scorer.ScoreRelevance(testArticles[0])
	jsScore := scorer.ScoreRelevance(testArticles[1])

	if goScore <= jsScore {
		t.Errorf("Go article should score higher than JS article for Go keywords: Go=%f, JS=%f", goScore, jsScore)
	}
}

func TestTermFrequencyCalculation(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"go", "programming"})
	scorer.SetCorpus(testArticles)

	article := testArticles[0]
	termFreqs := scorer.calculateTermFrequencies(article)

	if len(termFreqs) == 0 {
		t.Error("Expected non-empty term frequencies")
	}

	// Verify that terms are sorted by frequency
	for i := 1; i < len(termFreqs); i++ {
		if termFreqs[i-1].Frequency < termFreqs[i].Frequency {
			t.Error("Term frequencies should be sorted in descending order")
		}
	}

	// Check if important terms are captured
	foundGo := false
	foundProgramming := false

	for _, tf := range termFreqs {
		if strings.Contains(tf.Term, "go") {
			foundGo = true
		}
		if strings.Contains(tf.Term, "programm") { // stemmed version
			foundProgramming = true
		}
	}

	if !foundGo {
		t.Error("Expected to find 'go' in term frequencies")
	}
	if !foundProgramming {
		t.Error("Expected to find 'programming' (stemmed) in term frequencies")
	}
}

func TestIDFCalculation(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"go", "programming", "unique", "common"})

	// Create a more diverse corpus for better IDF testing
	testCorpus := []*models.Article{
		{ID: "1", Title: "Go programming tutorial", Content: "Go is a programming language", Tags: []string{"go", "programming"}},
		{ID: "2", Title: "JavaScript programming guide", Content: "JavaScript is a programming language", Tags: []string{"javascript", "programming"}},
		{ID: "3", Title: "Python tutorial", Content: "Python is great for scripting", Tags: []string{"python"}},
		{ID: "4", Title: "Database optimization", Content: "Optimize your database for better performance", Tags: []string{"database", "optimization"}},
	}

	scorer.SetCorpus(testCorpus)

	// Test IDF for common term (appears in multiple documents)
	programmingIDF := scorer.calculateIDF("programming")

	// Test IDF for term that doesn't exist
	uniqueIDF := scorer.calculateIDF("unique")

	// IDF should be higher for rarer terms
	if uniqueIDF <= programmingIDF {
		t.Errorf("IDF for non-existent term should be higher: unique=%f, programming=%f", uniqueIDF, programmingIDF)
	}

	// Test caching
	programmingIDF2 := scorer.calculateIDF("programming")
	if programmingIDF != programmingIDF2 {
		t.Error("IDF calculation should be consistent (cached)")
	}
}

func TestScoreRelevanceAccuracy(t *testing.T) {
	// Test for >85% relevance scoring accuracy requirement
	testCases := []struct {
		keywords     []string
		expectedHigh []string // Article IDs that should score high
		expectedLow  []string // Article IDs that should score low
	}{
		{
			keywords:     []string{"go", "programming"},
			expectedHigh: []string{"article1"},
			expectedLow:  []string{"article3", "article4"},
		},
		{
			keywords:     []string{"javascript", "react"},
			expectedHigh: []string{"article2"},
			expectedLow:  []string{"article1", "article4"},
		},
		{
			keywords:     []string{"python", "machine-learning"},
			expectedHigh: []string{"article3"},
			expectedLow:  []string{"article1", "article2"},
		},
	}

	correctPredictions := 0
	totalPredictions := 0

	for _, tc := range testCases {
		scorer := NewRelevanceScorer(tc.keywords)
		scorer.SetCorpus(testArticles)

		scores := make(map[string]float64)
		for _, article := range testArticles {
			scores[article.ID] = scorer.ScoreRelevance(article)
		}

		// Check high scoring articles
		for _, expectedHighID := range tc.expectedHigh {
			highScore := scores[expectedHighID]
			correctlyRankedHigh := 0

			for _, expectedLowID := range tc.expectedLow {
				lowScore := scores[expectedLowID]
				if highScore > lowScore {
					correctlyRankedHigh++
				}
				totalPredictions++
			}

			correctPredictions += correctlyRankedHigh
		}
	}

	accuracy := float64(correctPredictions) / float64(totalPredictions)
	if accuracy < 0.85 {
		t.Errorf("Relevance scoring accuracy %.2f%% is below required 85%%", accuracy*100)
	}

	t.Logf("Relevance scoring accuracy: %.2f%%", accuracy*100)
}

func TestWeightConfiguration(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})

	// Test custom weight configuration
	customConfig := WeightConfig{
		TitleWeight:   5.0,
		SummaryWeight: 3.0,
		ContentWeight: 1.5,
		TagWeight:     4.5,
		ExactMatch:    2.0,
		PartialMatch:  1.0,
	}

	scorer.SetWeightConfig(customConfig)
	retrievedConfig := scorer.GetWeightConfig()

	if retrievedConfig.TitleWeight != customConfig.TitleWeight {
		t.Errorf("Expected TitleWeight %f, got %f", customConfig.TitleWeight, retrievedConfig.TitleWeight)
	}
}

func TestUpdateKeywords(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"old", "keywords"})

	oldKeywords := scorer.GetKeywords()
	if len(oldKeywords) != 2 {
		t.Errorf("Expected 2 keywords initially, got %d", len(oldKeywords))
	}

	newKeywords := []string{"new", "updated", "keywords"}
	scorer.UpdateKeywords(newKeywords)

	updatedKeywords := scorer.GetKeywords()
	if len(updatedKeywords) != len(newKeywords) {
		t.Errorf("Expected %d keywords after update, got %d", len(newKeywords), len(updatedKeywords))
	}

	for i, keyword := range newKeywords {
		if updatedKeywords[i] != keyword {
			t.Errorf("Expected keyword %s at position %d, got %s", keyword, i, updatedKeywords[i])
		}
	}
}

func TestClearCache(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	scorer.SetCorpus(testArticles)

	// Calculate some scores to populate cache
	scorer.ScoreRelevance(testArticles[0])

	// Verify cache has entries
	if len(scorer.tfCache) == 0 && len(scorer.idfCache) == 0 {
		t.Skip("Cannot verify cache populated")
	}

	scorer.ClearCache()

	if len(scorer.tfCache) != 0 {
		t.Error("TF cache should be cleared")
	}

	if len(scorer.idfCache) != 0 {
		t.Error("IDF cache should be cleared")
	}
}

func TestGetTopTerms(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"go", "programming"})
	scorer.SetCorpus(testArticles)

	article := testArticles[0]
	topTerms := scorer.GetTopTerms(article, 5)

	if len(topTerms) > 5 {
		t.Errorf("Expected at most 5 terms, got %d", len(topTerms))
	}

	if len(topTerms) == 0 {
		t.Error("Expected at least some terms")
	}

	// Verify terms are sorted by frequency
	for i := 1; i < len(topTerms); i++ {
		if topTerms[i-1].Frequency < topTerms[i].Frequency {
			t.Error("Terms should be sorted by frequency in descending order")
		}
	}
}

func TestStopWordsFiltering(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"the", "and", "programming"}) // Include stop words
	scorer.SetCorpus(testArticles)

	article := testArticles[0]
	termFreqs := scorer.calculateTermFrequencies(article)

	// Check that common stop words are filtered out
	stopWords := []string{"the", "and", "is", "a", "to"}
	for _, stopWord := range stopWords {
		for _, tf := range termFreqs {
			if tf.Term == stopWord {
				t.Errorf("Stop word '%s' should be filtered out", stopWord)
			}
		}
	}
}

func TestNormalizeText(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})

	testCases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"Test123!@#", "test"},
		{"  Multiple   Spaces  ", "multiple   spaces"},
		{"", ""},
		{"CamelCase", "camelcase"},
	}

	for _, tc := range testCases {
		result := scorer.normalizeText(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeText(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestTokenizeText(t *testing.T) {
	keywords := []string{"test"}
	scorer := NewRelevanceScorer(keywords)
	km := scorer.keywordMatcher

	text := "Hello, world! This is a test-case with various123 symbols."
	tokens := km.tokenizeText(text)

	expected := []string{"hello", "world", "this", "is", "a", "test", "case", "with", "various", "symbols"}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, token := range tokens {
		if i < len(expected) && token != expected[i] {
			t.Errorf("Expected token %q at position %d, got %q", expected[i], i, token)
		}
	}
}

func TestScoreTextMatch(t *testing.T) {
	keywords := []string{"programming", "development"}
	scorer := NewRelevanceScorer(keywords)
	km := scorer.keywordMatcher

	// Test exact match
	exactScore := km.scoreTextMatch("Go programming is great", "programming")
	if exactScore <= 0 {
		t.Error("Exact match should have positive score")
	}

	// Test partial match
	partialScore := km.scoreTextMatch("Go programmer writes code", "programming")
	if partialScore <= 0 {
		t.Error("Partial match should have positive score")
	}

	// Exact match should score higher than partial match
	if exactScore <= partialScore {
		t.Errorf("Exact match should score higher: exact=%f, partial=%f", exactScore, partialScore)
	}

	// Test no match
	noMatchScore := km.scoreTextMatch("Completely unrelated text", "programming")
	if noMatchScore > 0.1 {
		t.Errorf("No match should have very low score, got %f", noMatchScore)
	}
}

func TestScoreTagMatch(t *testing.T) {
	keywords := []string{"go", "programming"}
	scorer := NewRelevanceScorer(keywords)
	km := scorer.keywordMatcher

	// Test exact tag match
	tags := []string{"go", "language", "tutorial"}
	exactScore := km.scoreTagMatch(tags, "go")
	if exactScore <= 0 {
		t.Error("Exact tag match should have positive score")
	}

	// Test partial tag match
	partialScore := km.scoreTagMatch(tags, "lang")
	if partialScore <= 0 {
		t.Error("Partial tag match should have positive score")
	}

	// Test no match
	noMatchScore := km.scoreTagMatch(tags, "python")
	if noMatchScore > 0 {
		t.Errorf("No match should have zero score, got %f", noMatchScore)
	}

	// Exact match should score higher than partial match
	if exactScore <= partialScore {
		t.Errorf("Exact tag match should score higher: exact=%f, partial=%f", exactScore, partialScore)
	}
}

func TestArticleContainsTerm(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})

	article := &models.Article{
		Title:   "Go Programming Tutorial",
		Summary: "Learn Go basics",
		Content: "Go is a programming language",
		Tags:    []string{"go", "programming"},
	}

	// Test term in title
	if !scorer.articleContainsTerm(article, "go") {
		t.Error("Should find 'go' in article")
	}

	// Test term in content
	if !scorer.articleContainsTerm(article, "language") {
		t.Error("Should find 'language' in article")
	}

	// Test term in tags
	if !scorer.articleContainsTerm(article, "programming") {
		t.Error("Should find 'programming' in tags")
	}

	// Test stemmed matching
	if !scorer.articleContainsTerm(article, "programm") {
		t.Error("Should find stemmed version of 'programming'")
	}

	// Test non-existent term
	if scorer.articleContainsTerm(article, "nonexistent") {
		t.Error("Should not find non-existent term")
	}
}

func BenchmarkScoreRelevance(b *testing.B) {
	keywords := []string{"go", "programming", "javascript", "python"}
	scorer := NewRelevanceScorer(keywords)
	scorer.SetCorpus(testArticles)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scorer.ScoreRelevance(testArticles[i%len(testArticles)])
	}
}

func BenchmarkCalculateTermFrequencies(b *testing.B) {
	scorer := NewRelevanceScorer([]string{"test"})
	article := testArticles[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scorer.calculateTermFrequencies(article)
	}
}

func BenchmarkKeywordMatch(b *testing.B) {
	keywords := []string{"go", "programming", "javascript"}
	scorer := NewRelevanceScorer(keywords)
	article := testArticles[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scorer.keywordMatcher.ScoreKeywordMatch(article)
	}
}

// Integration test combining scorer with real-world scenario
func TestScorerIntegration(t *testing.T) {
	// Setup a realistic scenario
	keywords := []string{"machine learning", "python", "tensorflow"}
	scorer := NewRelevanceScorer(keywords)

	// Create test corpus with varying relevance
	articles := []*models.Article{
		{
			ID:      "ml1",
			Title:   "Machine Learning with Python and TensorFlow",
			Summary: "Complete guide to ML using Python and TensorFlow framework",
			Content: "Machine learning has revolutionized AI. Python is the go-to language, and TensorFlow makes it accessible.",
			Tags:    []string{"machine-learning", "python", "tensorflow", "ai"},
		},
		{
			ID:      "ml2",
			Title:   "JavaScript for Web Development",
			Summary: "Building web applications with modern JavaScript",
			Content: "JavaScript frameworks have transformed web development. React and Vue are popular choices.",
			Tags:    []string{"javascript", "web", "react", "vue"},
		},
		{
			ID:      "ml3",
			Title:   "Python Programming Basics",
			Summary: "Introduction to Python programming language",
			Content: "Python is versatile and beginner-friendly. Great for data science and machine learning applications.",
			Tags:    []string{"python", "programming", "basics"},
		},
	}

	scorer.SetCorpus(articles)

	scores := make(map[string]float64)
	for _, article := range articles {
		scores[article.ID] = scorer.ScoreRelevance(article)
	}

	// ml1 should score highest (matches all keywords)
	// ml3 should score medium (matches python and mentions ML)
	// ml2 should score lowest (only tangentially related)

	if scores["ml1"] <= scores["ml2"] {
		t.Errorf("ML article should score higher than JavaScript article: ML=%f, JS=%f", scores["ml1"], scores["ml2"])
	}

	if scores["ml1"] <= scores["ml3"] {
		t.Errorf("Full ML article should score higher than Python basics: ML=%f, Python=%f", scores["ml1"], scores["ml3"])
	}

	t.Logf("Integration test scores - ML: %.3f, JS: %.3f, Python: %.3f", scores["ml1"], scores["ml2"], scores["ml3"])
}
