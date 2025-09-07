package models

import (
	"testing"
	"time"
)

func TestArticleCreation(t *testing.T) {
	// Test creating a new article
	article := NewArticle("Test Article", "https://example.com/article", "Example Source", "rss")
	
	if article == nil {
		t.Fatal("NewArticle returned nil")
	}
	
	if article.ID == "" {
		t.Error("Article ID should be generated")
	}
	
	if article.Title != "Test Article" {
		t.Errorf("Expected title 'Test Article', got '%s'", article.Title)
	}
	
	if article.URL != "https://example.com/article" {
		t.Errorf("Expected URL 'https://example.com/article', got '%s'", article.URL)
	}
	
	if article.Source != "Example Source" {
		t.Errorf("Expected source 'Example Source', got '%s'", article.Source)
	}
	
	if article.SourceType != "rss" {
		t.Errorf("Expected sourceType 'rss', got '%s'", article.SourceType)
	}
}

func TestArticleValidation(t *testing.T) {
	// Test valid article
	validArticle := NewArticle("Valid Title", "https://example.com", "Valid Source", "rss")
	if err := validArticle.Validate(); err != nil {
		t.Errorf("Valid article should pass validation, got error: %v", err)
	}
	
	// Test invalid article - empty title
	invalidArticle := &Article{
		ID:          "test",
		Title:       "",
		URL:         "https://example.com",
		Source:      "Source",
		SourceType:  "rss",
		PublishedAt: time.Now(),
	}
	if err := invalidArticle.Validate(); err == nil {
		t.Error("Article with empty title should fail validation")
	}
	
	// Test invalid source type
	invalidSourceType := &Article{
		ID:          "test",
		Title:       "Title",
		URL:         "https://example.com",
		Source:      "Source",
		SourceType:  "invalid",
		PublishedAt: time.Now(),
	}
	if err := invalidSourceType.Validate(); err == nil {
		t.Error("Article with invalid source type should fail validation")
	}
}

func TestArticleUtilityMethods(t *testing.T) {
	article := NewArticle("Test Article", "https://example.com", "Source", "rss")
	
	// Test adding tags
	article.AddTag("tech")
	article.AddTag("news")
	article.AddTag("tech") // duplicate
	
	if len(article.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(article.Tags))
	}
	
	if !article.HasTag("tech") {
		t.Error("Article should have 'tech' tag")
	}
	
	if article.HasTag("nonexistent") {
		t.Error("Article should not have 'nonexistent' tag")
	}
	
	// Test metadata
	article.SetMetadata("custom_field", "custom_value")
	value, exists := article.GetMetadata("custom_field")
	if !exists {
		t.Error("Metadata should exist")
	}
	if value != "custom_value" {
		t.Errorf("Expected 'custom_value', got '%v'", value)
	}
	
	// Test quality calculation
	article.Summary = "This is a test summary that is long enough to meet requirements."
	article.AddTags("tech", "programming", "go")
	article.UpdateQuality()
	
	if article.Quality <= 0 {
		t.Error("Quality score should be greater than 0")
	}
}

func TestRepositoryCreation(t *testing.T) {
	// Test creating a new repository
	repo := NewRepository("test-repo", "owner/test-repo", "https://github.com/owner/test-repo")
	
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
	
	if repo.ID == "" {
		t.Error("Repository ID should be generated")
	}
	
	if repo.Name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got '%s'", repo.Name)
	}
	
	if repo.FullName != "owner/test-repo" {
		t.Errorf("Expected fullName 'owner/test-repo', got '%s'", repo.FullName)
	}
	
	if repo.URL != "https://github.com/owner/test-repo" {
		t.Errorf("Expected URL 'https://github.com/owner/test-repo', got '%s'", repo.URL)
	}
}

func TestRepositoryValidation(t *testing.T) {
	// Test valid repository
	validRepo := NewRepository("test", "owner/test", "https://github.com/owner/test")
	if err := validRepo.Validate(); err != nil {
		t.Errorf("Valid repository should pass validation, got error: %v", err)
	}
	
	// Test invalid repository - empty name
	invalidRepo := &Repository{
		ID:        "test",
		Name:      "",
		FullName:  "owner/test",
		URL:       "https://github.com/owner/test",
		UpdatedAt: time.Now(),
	}
	if err := invalidRepo.Validate(); err == nil {
		t.Error("Repository with empty name should fail validation")
	}
	
	// Test invalid stars
	invalidStars := &Repository{
		ID:        "test",
		Name:      "test",
		FullName:  "owner/test",
		URL:       "https://github.com/owner/test",
		Stars:     -1,
		UpdatedAt: time.Now(),
	}
	if err := invalidStars.Validate(); err == nil {
		t.Error("Repository with negative stars should fail validation")
	}
}

func TestRepositoryUtilityMethods(t *testing.T) {
	repo := NewRepository("test-repo", "owner/test-repo", "https://github.com/owner/test-repo")
	
	// Test owner extraction
	if repo.GetOwner() != "owner" {
		t.Errorf("Expected owner 'owner', got '%s'", repo.GetOwner())
	}
	
	// Test repo name extraction
	if repo.GetRepoName() != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got '%s'", repo.GetRepoName())
	}
	
	// Test updating stats
	err := repo.UpdateStats(100, 20, time.Now())
	if err != nil {
		t.Errorf("UpdateStats should not return error, got: %v", err)
	}
	
	if repo.Stars != 100 {
		t.Errorf("Expected 100 stars, got %d", repo.Stars)
	}
	
	if repo.Forks != 20 {
		t.Errorf("Expected 20 forks, got %d", repo.Forks)
	}
	
	// Test popularity check
	if !repo.IsPopular() {
		t.Error("Repository with 100 stars should be considered popular")
	}
	
	// Test trend score calculation
	repo.CalculateTrendScore()
	if repo.TrendScore <= 0 {
		t.Error("Trend score should be greater than 0 for active repository")
	}
	
	// Test activity level
	activityLevel := repo.GetActivityLevel()
	if activityLevel == "" {
		t.Error("Activity level should not be empty")
	}
	
	// Test popularity tier
	popularityTier := repo.GetPopularityTier()
	if popularityTier != "popular" {
		t.Errorf("Expected popularity tier 'popular' for 100 stars, got '%s'", popularityTier)
	}
}

func TestHashGeneration(t *testing.T) {
	// Test article hash
	article1 := NewArticle("Same Title", "https://same-url.com", "Source", "rss")
	article2 := NewArticle("Same Title", "https://same-url.com", "Source", "api")
	
	hash1 := article1.CalculateHash()
	hash2 := article2.CalculateHash()
	
	if hash1 != hash2 {
		t.Error("Articles with same title and URL should have same hash for deduplication")
	}
	
	// Test repository hash
	repo1 := NewRepository("repo", "owner/repo", "https://github.com/owner/repo")
	repo2 := NewRepository("repo", "owner/repo", "https://gitlab.com/owner/repo")
	
	repoHash1 := repo1.CalculateHash()
	repoHash2 := repo2.CalculateHash()
	
	if repoHash1 == repoHash2 {
		t.Error("Repositories with different URLs should have different hashes")
	}
}