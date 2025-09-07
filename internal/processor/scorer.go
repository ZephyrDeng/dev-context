package processor

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"frontend-news-mcp/internal/models"
)

// TermFrequency represents the frequency of a term in a document
type TermFrequency struct {
	Term      string  `json:"term"`
	Count     int     `json:"count"`
	Frequency float64 `json:"frequency"`
}

// DocumentFrequency represents how many documents contain a specific term
type DocumentFrequency struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

// TFIDFScore represents the TF-IDF score for a term in a document
type TFIDFScore struct {
	Term   string  `json:"term"`
	TF     float64 `json:"tf"`     // Term Frequency
	IDF    float64 `json:"idf"`    // Inverse Document Frequency
	TFIDF  float64 `json:"tfidf"`  // TF-IDF Score
	Weight float64 `json:"weight"` // Additional weight based on position/importance
}

// KeywordMatcher handles keyword matching with weighted scoring
type KeywordMatcher struct {
	Keywords       []string          `json:"keywords"`
	WeightConfig   WeightConfig      `json:"weightConfig"`
	stopWords      map[string]bool   // Common words to ignore
	stemmer        *SimpleStemmer    // Simple stemming for better matching
}

// WeightConfig defines scoring weights for different text sections
type WeightConfig struct {
	TitleWeight   float64 `json:"titleWeight"`   // Weight for title matches (default: 3.0)
	SummaryWeight float64 `json:"summaryWeight"` // Weight for summary matches (default: 2.0)
	ContentWeight float64 `json:"contentWeight"` // Weight for content matches (default: 1.0)
	TagWeight     float64 `json:"tagWeight"`     // Weight for tag matches (default: 4.0)
	ExactMatch    float64 `json:"exactMatch"`    // Bonus for exact keyword match (default: 1.5)
	PartialMatch  float64 `json:"partialMatch"`  // Score for partial match (default: 0.8)
}

// SimpleStemmer provides basic word stemming functionality
type SimpleStemmer struct {
	// Common word endings to remove for better matching
	suffixes []string
}

// RelevanceScorer handles content relevance scoring using TF-IDF and keyword matching
type RelevanceScorer struct {
	keywordMatcher *KeywordMatcher
	corpus         []*models.Article // Used for IDF calculation
	tfCache        map[string][]TermFrequency
	idfCache       map[string]float64
}

// NewRelevanceScorer creates a new instance of RelevanceScorer
func NewRelevanceScorer(keywords []string) *RelevanceScorer {
	weightConfig := WeightConfig{
		TitleWeight:   3.0,
		SummaryWeight: 2.0,
		ContentWeight: 1.0,
		TagWeight:     4.0,
		ExactMatch:    1.5,
		PartialMatch:  0.8,
	}

	keywordMatcher := &KeywordMatcher{
		Keywords:     keywords,
		WeightConfig: weightConfig,
		stopWords:    createStopWords(),
		stemmer:      NewSimpleStemmer(),
	}

	return &RelevanceScorer{
		keywordMatcher: keywordMatcher,
		corpus:         make([]*models.Article, 0),
		tfCache:        make(map[string][]TermFrequency),
		idfCache:       make(map[string]float64),
	}
}

// NewSimpleStemmer creates a new stemmer instance
func NewSimpleStemmer() *SimpleStemmer {
	return &SimpleStemmer{
		suffixes: []string{"ing", "ed", "er", "est", "ly", "ion", "tion", "ness", "ment", "ful", "less"},
	}
}

// createStopWords returns a set of common English stop words
func createStopWords() map[string]bool {
	stopWords := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for", "from",
		"has", "he", "in", "is", "it", "its", "of", "on", "that", "the",
		"to", "was", "will", "with", "or", "but", "not", "this", "have",
	}
	
	stopWordsMap := make(map[string]bool)
	for _, word := range stopWords {
		stopWordsMap[word] = true
	}
	return stopWordsMap
}

// SetCorpus sets the corpus for IDF calculation
func (rs *RelevanceScorer) SetCorpus(articles []*models.Article) {
	rs.corpus = articles
	// Clear caches when corpus changes
	rs.tfCache = make(map[string][]TermFrequency)
	rs.idfCache = make(map[string]float64)
}

