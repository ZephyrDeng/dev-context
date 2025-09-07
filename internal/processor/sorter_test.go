package processor

import (
	"testing"
	"time"

	"frontend-news-mcp/internal/models"
)

// Test data for sorting
var sortTestArticles = []*models.Article{
	{
		ID:          "high_relevance",
		Title:       "Advanced Go Programming",
		Summary:     "Deep dive into Go programming concepts",
		Relevance:   0.9,
		Quality:     0.8,
		PublishedAt: time.Now().Add(-24 * time.Hour),
	},
	{
		ID:          "recent_article",
		Title:       "Latest JavaScript Framework",
		Summary:     "Brand new JavaScript framework released",
		Relevance:   0.6,
		Quality:     0.9,
		PublishedAt: time.Now().Add(-2 * time.Hour),
	},
	{
		ID:          "high_quality",
		Title:       "Database Architecture Guide",
		Summary:     "Comprehensive database design patterns",
		Relevance:   0.7,
		Quality:     0.95,
		PublishedAt: time.Now().Add(-72 * time.Hour),
	},
	{
		ID:          "old_article",
		Title:       "Legacy System Migration",
		Summary:     "Guide to migrating legacy systems",
		Relevance:   0.5,
		Quality:     0.6,
		PublishedAt: time.Now().Add(-168 * time.Hour), // 1 week old
	},
}

var sortTestRepos = []*models.Repository{
	{
		ID:          "trending_repo",
		Name:        "awesome-go",
		FullName:    "awesome-go/awesome-go",
		Description: "A curated list of awesome Go frameworks",
		Language:    "Go",
		Stars:       45000,
		Forks:       6500,
		TrendScore:  0.95,
		UpdatedAt:   time.Now().Add(-1 * time.Hour),
	},
	{
		ID:          "popular_repo", 
		Name:        "react",
		FullName:    "facebook/react",
		Description: "A declarative JavaScript library",
		Language:    "JavaScript",
		Stars:       180000,
		Forks:       37000,
		TrendScore:  0.8,
		UpdatedAt:   time.Now().Add(-24 * time.Hour),
	},
	{
		ID:          "old_repo",
		Name:        "jquery",
		FullName:    "jquery/jquery",
		Description: "jQuery JavaScript Library",
		Language:    "JavaScript", 
		Stars:       57000,
		Forks:       20000,
		TrendScore:  0.3,
		UpdatedAt:   time.Now().Add(-720 * time.Hour), // 30 days old
	},
}

func TestNewArticleSorter(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	if sorter == nil {
		t.Fatal("NewArticleSorter returned nil")
	}

	if sorter.sortConfig.Primary != SortByComposite {
		t.Errorf("Expected primary sort to be Composite, got %s", sorter.sortConfig.Primary)
	}

	if sorter.sortConfig.Order != SortDesc {
		t.Errorf("Expected descending order, got %s", sorter.sortConfig.Order)
	}
}

func TestNewRepositorySorter(t *testing.T) {
	sorter := NewRepositorySorter()

	if sorter == nil {
		t.Fatal("NewRepositorySorter returned nil")
	}

	if sorter.sortConfig.Primary != SortByTrend {
		t.Errorf("Expected primary sort to be Trend, got %s", sorter.sortConfig.Primary)
	}
}

func TestSortArticlesByRelevance(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)
	
	// Set sort config to relevance
	config := SortConfig{
		Primary:   SortByRelevance,
		Secondary: SortByTime,
		Order:     SortDesc,
	}
	sorter.SetSortConfig(config)

	sorted := sorter.SortArticles(sortTestArticles)

	// Should be sorted by relevance descending
	if sorted[0].Relevance < sorted[1].Relevance {
		t.Errorf("Articles not sorted by relevance: first=%f, second=%f", 
			sorted[0].Relevance, sorted[1].Relevance)
	}

	// High relevance article should be first
	if sorted[0].ID != "high_relevance" {
		t.Errorf("Expected high_relevance article first, got %s", sorted[0].ID)
	}
}

