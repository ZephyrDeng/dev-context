package models

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"
)

// Repository represents a unified data structure for code repositories from various sources
// This structure standardizes data from GitHub API, GitLab API, and other git hosting services
type Repository struct {
	// ID is a unique identifier for the repository, typically generated from FullName
	ID string `json:"id" validate:"required"`
	
	// Name is the repository name (without owner)
	Name string `json:"name" validate:"required,min=1,max=100"`
	
	// FullName is the complete repository identifier including owner (e.g., "owner/repo")
	FullName string `json:"fullName" validate:"required,min=1,max=200"`
	
	// Description is the repository description or summary
	Description string `json:"description" validate:"max=1000"`
	
	// URL is the repository homepage URL
	URL string `json:"url" validate:"required,url"`
	
	// Language is the primary programming language of the repository
	Language string `json:"language" validate:"max=50"`
	
	// Stars is the number of stars/likes the repository has received
	Stars int `json:"stars" validate:"min=0"`
	
	// Forks is the number of times the repository has been forked
	Forks int `json:"forks" validate:"min=0"`
	
	// TrendScore is a calculated score indicating the repository's trending status (0.0-1.0)
	TrendScore float64 `json:"trendScore" validate:"min=0,max=1"`
	
	// UpdatedAt is the timestamp of the last repository update
	UpdatedAt time.Time `json:"updatedAt" validate:"required"`
}

// NewRepository creates a new Repository instance with required fields and generates ID
func NewRepository(name, fullName, url string) *Repository {
	repo := &Repository{
		Name:       strings.TrimSpace(name),
		FullName:   strings.TrimSpace(fullName),
		URL:        strings.TrimSpace(url),
		Description: "",
		Language:   "",
		Stars:      0,
		Forks:      0,
		TrendScore: 0.0,
		UpdatedAt:  time.Now(),
	}
	repo.ID = repo.GenerateID()
	return repo
}

// Validate performs basic validation on the Repository fields
func (r *Repository) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("ID is required")
	}
	
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("Name is required")
	}
	
	if len(r.Name) > 100 {
		return fmt.Errorf("Name must not exceed 100 characters")
	}
	
	if strings.TrimSpace(r.FullName) == "" {
		return fmt.Errorf("FullName is required")
	}
	
	if len(r.FullName) > 200 {
		return fmt.Errorf("FullName must not exceed 200 characters")
	}
	
	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("URL is required")
	}
	
	if len(r.Description) > 1000 {
		return fmt.Errorf("Description must not exceed 1000 characters")
	}
	
	if len(r.Language) > 50 {
		return fmt.Errorf("Language must not exceed 50 characters")
	}
	
	if r.Stars < 0 {
		return fmt.Errorf("Stars cannot be negative")
	}
	
	if r.Forks < 0 {
		return fmt.Errorf("Forks cannot be negative")
	}
	
	if r.TrendScore < 0.0 || r.TrendScore > 1.0 {
		return fmt.Errorf("TrendScore must be between 0.0 and 1.0")
	}
	
	if r.UpdatedAt.IsZero() {
		return fmt.Errorf("UpdatedAt is required")
	}
	
	return nil
}

// GenerateID creates a unique identifier for the repository based on FullName
func (r *Repository) GenerateID() string {
	if r.FullName == "" {
		return ""
	}
	
	// Normalize FullName for consistent ID generation
	content := strings.ToLower(strings.TrimSpace(r.FullName))
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("repo_%x", hash)[:16] // Truncate to 16 chars for shorter IDs
}

// SetDescription sets the repository description with validation
func (r *Repository) SetDescription(description string) error {
	description = strings.TrimSpace(description)
	
	if len(description) > 1000 {
		return fmt.Errorf("description too long (maximum 1000 characters)")
	}
	
	r.Description = description
	return nil
}

// SetLanguage sets the primary programming language
func (r *Repository) SetLanguage(language string) error {
	language = strings.TrimSpace(language)
	
	if len(language) > 50 {
		return fmt.Errorf("language name too long (maximum 50 characters)")
	}
	
	r.Language = language
	return nil
}