// AddToCorpus adds an article to the corpus
func (rs *RelevanceScorer) AddToCorpus(article *models.Article) {
	rs.corpus = append(rs.corpus, article)
	// Clear caches when corpus changes
	rs.tfCache = make(map[string][]TermFrequency)
	rs.idfCache = make(map[string]float64)
}

// ScoreRelevance calculates the relevance score for an article
func (rs *RelevanceScorer) ScoreRelevance(article *models.Article) float64 {
	if rs.keywordMatcher == nil || len(rs.keywordMatcher.Keywords) == 0 {
		return 0.0
	}

	// Combine TF-IDF score and keyword matching score
	tfidfScore := rs.calculateTFIDFScore(article)
	keywordScore := rs.keywordMatcher.ScoreKeywordMatch(article)

	// Weighted combination (60% TF-IDF, 40% keyword matching)
	relevanceScore := (tfidfScore * 0.6) + (keywordScore * 0.4)

	// Ensure score is within [0, 1] range
	if relevanceScore > 1.0 {
		relevanceScore = 1.0
	} else if relevanceScore < 0.0 {
		relevanceScore = 0.0
	}

	return relevanceScore
}

// calculateTFIDFScore computes the TF-IDF based relevance score
func (rs *RelevanceScorer) calculateTFIDFScore(article *models.Article) float64 {
	if len(rs.corpus) == 0 {
		// If no corpus, fallback to simple keyword matching
		return rs.keywordMatcher.ScoreKeywordMatch(article) * 0.5
	}

	// Get term frequencies for the article
	termFreqs := rs.calculateTermFrequencies(article)
	
	totalScore := 0.0
	totalWeight := 0.0

	// Calculate TF-IDF for each keyword
	for _, keyword := range rs.keywordMatcher.Keywords {
		normalizedKeyword := rs.normalizeText(keyword)
		stemmedKeyword := rs.keywordMatcher.stemmer.Stem(normalizedKeyword)

		// Find TF for this keyword (or its variants)
		tf := rs.findTermFrequency(termFreqs, []string{normalizedKeyword, stemmedKeyword})
		
		if tf > 0 {
			// Calculate IDF
			idf := rs.calculateIDF(stemmedKeyword)
			
			// Calculate TF-IDF score
			tfidf := tf * idf
			
			// Apply weight based on keyword importance (can be enhanced)
			weight := 1.0
			totalScore += tfidf * weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		return 0.0
	}

	// Normalize by total weight and scale to [0, 1]
	avgScore := totalScore / totalWeight
	
	// Apply sigmoid normalization to prevent extreme values
	normalizedScore := 2.0 / (1.0 + math.Exp(-avgScore)) - 1.0
	
	return math.Max(0.0, math.Min(1.0, normalizedScore))
}

// calculateTermFrequencies computes term frequencies for an article
func (rs *RelevanceScorer) calculateTermFrequencies(article *models.Article) []TermFrequency {
	if cached, exists := rs.tfCache[article.ID]; exists {
		return cached
	}

	// Combine all text content with appropriate weights
	fullText := ""
	
	// Title (higher weight by repeating)
	if article.Title != "" {
		fullText += strings.Repeat(article.Title+" ", 3)
	}
	
	// Summary (medium weight)
	if article.Summary != "" {
		fullText += strings.Repeat(article.Summary+" ", 2)
	}
	
	// Content (base weight)
	if article.Content != "" {
		fullText += article.Content + " "
	}
	
	// Tags (highest weight)
	for _, tag := range article.Tags {
		fullText += strings.Repeat(tag+" ", 4)
	}

	// Tokenize and count terms
	terms := rs.keywordMatcher.tokenizeText(fullText)
	termCounts := make(map[string]int)
	totalTerms := 0

	for _, term := range terms {
		normalizedTerm := rs.keywordMatcher.stemmer.Stem(rs.normalizeText(term))
		if !rs.keywordMatcher.stopWords[normalizedTerm] && len(normalizedTerm) > 2 {
			termCounts[normalizedTerm]++
			totalTerms++
		}
	}

	// Convert to TermFrequency structs
	var termFreqs []TermFrequency
	for term, count := range termCounts {
		freq := float64(count) / float64(totalTerms)
		termFreqs = append(termFreqs, TermFrequency{
			Term:      term,
			Count:     count,
			Frequency: freq,
		})
	}

	// Sort by frequency (descending)
	sort.Slice(termFreqs, func(i, j int) bool {
		return termFreqs[i].Frequency > termFreqs[j].Frequency
	})

	// Cache the result
	rs.tfCache[article.ID] = termFreqs
	
	return termFreqs
}

// calculateIDF computes the inverse document frequency for a term
func (rs *RelevanceScorer) calculateIDF(term string) float64 {
	if cached, exists := rs.idfCache[term]; exists {
		return cached
	}

	docCount := 0
	totalDocs := len(rs.corpus)

	if totalDocs == 0 {
		return 0.0
	}

	// Count documents containing the term
	for _, article := range rs.corpus {
		if rs.articleContainsTerm(article, term) {
			docCount++
		}
	}

	var idf float64
	if docCount > 0 {
		idf = math.Log(float64(totalDocs) / float64(docCount))
	} else {
		idf = math.Log(float64(totalDocs))
	}

	// Cache the result
	rs.idfCache[term] = idf
	
	return idf
}

// articleContainsTerm checks if an article contains a specific term
func (rs *RelevanceScorer) articleContainsTerm(article *models.Article, term string) bool {
	// Check in all text fields
	texts := []string{article.Title, article.Summary, article.Content}
	texts = append(texts, article.Tags...)
	
	for _, text := range texts {
		if text != "" {
			normalizedText := strings.ToLower(text)
			if strings.Contains(normalizedText, term) {
				return true
			}
			
			// Also check stemmed versions
			words := rs.keywordMatcher.tokenizeText(normalizedText)
			for _, word := range words {
				if rs.keywordMatcher.stemmer.Stem(word) == term {
					return true
				}
			}
		}
	}
	
	return false
}

// findTermFrequency finds the frequency of terms (checking variants)
func (rs *RelevanceScorer) findTermFrequency(termFreqs []TermFrequency, terms []string) float64 {
	for _, termVariant := range terms {
		for _, tf := range termFreqs {
			if tf.Term == termVariant {
				return tf.Frequency
			}
		}
	}
	return 0.0
}

// ScoreKeywordMatch calculates keyword matching score with weights
func (km *KeywordMatcher) ScoreKeywordMatch(article *models.Article) float64 {
	if len(km.Keywords) == 0 {
		return 0.0
	}

	totalScore := 0.0
	maxPossibleScore := 0.0

	for _, keyword := range km.Keywords {
		keywordScore := km.scoreKeywordInArticle(article, keyword)
		totalScore += keywordScore
		
		// Calculate max possible score for this keyword
		maxPossibleScore += km.WeightConfig.TitleWeight * km.WeightConfig.ExactMatch
	}

	if maxPossibleScore == 0 {
		return 0.0
	}

	// Normalize to [0, 1]
	normalizedScore := totalScore / maxPossibleScore
	
	return math.Min(1.0, normalizedScore)
}

// scoreKeywordInArticle scores a single keyword against an article
func (km *KeywordMatcher) scoreKeywordInArticle(article *models.Article, keyword string) float64 {
	score := 0.0
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))

	// Score title matches
	titleScore := km.scoreTextMatch(article.Title, normalizedKeyword)
	score += titleScore * km.WeightConfig.TitleWeight

	// Score summary matches
	summaryScore := km.scoreTextMatch(article.Summary, normalizedKeyword)
	score += summaryScore * km.WeightConfig.SummaryWeight

	// Score content matches (limit content length for performance)
	contentPreview := article.Content
	if len(contentPreview) > 1000 {
		contentPreview = contentPreview[:1000]
	}
	contentScore := km.scoreTextMatch(contentPreview, normalizedKeyword)
	score += contentScore * km.WeightConfig.ContentWeight

	// Score tag matches (exact matches in tags are very valuable)
	tagScore := km.scoreTagMatch(article.Tags, normalizedKeyword)
	score += tagScore * km.WeightConfig.TagWeight

	return score
}

