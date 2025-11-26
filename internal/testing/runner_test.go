package testing

import (
	"context"
	"fmt"
	"os"
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

// TestTestingFramework tests the testing framework itself
func TestTestingFramework(t *testing.T) {
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

	t.Run("PropertyTestGenerators", func(t *testing.T) {
		generators := NewPropertyTestGenerators()

		// Test repository generator
		repoGen := generators.GenRepository()
		repo, ok := repoGen.Sample()
		assert.True(t, ok)

		if repoModel, ok := repo.(models.Repository); ok {
			assert.Greater(t, repoModel.ID, 0)
			assert.NotEmpty(t, repoModel.Name)
			assert.NotEmpty(t, repoModel.WebURL)
		}

		// Test merge request generator
		mrGen := generators.GenMergeRequest()
		mr, ok := mrGen.Sample()
		assert.True(t, ok)

		if mrModel, ok := mr.(models.MergeRequest); ok {
			assert.Greater(t, mrModel.ID, 0)
			assert.NotEmpty(t, mrModel.Title)
			assert.NotEmpty(t, mrModel.Author.Name)
		}
	})
}

// TestIntegrationTestSuite demonstrates the integration test suite
func TestIntegrationTestSuite(t *testing.T) {
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

		// Test merge request listing for each repository
		for _, repo := range repos {
			mrs, err := client.ListMergeRequests(ctx, repo.ID, "release", "master")
			assert.NoError(t, err)

			// Repo 1 should have no MRs, repo 2 should have conflicts, repo 3 should have non-conflicting MRs
			switch repo.ID {
			case 1:
				assert.Len(t, mrs, 0)
			case 2:
				assert.Len(t, mrs, 2)
				for _, mr := range mrs {
					assert.True(t, mr.HasConflicts)
				}
			case 3:
				assert.Len(t, mrs, 1)
				assert.False(t, mrs[0].HasConflicts)
			}
		}
	})

	t.Run("ErrorScenario", func(t *testing.T) {
		suite.SetupErrorScenario()

		client := gitlab.NewClient(suite.Server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()

		// Repository 1 should return error, repository 2 should be accessible
		repos, err := client.ListRepositories(ctx)
		assert.NoError(t, err)
		assert.Len(t, repos, 2)

		// Test MR listing - repo 1 should error, repo 2 should succeed
		_, err = client.ListMergeRequests(ctx, 1, "release", "master")
		assert.Error(t, err) // Should fail due to server error setup

		mrs, err := client.ListMergeRequests(ctx, 2, "release", "master")
		assert.NoError(t, err)
		assert.Len(t, mrs, 0) // No MRs configured for repo 2
	})
}

// TestFullApplicationWorkflow demonstrates testing the complete application workflow
func TestFullApplicationWorkflow(t *testing.T) {
	integrationTest := NewIntegrationTest(t)

	t.Run("SuccessfulWorkflow", func(t *testing.T) {
		integrationTest.TestFullWorkflow()
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		integrationTest.TestErrorHandling()
	})

	t.Run("AuthenticationFailure", func(t *testing.T) {
		integrationTest.TestAuthenticationFailure()
	})
}

// TestComponentIntegration tests integration between different components
func TestComponentIntegration(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()
	server := NewMockGitLabServer()
	defer server.Close()

	// Setup test data
	repos := generator.GenerateRepositories(2, 1)
	server.SetRepositories(repos)

	// Repo 1: conflicting MRs
	conflictingMRs := generator.GenerateMergeRequests(3, 1, true)
	server.SetMergeRequests(1, conflictingMRs)

	// Repo 2: no MRs
	server.SetMergeRequests(2, []models.MergeRequest{})

	// Create components
	client := gitlab.NewClient(server.URL(), "test-token")
	defer client.Close()

	repoScanner := scanner.NewRepositoryScanner(client)

	ctx := context.Background()

	// Step 1: Scan repositories
	scannedRepos, err := repoScanner.ScanRepositories(ctx)
	require.NoError(t, err)
	assert.Len(t, scannedRepos, 2)

	// Step 2: Analyze merge requests and build report
	report := &models.Report{}

	for _, repo := range scannedRepos {
		mrs, err := client.ListMergeRequests(ctx, repo.ID, "release", "master")
		require.NoError(t, err)

		// Filter conflicting MRs manually
		var conflictingMRs []models.MergeRequest
		for _, mr := range mrs {
			if mr.SourceBranch == "release" && mr.TargetBranch == "master" && mr.HasConflicts {
				conflictingMRs = append(conflictingMRs, mr)
			}
		}

		var status models.RepositoryStatus
		if len(conflictingMRs) > 0 {
			status = models.StatusConflicts
		} else {
			status = models.StatusNoMRs
		}

		report.AddRepository(repo, conflictingMRs, status, "")
	}

	// Step 3: Generate report
	reportPath, err := reporter.GenerateReport(report, helper.GetTempDir())
	require.NoError(t, err)
	assert.FileExists(t, reportPath)

	// Step 4: Validate report content
	content, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	contentStr := string(content)

	// Verify report structure and content
	assert.Contains(t, contentStr, "# MR Conflict Report -")
	assert.Contains(t, contentStr, "## Summary")
	assert.Contains(t, contentStr, "- Total Repositories Scanned: 2")
	assert.Contains(t, contentStr, "- Repositories with Conflicts: 1")
	assert.Contains(t, contentStr, "- Total Conflicting MRs: 3")

	// Verify repository details
	for _, repo := range repos {
		assert.Contains(t, contentStr, repo.Name)
		assert.Contains(t, contentStr, repo.WebURL)
	}

	// Verify conflicting MR details for repo 1
	for _, mr := range conflictingMRs {
		assert.Contains(t, contentStr, mr.Title)
		assert.Contains(t, contentStr, mr.Author.Name)
		assert.Contains(t, contentStr, mr.WebURL)
	}
}

