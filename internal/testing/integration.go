package testing

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mr-conflict-checker/config"
	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
	"mr-conflict-checker/reporter"
	"mr-conflict-checker/scanner"
)

// IntegrationTest provides utilities for end-to-end integration testing
type IntegrationTest struct {
	t       *testing.T
	suite   *IntegrationTestSuite
	config  *config.Config
	client  *gitlab.Client
	scanner *scanner.RepositoryScanner
}

// NewIntegrationTest creates a new integration test instance
func NewIntegrationTest(t *testing.T) *IntegrationTest {
	suite := NewIntegrationTestSuite(t)

	// Create configuration
	cfg := &config.Config{}
	cfg.GitLab.Token = "test-token"
	cfg.GitLab.URL = suite.Server.URL()

	// Create client
	client := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)

	// Create components
	repoScanner := scanner.NewRepositoryScanner(client)

	t.Cleanup(func() {
		client.Close()
		suite.Close()
	})

	return &IntegrationTest{
		t:       t,
		suite:   suite,
		config:  cfg,
		client:  client,
		scanner: repoScanner,
	}
}

// filterAndSortConflictingMRs filters merge requests for conflicts and sorts by creation date (newest first)
func (it *IntegrationTest) filterAndSortConflictingMRs(mrs []models.MergeRequest) []models.MergeRequest {
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

// TestFullWorkflow tests the complete application workflow
func (it *IntegrationTest) TestFullWorkflow() {
	// Setup test scenario
	it.suite.SetupBasicScenario()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Test connection
	err := it.client.TestConnection(ctx)
	require.NoError(it.t, err, "GitLab connection should succeed")

	// Step 2: Scan repositories
	repositories, err := it.scanner.ScanRepositories(ctx)
	require.NoError(it.t, err, "Repository scanning should succeed")
	assert.Len(it.t, repositories, 3, "Should scan all 3 repositories")

	// Step 3: Analyze merge requests
	report := &models.Report{}
	for _, repo := range repositories {
		mrs, err := it.client.ListMergeRequests(ctx, repo.ID, "release", "master")
		require.NoError(it.t, err, "MR listing should succeed for repo %d", repo.ID)

		conflictingMRs := it.filterAndSortConflictingMRs(mrs)

		var status models.RepositoryStatus
		if len(conflictingMRs) > 0 {
			status = models.StatusConflicts
		} else {
			status = models.StatusNoMRs
		}

		report.AddRepository(repo, conflictingMRs, status, "")
	}

	// Step 4: Generate report
	reportPath, err := reporter.GenerateReport(report, it.suite.Helper.GetTempDir())
	require.NoError(it.t, err, "Report generation should succeed")

	// Step 5: Validate report
	it.ValidateGeneratedReport(reportPath, report)
}

// TestErrorHandling tests error handling scenarios
func (it *IntegrationTest) TestErrorHandling() {
	// Setup error scenario
	it.suite.SetupErrorScenario()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection should still work
	err := it.client.TestConnection(ctx)
	require.NoError(it.t, err, "GitLab connection should succeed")

	// Scan repositories (should handle errors gracefully)
	repositories, err := it.scanner.ScanRepositories(ctx)
	require.NoError(it.t, err, "Repository scanning should succeed even with errors")
	assert.Len(it.t, repositories, 2, "Should scan all repositories despite errors")

	// Verify error handling
	errorFound := false
	accessibleFound := false
	for _, repo := range repositories {
		if repo.Status == models.StatusError {
			errorFound = true
			assert.NotNil(it.t, repo.Error, "Error repository should have error details")
		} else if repo.Status == models.StatusNoMRs {
			accessibleFound = true
			assert.Nil(it.t, repo.Error, "Accessible repository should not have error")
		}
	}

	assert.True(it.t, errorFound, "Should have at least one repository with error")
	assert.True(it.t, accessibleFound, "Should have at least one accessible repository")
}

// TestAuthenticationFailure tests authentication failure scenarios
func (it *IntegrationTest) TestAuthenticationFailure() {
	// Set server to return unauthorized
	it.suite.Server.SetUnauthorized(true)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test connection should fail
	err := it.client.TestConnection(ctx)
	assert.Error(it.t, err, "Connection should fail with invalid authentication")
	assert.Contains(it.t, err.Error(), "authentication failed", "Error should indicate authentication failure")
}

// TestConfigurationLoading tests configuration loading scenarios
func (it *IntegrationTest) TestConfigurationLoading() {
	// Test valid configuration
	validConfigPath := it.suite.Helper.CreateValidConfigFile("test-token", "https://gitlab.example.com")
	cfg, err := config.LoadConfig(validConfigPath)
	require.NoError(it.t, err, "Valid configuration should load successfully")
	assert.Equal(it.t, "test-token", cfg.GitLab.Token)
	assert.Equal(it.t, "https://gitlab.example.com", cfg.GitLab.URL)

	// Test invalid YAML
	invalidConfigPath := it.suite.Helper.CreateInvalidYAMLConfigFile()
	_, err = config.LoadConfig(invalidConfigPath)
	assert.Error(it.t, err, "Invalid YAML should return error")
	assert.Contains(it.t, err.Error(), "failed to parse YAML configuration")

	// Test missing token
	missingTokenPath := it.suite.Helper.CreateMissingTokenConfigFile()
	_, err = config.LoadConfig(missingTokenPath)
	assert.Error(it.t, err, "Missing token should return error")
	assert.Contains(it.t, err.Error(), "gitlab.token is required")

	// Test missing URL
	missingURLPath := it.suite.Helper.CreateMissingURLConfigFile()
	_, err = config.LoadConfig(missingURLPath)
	assert.Error(it.t, err, "Missing URL should return error")
	assert.Contains(it.t, err.Error(), "gitlab.url is required")

	// Test file not found
	_, err = config.LoadConfig("/nonexistent/config.yaml")
	assert.Error(it.t, err, "Nonexistent file should return error")
	assert.Contains(it.t, err.Error(), "configuration file not found")
}

// TestPagination tests repository pagination handling
func (it *IntegrationTest) TestPagination() {
	// Create many repositories to test pagination
	repos := it.suite.Generator.GenerateRepositories(25, 1)
	it.suite.Server.SetRepositories(repos)
	it.suite.Server.SetPerPage(10) // Force pagination

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List all repositories
	allRepos, err := it.client.ListRepositories(ctx)
	require.NoError(it.t, err, "Repository listing with pagination should succeed")
	assert.Len(it.t, allRepos, 25, "Should retrieve all repositories across pages")

	// Verify order is maintained
	for i, repo := range allRepos {
		assert.Equal(it.t, repos[i].ID, repo.ID, "Repository order should be maintained")
		assert.Equal(it.t, repos[i].Name, repo.Name, "Repository data should be correct")
	}
}

// TestMergeRequestAnalysis tests merge request analysis functionality
func (it *IntegrationTest) TestMergeRequestAnalysis() {
	// Create test data with various MR scenarios
	repos := it.suite.Generator.GenerateRepositories(1, 1)
	it.suite.Server.SetRepositories(repos)

	// Create mixed merge requests
	allMRs := []models.MergeRequest{
		// Should be included (release->master with conflicts)
		{
			ID: 1, SourceBranch: "release", TargetBranch: "master", HasConflicts: true,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
		{
			ID: 2, SourceBranch: "release", TargetBranch: "master", HasConflicts: true,
			CreatedAt: time.Now(),
		},
		// Should be excluded (different branches or no conflicts)
		{
			ID: 3, SourceBranch: "feature", TargetBranch: "master", HasConflicts: true,
			CreatedAt: time.Now(),
		},
		{
			ID: 4, SourceBranch: "release", TargetBranch: "develop", HasConflicts: true,
			CreatedAt: time.Now(),
		},
		{
			ID: 5, SourceBranch: "release", TargetBranch: "master", HasConflicts: false,
			CreatedAt: time.Now(),
		},
	}

	it.suite.Server.SetMergeRequests(1, allMRs)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get merge requests
	mrs, err := it.client.ListMergeRequests(ctx, 1, "release", "master")
	require.NoError(it.t, err, "MR listing should succeed")

	// Analyze merge requests
	conflictingMRs := it.filterAndSortConflictingMRs(mrs)

	// Verify filtering
	assert.Len(it.t, conflictingMRs, 2, "Should find exactly 2 conflicting MRs")

	// Verify sorting (newest first)
	assert.Equal(it.t, 2, conflictingMRs[0].ID, "Newest MR should be first")
	assert.Equal(it.t, 1, conflictingMRs[1].ID, "Older MR should be second")

	// Verify all results meet criteria
	for _, mr := range conflictingMRs {
		assert.Equal(it.t, "release", mr.SourceBranch, "Source branch should be release")
		assert.Equal(it.t, "master", mr.TargetBranch, "Target branch should be master")
		assert.True(it.t, mr.HasConflicts, "MR should have conflicts")
	}
}

// ValidateGeneratedReport validates the content and structure of a generated report
func (it *IntegrationTest) ValidateGeneratedReport(reportPath string, report *models.Report) {
	// Verify file exists
	assert.FileExists(it.t, reportPath, "Report file should exist")

	// Verify filename format
	filename := filepath.Base(reportPath)
	assert.Regexp(it.t, `^MR-conflict-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.md$`, filename, "Filename should match ISO 8601 pattern")

	// Read and validate content
	content, err := os.ReadFile(reportPath)
	require.NoError(it.t, err, "Should be able to read report file")

	contentStr := string(content)

	// Validate structure
	assert.Contains(it.t, contentStr, "# MR Conflict Report -", "Should have main header")
	assert.Contains(it.t, contentStr, "## Summary", "Should have summary section")
	assert.Contains(it.t, contentStr, "## Repository Details", "Should have repository details section")

	// Validate statistics
	assert.Contains(it.t, contentStr,
		"- Total Repositories Scanned:",
		"Should show total repositories")
	assert.Contains(it.t, contentStr,
		"- Repositories with Conflicts:",
		"Should show repositories with conflicts")
	assert.Contains(it.t, contentStr,
		"- Total Conflicting MRs:",
		"Should show total conflicting MRs")

	// Validate repository information
	for _, repoReport := range report.Repositories {
		assert.Contains(it.t, contentStr, repoReport.Repository.Name, "Should contain repository name")
		assert.Contains(it.t, contentStr, repoReport.Repository.WebURL, "Should contain repository URL")
		assert.Contains(it.t, contentStr, "**Status**:", "Should contain status information")

		// If repository has conflicts, validate MR information
		if len(repoReport.ConflictingMRs) > 0 {
			assert.Contains(it.t, contentStr, "#### Conflicting Merge Requests", "Should have conflicting MRs section")
			for _, mr := range repoReport.ConflictingMRs {
				assert.Contains(it.t, contentStr, mr.Title, "Should contain MR title")
				assert.Contains(it.t, contentStr, mr.Author.Name, "Should contain author name")
				assert.Contains(it.t, contentStr, mr.WebURL, "Should contain MR URL")
			}
		}
	}
}

// BenchmarkFullWorkflow benchmarks the complete application workflow
func (it *IntegrationTest) BenchmarkFullWorkflow(b *testing.B) {
	// Setup test scenario
	it.suite.SetupBasicScenario()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Run full workflow
		repositories, err := it.scanner.ScanRepositories(ctx)
		require.NoError(b, err)

		report := &models.Report{}
		for _, repo := range repositories {
			mrs, err := it.client.ListMergeRequests(ctx, repo.ID, "release", "master")
			require.NoError(b, err)

			conflictingMRs := it.filterAndSortConflictingMRs(mrs)

			var status models.RepositoryStatus
			if len(conflictingMRs) > 0 {
				status = models.StatusConflicts
			} else {
				status = models.StatusNoMRs
			}

			report.AddRepository(repo, conflictingMRs, status, "")
		}

		_, err = reporter.GenerateReport(report, it.suite.Helper.GetTempDir())
		require.NoError(b, err)

		cancel()
	}
}
