package processor

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ZephyrDeng/dev-context/internal/models"
)

// SortBy defines the primary sorting criteria
type SortBy string

const (
	SortByRelevance  SortBy = "relevance"  // Sort by relevance score
	SortByTime       SortBy = "time"       // Sort by publication time
	SortByPopularity SortBy = "popularity" // Sort by quality/popularity metrics
	SortByTrend      SortBy = "trend"      // Sort by trending score
	SortByComposite  SortBy = "composite"  // Weighted combination of multiple factors
)

// SortOrder defines ascending or descending order
type SortOrder string

const (
	SortAsc  SortOrder = "asc"  // Ascending order
	SortDesc SortOrder = "desc" // Descending order
)

// SortConfig defines the sorting configuration
type SortConfig struct {
	Primary   SortBy    `json:"primary"`   // Primary sorting criterion
	Secondary SortBy    `json:"secondary"` // Secondary sorting criterion (for tie-breaking)
	Order     SortOrder `json:"order"`     // Sort order

	// Weights for composite sorting
	RelevanceWeight  float64 `json:"relevanceWeight"`  // Weight for relevance (default: 0.4)
	TimeWeight       float64 `json:"timeWeight"`       // Weight for recency (default: 0.3)
	PopularityWeight float64 `json:"popularityWeight"` // Weight for popularity (default: 0.2)
	TrendWeight      float64 `json:"trendWeight"`      // Weight for trending (default: 0.1)
}

// PaginationConfig defines pagination settings
type PaginationConfig struct {
	Page     int `json:"page"`     // Current page (1-based)
	PageSize int `json:"pageSize"` // Items per page
}

// PaginationResult contains paginated results with metadata
type PaginationResult struct {
	Items       []*models.Article `json:"items"`
	CurrentPage int               `json:"currentPage"`
	PageSize    int               `json:"pageSize"`
	TotalItems  int               `json:"totalItems"`
	TotalPages  int               `json:"totalPages"`
	HasNext     bool              `json:"hasNext"`
	HasPrev     bool              `json:"hasPrev"`
}

// UserPreferences defines user-specific ranking preferences
type UserPreferences struct {
	FavoriteTopics    []string           `json:"favoriteTopics"`    // Preferred topic keywords
	PreferredSources  []string           `json:"preferredSources"`  // Preferred news sources
	ReadingHistory    []string           `json:"readingHistory"`    // Article IDs user has read
	TopicWeights      map[string]float64 `json:"topicWeights"`      // Custom weights for topics
	RecencyPreference float64            `json:"recencyPreference"` // How much user prefers recent articles (0-1)
	LanguagePrefs     []string           `json:"languagePrefs"`     // Preferred programming languages for repos
}

// ArticleSorter handles multi-dimensional sorting and pagination of articles
type ArticleSorter struct {
	relevanceScorer *RelevanceScorer
	sortConfig      SortConfig
	userPrefs       *UserPreferences
}

// RepositorySorter handles sorting of repositories
type RepositorySorter struct {
	sortConfig SortConfig
	userPrefs  *UserPreferences
}

// NewArticleSorter creates a new ArticleSorter instance
func NewArticleSorter(relevanceScorer *RelevanceScorer) *ArticleSorter {
	defaultConfig := SortConfig{
		Primary:          SortByComposite,
		Secondary:        SortByTime,
		Order:            SortDesc,
		RelevanceWeight:  0.4,
		TimeWeight:       0.3,
		PopularityWeight: 0.2,
		TrendWeight:      0.1,
	}

	return &ArticleSorter{
		relevanceScorer: relevanceScorer,
		sortConfig:      defaultConfig,
		userPrefs:       nil,
	}
}

// NewRepositorySorter creates a new RepositorySorter instance
func NewRepositorySorter() *RepositorySorter {
	defaultConfig := SortConfig{
		Primary:          SortByTrend,
		Secondary:        SortByPopularity,
		Order:            SortDesc,
		RelevanceWeight:  0.2,
		TimeWeight:       0.3,
		PopularityWeight: 0.3,
		TrendWeight:      0.2,
	}

	return &RepositorySorter{
		sortConfig: defaultConfig,
		userPrefs:  nil,
	}
}

// SetSortConfig updates the sorting configuration
func (as *ArticleSorter) SetSortConfig(config SortConfig) {
	as.sortConfig = config
}

