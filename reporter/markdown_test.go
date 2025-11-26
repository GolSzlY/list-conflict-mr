package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mr-conflict-checker/internal/models"
)

// **Feature: mr-conflict-checker, Property 6: File Naming Consistency**
// **Validates: Requirements 2.1, 2.2**
func TestProperty_FileNamingConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("generated report files follow ISO 8601 naming pattern", prop.ForAll(
		func(repoCount int) bool {
			// Create a temporary directory for testing
			tempDir, err := os.MkdirTemp("", "test_reports")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			// Create a minimal report
			report := &models.Report{
				TotalRepositories:         repoCount,
				RepositoriesWithConflicts: 0,
				TotalConflictingMRs:       0,
				Repositories:              []models.RepositoryReport{},
			}

			// Generate report
			filePath, err := GenerateReport(report, tempDir)
			if err != nil {
				t.Logf("GenerateReport failed: %v", err)
				return false
			}

			// Extract filename from path
			filename := filepath.Base(filePath)

			// Check if filename matches the expected pattern: MR-conflict-{ISO-8601}.md
			// ISO 8601 format for filename: YYYY-MM-DDTHH-MM-SS
			pattern := `^MR-conflict-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.md$`
			matched, err := regexp.MatchString(pattern, filename)
			if err != nil {
				t.Logf("Regex match failed: %v", err)
				return false
			}

			if !matched {
				t.Logf("Filename '%s' does not match expected pattern", filename)
				return false
			}

			// Verify the timestamp part is valid
			timestampPart := strings.TrimPrefix(filename, "MR-conflict-")
			timestampPart = strings.TrimSuffix(timestampPart, ".md")

			// Parse the timestamp to ensure it's valid ISO 8601
			_, err = time.Parse("2006-01-02T15-04-05", timestampPart)
			if err != nil {
				t.Logf("Invalid timestamp format in filename: %s", timestampPart)
				return false
			}

			return true
		},
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

// **Feature: mr-conflict-checker, Property 7: Report Structure Consistency**
// **Validates: Requirements 2.3, 2.4, 4.1, 4.2, 4.4**
func TestProperty_ReportStructureConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("generated reports contain proper markdown structure with required elements", prop.ForAll(
		func(repoName string, mrTitle string, authorName string, hasConflicts bool) bool {
			// Skip empty strings to ensure valid test data
			if repoName == "" || mrTitle == "" || authorName == "" {
				return true
			}

			// Create a temporary directory for testing
			tempDir, err := os.MkdirTemp("", "test_reports")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			// Create test data
			repo := models.Repository{
				ID:     1,
				Name:   repoName,
				WebURL: "https://gitlab.example.com/" + repoName,
			}

			var conflictingMRs []models.MergeRequest
			var status models.RepositoryStatus = models.StatusNoMRs

			if hasConflicts {
				mr := models.MergeRequest{
					ID:           1,
					Title:        mrTitle,
					Author:       models.Author{Name: authorName, Username: "testuser", Email: "test@example.com"},
					WebURL:       "https://gitlab.example.com/" + repoName + "/-/merge_requests/1",
					SourceBranch: "release",
					TargetBranch: "master",
					HasConflicts: true,
					CreatedAt:    time.Now(),
				}
				conflictingMRs = []models.MergeRequest{mr}
				status = models.StatusConflicts
			}

			// Create report
			report := &models.Report{
				TotalRepositories:         1,
				RepositoriesWithConflicts: 0,
				TotalConflictingMRs:       0,
				Repositories:              []models.RepositoryReport{},
			}

			report.AddRepository(repo, conflictingMRs, status, "")

			// Generate report
			filePath, err := GenerateReport(report, tempDir)
			if err != nil {
				t.Logf("GenerateReport failed: %v", err)
				return false
			}

			// Read the generated file
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Logf("Failed to read generated file: %v", err)
				return false
			}

			contentStr := string(content)

			// Check required markdown structure elements
			// 1. Main header with timestamp
			if !strings.Contains(contentStr, "# MR Conflict Report -") {
				t.Logf("Missing main header")
				return false
			}

			// 2. Summary section
			if !strings.Contains(contentStr, "## Summary") {
				t.Logf("Missing Summary section")
				return false
			}

			// 3. Repository Details section
			if !strings.Contains(contentStr, "## Repository Details") {
				t.Logf("Missing Repository Details section")
				return false
			}

			// 4. Repository name as clickable link
			expectedRepoLink := "[" + repoName + "](" + repo.WebURL + ")"
			if !strings.Contains(contentStr, expectedRepoLink) {
				t.Logf("Missing repository link: %s", expectedRepoLink)
				return false
			}

			// 5. Status information
			if !strings.Contains(contentStr, "**Status**:") {
				t.Logf("Missing status information")
				return false
			}

			// 6. If there are conflicts, check MR information
			if hasConflicts {
				// Check for MR title, author, and link
				if !strings.Contains(contentStr, mrTitle) {
					t.Logf("Missing MR title: %s", mrTitle)
					return false
				}

				if !strings.Contains(contentStr, authorName) {
					t.Logf("Missing author name: %s", authorName)
					return false
				}

				expectedMRLink := "[" + mrTitle + "](" + conflictingMRs[0].WebURL + ")"
				if !strings.Contains(contentStr, expectedMRLink) {
					t.Logf("Missing MR link: %s", expectedMRLink)
					return false
				}

				// Check for "Conflicting Merge Requests" section
				if !strings.Contains(contentStr, "#### Conflicting Merge Requests") {
					t.Logf("Missing Conflicting Merge Requests section")
					return false
				}
			}

			return true
		},
		gen.AlphaString().Map(func(s string) string {
			if len(s) == 0 {
				return "test-repo"
			}
			if len(s) > 30 {
				return s[:30]
			}
			return s
		}),
		gen.AlphaString().Map(func(s string) string {
			if len(s) == 0 {
				return "test-mr-title"
			}
			if len(s) > 50 {
				return s[:50]
			}
			return s
		}),
		gen.AlphaString().Map(func(s string) string {
			if len(s) == 0 {
				return "test-author"
			}
			if len(s) > 20 {
				return s[:20]
			}
			return s
		}),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// **Feature: mr-conflict-checker, Property 8: Summary Statistics Accuracy**
// **Validates: Requirements 2.5**
func TestProperty_SummaryStatisticsAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("summary statistics accurately reflect repository and conflict counts", prop.ForAll(
		func(repoCount int, conflictRepoCount int, conflictsPerRepo int) bool {
			// Ensure valid test parameters
			if repoCount < 0 || conflictRepoCount < 0 || conflictRepoCount > repoCount || conflictsPerRepo < 0 {
				return true // Skip invalid combinations
			}

			// Create a temporary directory for testing
			tempDir, err := os.MkdirTemp("", "test_reports")
			if err != nil {
				t.Logf("Failed to create temp dir: %v", err)
				return false
			}
			defer os.RemoveAll(tempDir)

			// Create report with calculated statistics
			report := &models.Report{
				TotalRepositories:         0,
				RepositoriesWithConflicts: 0,
				TotalConflictingMRs:       0,
				Repositories:              []models.RepositoryReport{},
			}

			expectedTotalMRs := 0

			// Add repositories without conflicts
			for i := 0; i < repoCount-conflictRepoCount; i++ {
				repo := models.Repository{
					ID:     i + 1,
					Name:   fmt.Sprintf("repo-%d", i+1),
					WebURL: fmt.Sprintf("https://gitlab.example.com/repo-%d", i+1),
				}
				report.AddRepository(repo, []models.MergeRequest{}, models.StatusNoMRs, "")
			}

			// Add repositories with conflicts
			for i := 0; i < conflictRepoCount; i++ {
				repo := models.Repository{
					ID:     repoCount - conflictRepoCount + i + 1,
					Name:   fmt.Sprintf("conflict-repo-%d", i+1),
					WebURL: fmt.Sprintf("https://gitlab.example.com/conflict-repo-%d", i+1),
				}

				// Create conflicting MRs for this repository
				var conflictingMRs []models.MergeRequest
				for j := 0; j < conflictsPerRepo; j++ {
					mr := models.MergeRequest{
						ID:           j + 1,
						Title:        fmt.Sprintf("MR %d", j+1),
						Author:       models.Author{Name: "Test Author", Username: "testuser", Email: "test@example.com"},
						WebURL:       fmt.Sprintf("https://gitlab.example.com/conflict-repo-%d/-/merge_requests/%d", i+1, j+1),
						SourceBranch: "release",
						TargetBranch: "master",
						HasConflicts: true,
						CreatedAt:    time.Now(),
					}
					conflictingMRs = append(conflictingMRs, mr)
				}

				expectedTotalMRs += conflictsPerRepo
				report.AddRepository(repo, conflictingMRs, models.StatusConflicts, "")
			}

			// Generate report
			filePath, err := GenerateReport(report, tempDir)
			if err != nil {
				t.Logf("GenerateReport failed: %v", err)
				return false
			}

			// Read the generated file
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Logf("Failed to read generated file: %v", err)
				return false
			}

			contentStr := string(content)

			// Verify summary statistics in the content
			expectedTotalRepos := fmt.Sprintf("- Total Repositories Scanned: %d", repoCount)
			if !strings.Contains(contentStr, expectedTotalRepos) {
				t.Logf("Incorrect total repositories. Expected: %s", expectedTotalRepos)
				return false
			}

			expectedConflictRepos := fmt.Sprintf("- Repositories with Conflicts: %d", conflictRepoCount)
			if !strings.Contains(contentStr, expectedConflictRepos) {
				t.Logf("Incorrect repositories with conflicts. Expected: %s", expectedConflictRepos)
				return false
			}

			expectedTotalConflicts := fmt.Sprintf("- Total Conflicting MRs: %d", expectedTotalMRs)
			if !strings.Contains(contentStr, expectedTotalConflicts) {
				t.Logf("Incorrect total conflicting MRs. Expected: %s", expectedTotalConflicts)
				return false
			}

			// Verify the report object statistics match what we expect
			if report.TotalRepositories != repoCount {
				t.Logf("Report TotalRepositories mismatch: got %d, expected %d", report.TotalRepositories, repoCount)
				return false
			}

			if report.RepositoriesWithConflicts != conflictRepoCount {
				t.Logf("Report RepositoriesWithConflicts mismatch: got %d, expected %d", report.RepositoriesWithConflicts, conflictRepoCount)
				return false
			}

			if report.TotalConflictingMRs != expectedTotalMRs {
				t.Logf("Report TotalConflictingMRs mismatch: got %d, expected %d", report.TotalConflictingMRs, expectedTotalMRs)
				return false
			}

			return true
		},
		gen.IntRange(0, 20), // repoCount
		gen.IntRange(0, 20), // conflictRepoCount
		gen.IntRange(0, 5),  // conflictsPerRepo
	))

	properties.TestingRun(t)
}

// Unit tests for basic functionality
func TestGenerateTimestamp(t *testing.T) {
	timestamp := generateTimestamp()

	// Should match ISO 8601 format: YYYY-MM-DDTHH-MM-SS
	matched, err := regexp.MatchString(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}$`, timestamp)
	require.NoError(t, err)
	assert.True(t, matched, "Timestamp should match ISO 8601 format")

	// Should be parseable as time
	_, err = time.Parse("2006-01-02T15-04-05", timestamp)
	assert.NoError(t, err, "Timestamp should be valid time format")
}

func TestGenerateMarkdownContent_EmptyReport(t *testing.T) {
	report := &models.Report{
		Timestamp:                 "2024-01-01T12-00-00",
		TotalRepositories:         0,
		RepositoriesWithConflicts: 0,
		TotalConflictingMRs:       0,
		Repositories:              []models.RepositoryReport{},
	}

	content := generateMarkdownContent(report)

	assert.Contains(t, content, "# MR Conflict Report - 2024-01-01T12-00-00")
	assert.Contains(t, content, "## Summary")
	assert.Contains(t, content, "- Total Repositories Scanned: 0")
	assert.Contains(t, content, "- Repositories with Conflicts: 0")
	assert.Contains(t, content, "- Total Conflicting MRs: 0")
	assert.Contains(t, content, "## Repository Details")
}

func TestGenerateRepositorySection_WithError(t *testing.T) {
	repo := models.Repository{
		ID:     1,
		Name:   "test-repo",
		WebURL: "https://gitlab.example.com/test-repo",
	}

	repoReport := models.RepositoryReport{
		Repository:     repo,
		ConflictingMRs: []models.MergeRequest{},
		Status:         models.StatusError,
		ErrorMessage:   "Access denied",
	}

	section := generateRepositorySection(repoReport)

	assert.Contains(t, section, "### [test-repo](https://gitlab.example.com/test-repo)")
	assert.Contains(t, section, "**Status**: Error")
	assert.Contains(t, section, "**Error**: Access denied")
}

func TestGenerateRepositorySection_WithConflicts(t *testing.T) {
	repo := models.Repository{
		ID:     1,
		Name:   "test-repo",
		WebURL: "https://gitlab.example.com/test-repo",
	}

	mr := models.MergeRequest{
		ID:           1,
		Title:        "Test MR",
		Author:       models.Author{Name: "Test Author", Username: "testuser", Email: "test@example.com"},
		WebURL:       "https://gitlab.example.com/test-repo/-/merge_requests/1",
		SourceBranch: "release",
		TargetBranch: "master",
		HasConflicts: true,
		CreatedAt:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	repoReport := models.RepositoryReport{
		Repository:     repo,
		ConflictingMRs: []models.MergeRequest{mr},
		Status:         models.StatusConflicts,
		ErrorMessage:   "",
	}

	section := generateRepositorySection(repoReport)

	assert.Contains(t, section, "### [test-repo](https://gitlab.example.com/test-repo)")
	assert.Contains(t, section, "**Status**: Conflicts Found")
	assert.Contains(t, section, "#### Conflicting Merge Requests")
	assert.Contains(t, section, "[Test MR](https://gitlab.example.com/test-repo/-/merge_requests/1)")
	assert.Contains(t, section, "Author: Test Author")
	assert.Contains(t, section, "Created: 2024-01-01 12:00:00")
}

func TestHandleFileConflict(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create an existing file
	existingFile := filepath.Join(tempDir, "test-file.md")
	err := os.WriteFile(existingFile, []byte("existing content"), 0644)
	require.NoError(t, err)

	// Test conflict handling
	newPath := handleFileConflict(existingFile)

	// Should generate a new path with suffix
	assert.NotEqual(t, existingFile, newPath)
	assert.Contains(t, newPath, "test-file_1.md")

	// New path should not exist yet
	_, err = os.Stat(newPath)
	assert.True(t, os.IsNotExist(err))
}

func TestGenerateReport_DirectoryCreation(t *testing.T) {
	// Use a temporary directory that doesn't exist yet
	tempDir := filepath.Join(os.TempDir(), "test-reports-"+fmt.Sprintf("%d", time.Now().UnixNano()))

	report := &models.Report{
		TotalRepositories:         1,
		RepositoriesWithConflicts: 0,
		TotalConflictingMRs:       0,
		Repositories:              []models.RepositoryReport{},
	}

	filePath, err := GenerateReport(report, tempDir)

	require.NoError(t, err)
	assert.FileExists(t, filePath)

	// Verify directory was created
	assert.DirExists(t, tempDir)

	// Clean up
	os.RemoveAll(tempDir)
}
