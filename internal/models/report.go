package models

// Report represents the summary statistics and data for the conflict report
type Report struct {
	Timestamp                 string             `json:"timestamp"`
	TotalRepositories         int                `json:"total_repositories"`
	RepositoriesWithConflicts int                `json:"repositories_with_conflicts"`
	TotalConflictingMRs       int                `json:"total_conflicting_mrs"`
	Repositories              []RepositoryReport `json:"repositories"`
}

// RepositoryReport represents a repository's data in the report
type RepositoryReport struct {
	Repository     Repository       `json:"repository"`
	ConflictingMRs []MergeRequest   `json:"conflicting_mrs"`
	Status         RepositoryStatus `json:"status"`
	ErrorMessage   string           `json:"error_message,omitempty"`
}

// AddRepository adds a repository to the report with its conflicting MRs
func (r *Report) AddRepository(repo Repository, conflictingMRs []MergeRequest, status RepositoryStatus, errorMsg string) {
	repoReport := RepositoryReport{
		Repository:     repo,
		ConflictingMRs: conflictingMRs,
		Status:         status,
		ErrorMessage:   errorMsg,
	}

	r.Repositories = append(r.Repositories, repoReport)
	r.TotalRepositories++

	if status == StatusConflicts {
		r.RepositoriesWithConflicts++
		r.TotalConflictingMRs += len(conflictingMRs)
	}
}

// GetSummaryStats returns the summary statistics for the report
func (r *Report) GetSummaryStats() (int, int, int) {
	return r.TotalRepositories, r.RepositoriesWithConflicts, r.TotalConflictingMRs
}
