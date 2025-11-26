package scanner

import (
	"context"
	"fmt"
	"log"

	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
)

// RepositoryScanner handles scanning repositories for merge request conflicts
type RepositoryScanner struct {
	client        *gitlab.Client
	includeGroups []int
}

// NewRepositoryScanner creates a new repository scanner with the provided GitLab client
func NewRepositoryScanner(client *gitlab.Client, includeGroups []int) *RepositoryScanner {
	return &RepositoryScanner{
		client:        client,
		includeGroups: includeGroups,
	}
}

// ScanRepositories retrieves all accessible repositories and determines their status
// It handles API errors gracefully and continues processing other repositories
func (rs *RepositoryScanner) ScanRepositories(ctx context.Context) ([]models.Repository, error) {
	// Get all repositories from GitLab API
	repos, err := rs.client.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve repositories: %w", err)
	}

	// Filter out repositories from ignored groups
	filteredRepos := rs.filterRepositories(repos)

	// Process each repository to determine its status
	processedRepos := make([]models.Repository, len(filteredRepos))
	for i, repo := range filteredRepos {
		processedRepos[i] = rs.processRepository(ctx, repo)
	}

	return processedRepos, nil
}

// processRepository determines the status of a single repository
func (rs *RepositoryScanner) processRepository(ctx context.Context, repo models.Repository) models.Repository {
	// Initialize repository with accessible status
	repo.Status = models.StatusAccessible
	repo.Error = nil

	// Try to get merge requests for this repository
	mrs, err := rs.client.ListMergeRequests(ctx, repo.ID, "release", "master")
	if err != nil {
		// Log the error but continue processing
		log.Printf("Error accessing repository %s (ID: %d): %v", repo.Name, repo.ID, err)
		repo.Status = models.StatusError
		repo.Error = err
		return repo
	}

	// Check if there are any release->master MRs
	if len(mrs) == 0 {
		repo.Status = models.StatusNoMRs
		return repo
	}

	// Check if any MRs have conflicts
	hasConflicts := false
	for _, mr := range mrs {
		if mr.HasConflicts {
			hasConflicts = true
			break
		}
	}

	if hasConflicts {
		repo.Status = models.StatusConflicts
	} else {
		repo.Status = models.StatusNoMRs // Has MRs but no conflicts
	}

	return repo
}

// filterRepositories keeps only repositories from included groups (whitelist)
func (rs *RepositoryScanner) filterRepositories(repos []models.Repository) []models.Repository {
	// If no include groups specified, include all repositories
	if len(rs.includeGroups) == 0 {
		return repos
	}

	// Create a map for faster lookup
	includeMap := make(map[int]bool)
	for _, groupID := range rs.includeGroups {
		includeMap[groupID] = true
	}

	// Filter repositories - only keep those in included groups
	var filtered []models.Repository
	for _, repo := range repos {
		if includeMap[repo.Namespace.ID] {
			filtered = append(filtered, repo)
		} else {
			log.Printf("Excluding repository %s from group %s (ID: %d) - not in whitelist", repo.Name, repo.Namespace.Name, repo.Namespace.ID)
		}
	}

	return filtered
}

// GetRepositoryCount returns the total number of repositories that would be scanned
func (rs *RepositoryScanner) GetRepositoryCount(ctx context.Context) (int, error) {
	repos, err := rs.client.ListRepositories(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count repositories: %w", err)
	}

	// Apply filtering to get accurate count
	filteredRepos := rs.filterRepositories(repos)
	return len(filteredRepos), nil
}