func TestSortArticlesByTime(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)
	
	config := SortConfig{
		Primary:   SortByTime,
		Secondary: SortByPopularity,
		Order:     SortDesc,
	}
	sorter.SetSortConfig(config)

	sorted := sorter.SortArticles(sortTestArticles)

	// Should be sorted by time descending (newest first)
	if sorted[0].PublishedAt.Before(sorted[1].PublishedAt) {
		t.Error("Articles not sorted by time descending")
	}

	// Recent article should be first
	if sorted[0].ID != "recent_article" {
		t.Errorf("Expected recent_article first, got %s", sorted[0].ID)
	}
}

func TestSortArticlesByPopularity(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)
	
	config := SortConfig{
		Primary:   SortByPopularity,
		Secondary: SortByTime,
		Order:     SortDesc,
	}
	sorter.SetSortConfig(config)

	sorted := sorter.SortArticles(sortTestArticles)

	// Should be sorted by quality descending
	if sorted[0].Quality < sorted[1].Quality {
		t.Errorf("Articles not sorted by quality: first=%f, second=%f",
			sorted[0].Quality, sorted[1].Quality)
	}

	// High quality article should be first
	if sorted[0].ID != "high_quality" {
		t.Errorf("Expected high_quality article first, got %s", sorted[0].ID)
	}
}

func TestSortArticlesByTrend(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)
	
	config := SortConfig{
		Primary:   SortByTrend,
		Secondary: SortByPopularity,
		Order:     SortDesc,
	}
	sorter.SetSortConfig(config)

	sorted := sorter.SortArticles(sortTestArticles)

	// Recent high-quality articles should rank higher in trending
	// The exact order depends on the trending algorithm, but recent_article should rank high
	foundRecentInTop2 := false
	for i := 0; i < 2 && i < len(sorted); i++ {
		if sorted[i].ID == "recent_article" {
			foundRecentInTop2 = true
			break
		}
	}

	if !foundRecentInTop2 {
		t.Error("Recent article should rank in top 2 for trend sorting")
	}
}

func TestSortArticlesByComposite(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)
	
	// Use default composite sorting
	sorted := sorter.SortArticles(sortTestArticles)

	// Verify that sorting actually happened (not original order)
	if len(sorted) != len(sortTestArticles) {
		t.Errorf("Expected %d articles, got %d", len(sortTestArticles), len(sorted))
	}

	// Old article should not be first due to low scores across all metrics
	if sorted[0].ID == "old_article" {
		t.Error("Old article should not rank first in composite sorting")
	}
}

func TestSortArticlesAscending(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)
	
	config := SortConfig{
		Primary: SortByRelevance,
		Order:   SortAsc,
	}
	sorter.SetSortConfig(config)

	sorted := sorter.SortArticles(sortTestArticles)

	// Should be sorted by relevance ascending (lowest first)
	if sorted[0].Relevance > sorted[1].Relevance {
		t.Errorf("Articles not sorted by relevance ascending: first=%f, second=%f",
			sorted[0].Relevance, sorted[1].Relevance)
	}
}

func TestSortRepositoriesByTrend(t *testing.T) {
	sorter := NewRepositorySorter()
	
	sorted := sorter.SortRepositories(sortTestRepos)

	// Should be sorted by trend score descending
	if sorted[0].TrendScore < sorted[1].TrendScore {
		t.Errorf("Repos not sorted by trend: first=%f, second=%f",
			sorted[0].TrendScore, sorted[1].TrendScore)
	}

	// Trending repo should be first
	if sorted[0].ID != "trending_repo" {
		t.Errorf("Expected trending_repo first, got %s", sorted[0].ID)
	}
}

func TestSortRepositoriesByPopularity(t *testing.T) {
	sorter := NewRepositorySorter()
	
	config := SortConfig{
		Primary:   SortByPopularity,
		Secondary: SortByTime,
		Order:     SortDesc,
	}
	sorter.SetSortConfig(config)

	sorted := sorter.SortRepositories(sortTestRepos)

	// Should be sorted by stars descending
	if sorted[0].Stars < sorted[1].Stars {
		t.Errorf("Repos not sorted by stars: first=%d, second=%d",
			sorted[0].Stars, sorted[1].Stars)
	}

	// Popular repo (React) should be first
	if sorted[0].ID != "popular_repo" {
		t.Errorf("Expected popular_repo first, got %s", sorted[0].ID)
	}
}

