package testing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/suite"

	"mr-conflict-checker/config"
	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
	"mr-conflict-checker/reporter"
	"mr-conflict-checker/scanner"
)

// TestSuite provides a comprehensive test suite for the MR Conflict Checker
type TestSuite struct {
	suite.Suite
	helper     *TestHelper
	generator  *TestDataGenerator
	validator  *ValidationHelper
	generators *PropertyTestGenerators
	server     *MockGitLabServer
}

// filterAndSortConflictingMRs filters merge requests for conflicts and sorts by creation date (newest first)
func (s *TestSuite) filterAndSortConflictingMRs(mrs []models.MergeRequest) []models.MergeRequest {
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

// SetupSuite initializes the test suite
func (s *TestSuite) SetupSuite() {
	s.helper = NewTestHelper(s.T())
	s.generator = NewTestDataGenerator()
	s.validator = NewValidationHelper()
	s.generators = NewPropertyTestGenerators()
	s.server = NewMockGitLabServer()
}

// TearDownSuite cleans up the test suite
func (s *TestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
}

// SetupTest runs before each test
func (s *TestSuite) SetupTest() {
	// Reset server state for each test
	s.server.SetRepositories([]models.Repository{})
	s.server.SetUnauthorized(false)
	s.server.SetPerPage(100)
}

// TestConfigurationManagement tests all configuration-related functionality
func (s *TestSuite) TestConfigurationManagement() {
	s.Run("ValidConfiguration", func() {
		configPath := s.helper.CreateValidConfigFile("test-token", "https://gitlab.example.com")
		cfg, err := config.LoadConfig(configPath)
		s.NoError(err)
		s.Equal("test-token", cfg.GitLab.Token)
		s.Equal("https://gitlab.example.com", cfg.GitLab.URL)
	})

	s.Run("InvalidYAML", func() {
		configPath := s.helper.CreateInvalidYAMLConfigFile()
		_, err := config.LoadConfig(configPath)
		s.Error(err)
		s.Contains(err.Error(), "failed to parse YAML configuration")
	})

	s.Run("MissingFields", func() {
		// Test missing token
		configPath := s.helper.CreateMissingTokenConfigFile()
		_, err := config.LoadConfig(configPath)
		s.Error(err)
		s.Contains(err.Error(), "gitlab.token is required")

		// Test missing URL
		configPath = s.helper.CreateMissingURLConfigFile()
		_, err = config.LoadConfig(configPath)
		s.Error(err)
		s.Contains(err.Error(), "gitlab.url is required")
	})

	s.Run("FileNotFound", func() {
		_, err := config.LoadConfig("/nonexistent/config.yaml")
		s.Error(err)
		s.Contains(err.Error(), "configuration file not found")
	})
}

// TestGitLabClientFunctionality tests GitLab client operations
func (s *TestSuite) TestGitLabClientFunctionality() {
	s.Run("ConnectionTest", func() {
		client := gitlab.NewClient(s.server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()
		err := client.TestConnection(ctx)
		s.NoError(err)
	})

	s.Run("AuthenticationFailure", func() {
		s.server.SetUnauthorized(true)
		client := gitlab.NewClient(s.server.URL(), "invalid-token")
		defer client.Close()

		ctx := context.Background()
		err := client.TestConnection(ctx)
		s.Error(err)
		s.Contains(err.Error(), "authentication failed")
	})

	s.Run("RepositoryListing", func() {
		repos := s.generator.GenerateRepositories(5, 1)
		s.server.SetRepositories(repos)

		client := gitlab.NewClient(s.server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()
		retrievedRepos, err := client.ListRepositories(ctx)
		s.NoError(err)
		s.Len(retrievedRepos, 5)
		s.True(s.validator.ValidateRepositoryCompleteness(repos, retrievedRepos))
	})

	s.Run("PaginationHandling", func() {
		repos := s.generator.GenerateRepositories(25, 1)
		s.server.SetRepositories(repos)
		s.server.SetPerPage(10) // Force pagination

		client := gitlab.NewClient(s.server.URL(), "test-token")
		defer client.Close()

		ctx := context.Background()
		retrievedRepos, err := client.ListRepositories(ctx)
		s.NoError(err)
		s.Len(retrievedRepos, 25)
		s.True(s.validator.ValidateRepositoryCompleteness(repos, retrievedRepos))
	})
}

// TestRepositoryScanning tests repository scanning functionality
func (s *TestSuite) TestRepositoryScanning() {
	s.Run("BasicScanning", func() {
		repos := s.generator.GenerateRepositories(3, 1)
		s.server.SetRepositories(repos)

		// Set up different scenarios
		s.server.SetMergeRequests(1, []models.MergeRequest{})                       // No MRs
		s.server.SetMergeRequests(2, s.generator.GenerateMergeRequests(2, 2, true)) // Conflicts
		s.server.SetRepositoryError(3, true)                                        // Error

		client := gitlab.NewClient(s.server.URL(), "test-token")
		defer client.Close()
		scanner := scanner.NewRepositoryScanner(client)

		ctx := context.Background()
		scannedRepos, err := scanner.ScanRepositories(ctx)
		s.NoError(err)
		s.Len(scannedRepos, 3)

		// Verify statuses
		statusCounts := make(map[models.RepositoryStatus]int)
		for _, repo := range scannedRepos {
			statusCounts[repo.Status]++
		}

		s.Equal(1, statusCounts[models.StatusNoMRs])
		s.Equal(1, statusCounts[models.StatusConflicts])
		s.Equal(1, statusCounts[models.StatusError])
	})

	s.Run("ErrorHandling", func() {
		repos := s.generator.GenerateRepositories(2, 1)
		s.server.SetRepositories(repos)
		s.server.SetRepositoryError(1, true)
		s.server.SetMergeRequests(2, []models.MergeRequest{})

		client := gitlab.NewClient(s.server.URL(), "test-token")
		defer client.Close()
		scanner := scanner.NewRepositoryScanner(client)

		ctx := context.Background()
		scannedRepos, err := scanner.ScanRepositories(ctx)
		s.NoError(err) // Should not fail even with repository errors
		s.Len(scannedRepos, 2)

		// Verify error repository has error status and details
		errorRepo := scannedRepos[0] // First repo should have error
		s.Equal(models.StatusError, errorRepo.Status)
		s.NotNil(errorRepo.Error)
	})
}

// TestMergeRequestAnalysis tests merge request analysis functionality
func (s *TestSuite) TestMergeRequestAnalysis() {
	s.Run("FilteringAccuracy", func() {
		// Create mixed merge requests
		mrs := []models.MergeRequest{
			{ID: 1, SourceBranch: "release", TargetBranch: "master", HasConflicts: true},
			{ID: 2, SourceBranch: "feature", TargetBranch: "master", HasConflicts: true},
			{ID: 3, SourceBranch: "release", TargetBranch: "develop", HasConflicts: true},
			{ID: 4, SourceBranch: "release", TargetBranch: "master", HasConflicts: false},
			{ID: 5, SourceBranch: "release", TargetBranch: "master", HasConflicts: true},
		}

		filtered := s.filterAndSortConflictingMRs(mrs)
		s.Len(filtered, 2) // Only MRs 1 and 5 should pass
		s.True(s.validator.ValidateMergeRequestFiltering(mrs, filtered))
	})

	s.Run("SortingConsistency", func() {
		now := time.Now()
		mrs := []models.MergeRequest{
			{ID: 1, SourceBranch: "release", TargetBranch: "master", HasConflicts: true, CreatedAt: now.Add(-2 * time.Hour)},
			{ID: 2, SourceBranch: "release", TargetBranch: "master", HasConflicts: true, CreatedAt: now},
			{ID: 3, SourceBranch: "release", TargetBranch: "master", HasConflicts: true, CreatedAt: now.Add(-1 * time.Hour)},
		}

		sorted := s.filterAndSortConflictingMRs(mrs)
		s.Len(sorted, 3)
		s.True(s.validator.ValidateMergeRequestSorting(sorted))

		// Verify specific order (newest first)
		s.Equal(2, sorted[0].ID) // Most recent
		s.Equal(3, sorted[1].ID) // Middle
		s.Equal(1, sorted[2].ID) // Oldest
	})
}

// TestReportGeneration tests report generation functionality
func (s *TestSuite) TestReportGeneration() {
	s.Run("BasicReportGeneration", func() {
		// Create test data
		repo := models.Repository{
			ID: 1, Name: "test-repo", WebURL: "https://gitlab.example.com/test/repo",
		}
		mr := s.generator.GenerateConflictingMergeRequest(1, 1)

		report := &models.Report{}
		report.AddRepository(repo, []models.MergeRequest{mr}, models.StatusConflicts, "")

		// Generate report
		reportPath, err := reporter.GenerateReport(report, s.helper.GetTempDir())
		s.NoError(err)
		s.FileExists(reportPath)

		// Validate content
		content, err := os.ReadFile(reportPath)
		s.NoError(err)
		s.True(s.validator.ValidateReportStructure(string(content), []models.Repository{repo}, []models.MergeRequest{mr}))
	})

	s.Run("FileNamingConsistency", func() {
		report := &models.Report{}
		reportPath, err := reporter.GenerateReport(report, s.helper.GetTempDir())
		s.NoError(err)

		filename := filepath.Base(reportPath)
		s.Regexp(`^MR-conflict-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.md$`, filename)
	})

	s.Run("SummaryStatisticsAccuracy", func() {
		report := &models.Report{}

		// Add repositories with different statuses
		repo1 := models.Repository{ID: 1, Name: "repo1", WebURL: "https://gitlab.example.com/repo1"}
		repo2 := models.Repository{ID: 2, Name: "repo2", WebURL: "https://gitlab.example.com/repo2"}

		mr1 := s.generator.GenerateConflictingMergeRequest(1, 1)
		mr2 := s.generator.GenerateConflictingMergeRequest(2, 1)

		report.AddRepository(repo1, []models.MergeRequest{mr1, mr2}, models.StatusConflicts, "")
		report.AddRepository(repo2, []models.MergeRequest{}, models.StatusNoMRs, "")

		reportPath, err := reporter.GenerateReport(report, s.helper.GetTempDir())
		s.NoError(err)

		content, err := os.ReadFile(reportPath)
		s.NoError(err)
		contentStr := string(content)

		s.Contains(contentStr, "- Total Repositories Scanned: 2")
		s.Contains(contentStr, "- Repositories with Conflicts: 1")
		s.Contains(contentStr, "- Total Conflicting MRs: 2")
	})
}

// TestPropertyBasedTests runs all property-based tests
func (s *TestSuite) TestPropertyBasedTests() {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	s.Run("ConfigurationParsingCompleteness", func() {
		properties := gopter.NewProperties(parameters)

		properties.Property("valid YAML configurations should parse successfully", prop.ForAll(
			func(yamlContent string) bool {
				configPath := s.helper.CreateTempConfigFile(yamlContent)
				_, err := config.LoadConfig(configPath)
				return err == nil
			},
			s.generators.GenYAMLConfig(),
		))

		properties.TestingRun(s.T())
	})

	s.Run("RepositoryAccessCompleteness", func() {
		properties := gopter.NewProperties(parameters)

		properties.Property("all repositories should be retrieved without missing any", prop.ForAll(
			func(repoCount int) bool {
				repos := s.generator.GenerateRepositories(repoCount, 1)
				s.server.SetRepositories(repos)

				client := gitlab.NewClient(s.server.URL(), "test-token")
				defer client.Close()

				ctx := context.Background()
				retrievedRepos, err := client.ListRepositories(ctx)
				if err != nil {
					return false
				}

				return s.validator.ValidateRepositoryCompleteness(repos, retrievedRepos)
			},
			gen.IntRange(1, 50),
		))

		properties.TestingRun(s.T())
	})

	s.Run("BranchFilteringAccuracy", func() {
		properties := gopter.NewProperties(parameters)

		properties.Property("MR filtering should only include release->master conflicts", prop.ForAll(
			func(mrs []models.MergeRequest) bool {
				filtered := s.filterAndSortConflictingMRs(mrs)
				return s.validator.ValidateMergeRequestFiltering(mrs, filtered)
			},
			s.generators.GenMergeRequestSlice(),
		))

		properties.TestingRun(s.T())
	})

	s.Run("MRSortingConsistency", func() {
		properties := gopter.NewProperties(parameters)

		properties.Property("conflicting MRs should be sorted by creation date (newest first)", prop.ForAll(
			func(mrs []models.MergeRequest) bool {
				sorted := s.filterAndSortConflictingMRs(mrs)
				return s.validator.ValidateMergeRequestSorting(sorted)
			},
			s.generators.GenConflictingMergeRequestSlice(),
		))

		properties.TestingRun(s.T())
	})
}

// TestEndToEndIntegration runs comprehensive end-to-end integration tests
func (s *TestSuite) TestEndToEndIntegration() {
	s.Run("FullWorkflow", func() {
		integrationTest := NewIntegrationTest(s.T())
		integrationTest.TestFullWorkflow()
	})

	s.Run("ErrorHandling", func() {
		integrationTest := NewIntegrationTest(s.T())
		integrationTest.TestErrorHandling()
	})

	s.Run("AuthenticationFailure", func() {
		integrationTest := NewIntegrationTest(s.T())
		integrationTest.TestAuthenticationFailure()
	})

	s.Run("ConfigurationLoading", func() {
		integrationTest := NewIntegrationTest(s.T())
		integrationTest.TestConfigurationLoading()
	})

	s.Run("PaginationHandling", func() {
		integrationTest := NewIntegrationTest(s.T())
		integrationTest.TestPagination()
	})

	s.Run("MergeRequestAnalysis", func() {
		integrationTest := NewIntegrationTest(s.T())
		integrationTest.TestMergeRequestAnalysis()
	})
}

// RunAllTests runs the complete test suite
func RunAllTests(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// BenchmarkSuite runs performance benchmarks
func BenchmarkSuite(b *testing.B) {
	helper := NewTestHelper(&testing.T{})
	generator := NewTestDataGenerator()
	server := NewMockGitLabServer()
	defer server.Close()

	b.Run("RepositoryScanning", func(b *testing.B) {
		repos := generator.GenerateRepositories(100, 1)
		server.SetRepositories(repos)

		client := gitlab.NewClient(server.URL(), "test-token")
		defer client.Close()
		scanner := scanner.NewRepositoryScanner(client)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_, err := scanner.ScanRepositories(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ReportGeneration", func(b *testing.B) {
		// Create large report
		report := &models.Report{}
		for i := 0; i < 50; i++ {
			repo := models.Repository{
				ID:     i + 1,
				Name:   fmt.Sprintf("repo-%d", i+1),
				WebURL: fmt.Sprintf("https://gitlab.example.com/repo-%d", i+1),
			}
			mrs := generator.GenerateMergeRequests(5, i+1, true)
			report.AddRepository(repo, mrs, models.StatusConflicts, "")
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := reporter.GenerateReport(report, helper.GetTempDir())
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
