package analyzer

import (
	"context"
	"fmt"
	"sort"

	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
)

// AnalyzeMRs analyzes repositories for conflicting merge requests from release to master branch
func AnalyzeMRs(ctx context.Context, client *gitlab.Client, repositories []models.Repository) ([]models.Repository, error) {
	if client == nil {
		return nil, fmt.Errorf("gitlab client cannot be nil")
	}

	var analyzedRepos []models.Repository

	for _, repo := range repositories {
		// Skip repositories that already have errors
		if repo.Status == models.StatusError {
			analyzedRepos = append(analyzedRepos, repo)
			continue
		}

		// Analyze this repository for conflicting MRs
		analyzedRepo, err := analyzeRepository(ctx, client, repo)
		if err != nil {
			// Set error status and continue with other repositories
			analyzedRepo = repo
			analyzedRepo.Status = models.StatusError
			analyzedRepo.Error = err
		}

		analyzedRepos = append(analyzedRepos, analyzedRepo)
	}

	return analyzedRepos, nil
}

// analyzeRepository analyzes a single repository for conflicting merge requests
func analyzeRepository(ctx context.Context, client *gitlab.Client, repo models.Repository) (models.Repository, error) {
	// Get merge requests from release to master branch
	mrs, err := client.ListMergeRequests(ctx, repo.ID, "release", "master")
	if err != nil {
		return repo, fmt.Errorf("failed to fetch merge requests for repository %s: %w", repo.Name, err)
	}

	// Filter for conflicting MRs and sort by creation date (newest first)
	conflictingMRs := filterAndSortConflictingMRs(mrs)

	// Update repository status based on findings
	updatedRepo := repo
	if len(conflictingMRs) > 0 {
		updatedRepo.Status = models.StatusConflicts
	} else if len(mrs) > 0 {
		// Has MRs but no conflicts
		updatedRepo.Status = models.StatusAccessible
	} else {
		// No release->master MRs found
		updatedRepo.Status = models.StatusNoMRs
	}

	return updatedRepo, nil
}

// filterAndSortConflictingMRs filters merge requests for conflicts and sorts by creation date (newest first)
func filterAndSortConflictingMRs(mrs []models.MergeRequest) []models.MergeRequest {
	var conflictingMRs []models.MergeRequest

	// Filter for conflicting MRs with exact branch matching
	for _, mr := range mrs {
		if mr.SourceBranch == "release" && mr.TargetBranch == "master" && mr.HasConflicts {
			conflictingMRs = append(conflictingMRs, mr)
		}
	}

	// Sort by creation date (newest first)
	sort.Slice(conflictingMRs, func(i, j int) bool {
		return conflictingMRs[i].CreatedAt.After(conflictingMRs[j].CreatedAt)
	})

	return conflictingMRs
}

// GetConflictingMRs retrieves all conflicting merge requests from analyzed repositories
func GetConflictingMRs(ctx context.Context, client *gitlab.Client, repositories []models.Repository) (map[int][]models.MergeRequest, error) {
	conflictingMRs := make(map[int][]models.MergeRequest)

	for _, repo := range repositories {
		if repo.Status == models.StatusConflicts {
			// Get merge requests for this repository
			mrs, err := client.ListMergeRequests(ctx, repo.ID, "release", "master")
			if err != nil {
				// Log error but continue processing other repositories
				continue
			}

			// Filter and sort conflicting MRs
			conflicts := filterAndSortConflictingMRs(mrs)
			if len(conflicts) > 0 {
				conflictingMRs[repo.ID] = conflicts
			}
		}
	}

	return conflictingMRs, nil
}
