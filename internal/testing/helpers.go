package testing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"mr-conflict-checker/internal/models"
)

// TestHelper provides common testing utilities and helpers
type TestHelper struct {
	t       *testing.T
	tempDir string
}

// NewTestHelper creates a new test helper instance
func NewTestHelper(t *testing.T) *TestHelper {
	tempDir, err := os.MkdirTemp("", "mr-conflict-checker-test")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &TestHelper{
		t:       t,
		tempDir: tempDir,
	}
}

// GetTempDir returns the temporary directory for this test
func (h *TestHelper) GetTempDir() string {
	return h.tempDir
}

// CreateTempConfigFile creates a temporary YAML config file with the given content
func (h *TestHelper) CreateTempConfigFile(yamlContent string) string {
	configFile := filepath.Join(h.tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	require.NoError(h.t, err)
	return configFile
}

// CreateValidConfigFile creates a temporary config file with valid GitLab credentials
func (h *TestHelper) CreateValidConfigFile(token, url string) string {
	yamlContent := fmt.Sprintf(`gitlab:
  token: %s
  url: %s
`, token, url)
	return h.CreateTempConfigFile(yamlContent)
}

// CreateInvalidYAMLConfigFile creates a temporary config file with invalid YAML syntax
func (h *TestHelper) CreateInvalidYAMLConfigFile() string {
	invalidYAML := `gitlab:
  token: test-token
  url: https://gitlab.com
  invalid: [unclosed bracket
`
	return h.CreateTempConfigFile(invalidYAML)
}

// CreateMissingTokenConfigFile creates a config file missing the required token field
func (h *TestHelper) CreateMissingTokenConfigFile() string {
	yamlContent := `gitlab:
  url: https://gitlab.com
`
	return h.CreateTempConfigFile(yamlContent)
}

// CreateMissingURLConfigFile creates a config file missing the required URL field
func (h *TestHelper) CreateMissingURLConfigFile() string {
	yamlContent := `gitlab:
  token: test-token
`
	return h.CreateTempConfigFile(yamlContent)
}

// MockGitLabServer provides a configurable mock GitLab API server for testing
type MockGitLabServer struct {
	server          *httptest.Server
	repositories    []models.Repository
	mergeRequests   map[int][]models.MergeRequest // repo ID -> MRs
	errorRepos      map[int]bool                  // repo IDs that should return errors
	unauthorizedReq bool                          // whether to return 401 for all requests
	perPage         int                           // pagination size
}

// NewMockGitLabServer creates a new mock GitLab server
func NewMockGitLabServer() *MockGitLabServer {
	mock := &MockGitLabServer{
		repositories:  []models.Repository{},
		mergeRequests: make(map[int][]models.MergeRequest),
		errorRepos:    make(map[int]bool),
		perPage:       100,
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// Close shuts down the mock server
func (m *MockGitLabServer) Close() {
	m.server.Close()
}

// URL returns the mock server's URL
func (m *MockGitLabServer) URL() string {
	return m.server.URL
}

// SetRepositories sets the repositories that the mock server should return
func (m *MockGitLabServer) SetRepositories(repos []models.Repository) {
	m.repositories = repos
}

// SetMergeRequests sets the merge requests for a specific repository
func (m *MockGitLabServer) SetMergeRequests(repoID int, mrs []models.MergeRequest) {
	m.mergeRequests[repoID] = mrs
}

// SetRepositoryError marks a repository to return an error when accessed
func (m *MockGitLabServer) SetRepositoryError(repoID int, hasError bool) {
	if hasError {
		m.errorRepos[repoID] = true
	} else {
		delete(m.errorRepos, repoID)
	}
}

// SetUnauthorized makes all requests return 401 Unauthorized
func (m *MockGitLabServer) SetUnauthorized(unauthorized bool) {
	m.unauthorizedReq = unauthorized
}

// SetPerPage sets the pagination size for repository listing
func (m *MockGitLabServer) SetPerPage(perPage int) {
	m.perPage = perPage
}

// handleRequest handles HTTP requests to the mock server
func (m *MockGitLabServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Check for unauthorized requests
	if m.unauthorizedReq {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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
		m.handleRepositoryList(w, r)

	default:
		// Handle merge request endpoints
		if len(r.URL.Path) > 20 && r.URL.Path[:20] == "/api/v4/projects/" {
			m.handleMergeRequestList(w, r)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// handleRepositoryList handles repository listing with pagination
func (m *MockGitLabServer) handleRepositoryList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse pagination parameters
	page := 1
	if p := query.Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			page = parsed
		}
	}

	pageSize := m.perPage
	if ps := query.Get("per_page"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil {
			pageSize = parsed
		}
	}

	// Calculate pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(m.repositories) {
		end = len(m.repositories)
	}

	var pageRepos []models.Repository
	if start < len(m.repositories) {
		pageRepos = m.repositories[start:end]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pageRepos)
}

// handleMergeRequestList handles merge request listing for a specific repository
func (m *MockGitLabServer) handleMergeRequestList(w http.ResponseWriter, r *http.Request) {
	// Extract project ID from path
	pathParts := r.URL.Path[20:] // Remove "/api/v4/projects/"
	var projectID int
	if _, err := fmt.Sscanf(pathParts, "%d/merge_requests", &projectID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Check if this repository should return an error
	if m.errorRepos[projectID] {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "403 Forbidden",
		})
		return
	}

	// Return merge requests for this repository
	mrs, exists := m.mergeRequests[projectID]
	if !exists {
		mrs = []models.MergeRequest{} // Empty slice if no MRs defined
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mrs)
}

// TestDataGenerator provides utilities for generating test data
type TestDataGenerator struct{}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{}
}