// TestPerformance runs performance tests using the testing framework
func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	generator := NewTestDataGenerator()
	server := NewMockGitLabServer()
	defer server.Close()

	t.Run("LargeRepositorySet", func(t *testing.T) {
		// Create 100 repositories
		repos := generator.GenerateRepositories(100, 1)
		server.SetRepositories(repos)

		// Add MRs to some repositories
		for i := 0; i < 50; i++ {
			mrs := generator.GenerateMergeRequests(5, i+1, i%2 == 0) // Alternate between conflicting and non-conflicting
			server.SetMergeRequests(i+1, mrs)
		}

		client := gitlab.NewClient(server.URL(), "test-token")
		defer client.Close()

		start := time.Now()

		ctx := context.Background()
		repos, err := client.ListRepositories(ctx)
		require.NoError(t, err)

		elapsed := time.Since(start)
		t.Logf("Listed %d repositories in %v", len(repos), elapsed)

		// Should complete within reasonable time
		assert.Less(t, elapsed, 5*time.Second, "Repository listing should complete quickly")
	})

	t.Run("LargeReportGeneration", func(t *testing.T) {
		helper := NewTestHelper(t)

		// Create large report with many repositories and MRs
		report := &models.Report{}

		for i := 0; i < 100; i++ {
			repo := models.Repository{
				ID:     i + 1,
				Name:   fmt.Sprintf("large-repo-%d", i+1),
				WebURL: fmt.Sprintf("https://gitlab.example.com/large-repo-%d", i+1),
			}

			mrs := generator.GenerateMergeRequests(10, i+1, true)
			report.AddRepository(repo, mrs, models.StatusConflicts, "")
		}

		start := time.Now()
		reportPath, err := reporter.GenerateReport(report, helper.GetTempDir())
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.FileExists(t, reportPath)

		t.Logf("Generated report with 100 repositories and 1000 MRs in %v", elapsed)

		// Verify file size is reasonable
		stat, err := os.Stat(reportPath)
		require.NoError(t, err)
		t.Logf("Report file size: %d bytes", stat.Size())

		// Should complete within reasonable time
		assert.Less(t, elapsed, 2*time.Second, "Report generation should complete quickly")
	})
}

// ExampleTestingFrameworkUsage demonstrates how to use the testing framework
func ExampleTestingFrameworkUsage() {
	// This example shows how to use the testing framework components

	// Create a test helper (normally done in test function with *testing.T)
	// helper := NewTestHelper(t)

	// Create test data generator
	generator := NewTestDataGenerator()

	// Generate test repositories
	repos := generator.GenerateRepositories(3, 1)
	fmt.Printf("Generated %d repositories\n", len(repos))

	// Generate test merge requests
	mrs := generator.GenerateMergeRequests(2, 1, true)
	fmt.Printf("Generated %d merge requests\n", len(mrs))

	// Create mock server
	server := NewMockGitLabServer()
	server.SetRepositories(repos)
	server.SetMergeRequests(1, mrs)
	defer server.Close()

	fmt.Printf("Mock server running at: %s\n", server.URL())

	// Create validation helper
	validator := NewValidationHelper()

	// Validate repository completeness
	isComplete := validator.ValidateRepositoryCompleteness(repos, repos)
	fmt.Printf("Repository completeness validation: %t\n", isComplete)

	// Output:
	// Generated 3 repositories
	// Generated 2 merge requests
	// Mock server running at: http://127.0.0.1:xxxxx
	// Repository completeness validation: true
}

// BenchmarkTestingFramework benchmarks the testing framework components
func BenchmarkTestingFramework(b *testing.B) {
	generator := NewTestDataGenerator()

	b.Run("RepositoryGeneration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = generator.GenerateRepositories(100, 1)
		}
	})

	b.Run("MergeRequestGeneration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = generator.GenerateMergeRequests(50, 1, true)
		}
	})

	b.Run("MockServerSetup", func(b *testing.B) {
		repos := generator.GenerateRepositories(10, 1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			server := NewMockGitLabServer()
			server.SetRepositories(repos)
			server.Close()
		}
	})
}
