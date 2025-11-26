package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
)

// **Feature: mr-conflict-checker, Property 3: Branch Filtering Accuracy**
// **Validates: Requirements 1.3, 4.3**
func TestBranchFilteringAccuracy(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("filterAndSortConflictingMRs should only include MRs with source=release, target=master, and has_conflicts=true", prop.ForAll(
		func(mrs []models.MergeRequest) bool {
			// Filter the MRs using our function
			filtered := filterAndSortConflictingMRs(mrs)

			// Verify that all filtered MRs meet the criteria
			for _, mr := range filtered {
				if mr.SourceBranch != "release" || mr.TargetBranch != "master" || !mr.HasConflicts {
					return false
				}
			}

			// Verify that no valid conflicting MRs were excluded
			expectedCount := 0
			for _, mr := range mrs {
				if mr.SourceBranch == "release" && mr.TargetBranch == "master" && mr.HasConflicts {
					expectedCount++
				}
			}

			return len(filtered) == expectedCount
		},
		genMergeRequestSlice(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: mr-conflict-checker, Property 9: MR Sorting Consistency**
// **Validates: Requirements 4.5**
func TestMRSortingConsistency(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("filterAndSortConflictingMRs should sort MRs by creation date with newest first", prop.ForAll(
		func(mrs []models.MergeRequest) bool {
			// Filter and sort the MRs
			sorted := filterAndSortConflictingMRs(mrs)

			// Verify sorting order (newest first)
			for i := 1; i < len(sorted); i++ {
				if sorted[i-1].CreatedAt.Before(sorted[i].CreatedAt) {
					return false // Not sorted correctly
				}
			}

			return true
		},
		genConflictingMergeRequestSlice(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Unit test for basic functionality
func TestFilterAndSortConflictingMRs(t *testing.T) {
	now := time.Now()

	mrs := []models.MergeRequest{
		{
			ID:           1,
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: true,
			CreatedAt:    now.Add(-1 * time.Hour), // Older
		},
		{
			ID:           2,
			SourceBranch: "feature",
			TargetBranch: "master",
			HasConflicts: true,
			CreatedAt:    now,
		},
		{
			ID:           3,
			SourceBranch: "release",
			TargetBranch: "develop",
			HasConflicts: true,
			CreatedAt:    now,
		},
		{
			ID:           4,
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: false,
			CreatedAt:    now,
		},
		{
			ID:           5,
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: true,
			CreatedAt:    now, // Newer
		},
	}

	result := filterAndSortConflictingMRs(mrs)

	// Should only include MRs 1 and 5 (release->master with conflicts)
	assert.Len(t, result, 2)

	// Should be sorted by creation date (newest first)
	assert.Equal(t, 5, result[0].ID) // Newer MR first
	assert.Equal(t, 1, result[1].ID) // Older MR second
}

func TestFilterAndSortConflictingMRs_EmptyInput(t *testing.T) {
	result := filterAndSortConflictingMRs([]models.MergeRequest{})
	assert.Len(t, result, 0)
}

func TestFilterAndSortConflictingMRs_NoConflicts(t *testing.T) {
	mrs := []models.MergeRequest{
		{
			ID:           1,
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: false,
			CreatedAt:    time.Now(),
		},
		{
			ID:           2,
			SourceBranch: "feature",
			TargetBranch: "master",
			HasConflicts: true,
			CreatedAt:    time.Now(),
		},
	}

	result := filterAndSortConflictingMRs(mrs)
	assert.Len(t, result, 0)
}

func TestAnalyzeMRs_NilClient(t *testing.T) {
	repos := []models.Repository{
		{ID: 1, Name: "test-repo", Status: models.StatusAccessible},
	}

	_, err := AnalyzeMRs(context.Background(), nil, repos)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gitlab client cannot be nil")
}

func TestAnalyzeMRs_WithErrorRepository(t *testing.T) {
	// Create a mock client (we'll use a simple mock here)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := gitlab.NewClient(server.URL, "test-token")
	defer client.Close()

	repos := []models.Repository{
		{ID: 1, Name: "test-repo", Status: models.StatusError, Error: fmt.Errorf("test error")},
		{ID: 2, Name: "test-repo-2", Status: models.StatusAccessible},
	}

	result, err := AnalyzeMRs(context.Background(), client, repos)

	require.NoError(t, err)
	assert.Len(t, result, 2)

	// First repo should maintain its error status
	assert.Equal(t, models.StatusError, result[0].Status)
	assert.NotNil(t, result[0].Error)

	// Second repo should have error status due to auth failure
	assert.Equal(t, models.StatusError, result[1].Status)
}

// Generator for merge request slices with various branch combinations
func genMergeRequestSlice() gopter.Gen {
	return gen.SliceOf(genMergeRequest())
}

// Generator for merge request slices containing only conflicting MRs
func genConflictingMergeRequestSlice() gopter.Gen {
	return gen.SliceOf(genConflictingMergeRequest())
}

// Generator for individual merge requests with random properties
func genMergeRequest() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 1000), // ID
		gen.OneConstOf("release", "feature", "hotfix", "develop"), // SourceBranch
		gen.OneConstOf("master", "main", "develop"),               // TargetBranch
		gen.Bool(), // HasConflicts
		genTime(),  // CreatedAt
	).Map(func(values []interface{}) models.MergeRequest {
		return models.MergeRequest{
			ID:           values[0].(int),
			SourceBranch: values[1].(string),
			TargetBranch: values[2].(string),
			HasConflicts: values[3].(bool),
			CreatedAt:    values[4].(time.Time),
		}
	})
}

// Generator for conflicting merge requests (release->master with conflicts)
func genConflictingMergeRequest() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 1000), // ID
		genTime(),             // CreatedAt
	).Map(func(values []interface{}) models.MergeRequest {
		return models.MergeRequest{
			ID:           values[0].(int),
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: true,
			CreatedAt:    values[1].(time.Time),
		}
	})
}

// Generator for time values within a reasonable range
func genTime() gopter.Gen {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return gen.Int64Range(0, 365*24*3600).Map(func(seconds int64) time.Time {
		return baseTime.Add(time.Duration(seconds) * time.Second)
	})
}