// SetUserPreferences sets user-specific preferences for personalized ranking
func (as *ArticleSorter) SetUserPreferences(prefs *UserPreferences) {
	as.userPrefs = prefs
}

// SortAndPaginate sorts articles and applies pagination
func (as *ArticleSorter) SortAndPaginate(articles []*models.Article, paginationConfig PaginationConfig) (*PaginationResult, error) {
	if articles == nil {
		return &PaginationResult{}, nil
	}

	// Apply personalization if user preferences are set
	if as.userPrefs != nil {
		articles = as.applyPersonalization(articles)
	}

	// Sort articles
	sortedArticles := as.SortArticles(articles)

	// Apply pagination
	return as.paginate(sortedArticles, paginationConfig), nil
}

// SortArticles sorts articles according to the configured criteria
func (as *ArticleSorter) SortArticles(articles []*models.Article) []*models.Article {
	if len(articles) <= 1 {
		return articles
	}

	// Create a copy to avoid modifying the original slice
	sorted := make([]*models.Article, len(articles))
	copy(sorted, articles)

	// Sort based on configuration
	sort.Slice(sorted, func(i, j int) bool {
		return as.compareArticles(sorted[i], sorted[j])
	})

	return sorted
}

// compareArticles compares two articles based on sort configuration
func (as *ArticleSorter) compareArticles(a, b *models.Article) bool {
	primaryResult := as.compareByMetric(a, b, as.sortConfig.Primary)

	if primaryResult == 0 {
		// Use secondary criterion for tie-breaking
		secondaryResult := as.compareByMetric(a, b, as.sortConfig.Secondary)
		return as.applyOrder(secondaryResult)
	}

	return as.applyOrder(primaryResult)
}

// compareByMetric compares two articles by a specific metric
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func (as *ArticleSorter) compareByMetric(a, b *models.Article, metric SortBy) int {
	switch metric {
	case SortByRelevance:
		return as.compareFloat64(a.Relevance, b.Relevance)

	case SortByTime:
		if a.PublishedAt.Before(b.PublishedAt) {
			return -1
		} else if a.PublishedAt.After(b.PublishedAt) {
			return 1
		}
		return 0

	case SortByPopularity:
		return as.compareFloat64(a.Quality, b.Quality)

	case SortByTrend:
		// Calculate trend score based on recency and quality
		trendA := as.calculateTrendScore(a)
		trendB := as.calculateTrendScore(b)
		return as.compareFloat64(trendA, trendB)

	case SortByComposite:
		compositeA := as.calculateCompositeScore(a)
		compositeB := as.calculateCompositeScore(b)
		return as.compareFloat64(compositeA, compositeB)

	default:
		return 0
	}
}