// GenerateRepositories creates a slice of test repositories
func (g *TestDataGenerator) GenerateRepositories(count, startID int) []models.Repository {
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

// GenerateMergeRequests creates a slice of test merge requests
func (g *TestDataGenerator) GenerateMergeRequests(count int, repoID int, hasConflicts bool) []models.MergeRequest {
	mrs := make([]models.MergeRequest, count)
	baseTime := time.Now()

	for i := 0; i < count; i++ {
		mrs[i] = models.MergeRequest{
			ID:           i + 1,
			Title:        fmt.Sprintf("Test MR %d for repo %d", i+1, repoID),
			Author:       models.Author{Name: "Test User", Username: "testuser", Email: "test@example.com"},
			WebURL:       fmt.Sprintf("https://gitlab.example.com/repo-%d/-/merge_requests/%d", repoID, i+1),
			SourceBranch: "release",
			TargetBranch: "master",
			HasConflicts: hasConflicts,
			CreatedAt:    baseTime.Add(time.Duration(i) * time.Hour), // Spread creation times
		}
	}
	return mrs
}

// GenerateConflictingMergeRequest creates a single conflicting merge request
func (g *TestDataGenerator) GenerateConflictingMergeRequest(id, repoID int) models.MergeRequest {
	return models.MergeRequest{
		ID:           id,
		Title:        fmt.Sprintf("Conflicting MR %d", id),
		Author:       models.Author{Name: "Test Author", Username: "testauthor", Email: "author@example.com"},
		WebURL:       fmt.Sprintf("https://gitlab.example.com/repo-%d/-/merge_requests/%d", repoID, id),
		SourceBranch: "release",
		TargetBranch: "master",
		HasConflicts: true,
		CreatedAt:    time.Now(),
	}
}

// GenerateNonConflictingMergeRequest creates a single non-conflicting merge request
func (g *TestDataGenerator) GenerateNonConflictingMergeRequest(id, repoID int) models.MergeRequest {
	return models.MergeRequest{
		ID:           id,
		Title:        fmt.Sprintf("Non-conflicting MR %d", id),
		Author:       models.Author{Name: "Test Author", Username: "testauthor", Email: "author@example.com"},
		WebURL:       fmt.Sprintf("https://gitlab.example.com/repo-%d/-/merge_requests/%d", repoID, id),
		SourceBranch: "release",
		TargetBranch: "master",
		HasConflicts: false,
		CreatedAt:    time.Now(),
	}
}

// ValidationHelper provides utilities for validating test results
type ValidationHelper struct{}

// NewValidationHelper creates a new validation helper
func NewValidationHelper() *ValidationHelper {
	return &ValidationHelper{}
}

// ValidateRepositoryCompleteness checks that all expected repositories are present in results
func (v *ValidationHelper) ValidateRepositoryCompleteness(expected, actual []models.Repository) bool {
	if len(expected) != len(actual) {
		return false
	}

	// Create map for efficient lookup
	actualMap := make(map[int]models.Repository)
	for _, repo := range actual {
		actualMap[repo.ID] = repo
	}

	// Verify each expected repository is present with correct data
	for _, expectedRepo := range expected {
		actualRepo, exists := actualMap[expectedRepo.ID]
		if !exists {
			return false
		}

		// Verify repository data matches
		if actualRepo.Name != expectedRepo.Name || actualRepo.WebURL != expectedRepo.WebURL {
			return false
		}
	}

	return true
}

// ValidateMergeRequestFiltering checks that MR filtering works correctly
func (v *ValidationHelper) ValidateMergeRequestFiltering(input, filtered []models.MergeRequest) bool {
	// Verify that all filtered MRs meet the criteria
	for _, mr := range filtered {
		if mr.SourceBranch != "release" || mr.TargetBranch != "master" || !mr.HasConflicts {
			return false
		}
	}

	// Verify that no valid conflicting MRs were excluded
	expectedCount := 0
	for _, mr := range input {
		if mr.SourceBranch == "release" && mr.TargetBranch == "master" && mr.HasConflicts {
			expectedCount++
		}
	}

	return len(filtered) == expectedCount
}

// ValidateMergeRequestSorting checks that MRs are sorted by creation date (newest first)
func (v *ValidationHelper) ValidateMergeRequestSorting(mrs []models.MergeRequest) bool {
	for i := 1; i < len(mrs); i++ {
		if mrs[i-1].CreatedAt.Before(mrs[i].CreatedAt) {
			return false // Not sorted correctly
		}
	}
	return true
}

// ValidateReportStructure checks that a report contains required markdown elements
func (v *ValidationHelper) ValidateReportStructure(content string, expectedRepos []models.Repository, expectedMRs []models.MergeRequest) bool {
	// Check required markdown structure elements
	requiredElements := []string{
		"# MR Conflict Report -",
		"## Summary",
		"## Repository Details",
		"**Status**:",
	}

	for _, element := range requiredElements {
		if !containsString(content, element) {
			return false
		}
	}

	// Check repository links
	for _, repo := range expectedRepos {
		expectedRepoLink := "[" + repo.Name + "](" + repo.WebURL + ")"
		if !containsString(content, expectedRepoLink) {
			return false
		}
	}

	// Check MR information
	for _, mr := range expectedMRs {
		if !containsString(content, mr.Title) {
			return false
		}
		if !containsString(content, mr.Author.Name) {
			return false
		}
		expectedMRLink := "[" + mr.Title + "](" + mr.WebURL + ")"
		if !containsString(content, expectedMRLink) {
			return false
		}
	}

	return true
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr))))
}