// scoreTextMatch scores keyword matching in a text field
func (km *KeywordMatcher) scoreTextMatch(text, keyword string) float64 {
	if text == "" || keyword == "" {
		return 0.0
	}

	normalizedText := strings.ToLower(text)
	
	// Exact phrase match (highest score)
	if strings.Contains(normalizedText, keyword) {
		return km.WeightConfig.ExactMatch
	}

	// Partial word matching
	words := km.tokenizeText(normalizedText)
	keywordWords := km.tokenizeText(keyword)
	
	matchCount := 0.0
	totalKeywordWords := float64(len(keywordWords))

	for _, keywordWord := range keywordWords {
		stemmedKeyword := km.stemmer.Stem(keywordWord)
		
		for _, textWord := range words {
			stemmedText := km.stemmer.Stem(textWord)
			
			if stemmedText == stemmedKeyword {
				matchCount += 1.0
				break
			} else if strings.Contains(stemmedText, stemmedKeyword) || strings.Contains(stemmedKeyword, stemmedText) {
				matchCount += 0.7
				break
			}
		}
	}

	if totalKeywordWords > 0 {
		matchRatio := matchCount / totalKeywordWords
		return matchRatio * km.WeightConfig.PartialMatch
	}

	return 0.0
}

// scoreTagMatch scores keyword matching in tags
func (km *KeywordMatcher) scoreTagMatch(tags []string, keyword string) float64 {
	if len(tags) == 0 {
		return 0.0
	}

	bestScore := 0.0
	
	for _, tag := range tags {
		normalizedTag := strings.ToLower(strings.TrimSpace(tag))
		
		// Exact tag match
		if normalizedTag == keyword {
			bestScore = math.Max(bestScore, km.WeightConfig.ExactMatch)
		} else if strings.Contains(normalizedTag, keyword) || strings.Contains(keyword, normalizedTag) {
			bestScore = math.Max(bestScore, km.WeightConfig.PartialMatch)
		} else {
			// Stemmed comparison
			stemmedTag := km.stemmer.Stem(normalizedTag)
			stemmedKeyword := km.stemmer.Stem(keyword)
			
			if stemmedTag == stemmedKeyword {
				bestScore = math.Max(bestScore, km.WeightConfig.ExactMatch * 0.9)
			}
		}
	}

	return bestScore
}

