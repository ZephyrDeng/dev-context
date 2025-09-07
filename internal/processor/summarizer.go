package processor

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"frontend-news-mcp/internal/models"
)

// SentenceScore represents a sentence with its calculated importance score
type SentenceScore struct {
	Text     string
	Score    float64
	Position int
	Length   int
}

// Summarizer handles intelligent content summarization and processing
type Summarizer struct {
	// Configuration for summary generation
	MinSummaryLength int     // Minimum summary length in characters
	MaxSummaryLength int     // Maximum summary length in characters
	SentenceCount    int     // Target number of sentences in summary
	PositionWeight   float64 // Weight for sentence position (earlier sentences score higher)
	LengthWeight     float64 // Weight for sentence length
	KeywordWeight    float64 // Weight for keyword density
}

// NewSummarizer creates a new Summarizer with default configuration
func NewSummarizer() *Summarizer {
	return &Summarizer{
		MinSummaryLength: 50,
		MaxSummaryLength: 150,
		SentenceCount:    2,
		PositionWeight:   0.3,
		LengthWeight:     0.2,
		KeywordWeight:    0.5,
	}
}

// ProcessArticle performs complete content processing on an article
// This includes cleaning, summarization, and quality assessment
func (s *Summarizer) ProcessArticle(article *models.Article) error {
	if article == nil {
		return fmt.Errorf("article cannot be nil")
	}

	// Step 1: Clean the content
	if err := s.CleanContent(article); err != nil {
		return fmt.Errorf("content cleaning failed: %w", err)
	}

	// Step 2: Generate summary if content is available and summary is empty
	if article.Content != "" && article.Summary == "" {
		summary, err := s.GenerateSummary(article.Content)
		if err != nil {
			return fmt.Errorf("summary generation failed: %w", err)
		}
		if err := article.SetSummary(summary); err != nil {
			return fmt.Errorf("setting summary failed: %w", err)
		}
	}

	// Step 3: Assess content quality
	quality := s.AssessQuality(article)
	article.Quality = quality

	return nil
}

// GenerateSummary creates an intelligent summary of the given text
// Uses sentence extraction based on position, length, and keyword density
func (s *Summarizer) GenerateSummary(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text cannot be empty")
	}

	// Clean and normalize the text
	cleanText := s.cleanText(text)
	if len(cleanText) < s.MinSummaryLength {
		return cleanText, nil
	}

	// Extract sentences
	sentences := s.extractSentences(cleanText)
	if len(sentences) == 0 {
		return "", fmt.Errorf("no sentences found in text")
	}

	// If we have very few sentences, return them all
	if len(sentences) <= s.SentenceCount {
		return strings.Join(sentences, " "), nil
	}

	// Calculate scores for each sentence
	sentenceScores := s.calculateSentenceScores(sentences, cleanText)

	// Select best sentences
	selectedSentences := s.selectBestSentences(sentenceScores)

	// Build summary
	summary := strings.Join(selectedSentences, " ")

	// Ensure summary length is within bounds
	summary = s.adjustSummaryLength(summary)

	return summary, nil
}

// CleanContent performs content cleaning and preprocessing
func (s *Summarizer) CleanContent(article *models.Article) error {
	if article == nil {
		return fmt.Errorf("article cannot be nil")
	}

	// Clean title
	article.Title = s.cleanText(article.Title)
	if article.Title == "" {
		return fmt.Errorf("title cannot be empty after cleaning")
	}

	// Clean content if present
	if article.Content != "" {
		article.Content = s.cleanText(article.Content)
	}

	// Clean summary if present
	if article.Summary != "" {
		article.Summary = s.cleanText(article.Summary)
	}

	// Clean tags
	cleanedTags := make([]string, 0, len(article.Tags))
	for _, tag := range article.Tags {
		cleanTag := s.cleanText(tag)
		if cleanTag != "" {
			cleanedTags = append(cleanedTags, cleanTag)
		}
	}
	article.Tags = cleanedTags

	return nil
}