// compareFloat64 compares two float64 values with tolerance
func (as *ArticleSorter) compareFloat64(a, b float64) int {
	tolerance := 1e-9
	if math.Abs(a-b) < tolerance {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

// calculateTrendScore calculates trending score based on recency and engagement
func (as *ArticleSorter) calculateTrendScore(article *models.Article) float64 {
	// Base score from quality
	score := article.Quality * 0.4

	// Recency bonus (articles from last 24 hours get highest bonus)
	hoursOld := time.Since(article.PublishedAt).Hours()
	if hoursOld <= 24 {
		score += 0.6 // Maximum recency bonus
	} else if hoursOld <= 7*24 {
		score += 0.4 * (1.0 - (hoursOld-24)/(7*24-24))
	} else if hoursOld <= 30*24 {
		score += 0.2 * (1.0 - (hoursOld-7*24)/(30*24-7*24))
	}

	return math.Min(1.0, score)
}

// calculateCompositeScore calculates weighted composite score
func (as *ArticleSorter) calculateCompositeScore(article *models.Article) float64 {
	config := as.sortConfig

	score := 0.0
	score += article.Relevance * config.RelevanceWeight
	score += as.calculateTimeScore(article) * config.TimeWeight
	score += article.Quality * config.PopularityWeight
	score += as.calculateTrendScore(article) * config.TrendWeight

	return score
}

// calculateTimeScore converts publication time to a score (newer = higher)
func (as *ArticleSorter) calculateTimeScore(article *models.Article) float64 {
	hoursOld := time.Since(article.PublishedAt).Hours()

	// Use exponential decay to favor recent articles
	// Score approaches 0 as articles get older
	decayRate := 0.01 // Adjust this to change how quickly scores decay
	return math.Exp(-decayRate * hoursOld)
}

// applyOrder applies the sort order (ascending/descending)
func (as *ArticleSorter) applyOrder(comparison int) bool {
	if as.sortConfig.Order == SortAsc {
		return comparison < 0
	}
	return comparison > 0
}

// applyPersonalization applies user preferences to boost relevant articles
func (as *ArticleSorter) applyPersonalization(articles []*models.Article) []*models.Article {
	if as.userPrefs == nil {
		return articles
	}

	for _, article := range articles {
		// Boost articles from preferred sources
		for _, preferredSource := range as.userPrefs.PreferredSources {
			if article.Source == preferredSource {
				article.Quality = math.Min(1.0, article.Quality+0.1)
				break
			}
		}

		// Boost articles matching favorite topics
		for _, topic := range as.userPrefs.FavoriteTopics {
			if as.articleMatchesToopic(article, topic) {
				weight := 0.1 // default boost
				if customWeight, exists := as.userPrefs.TopicWeights[topic]; exists {
					weight = customWeight
				}
				article.Relevance = math.Min(1.0, article.Relevance+weight)
				break
			}
		}

		// Apply recency preference
		if as.userPrefs.RecencyPreference > 0 {
			timeScore := as.calculateTimeScore(article)
			boost := timeScore * as.userPrefs.RecencyPreference * 0.1
			article.Quality = math.Min(1.0, article.Quality+boost)
		}
	}

	return articles
}

// articleMatchesToopic checks if article matches a topic
func (as *ArticleSorter) articleMatchesToopic(article *models.Article, topic string) bool {
	topicLower := strings.ToLower(topic)

	// Check title, summary, and tags
	if strings.Contains(strings.ToLower(article.Title), topicLower) ||
		strings.Contains(strings.ToLower(article.Summary), topicLower) {
		return true
	}

	for _, tag := range article.Tags {
		if strings.Contains(strings.ToLower(tag), topicLower) {
			return true
		}
	}

	return false
}

// paginate applies pagination to the sorted articles
func (as *ArticleSorter) paginate(articles []*models.Article, config PaginationConfig) *PaginationResult {
	totalItems := len(articles)

	// Validate pagination config
	if config.PageSize <= 0 {
		config.PageSize = 20 // default page size
	}
	if config.Page <= 0 {
		config.Page = 1 // default to first page
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(config.PageSize)))

	// Calculate start and end indices
	startIdx := (config.Page - 1) * config.PageSize
	endIdx := startIdx + config.PageSize

	if startIdx >= totalItems {
		// Page is beyond available data
		return &PaginationResult{
			Items:       []*models.Article{},
			CurrentPage: config.Page,
			PageSize:    config.PageSize,
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			HasNext:     false,
			HasPrev:     config.Page > 1,
		}
	}

	if endIdx > totalItems {
		endIdx = totalItems
	}

	return &PaginationResult{
		Items:       articles[startIdx:endIdx],
		CurrentPage: config.Page,
		PageSize:    config.PageSize,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasNext:     config.Page < totalPages,
		HasPrev:     config.Page > 1,
	}
}

// Repository sorting methods

// SetSortConfig updates the sorting configuration for repositories
func (rs *RepositorySorter) SetSortConfig(config SortConfig) {
	rs.sortConfig = config
}

// SetUserPreferences sets user preferences for repository sorting
func (rs *RepositorySorter) SetUserPreferences(prefs *UserPreferences) {
	rs.userPrefs = prefs
}

// SortAndPaginate sorts repositories and applies pagination
func (rs *RepositorySorter) SortAndPaginate(repos []*models.Repository, paginationConfig PaginationConfig) (*RepositoryPaginationResult, error) {
	if repos == nil {
		return &RepositoryPaginationResult{}, nil
	}

	// Apply personalization if user preferences are set
	if rs.userPrefs != nil {
		repos = rs.applyRepositoryPersonalization(repos)
	}

	// Sort repositories
	sortedRepos := rs.SortRepositories(repos)

	// Apply pagination
	return rs.paginateRepositories(sortedRepos, paginationConfig), nil
}

// RepositoryPaginationResult contains paginated repository results
type RepositoryPaginationResult struct {
	Items       []*models.Repository `json:"items"`
	CurrentPage int                  `json:"currentPage"`
	PageSize    int                  `json:"pageSize"`
	TotalItems  int                  `json:"totalItems"`
	TotalPages  int                  `json:"totalPages"`
	HasNext     bool                 `json:"hasNext"`
	HasPrev     bool                 `json:"hasPrev"`
}

// SortRepositories sorts repositories according to the configured criteria
func (rs *RepositorySorter) SortRepositories(repos []*models.Repository) []*models.Repository {
	if len(repos) <= 1 {
		return repos
	}

	// Create a copy to avoid modifying the original slice
	sorted := make([]*models.Repository, len(repos))
	copy(sorted, repos)

	// Sort based on configuration
	sort.Slice(sorted, func(i, j int) bool {
		return rs.compareRepositories(sorted[i], sorted[j])
	})

	return sorted
}

// compareRepositories compares two repositories based on sort configuration
func (rs *RepositorySorter) compareRepositories(a, b *models.Repository) bool {
	primaryResult := rs.compareRepoByMetric(a, b, rs.sortConfig.Primary)

	if primaryResult == 0 {
		// Use secondary criterion for tie-breaking
		secondaryResult := rs.compareRepoByMetric(a, b, rs.sortConfig.Secondary)
		return rs.applyRepoOrder(secondaryResult)
	}

	return rs.applyRepoOrder(primaryResult)
}

// compareRepoByMetric compares two repositories by a specific metric
func (rs *RepositorySorter) compareRepoByMetric(a, b *models.Repository, metric SortBy) int {
	switch metric {
	case SortByTrend:
		return rs.compareFloat64(a.TrendScore, b.TrendScore)

	case SortByTime:
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return -1
		} else if a.UpdatedAt.After(b.UpdatedAt) {
			return 1
		}
		return 0

	case SortByPopularity:
		// Use stars as popularity metric
		if a.Stars < b.Stars {
			return -1
		} else if a.Stars > b.Stars {
			return 1
		}
		return 0

	case SortByComposite:
		compositeA := rs.calculateRepoCompositeScore(a)
		compositeB := rs.calculateRepoCompositeScore(b)
		return rs.compareFloat64(compositeA, compositeB)

	default:
		return rs.compareFloat64(a.TrendScore, b.TrendScore)
	}
}