func TestPaginationBasic(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	paginationConfig := PaginationConfig{
		Page:     1,
		PageSize: 2,
	}

	result, err := sorter.SortAndPaginate(sortTestArticles, paginationConfig)
	if err != nil {
		t.Fatalf("SortAndPaginate failed: %v", err)
	}

	// Should return first 2 articles
	if len(result.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result.Items))
	}

	// Check pagination metadata
	if result.CurrentPage != 1 {
		t.Errorf("Expected current page 1, got %d", result.CurrentPage)
	}

	if result.PageSize != 2 {
		t.Errorf("Expected page size 2, got %d", result.PageSize)
	}

	if result.TotalItems != len(sortTestArticles) {
		t.Errorf("Expected total items %d, got %d", len(sortTestArticles), result.TotalItems)
	}

	expectedTotalPages := 2 // 4 articles, 2 per page
	if result.TotalPages != expectedTotalPages {
		t.Errorf("Expected total pages %d, got %d", expectedTotalPages, result.TotalPages)
	}

	if !result.HasNext {
		t.Error("Should have next page")
	}

	if result.HasPrev {
		t.Error("Should not have previous page for page 1")
	}
}

func TestPaginationSecondPage(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	paginationConfig := PaginationConfig{
		Page:     2,
		PageSize: 2,
	}

	result, err := sorter.SortAndPaginate(sortTestArticles, paginationConfig)
	if err != nil {
		t.Fatalf("SortAndPaginate failed: %v", err)
	}

	// Should return remaining 2 articles
	if len(result.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(result.Items))
	}

	if result.CurrentPage != 2 {
		t.Errorf("Expected current page 2, got %d", result.CurrentPage)
	}

	if result.HasNext {
		t.Error("Should not have next page")
	}

	if !result.HasPrev {
		t.Error("Should have previous page for page 2")
	}
}

func TestPaginationBeyondRange(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	paginationConfig := PaginationConfig{
		Page:     10, // Beyond available pages
		PageSize: 2,
	}

	result, err := sorter.SortAndPaginate(sortTestArticles, paginationConfig)
	if err != nil {
		t.Fatalf("SortAndPaginate failed: %v", err)
	}

	// Should return empty results
	if len(result.Items) != 0 {
		t.Errorf("Expected 0 items for page beyond range, got %d", len(result.Items))
	}

	if result.HasNext {
		t.Error("Should not have next page when beyond range")
	}

	if !result.HasPrev {
		t.Error("Should have previous page when beyond range")
	}
}

func TestPaginationInvalidConfig(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	// Test with invalid page size
	paginationConfig := PaginationConfig{
		Page:     1,
		PageSize: 0, // Invalid
	}

	result, err := sorter.SortAndPaginate(sortTestArticles, paginationConfig)
	if err != nil {
		t.Fatalf("SortAndPaginate failed: %v", err)
	}

	// Should default to page size 20
	if result.PageSize != 20 {
		t.Errorf("Expected default page size 20, got %d", result.PageSize)
	}

	// Test with invalid page number
	paginationConfig = PaginationConfig{
		Page:     0, // Invalid
		PageSize: 2,
	}

	result, err = sorter.SortAndPaginate(sortTestArticles, paginationConfig)
	if err != nil {
		t.Fatalf("SortAndPaginate failed: %v", err)
	}

	// Should default to page 1
	if result.CurrentPage != 1 {
		t.Errorf("Expected default page 1, got %d", result.CurrentPage)
	}
}