// AssessQuality evaluates content quality based on multiple factors
func (s *Summarizer) AssessQuality(article *models.Article) float64 {
	if article == nil {
		return 0.0
	}

	var score float64

	// Completeness assessment (40%)
	completeness := s.assessCompleteness(article)
	score += completeness * 0.4

	// Readability assessment (30%)
	readability := s.assessReadability(article)
	score += readability * 0.3

	// Relevance assessment (30%)
	relevance := s.assessRelevance(article)
	score += relevance * 0.3

	// Ensure score is within bounds
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// cleanText removes HTML tags, normalizes whitespace, and handles special characters
func (s *Summarizer) cleanText(text string) string {
	if text == "" {
		return ""
	}

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Remove HTML tags (more comprehensive)
	htmlRegex := regexp.MustCompile(`<[^>]*?>`)
	text = htmlRegex.ReplaceAllString(text, "")

	// Remove script and style content
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	text = scriptRegex.ReplaceAllString(text, "")
	
	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	text = styleRegex.ReplaceAllString(text, "")

	// Remove excessive whitespace and normalize
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	// Trim and return
	return strings.TrimSpace(text)
}

// extractSentences splits text into sentences using punctuation and patterns
func (s *Summarizer) extractSentences(text string) []string {
	// Split by sentence-ending punctuation
	sentenceRegex := regexp.MustCompile(`[.!?]+\s+`)
	rawSentences := sentenceRegex.Split(text, -1)

	sentences := make([]string, 0)
	for _, sentence := range rawSentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) > 10 { // Minimum sentence length
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// calculateSentenceScores assigns importance scores to sentences
func (s *Summarizer) calculateSentenceScores(sentences []string, fullText string) []SentenceScore {
	scores := make([]SentenceScore, 0, len(sentences))
	keywords := s.extractKeywords(fullText)

	for i, sentence := range sentences {
		score := s.scoreSentence(sentence, i, len(sentences), keywords)
		scores = append(scores, SentenceScore{
			Text:     sentence,
			Score:    score,
			Position: i,
			Length:   utf8.RuneCountInString(sentence),
		})
	}

	return scores
}

// scoreSentence calculates the importance score for a single sentence
func (s *Summarizer) scoreSentence(sentence string, position, totalSentences int, keywords map[string]int) float64 {
	var score float64

	// Position score (earlier sentences are generally more important)
	positionScore := 1.0 - (float64(position) / float64(totalSentences))
	score += positionScore * s.PositionWeight

	// Length score (prefer sentences of moderate length)
	lengthScore := s.calculateLengthScore(sentence)
	score += lengthScore * s.LengthWeight

	// Keyword density score
	keywordScore := s.calculateKeywordScore(sentence, keywords)
	score += keywordScore * s.KeywordWeight

	return score
}

// calculateLengthScore gives higher scores to sentences of optimal length
func (s *Summarizer) calculateLengthScore(sentence string) float64 {
	length := utf8.RuneCountInString(sentence)

	// Optimal sentence length is between 50-200 characters
	if length >= 50 && length <= 200 {
		return 1.0
	}

	// Penalize very short or very long sentences
	if length < 20 {
		return 0.2
	}
	if length > 300 {
		return 0.3
	}

	// Moderate penalty for suboptimal lengths
	return 0.7
}

// calculateKeywordScore measures keyword density in a sentence
func (s *Summarizer) calculateKeywordScore(sentence string, keywords map[string]int) float64 {
	if len(keywords) == 0 {
		return 0.5 // Neutral score if no keywords
	}

	sentenceLower := strings.ToLower(sentence)
	words := strings.Fields(sentenceLower)
	
	keywordCount := 0
	for _, word := range words {
		if _, exists := keywords[word]; exists {
			keywordCount++
		}
	}

	if len(words) == 0 {
		return 0.0
	}

	// Return keyword density ratio
	density := float64(keywordCount) / float64(len(words))
	
	// Cap at 1.0 for very keyword-dense sentences
	if density > 1.0 {
		density = 1.0
	}

	return density
}

// extractKeywords identifies important words in the text
func (s *Summarizer) extractKeywords(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text))
	wordCount := make(map[string]int)

	// Count word frequencies
	for _, word := range words {
		// Clean word of punctuation
		word = s.cleanWord(word)
		if s.isValidKeyword(word) {
			wordCount[word]++
		}
	}

	// Keep only words that appear multiple times or are significant
	keywords := make(map[string]int)
	for word, count := range wordCount {
		if count > 1 || len(word) > 6 { // Either frequent or long words
			keywords[word] = count
		}
	}

	return keywords
}

// cleanWord removes punctuation from a word
func (s *Summarizer) cleanWord(word string) string {
	// Remove punctuation from beginning and end
	word = strings.Trim(word, ".,!?;:\"'()[]{}~")
	return strings.TrimSpace(word)
}

// isValidKeyword checks if a word should be considered as a keyword
func (s *Summarizer) isValidKeyword(word string) bool {
	if len(word) < 3 {
		return false
	}

	// Skip common stop words
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true,
		"on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
		"a": true, "an": true, "it": true, "its": true, "they": true,
		"them": true, "their": true, "we": true, "our": true, "you": true,
		"your": true, "he": true, "him": true, "his": true, "she": true,
		"her": true, "hers": true,
	}

	return !stopWords[word]
}

