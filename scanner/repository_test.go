package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// **Feature: mr-conflict-checker, Property 5: Report Inclusion Completeness**
// **Validates: Requirements 1.6**
func TestProperty_ReportInclusionCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("all repositories should be included in report regardless of status", prop.ForAll(
		func(numRepos int, errorRepoIndices []int, noMRRepoIndices []int, conflictRepoIndices []int) bool {
			// Generate test repositories
			repos := generateTestRepositories(numRepos, 1)

			// Create mock server that simulates different repository states
			server := createMockServerWithVariousStates(repos, errorRepoIndices, noMRRepoIndices, conflictRepoIndices)
			defer server.Close()

			// Create client and scanner
			client := gitlab.NewClient(server.URL, "test-token")
			defer client.Close()
			scanner := NewRepositoryScanner(client)

			// Scan repositories
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			scannedRepos, err := scanner.ScanRepositories(ctx)
			if err != nil {
				return false
			}

			// Verify completeness: all repositories should be included regardless of their status
			if len(scannedRepos) != len(repos) {
				return false
			}

			// Create map for efficient lookup
			scannedMap := make(map[int]models.Repository)
			for _, repo := range scannedRepos {
				scannedMap[repo.ID] = repo
			}

			// Verify each original repository is present in the scanned results
			for _, originalRepo := range repos {
				scannedRepo, exists := scannedMap[originalRepo.ID]
				if !exists {
					return false
				}

				// Verify basic repository data is preserved
				if scannedRepo.Name != originalRepo.Name || scannedRepo.WebURL != originalRepo.WebURL {
					return false
				}

				// Verify that repository has a valid status (not uninitialized)
				if scannedRepo.Status < models.StatusAccessible || scannedRepo.Status > models.StatusConflicts {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20),                  // Number of repositories (1-20)
		gen.SliceOfN(5, gen.IntRange(0, 19)), // Indices of repos that should have errors
		gen.SliceOfN(5, gen.IntRange(0, 19)), // Indices of repos with no MRs
		gen.SliceOfN(5, gen.IntRange(0, 19)), // Indices of repos with conflicts
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// generateTestRepositories creates a slice of test repositories
func generateTestRepositories(count, startID int) []models.Repository {
	repos := make([]models.Repository, count)
	for i := 0; i < count; i++ {
		repos[i] = models.Repository{
			ID:     startID + i,
			Name:   fmt.Sprintf("test-repo-%d", startID+i),
			WebURL: fmt.Sprintf("https://gitlab.example.com/user/test-repo-%d", startID+i),
		}
	}
	return repos
}

// createMockServerWithVariousStates creates a test server that simulates different repository states
func createMockServerWithVariousStates(repos []models.Repository, errorIndices, noMRIndices, conflictIndices []int) *httptest.Server {
	// Create sets for quick lookup
	errorSet := make(map[int]bool)
	for _, idx := range errorIndices {
		if idx < len(repos) {
			errorSet[repos[idx].ID] = true
		}
	}

	noMRSet := make(map[int]bool)
	for _, idx := range noMRIndices {
		if idx < len(repos) {
			noMRSet[repos[idx].ID] = true
		}
	}

	conflictSet := make(map[int]bool)
	for _, idx := range conflictIndices {
		if idx < len(repos) {
			conflictSet[repos[idx].ID] = true
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Handle different endpoints
		switch {
		case r.URL.Path == "/api/v4/user":
			// Test connection endpoint
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       1,
				"username": "testuser",
			})

		case r.URL.Path == "/api/v4/projects":
			// List repositories with pagination
			query := r.URL.Query()
			page := 1
			if p := query.Get("page"); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil {
					page = parsed
				}
			}

			perPage := 100
			if ps := query.Get("per_page"); ps != "" {
				if parsed, err := strconv.Atoi(ps); err == nil {
					perPage = parsed
				}
			}

			// Calculate pagination
			start := (page - 1) * perPage
			end := start + perPage
			if end > len(repos) {
				end = len(repos)
			}

			var pageRepos []models.Repository
			if start < len(repos) {
				pageRepos = repos[start:end]
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(pageRepos)

		default:
			// Handle merge request endpoints
			if len(r.URL.Path) > 20 && r.URL.Path[:20] == "/api/v4/projects/" {
				// Extract project ID from path
				pathParts := r.URL.Path[20:] // Remove "/api/v4/projects/"
				var projectID int
				if _, err := fmt.Sscanf(pathParts, "%d/merge_requests", &projectID); err == nil {
					// Check if this repository should return an error
					if errorSet[projectID] {
						w.WriteHeader(http.StatusForbidden)
						json.NewEncoder(w).Encode(map[string]string{
							"message": "403 Forbidden",
						})
						return
					}

					// Check if this repository should have no MRs
					if noMRSet[projectID] {
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode([]models.MergeRequest{})
						return
					}

					// Check if this repository should have conflicting MRs
					if conflictSet[projectID] {
						mrs := []models.MergeRequest{
							{
								ID:           1,
								Title:        fmt.Sprintf("Conflicting MR for repo %d", projectID),
								SourceBranch: "release",
								TargetBranch: "master",
								HasConflicts: true,
								Author:       models.Author{Name: "Test User", Username: "testuser"},
								WebURL:       fmt.Sprintf("https://gitlab.example.com/project/%d/merge_requests/1", projectID),
								CreatedAt:    time.Now(),
							},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(mrs)
						return
					}

					// Default: repository with non-conflicting MRs
					mrs := []models.MergeRequest{
						{
							ID:           1,
							Title:        fmt.Sprintf("Non-conflicting MR for repo %d", projectID),
							SourceBranch: "release",
							TargetBranch: "master",
							HasConflicts: false,
							Author:       models.Author{Name: "Test User", Username: "testuser"},
							WebURL:       fmt.Sprintf("https://gitlab.example.com/project/%d/merge_requests/1", projectID),
							CreatedAt:    time.Now(),
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(mrs)
					return
				}
			}

			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// Unit tests for basic functionality
func TestNewRepositoryScanner(t *testing.T) {
	client := gitlab.NewClient("https://gitlab.example.com", "test-token")
	defer client.Close()

	scanner := NewRepositoryScanner(client)
	assert.NotNil(t, scanner)
	assert.Equal(t, client, scanner.client)
}

func TestRepositoryScanner_ScanRepositories_Success(t *testing.T) {
	repos := []models.Repository{
		{ID: 1, Name: "repo1", WebURL: "https://gitlab.example.com/user/repo1"},
		{ID: 2, Name: "repo2", WebURL: "https://gitlab.example.com/user/repo2"},
	}

	// Create mock server with no errors, no MRs for repo1, conflicts for repo2
	server := createMockServerWithVariousStates(repos, []int{}, []int{0}, []int{1})
	defer server.Close()

	client := gitlab.NewClient(server.URL, "test-token")
	defer client.Close()
	scanner := NewRepositoryScanner(client)

	ctx := context.Background()
	scannedRepos, err := scanner.ScanRepositories(ctx)

	require.NoError(t, err)
	assert.Len(t, scannedRepos, 2)

	// Verify repo1 has no MRs status
	assert.Equal(t, models.StatusNoMRs, scannedRepos[0].Status)
	assert.Nil(t, scannedRepos[0].Error)

	// Verify repo2 has conflicts status
	assert.Equal(t, models.StatusConflicts, scannedRepos[1].Status)
	assert.Nil(t, scannedRepos[1].Error)
}

func TestRepositoryScanner_ScanRepositories_WithErrors(t *testing.T) {
	repos := []models.Repository{
		{ID: 1, Name: "repo1", WebURL: "https://gitlab.example.com/user/repo1"},
		{ID: 2, Name: "repo2", WebURL: "https://gitlab.example.com/user/repo2"},
	}

	// Create mock server with error for repo1, accessible repo2
	server := createMockServerWithVariousStates(repos, []int{0}, []int{1}, []int{})
	defer server.Close()

	client := gitlab.NewClient(server.URL, "test-token")
	defer client.Close()
	scanner := NewRepositoryScanner(client)

	ctx := context.Background()
	scannedRepos, err := scanner.ScanRepositories(ctx)

	require.NoError(t, err)
	assert.Len(t, scannedRepos, 2)

	// Verify repo1 has error status
	assert.Equal(t, models.StatusError, scannedRepos[0].Status)
	assert.NotNil(t, scannedRepos[0].Error)

	// Verify repo2 has no MRs status
	assert.Equal(t, models.StatusNoMRs, scannedRepos[1].Status)
	assert.Nil(t, scannedRepos[1].Error)
}

func TestRepositoryScanner_GetRepositoryCount(t *testing.T) {
	repos := generateTestRepositories(5, 1)

	server := createMockServerWithVariousStates(repos, []int{}, []int{}, []int{})
	defer server.Close()

	client := gitlab.NewClient(server.URL, "test-token")
	defer client.Close()
	scanner := NewRepositoryScanner(client)

	ctx := context.Background()
	count, err := scanner.GetRepositoryCount(ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, count)
}