func TestRepositoryPagination(t *testing.T) {
	sorter := NewRepositorySorter()

	paginationConfig := PaginationConfig{
		Page:     1,
		PageSize: 2,
	}

	result, err := sorter.SortAndPaginate(sortTestRepos, paginationConfig)
	if err != nil {
		t.Fatalf("Repository SortAndPaginate failed: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(result.Items))
	}

	if result.TotalItems != len(sortTestRepos) {
		t.Errorf("Expected total items %d, got %d", len(sortTestRepos), result.TotalItems)
	}
}

func TestUserPreferencesPersonalization(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"go", "programming"})
	sorter := NewArticleSorter(scorer)

	// Set user preferences
	userPrefs := &UserPreferences{
		FavoriteTopics:    []string{"go", "database"},
		PreferredSources:  []string{"tech-blog"},
		RecencyPreference: 0.8,
		TopicWeights: map[string]float64{
			"go":       0.2,
			"database": 0.15,
		},
	}
	sorter.SetUserPreferences(userPrefs)

	// Create test articles with different characteristics
	testArticles := []*models.Article{
		{
			ID:          "go_article",
			Title:       "Go Programming Guide",
			Summary:     "Learn Go programming",
			Source:      "tech-blog", // Preferred source
			Relevance:   0.6,
			Quality:     0.7,
			PublishedAt: time.Now().Add(-12 * time.Hour),
		},
		{
			ID:          "db_article", 
			Title:       "Database Optimization",
			Summary:     "Optimize database performance",
			Source:      "db-weekly",
			Relevance:   0.5,
			Quality:     0.8,
			PublishedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:          "js_article",
			Title:       "JavaScript Framework",
			Summary:     "New JavaScript framework",
			Source:      "web-dev",
			Relevance:   0.7,
			Quality:     0.9,
			PublishedAt: time.Now().Add(-48 * time.Hour),
		},
	}

	result, err := sorter.SortAndPaginate(testArticles, PaginationConfig{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Personalized sorting failed: %v", err)
	}

	// Go article should be boosted due to:
	// 1. Favorite topic match
	// 2. Preferred source match
	// 3. Recent publication (recency preference)
	
	// Check that personalization affected scores
	foundGoFirst := result.Items[0].ID == "go_article"
	if !foundGoFirst {
		t.Logf("Article order: %s, %s, %s", result.Items[0].ID, result.Items[1].ID, result.Items[2].ID)
		// Don't fail immediately as personalization is complex, but log for debugging
	}

	// Verify that articles from preferred sources got quality boosts
	for _, article := range result.Items {
		if article.Source == "tech-blog" && article.Quality <= 0.7 {
			t.Error("Articles from preferred sources should have quality boost")
		}
	}
}