// selectBestSentences chooses the highest-scoring sentences for the summary
func (s *Summarizer) selectBestSentences(scores []SentenceScore) []string {
	// Sort by score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Select top sentences up to the target count
	count := s.SentenceCount
	if count > len(scores) {
		count = len(scores)
	}

	selectedScores := scores[:count]

	// Sort selected sentences by original position to maintain logical flow
	sort.Slice(selectedScores, func(i, j int) bool {
		return selectedScores[i].Position < selectedScores[j].Position
	})

	// Extract text
	sentences := make([]string, count)
	for i, score := range selectedScores {
		sentences[i] = score.Text
	}

	return sentences
}

// adjustSummaryLength ensures the summary is within the specified length bounds
func (s *Summarizer) adjustSummaryLength(summary string) string {
	if len(summary) <= s.MaxSummaryLength {
		return summary
	}

	// Truncate at word boundary
	words := strings.Fields(summary)
	result := ""
	
	for _, word := range words {
		testResult := result
		if testResult != "" {
			testResult += " "
		}
		testResult += word

		if len(testResult) > s.MaxSummaryLength-3 { // Leave room for "..."
			if result != "" {
				result += "..."
			}
			break
		}
		
		result = testResult
	}

	return result
}

// assessCompleteness evaluates how complete the article data is
func (s *Summarizer) assessCompleteness(article *models.Article) float64 {
	var score float64

	// Required fields check
	if article.Title != "" {
		score += 0.3
	}
	if article.URL != "" {
		score += 0.1
	}
	if article.Source != "" {
		score += 0.1
	}

	// Optional but important fields
	if article.Summary != "" {
		score += 0.2
	}
	if article.Content != "" {
		score += 0.2
	}
	if len(article.Tags) > 0 {
		score += 0.1
	}

	return math.Min(score, 1.0)
}

// assessReadability evaluates how readable the content is
func (s *Summarizer) assessReadability(article *models.Article) float64 {
	var score float64
	textToAnalyze := article.Content
	if textToAnalyze == "" {
		textToAnalyze = article.Summary
	}
	if textToAnalyze == "" {
		textToAnalyze = article.Title
	}

	if textToAnalyze == "" {
		return 0.0
	}

	// Basic readability metrics
	words := strings.Fields(textToAnalyze)
	sentences := s.extractSentences(textToAnalyze)

	if len(words) == 0 || len(sentences) == 0 {
		return 0.0
	}

	// Average words per sentence (optimal range: 15-20)
	avgWordsPerSentence := float64(len(words)) / float64(len(sentences))
	if avgWordsPerSentence >= 10 && avgWordsPerSentence <= 25 {
		score += 0.4
	} else {
		score += 0.2
	}

	// Character diversity (avoid repetitive text)
	uniqueChars := s.countUniqueCharacters(textToAnalyze)
	if uniqueChars > 20 {
		score += 0.3
	} else {
		score += 0.1
	}

	// Sentence length variety
	lengthVariety := s.calculateLengthVariety(sentences)
	score += lengthVariety * 0.3

	return math.Min(score, 1.0)
}

// assessRelevance evaluates content relevance (basic implementation)
func (s *Summarizer) assessRelevance(article *models.Article) float64 {
	// This is a basic implementation - in a real system, this might compare
	// against user interests, trending topics, etc.
	
	var score float64 = 0.5 // Base relevance score

	// Boost score for articles with tags (indicates categorization)
	if len(article.Tags) > 0 {
		score += 0.2
	}

	// Boost score for recent articles
	if article.IsRecent(7 * 24 * 3600 * 1000 * 1000 * 1000) { // 7 days in nanoseconds
		score += 0.2
	}

	// Boost score for articles with substantial content
	if len(article.Content) > 500 {
		score += 0.1
	}

	return math.Min(score, 1.0)
}

// countUniqueCharacters counts the number of unique characters in text
func (s *Summarizer) countUniqueCharacters(text string) int {
	charSet := make(map[rune]bool)
	for _, char := range text {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			charSet[char] = true
		}
	}
	return len(charSet)
}

// calculateLengthVariety measures sentence length diversity
func (s *Summarizer) calculateLengthVariety(sentences []string) float64 {
	if len(sentences) <= 1 {
		return 0.5
	}

	lengths := make([]int, len(sentences))
	totalLength := 0
	
	for i, sentence := range sentences {
		lengths[i] = len(strings.Fields(sentence))
		totalLength += lengths[i]
	}

	avgLength := float64(totalLength) / float64(len(sentences))
	
	// Calculate variance
	variance := 0.0
	for _, length := range lengths {
		diff := float64(length) - avgLength
		variance += diff * diff
	}
	variance /= float64(len(sentences))

	// Normalize variance to 0-1 scale (higher variance = more variety = better score)
	// Use square root to moderate the effect
	variety := math.Sqrt(variance) / avgLength
	
	return math.Min(variety, 1.0)
}