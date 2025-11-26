package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mr-conflict-checker/internal/models"
)

// GenerateReport creates a markdown report file with the given report data
func GenerateReport(report *models.Report, outputDir string) (string, error) {
	// Generate ISO 8601 timestamp for filename
	timestamp := generateTimestamp()
	filename := fmt.Sprintf("MR-conflict-%s.md", timestamp)

	// Set timestamp in report if not already set
	if report.Timestamp == "" {
		report.Timestamp = timestamp
	}

	// Create full file path
	fullPath := filepath.Join(outputDir, filename)

	// Generate markdown content
	content := generateMarkdownContent(report)

	// Check if file already exists and handle conflicts
	if _, err := os.Stat(fullPath); err == nil {
		// File exists, add suffix to make it unique
		fullPath = handleFileConflict(fullPath)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write report file: %w", err)
	}

	return fullPath, nil
}

// generateTimestamp creates an ISO 8601 formatted timestamp for file naming
func generateTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15-04-05")
}

// generateMarkdownContent creates the markdown content for the report
func generateMarkdownContent(report *models.Report) string {
	var content strings.Builder

	// Header with timestamp
	content.WriteString(fmt.Sprintf("# MR Conflict Report - %s\n\n", report.Timestamp))

	// Summary statistics
	content.WriteString("## Summary\n")
	content.WriteString(fmt.Sprintf("- Total Repositories Scanned: %d\n", report.TotalRepositories))
	content.WriteString(fmt.Sprintf("- Repositories with Conflicts: %d\n", report.RepositoriesWithConflicts))
	content.WriteString(fmt.Sprintf("- Total Conflicting MRs: %d\n\n", report.TotalConflictingMRs))

	// Repository details
	content.WriteString("## Repository Details\n\n")

	// Sort repositories by name for consistent output
	sortedRepos := make([]models.RepositoryReport, len(report.Repositories))
	copy(sortedRepos, report.Repositories)
	sort.Slice(sortedRepos, func(i, j int) bool {
		return sortedRepos[i].Repository.Name < sortedRepos[j].Repository.Name
	})

	for _, repoReport := range sortedRepos {
		content.WriteString(generateRepositorySection(repoReport))
	}

	return content.String()
}

// generateRepositorySection creates the markdown section for a single repository
func generateRepositorySection(repoReport models.RepositoryReport) string {
	var section strings.Builder

	// Repository header with link and conflict indicator
	conflictIcon := ""
	if repoReport.Status == models.StatusConflicts {
		conflictIcon = " ❌"
	}

	section.WriteString(fmt.Sprintf("### [%s](%s)%s\n", repoReport.Repository.Name, repoReport.Repository.WebURL, conflictIcon))
	section.WriteString(fmt.Sprintf("**Status**: %s\n", repoReport.Status.String()))

	// Add error message if present
	if repoReport.ErrorMessage != "" {
		section.WriteString(fmt.Sprintf("**Error**: %s\n", repoReport.ErrorMessage))
	}

	// Add conflicting MRs if any
	if len(repoReport.ConflictingMRs) > 0 {
		section.WriteString("\n#### Conflicting Merge Requests\n")

		// Sort MRs by creation date (newest first)
		sortedMRs := make([]models.MergeRequest, len(repoReport.ConflictingMRs))
		copy(sortedMRs, repoReport.ConflictingMRs)
		sort.Slice(sortedMRs, func(i, j int) bool {
			return sortedMRs[i].CreatedAt.After(sortedMRs[j].CreatedAt)
		})

		for _, mr := range sortedMRs {
			section.WriteString(fmt.Sprintf("- ❌ [%s](%s) - Author: %s - Created: %s\n",
				mr.Title,
				mr.WebURL,
				mr.Author.Name,
				mr.CreatedAt.Format("2006-01-02 15:04:05")))
		}
	}

	section.WriteString("\n")
	return section.String()
}

// handleFileConflict generates a unique filename when a file already exists
func handleFileConflict(originalPath string) string {
	dir := filepath.Dir(originalPath)
	ext := filepath.Ext(originalPath)
	nameWithoutExt := strings.TrimSuffix(filepath.Base(originalPath), ext)

	counter := 1
	for {
		newName := fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		newPath := filepath.Join(dir, newName)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}