// Simple substring search helper
func findSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// IntegrationTestSuite provides a complete testing environment for integration tests
type IntegrationTestSuite struct {
	Helper    *TestHelper
	Generator *TestDataGenerator
	Validator *ValidationHelper
	Server    *MockGitLabServer
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		Helper:    NewTestHelper(t),
		Generator: NewTestDataGenerator(),
		Validator: NewValidationHelper(),
		Server:    NewMockGitLabServer(),
	}
}

// Close cleans up the integration test suite
func (s *IntegrationTestSuite) Close() {
	s.Server.Close()
}

// SetupBasicScenario sets up a basic test scenario with repositories and merge requests
func (s *IntegrationTestSuite) SetupBasicScenario() {
	// Create test repositories
	repos := s.Generator.GenerateRepositories(3, 1)
	s.Server.SetRepositories(repos)

	// Set up different scenarios for each repository
	// Repo 1: No MRs
	s.Server.SetMergeRequests(1, []models.MergeRequest{})

	// Repo 2: Conflicting MRs
	conflictingMRs := s.Generator.GenerateMergeRequests(2, 2, true)
	s.Server.SetMergeRequests(2, conflictingMRs)

	// Repo 3: Non-conflicting MRs
	nonConflictingMRs := s.Generator.GenerateMergeRequests(1, 3, false)
	s.Server.SetMergeRequests(3, nonConflictingMRs)
}

// SetupErrorScenario sets up a test scenario with repository access errors
func (s *IntegrationTestSuite) SetupErrorScenario() {
	// Create test repositories
	repos := s.Generator.GenerateRepositories(2, 1)
	s.Server.SetRepositories(repos)

	// Set repo 1 to return an error
	s.Server.SetRepositoryError(1, true)

	// Set repo 2 to be accessible with no MRs
	s.Server.SetMergeRequests(2, []models.MergeRequest{})
}
