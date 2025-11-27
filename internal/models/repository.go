package models

import "time"

// RepositoryStatus represents the status of a repository during scanning
type RepositoryStatus int

const (
	StatusAccessible RepositoryStatus = iota
	StatusError
	StatusNoMRs
	StatusConflicts
)

// String returns the string representation of RepositoryStatus
func (rs RepositoryStatus) String() string {
	switch rs {
	case StatusAccessible:
		return "Accessible"
	case StatusError:
		return "Error"
	case StatusNoMRs:
		return "No Release->Master MRs"
	case StatusConflicts:
		return "Conflicts Found"
	default:
		return "Unknown"
	}
}

// IsError returns true if the status indicates an error condition
func (rs RepositoryStatus) IsError() bool {
	return rs == StatusError
}

// HasConflicts returns true if the status indicates conflicts were found
func (rs RepositoryStatus) HasConflicts() bool {
	return rs == StatusConflicts
}

// Namespace represents the GitLab namespace (group) information
type Namespace struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Repository represents a GitLab repository
type Repository struct {
	ID        int              `json:"id"`
	Name      string           `json:"name"`
	WebURL    string           `json:"web_url"`
	Namespace Namespace        `json:"namespace"`
	Status    RepositoryStatus `json:"-"`
	Error     error            `json:"-"`
}

// Author represents the author of a merge request
type Author struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// MergeRequest represents a GitLab merge request
type MergeRequest struct {
	ID           int       `json:"iid"`
	Title        string    `json:"title"`
	Author       Author    `json:"author"`
	WebURL       string    `json:"web_url"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	HasConflicts bool      `json:"has_conflicts"`
	CreatedAt    time.Time `json:"created_at"`
	MergeStatus  string    `json:"merge_status"`
	ChangesCount string    `json:"changes_count"`
}