// calculateRepoCompositeScore calculates composite score for repository
func (rs *RepositorySorter) calculateRepoCompositeScore(repo *models.Repository) float64 {
	config := rs.sortConfig

	// Normalize stars for scoring (logarithmic scale)
	normalizedStars := math.Log10(float64(repo.Stars+1)) / 5.0 // Assume max ~10^5 stars
	if normalizedStars > 1.0 {
		normalizedStars = 1.0
	}

	score := 0.0
	score += repo.TrendScore * config.TrendWeight
	score += rs.calculateRepoTimeScore(repo) * config.TimeWeight
	score += normalizedStars * config.PopularityWeight

	return score
}

// calculateRepoTimeScore converts repository update time to score
func (rs *RepositorySorter) calculateRepoTimeScore(repo *models.Repository) float64 {
	hoursOld := time.Since(repo.UpdatedAt).Hours()

	// Repositories updated in last week get high scores
	if hoursOld <= 7*24 {
		return 1.0
	} else if hoursOld <= 30*24 {
		return 0.8
	} else if hoursOld <= 90*24 {
		return 0.6
	} else if hoursOld <= 365*24 {
		return 0.4
	}

	return 0.2
}

// compareFloat64 compares two float64 values for repositories
func (rs *RepositorySorter) compareFloat64(a, b float64) int {
	tolerance := 1e-9
	if math.Abs(a-b) < tolerance {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

// applyRepoOrder applies sort order for repositories
func (rs *RepositorySorter) applyRepoOrder(comparison int) bool {
	if rs.sortConfig.Order == SortAsc {
		return comparison < 0
	}
	return comparison > 0
}

// applyRepositoryPersonalization applies user preferences to repositories
func (rs *RepositorySorter) applyRepositoryPersonalization(repos []*models.Repository) []*models.Repository {
	if rs.userPrefs == nil {
		return repos
	}

	for _, repo := range repos {
		// Boost repositories in preferred languages
		for _, lang := range rs.userPrefs.LanguagePrefs {
			if strings.EqualFold(repo.Language, lang) {
				repo.TrendScore = math.Min(1.0, repo.TrendScore+0.1)
				break
			}
		}

		// Boost repositories matching favorite topics in name/description
		for _, topic := range rs.userPrefs.FavoriteTopics {
			if rs.repositoryMatchesToopic(repo, topic) {
				weight := 0.05 // smaller boost for repos
				if customWeight, exists := rs.userPrefs.TopicWeights[topic]; exists {
					weight = customWeight * 0.5 // scale down for repos
				}
				repo.TrendScore = math.Min(1.0, repo.TrendScore+weight)
				break
			}
		}
	}

	return repos
}

// repositoryMatchesToopic checks if repository matches a topic
func (rs *RepositorySorter) repositoryMatchesToopic(repo *models.Repository, topic string) bool {
	topicLower := strings.ToLower(topic)

	return strings.Contains(strings.ToLower(repo.Name), topicLower) ||
		strings.Contains(strings.ToLower(repo.Description), topicLower) ||
		strings.Contains(strings.ToLower(repo.FullName), topicLower)
}

// paginateRepositories applies pagination to repositories
func (rs *RepositorySorter) paginateRepositories(repos []*models.Repository, config PaginationConfig) *RepositoryPaginationResult {
	totalItems := len(repos)

	// Validate pagination config
	if config.PageSize <= 0 {
		config.PageSize = 20 // default page size
	}
	if config.Page <= 0 {
		config.Page = 1 // default to first page
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(config.PageSize)))

	// Calculate start and end indices
	startIdx := (config.Page - 1) * config.PageSize
	endIdx := startIdx + config.PageSize

	if startIdx >= totalItems {
		return &RepositoryPaginationResult{
			Items:       []*models.Repository{},
			CurrentPage: config.Page,
			PageSize:    config.PageSize,
			TotalItems:  totalItems,
			TotalPages:  totalPages,
			HasNext:     false,
			HasPrev:     config.Page > 1,
		}
	}

	if endIdx > totalItems {
		endIdx = totalItems
	}

	return &RepositoryPaginationResult{
		Items:       repos[startIdx:endIdx],
		CurrentPage: config.Page,
		PageSize:    config.PageSize,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasNext:     config.Page < totalPages,
		HasPrev:     config.Page > 1,
	}
}

// GetDefaultSortConfig returns default sorting configuration
func GetDefaultSortConfig() SortConfig {
	return SortConfig{
		Primary:          SortByComposite,
		Secondary:        SortByTime,
		Order:            SortDesc,
		RelevanceWeight:  0.4,
		TimeWeight:       0.3,
		PopularityWeight: 0.2,
		TrendWeight:      0.1,
	}
}

// GetDefaultPaginationConfig returns default pagination configuration
func GetDefaultPaginationConfig() PaginationConfig {
	return PaginationConfig{
		Page:     1,
		PageSize: 20,
	}
}

// ValidateSortConfig validates a sort configuration
func ValidateSortConfig(config SortConfig) error {
	validSortBy := map[SortBy]bool{
		SortByRelevance:  true,
		SortByTime:       true,
		SortByPopularity: true,
		SortByTrend:      true,
		SortByComposite:  true,
	}

	if !validSortBy[config.Primary] {
		return fmt.Errorf("invalid primary sort criterion: %s", config.Primary)
	}

	if !validSortBy[config.Secondary] {
		return fmt.Errorf("invalid secondary sort criterion: %s", config.Secondary)
	}

	if config.Order != SortAsc && config.Order != SortDesc {
		return fmt.Errorf("invalid sort order: %s", config.Order)
	}

	// Validate weights for composite sorting
	totalWeight := config.RelevanceWeight + config.TimeWeight + config.PopularityWeight + config.TrendWeight
	if totalWeight <= 0 {
		return fmt.Errorf("total weight must be greater than 0")
	}

	return nil
}

// ValidatePaginationConfig validates pagination configuration
func ValidatePaginationConfig(config PaginationConfig) error {
	if config.Page < 1 {
		return fmt.Errorf("page must be >= 1")
	}

	if config.PageSize < 1 || config.PageSize > 100 {
		return fmt.Errorf("page size must be between 1 and 100")
	}

	return nil
}
