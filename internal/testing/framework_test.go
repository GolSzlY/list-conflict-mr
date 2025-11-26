package testing

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mr-conflict-checker/config"
	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
	"mr-conflict-checker/reporter"
)

// TestFrameworkBasics tests the basic functionality of the testing framework
func TestFrameworkBasics(t *testing.T) {
	t.Run("TestHelper", func(t *testing.T) {
		helper := NewTestHelper(t)

		// Test temp directory creation
		tempDir := helper.GetTempDir()
		assert.DirExists(t, tempDir)

		// Test config file creation
		configPath := helper.CreateValidConfigFile("test-token", "https://gitlab.example.com")
		assert.FileExists(t, configPath)

		// Verify config content
		cfg, err := config.LoadConfig(configPath)
		require.NoError(t, err)
		assert.Equal(t, "test-token", cfg.GitLab.Token)
		assert.Equal(t, "https://gitlab.example.com", cfg.GitLab.URL)
	})

	t.Run("TestDataGenerator", func(t *testing.T) {
		generator := NewTestDataGenerator()

		// Test repository generation
		repos := generator.GenerateRepositories(5, 1)
		assert.Len(t, repos, 5)
		assert.Equal(t, 1, repos[0].ID)
		assert.Equal(t, 5, repos[4].ID)

		// Test merge request generation
		mrs := generator.GenerateMergeRequests(3, 1, true)
		assert.Len(t, mrs, 3)
		for _, mr := range mrs {
			assert.Equal(t, "release", mr.SourceBranch)
			assert.Equal(t, "master", mr.TargetBranch)
			assert.True(t, mr.HasConflicts)
		}
	})

	t.Run("MockGitLabServer", func(t *testing.T) {
		server := NewMockGitLabServer()
		defer server.Close()

		generator := NewTestDataGenerator()
		repos := generator.GenerateRepositories(3, 1)
		server.SetRepositories(repos)

		// Test client interaction
		client := gitlab.NewClient(server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()

		// Test connection
		err := client.TestConnection(ctx)
		assert.NoError(t, err)

		// Test repository listing
		retrievedRepos, err := client.ListRepositories(ctx)
		assert.NoError(t, err)
		assert.Len(t, retrievedRepos, 3)
	})

	t.Run("ValidationHelper", func(t *testing.T) {
		validator := NewValidationHelper()

		// Test repository completeness validation
		expected := []models.Repository{
			{ID: 1, Name: "repo1", WebURL: "https://gitlab.com/repo1"},
			{ID: 2, Name: "repo2", WebURL: "https://gitlab.com/repo2"},
		}
		actual := []models.Repository{
			{ID: 1, Name: "repo1", WebURL: "https://gitlab.com/repo1"},
			{ID: 2, Name: "repo2", WebURL: "https://gitlab.com/repo2"},
		}

		assert.True(t, validator.ValidateRepositoryCompleteness(expected, actual))

		// Test with missing repository
		actualIncomplete := []models.Repository{
			{ID: 1, Name: "repo1", WebURL: "https://gitlab.com/repo1"},
		}
		assert.False(t, validator.ValidateRepositoryCompleteness(expected, actualIncomplete))
	})
}

// TestIntegrationSuite tests the integration test suite
func TestIntegrationSuite(t *testing.T) {
	suite := NewIntegrationTestSuite(t)
	defer suite.Close()

	t.Run("BasicScenario", func(t *testing.T) {
		suite.SetupBasicScenario()

		// Create client and test basic functionality
		client := gitlab.NewClient(suite.Server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()

		// Test connection
		err := client.TestConnection(ctx)
		assert.NoError(t, err)

		// Test repository listing
		repos, err := client.ListRepositories(ctx)
		assert.NoError(t, err)
		assert.Len(t, repos, 3)
	})

	t.Run("ErrorScenario", func(t *testing.T) {
		suite.SetupErrorScenario()

		client := gitlab.NewClient(suite.Server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()

		// Repository listing should still work
		repos, err := client.ListRepositories(ctx)
		assert.NoError(t, err)
		assert.Len(t, repos, 2)
	})
}

// TestReportGeneration tests report generation with the testing framework
func TestReportGeneration(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()

	// Create test data
	repo := models.Repository{
		ID:     1,
		Name:   "test-repo",
		WebURL: "https://gitlab.example.com/test/repo",
	}
	mr := generator.GenerateConflictingMergeRequest(1, 1)

	report := &models.Report{}
	report.AddRepository(repo, []models.MergeRequest{mr}, models.StatusConflicts, "")

	// Generate report
	reportPath, err := reporter.GenerateReport(report, helper.GetTempDir())
	require.NoError(t, err)
	assert.FileExists(t, reportPath)

	// Validate basic content
	content, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	contentStr := string(content)

	assert.Contains(t, contentStr, "# MR Conflict Report -")
	assert.Contains(t, contentStr, "## Summary")
	assert.Contains(t, contentStr, repo.Name)
	assert.Contains(t, contentStr, mr.Title)
}