// tokenizeText splits text into words
func (km *KeywordMatcher) tokenizeText(text string) []string {
	// Use regex to split on non-letter characters
	re := regexp.MustCompile(`[^\p{L}]+`)
	words := re.Split(text, -1)
	
	var result []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) > 0 {
			result = append(result, strings.ToLower(word))
		}
	}
	
	return result
}

// normalizeText normalizes text for consistent processing
func (rs *RelevanceScorer) normalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Remove extra whitespace
	text = strings.TrimSpace(text)
	
	// Remove non-letter characters for better matching
	var result strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	
	return strings.TrimSpace(result.String())
}

// Stem applies basic stemming to a word
func (ss *SimpleStemmer) Stem(word string) string {
	if len(word) <= 3 {
		return word
	}

	word = strings.ToLower(strings.TrimSpace(word))
	
	// Apply suffix removal
	for _, suffix := range ss.suffixes {
		if strings.HasSuffix(word, suffix) && len(word) > len(suffix)+2 {
			return word[:len(word)-len(suffix)]
		}
	}
	
	return word
}

// UpdateKeywords updates the keywords used for scoring
func (rs *RelevanceScorer) UpdateKeywords(keywords []string) {
	rs.keywordMatcher.Keywords = keywords
	// Clear caches when keywords change
	rs.tfCache = make(map[string][]TermFrequency)
	rs.idfCache = make(map[string]float64)
}

// GetKeywords returns the current keywords
func (rs *RelevanceScorer) GetKeywords() []string {
	return rs.keywordMatcher.Keywords
}

// SetWeightConfig updates the weight configuration
func (rs *RelevanceScorer) SetWeightConfig(config WeightConfig) {
	rs.keywordMatcher.WeightConfig = config
}

// GetWeightConfig returns the current weight configuration
func (rs *RelevanceScorer) GetWeightConfig() WeightConfig {
	return rs.keywordMatcher.WeightConfig
}

// ClearCache clears all internal caches
func (rs *RelevanceScorer) ClearCache() {
	rs.tfCache = make(map[string][]TermFrequency)
	rs.idfCache = make(map[string]float64)
}

// GetTopTerms returns the top N terms for an article based on TF-IDF
func (rs *RelevanceScorer) GetTopTerms(article *models.Article, n int) []TermFrequency {
	termFreqs := rs.calculateTermFrequencies(article)
	
	if len(termFreqs) <= n {
		return termFreqs
	}
	
	return termFreqs[:n]
}