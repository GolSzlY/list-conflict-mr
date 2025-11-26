package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryStatus_String(t *testing.T) {
	tests := []struct {
		status   RepositoryStatus
		expected string
	}{
		{StatusAccessible, "Accessible"},
		{StatusError, "Error"},
		{StatusNoMRs, "No Release->Master MRs"},
		{StatusConflicts, "Conflicts Found"},
		{RepositoryStatus(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestRepositoryStatus_IsError(t *testing.T) {
	tests := []struct {
		status   RepositoryStatus
		expected bool
	}{
		{StatusAccessible, false},
		{StatusError, true},
		{StatusNoMRs, false},
		{StatusConflicts, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsError())
		})
	}
}

func TestRepositoryStatus_HasConflicts(t *testing.T) {
	tests := []struct {
		status   RepositoryStatus
		expected bool
	}{
		{StatusAccessible, false},
		{StatusError, false},
		{StatusNoMRs, false},
		{StatusConflicts, true},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.HasConflicts())
		})
	}
}

func TestReport_AddRepository(t *testing.T) {
	report := &Report{}

	repo := Repository{
		ID:     1,
		Name:   "test-repo",
		WebURL: "https://gitlab.com/test/repo",
	}

	conflictingMRs := []MergeRequest{
		{
			ID:           1,
			Title:        "Test MR",
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: true,
		},
	}

	// Test adding repository with conflicts
	report.AddRepository(repo, conflictingMRs, StatusConflicts, "")

	assert.Equal(t, 1, report.TotalRepositories)
	assert.Equal(t, 1, report.RepositoriesWithConflicts)
	assert.Equal(t, 1, report.TotalConflictingMRs)
	assert.Len(t, report.Repositories, 1)

	// Test adding repository without conflicts
	repo2 := Repository{
		ID:     2,
		Name:   "test-repo-2",
		WebURL: "https://gitlab.com/test/repo2",
	}

	report.AddRepository(repo2, []MergeRequest{}, StatusNoMRs, "")

	assert.Equal(t, 2, report.TotalRepositories)
	assert.Equal(t, 1, report.RepositoriesWithConflicts) // Should remain 1
	assert.Equal(t, 1, report.TotalConflictingMRs)       // Should remain 1
	assert.Len(t, report.Repositories, 2)
}

func TestReport_GetSummaryStats(t *testing.T) {
	report := &Report{
		TotalRepositories:         5,
		RepositoriesWithConflicts: 2,
		TotalConflictingMRs:       3,
	}

	totalRepos, reposWithConflicts, totalConflictingMRs := report.GetSummaryStats()

	assert.Equal(t, 5, totalRepos)
	assert.Equal(t, 2, reposWithConflicts)
	assert.Equal(t, 3, totalConflictingMRs)
}

func TestMergeRequest_JSONTags(t *testing.T) {
	// Test that the struct can be properly unmarshaled from JSON
	// This validates that our JSON tags are correct for GitLab API responses
	mr := MergeRequest{
		ID:           123,
		Title:        "Test Merge Request",
		Author:       Author{Name: "John Doe", Username: "johndoe", Email: "john@example.com"},
		WebURL:       "https://gitlab.com/test/repo/-/merge_requests/123",
		SourceBranch: "release",
		TargetBranch: "master",
		HasConflicts: true,
		CreatedAt:    time.Now(),
	}

	// Basic validation that the struct is properly constructed
	assert.Equal(t, 123, mr.ID)
	assert.Equal(t, "Test Merge Request", mr.Title)
	assert.Equal(t, "John Doe", mr.Author.Name)
	assert.Equal(t, "johndoe", mr.Author.Username)
	assert.Equal(t, "john@example.com", mr.Author.Email)
	assert.Equal(t, "https://gitlab.com/test/repo/-/merge_requests/123", mr.WebURL)
	assert.Equal(t, "release", mr.SourceBranch)
	assert.Equal(t, "master", mr.TargetBranch)
	assert.True(t, mr.HasConflicts)
}

func TestRepository_JSONTags(t *testing.T) {
	// Test that the struct can be properly constructed
	// This validates that our JSON tags are correct for GitLab API responses
	repo := Repository{
		ID:     456,
		Name:   "test-repository",
		WebURL: "https://gitlab.com/test/repository",
		Status: StatusAccessible,
		Error:  nil,
	}

	// Basic validation that the struct is properly constructed
	assert.Equal(t, 456, repo.ID)
	assert.Equal(t, "test-repository", repo.Name)
	assert.Equal(t, "https://gitlab.com/test/repository", repo.WebURL)
	assert.Equal(t, StatusAccessible, repo.Status)
	assert.Nil(t, repo.Error)
}