// UpdateStats updates the repository statistics (stars, forks, and last update time)
func (r *Repository) UpdateStats(stars, forks int, updatedAt time.Time) error {
	if stars < 0 {
		return fmt.Errorf("stars cannot be negative")
	}
	
	if forks < 0 {
		return fmt.Errorf("forks cannot be negative")
	}
	
	r.Stars = stars
	r.Forks = forks
	r.UpdatedAt = updatedAt
	
	// Recalculate trend score after stats update
	r.CalculateTrendScore()
	
	return nil
}

// CalculateTrendScore computes the trending score based on multiple factors
func (r *Repository) CalculateTrendScore() {
	score := 0.0
	
	// Base score for having stars (logarithmic scale to prevent dominant effect)
	if r.Stars > 0 {
		// Normalize stars using log scale (max score: 0.4)
		if r.Stars >= 1000 {
			score += 0.4
		} else if r.Stars >= 100 {
			score += 0.3
		} else if r.Stars >= 10 {
			score += 0.2
		} else {
			score += 0.1
		}
	}
	
	// Score for fork activity (indicates active usage)
	if r.Forks > 0 {
		// Normalize forks (max score: 0.2)
		if r.Forks >= 100 {
			score += 0.2
		} else if r.Forks >= 10 {
			score += 0.15
		} else {
			score += 0.1
		}
	}
	
	// Score for recent activity (repositories updated within last 30 days get bonus)
	if r.IsRecentlyUpdated(30 * 24 * time.Hour) {
		score += 0.2
		
		// Extra bonus for very recent updates (within last 7 days)
		if r.IsRecentlyUpdated(7 * 24 * time.Hour) {
			score += 0.1
		}
	}
	
	// Score for having a description (indicates maintained project)
	if strings.TrimSpace(r.Description) != "" {
		score += 0.1
	}
	
	// Ensure score is within valid range
	if score > 1.0 {
		score = 1.0
	}
	
	r.TrendScore = score
}

// IsRecentlyUpdated checks if the repository was updated within the specified duration
func (r *Repository) IsRecentlyUpdated(within time.Duration) bool {
	return time.Since(r.UpdatedAt) <= within
}

// GetOwner extracts the owner name from the FullName (before the slash)
func (r *Repository) GetOwner() string {
	parts := strings.Split(r.FullName, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// GetRepoName extracts the repository name from the FullName (after the slash)
func (r *Repository) GetRepoName() string {
	parts := strings.Split(r.FullName, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return r.Name
}

// CalculateHash generates a hash for deduplication purposes
func (r *Repository) CalculateHash() string {
	// Use FullName and URL for deduplication
	content := strings.ToLower(r.FullName) + "|" + r.URL
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// IsPopular determines if the repository is considered popular based on stars and forks
func (r *Repository) IsPopular() bool {
	// Consider a repository popular if it has significant stars or active forks
	return r.Stars >= 100 || (r.Stars >= 10 && r.Forks >= 5)
}

// IsTrending determines if the repository is currently trending based on trend score
func (r *Repository) IsTrending() bool {
	return r.TrendScore >= 0.7
}

// GetActivityLevel returns a string describing the repository's activity level
func (r *Repository) GetActivityLevel() string {
	if r.IsRecentlyUpdated(7 * 24 * time.Hour) {
		return "very_active"
	} else if r.IsRecentlyUpdated(30 * 24 * time.Hour) {
		return "active"
	} else if r.IsRecentlyUpdated(90 * 24 * time.Hour) {
		return "moderate"
	} else {
		return "inactive"
	}
}

// GetPopularityTier returns a string describing the repository's popularity tier
func (r *Repository) GetPopularityTier() string {
	if r.Stars >= 10000 {
		return "viral"
	} else if r.Stars >= 1000 {
		return "very_popular"
	} else if r.Stars >= 100 {
		return "popular"
	} else if r.Stars >= 10 {
		return "gaining_traction"
	} else {
		return "new_or_niche"
	}
}

// HasLanguage checks if the repository has a specified programming language
func (r *Repository) HasLanguage() bool {
	return strings.TrimSpace(r.Language) != ""
}

// String returns a string representation of the repository
func (r *Repository) String() string {
	return fmt.Sprintf("Repository{ID: %s, FullName: %s, Stars: %d, TrendScore: %.2f}", 
		r.ID, r.FullName, r.Stars, r.TrendScore)
}