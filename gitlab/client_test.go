package gitlab

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

	"mr-conflict-checker/internal/models"
)

// **Feature: mr-conflict-checker, Property 2: Repository Access Completeness**
// **Validates: Requirements 1.2**
func TestProperty_RepositoryAccessCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("system should retrieve all repositories accessible to token without missing any", prop.ForAll(
		func(numRepos int, repoIDStart int) bool {
			// Generate test repositories
			expectedRepos := generateTestRepositories(numRepos, repoIDStart)

			// Create mock server that serves repositories with pagination
			server := createMockGitLabServer(expectedRepos, 10) // 10 repos per page
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "test-token")
			defer client.Close()

			// Test repository retrieval
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			repos, err := client.ListRepositories(ctx)
			if err != nil {
				return false
			}

			// Verify completeness: all expected repositories should be retrieved
			if len(repos) != len(expectedRepos) {
				return false
			}

			// Create map for efficient lookup
			repoMap := make(map[int]models.Repository)
			for _, repo := range repos {
				repoMap[repo.ID] = repo
			}

			// Verify each expected repository is present with correct data
			for _, expected := range expectedRepos {
				retrieved, exists := repoMap[expected.ID]
				if !exists {
					return false
				}

				// Verify repository data matches
				if retrieved.Name != expected.Name || retrieved.WebURL != expected.WebURL {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),   // Number of repositories (1-50)
		gen.IntRange(1, 1000), // Starting repository ID
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

// createMockGitLabServer creates a test HTTP server that mimics GitLab API behavior
func createMockGitLabServer(repos []models.Repository, perPage int) *httptest.Server {
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

			// Parse pagination parameters
			page := 1
			if p := query.Get("page"); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil {
					page = parsed
				}
			}

			pageSize := perPage
			if ps := query.Get("per_page"); ps != "" {
				if parsed, err := strconv.Atoi(ps); err == nil {
					pageSize = parsed
				}
			}

			// Calculate pagination
			start := (page - 1) * pageSize
			end := start + pageSize
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
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// Unit tests for basic functionality
func TestNewClient(t *testing.T) {
	client := NewClient("https://gitlab.example.com", "test-token")
	defer client.Close()

	assert.Equal(t, "https://gitlab.example.com", client.baseURL)
	assert.Equal(t, "test-token", client.token)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.rateLimiter)
}

func TestClient_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/user" && r.Header.Get("Authorization") == "Bearer valid-token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       1,
				"username": "testuser",
			})
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "valid-token")
	defer client.Close()

	ctx := context.Background()
	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestClient_TestConnection_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "invalid-token")
	defer client.Close()

	ctx := context.Background()
	err := client.TestConnection(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestClient_ListRepositories_Success(t *testing.T) {
	expectedRepos := []models.Repository{
		{ID: 1, Name: "repo1", WebURL: "https://gitlab.example.com/user/repo1"},
		{ID: 2, Name: "repo2", WebURL: "https://gitlab.example.com/user/repo2"},
	}

	server := createMockGitLabServer(expectedRepos, 10)
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	defer client.Close()

	ctx := context.Background()
	repos, err := client.ListRepositories(ctx)

	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, expectedRepos[0].ID, repos[0].ID)
	assert.Equal(t, expectedRepos[0].Name, repos[0].Name)
	assert.Equal(t, expectedRepos[1].ID, repos[1].ID)
	assert.Equal(t, expectedRepos[1].Name, repos[1].Name)
}

func TestClient_ListRepositories_Pagination(t *testing.T) {
	// Create 25 repositories to test pagination (with page size of 10)
	expectedRepos := generateTestRepositories(25, 1)

	server := createMockGitLabServer(expectedRepos, 10)
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	defer client.Close()

	ctx := context.Background()
	repos, err := client.ListRepositories(ctx)

	require.NoError(t, err)
	assert.Len(t, repos, 25)

	// Verify all repositories are retrieved in correct order
	for i, repo := range repos {
		assert.Equal(t, expectedRepos[i].ID, repo.ID)
		assert.Equal(t, expectedRepos[i].Name, repo.Name)
	}
}

func TestClient_ListMergeRequests_Success(t *testing.T) {
	expectedMRs := []models.MergeRequest{
		{
			ID:           1,
			Title:        "Test MR 1",
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: true,
			Author:       models.Author{Name: "Test User", Username: "testuser"},
			WebURL:       "https://gitlab.example.com/project/merge_requests/1",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/api/v4/projects/1/merge_requests" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(expectedMRs)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	defer client.Close()

	ctx := context.Background()
	mrs, err := client.ListMergeRequests(ctx, 1, "release", "master")

	require.NoError(t, err)
	assert.Len(t, mrs, 1)
	assert.Equal(t, expectedMRs[0].ID, mrs[0].ID)
	assert.Equal(t, expectedMRs[0].Title, mrs[0].Title)
	assert.Equal(t, expectedMRs[0].SourceBranch, mrs[0].SourceBranch)
	assert.Equal(t, expectedMRs[0].TargetBranch, mrs[0].TargetBranch)
}

func TestClient_GetMergeRequest_Success(t *testing.T) {
	expectedMR := models.MergeRequest{
		ID:           1,
		Title:        "Test MR 1",
		SourceBranch: "release",
		TargetBranch: "master",
		HasConflicts: true,
		Author:       models.Author{Name: "Test User", Username: "testuser"},
		WebURL:       "https://gitlab.example.com/project/merge_requests/1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/api/v4/projects/1/merge_requests/1" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(expectedMR)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	defer client.Close()

	ctx := context.Background()
	mr, err := client.GetMergeRequest(ctx, 1, 1)

	require.NoError(t, err)
	assert.Equal(t, expectedMR.ID, mr.ID)
	assert.Equal(t, expectedMR.Title, mr.Title)
	assert.Equal(t, expectedMR.SourceBranch, mr.SourceBranch)
	assert.Equal(t, expectedMR.TargetBranch, mr.TargetBranch)
}

func TestClient_ListMergeRequests_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/api/v4/projects/1/merge_requests" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]models.MergeRequest{})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	defer client.Close()

	ctx := context.Background()
	mrs, err := client.ListMergeRequests(ctx, 1, "release", "master")

	require.NoError(t, err)
	assert.Len(t, mrs, 0)
}

func TestClient_RateLimiting(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/api/v4/user" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       1,
				"username": "testuser",
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	defer client.Close()

	ctx := context.Background()

	// Make multiple requests quickly to test rate limiting
	start := time.Now()
	for i := 0; i < 3; i++ {
		err := client.TestConnection(ctx)
		assert.NoError(t, err)
	}
	elapsed := time.Since(start)

	// Should take at least 200ms due to rate limiting (100ms between requests)
	assert.True(t, elapsed >= 200*time.Millisecond, "Rate limiting should enforce delays between requests")
	assert.Equal(t, 3, requestCount)
}