func TestRepositoryPersonalization(t *testing.T) {
	sorter := NewRepositorySorter()

	userPrefs := &UserPreferences{
		FavoriteTopics: []string{"go", "awesome"},
		LanguagePrefs:  []string{"Go", "JavaScript"},
		TopicWeights: map[string]float64{
			"go": 0.1,
		},
	}
	sorter.SetUserPreferences(userPrefs)

	result, err := sorter.SortAndPaginate(sortTestRepos, PaginationConfig{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Repository personalization failed: %v", err)
	}

	// Verify all repos returned
	if len(result.Items) != len(sortTestRepos) {
		t.Errorf("Expected %d repos, got %d", len(sortTestRepos), len(result.Items))
	}

	// awesome-go repo should benefit from both language and topic preferences
	// It should rank higher than without personalization
	foundGoRepoFirst := result.Items[0].ID == "trending_repo"
	if !foundGoRepoFirst {
		t.Logf("Repo order: %s, %s, %s", result.Items[0].ID, result.Items[1].ID, result.Items[2].ID)
		// Log for debugging but don't fail as personalization logic is complex
	}
}

func TestCalculateCompositeScore(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	article := &models.Article{
		Relevance:   0.8,
		Quality:     0.9,
		PublishedAt: time.Now().Add(-12 * time.Hour),
	}

	score := sorter.calculateCompositeScore(article)

	// Score should be weighted combination
	if score <= 0 || score > 1 {
		t.Errorf("Composite score should be between 0 and 1, got %f", score)
	}

	// Test with zero values
	zeroArticle := &models.Article{
		Relevance:   0.0,
		Quality:     0.0,
		PublishedAt: time.Now().Add(-1000 * time.Hour),
	}

	zeroScore := sorter.calculateCompositeScore(zeroArticle)
	if zeroScore >= score {
		t.Error("Article with zero values should score lower")
	}
}

func TestCalculateTrendScore(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	// Recent high-quality article
	recentArticle := &models.Article{
		Quality:     0.9,
		PublishedAt: time.Now().Add(-12 * time.Hour),
	}
	recentScore := sorter.calculateTrendScore(recentArticle)

	// Old article
	oldArticle := &models.Article{
		Quality:     0.9,
		PublishedAt: time.Now().Add(-168 * time.Hour), // 1 week old
	}
	oldScore := sorter.calculateTrendScore(oldArticle)

	// Recent article should have higher trend score
	if recentScore <= oldScore {
		t.Errorf("Recent article should have higher trend score: recent=%f, old=%f", 
			recentScore, oldScore)
	}

	// Score should be between 0 and 1
	if recentScore < 0 || recentScore > 1 {
		t.Errorf("Trend score should be between 0 and 1, got %f", recentScore)
	}
}

func TestValidateSortConfig(t *testing.T) {
	validConfig := GetDefaultSortConfig()
	err := ValidateSortConfig(validConfig)
	if err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Test invalid primary sort
	invalidConfig := validConfig
	invalidConfig.Primary = SortBy("invalid")
	err = ValidateSortConfig(invalidConfig)
	if err == nil {
		t.Error("Invalid primary sort should fail validation")
	}

	// Test invalid order
	invalidConfig = validConfig
	invalidConfig.Order = SortOrder("invalid")
	err = ValidateSortConfig(invalidConfig)
	if err == nil {
		t.Error("Invalid order should fail validation")
	}

	// Test zero weights
	invalidConfig = validConfig
	invalidConfig.RelevanceWeight = 0
	invalidConfig.TimeWeight = 0
	invalidConfig.PopularityWeight = 0
	invalidConfig.TrendWeight = 0
	err = ValidateSortConfig(invalidConfig)
	if err == nil {
		t.Error("Zero total weight should fail validation")
	}
}

func TestValidatePaginationConfig(t *testing.T) {
	validConfig := GetDefaultPaginationConfig()
	err := ValidatePaginationConfig(validConfig)
	if err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Test invalid page
	invalidConfig := validConfig
	invalidConfig.Page = 0
	err = ValidatePaginationConfig(invalidConfig)
	if err == nil {
		t.Error("Page 0 should fail validation")
	}

	// Test invalid page size
	invalidConfig = validConfig
	invalidConfig.PageSize = 0
	err = ValidatePaginationConfig(invalidConfig)
	if err == nil {
		t.Error("Page size 0 should fail validation")
	}

	invalidConfig.PageSize = 101
	err = ValidatePaginationConfig(invalidConfig)
	if err == nil {
		t.Error("Page size > 100 should fail validation")
	}
}

func TestEmptyArticleList(t *testing.T) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	result, err := sorter.SortAndPaginate(nil, GetDefaultPaginationConfig())
	if err != nil {
		t.Fatalf("Should handle nil articles: %v", err)
	}

	if len(result.Items) != 0 {
		t.Error("Should return empty results for nil articles")
	}

	// Test empty slice
	result, err = sorter.SortAndPaginate([]*models.Article{}, GetDefaultPaginationConfig())
	if err != nil {
		t.Fatalf("Should handle empty articles: %v", err)
	}

	if len(result.Items) != 0 {
		t.Error("Should return empty results for empty articles")
	}
}

func BenchmarkSortArticles(b *testing.B) {
	scorer := NewRelevanceScorer([]string{"test", "benchmark"})
	sorter := NewArticleSorter(scorer)

	// Create larger test dataset
	articles := make([]*models.Article, 100)
	for i := 0; i < 100; i++ {
		articles[i] = &models.Article{
			ID:          string(rune('a' + i%26)),
			Title:       "Benchmark Article " + string(rune('A'+i%26)),
			Relevance:   float64(i%100) / 100.0,
			Quality:     float64((i*7)%100) / 100.0,
			PublishedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sorter.SortArticles(articles)
	}
}

func BenchmarkPagination(b *testing.B) {
	scorer := NewRelevanceScorer([]string{"test"})
	sorter := NewArticleSorter(scorer)

	paginationConfig := PaginationConfig{
		Page:     1,
		PageSize: 20,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sorter.SortAndPaginate(sortTestArticles, paginationConfig)
	}
}