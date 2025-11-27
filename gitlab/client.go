package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mr-conflict-checker/internal/models"
)

// Client represents a GitLab API client
type Client struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	rateLimiter *time.Ticker
}

// NewClient creates a new GitLab API client with authentication and rate limiting
func NewClient(baseURL, token string) *Client {
	// Remove trailing slash from baseURL if present
	baseURL = strings.TrimSuffix(baseURL, "/")

	// If baseURL already contains /api/v4, remove it since we'll add it in endpoints
	if strings.HasSuffix(baseURL, "/api/v4") {
		baseURL = strings.TrimSuffix(baseURL, "/api/v4")
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		// Rate limit to 10 requests per second to be conservative with GitLab API
		rateLimiter: time.NewTicker(100 * time.Millisecond),
	}
}

// Close cleans up the client resources
func (c *Client) Close() {
	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}
}

// makeRequest performs an authenticated HTTP request with rate limiting
func (c *Client) makeRequest(ctx context.Context, method, endpoint string) (*http.Response, error) {
	// Wait for rate limiter
	select {
	case <-c.rateLimiter.C:
		// Continue with request
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Construct full URL
	fullURL := c.baseURL + endpoint

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	// Check for rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Check for other client/server errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// ListRepositories retrieves all repositories accessible to the authenticated user with pagination
func (c *Client) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	var allRepos []models.Repository
	page := 1
	perPage := 100 // GitLab API default max per page

	for {
		// Construct endpoint with pagination parameters
		endpoint := fmt.Sprintf("/api/v4/projects?membership=true&page=%d&per_page=%d", page, perPage)

		resp, err := c.makeRequest(ctx, "GET", endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories (page %d): %w", page, err)
		}

		// Parse response body
		var repos []models.Repository
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode repositories response: %w", err)
		}
		resp.Body.Close()

		// Add to results
		allRepos = append(allRepos, repos...)

		// Check if we've reached the end (no more pages)
		if len(repos) < perPage {
			break
		}

		page++
	}

	return allRepos, nil
}

// ListMergeRequests retrieves merge requests for a specific repository with filtering
func (c *Client) ListMergeRequests(ctx context.Context, projectID int, sourceBranch, targetBranch string) ([]models.MergeRequest, error) {
	var allMRs []models.MergeRequest
	page := 1
	perPage := 100

	for {
		// Construct endpoint with filtering parameters
		params := url.Values{}
		params.Set("state", "opened")
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(perPage))

		if sourceBranch != "" {
			params.Set("source_branch", sourceBranch)
		}
		if targetBranch != "" {
			params.Set("target_branch", targetBranch)
		}

		endpoint := fmt.Sprintf("/api/v4/projects/%d/merge_requests?%s", projectID, params.Encode())

		resp, err := c.makeRequest(ctx, "GET", endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to list merge requests for project %d (page %d): %w", projectID, page, err)
		}

		// Parse response body
		var mrs []models.MergeRequest
		if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode merge requests response: %w", err)
		}
		resp.Body.Close()

		// Add to results
		allMRs = append(allMRs, mrs...)

		// Check if we've reached the end (no more pages)
		if len(mrs) < perPage {
			break
		}

		page++
	}

	return allMRs, nil
}

// GetMergeRequest retrieves a specific merge request by ID
func (c *Client) GetMergeRequest(ctx context.Context, projectID, mrID int) (*models.MergeRequest, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%d/merge_requests/%d", projectID, mrID)

	resp, err := c.makeRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request %d for project %d: %w", mrID, projectID, err)
	}
	defer resp.Body.Close()

	var mr models.MergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, fmt.Errorf("failed to decode merge request response: %w", err)
	}

	return &mr, nil
}

// GetMergeRequestChanges retrieves the changes/diff information for a merge request
func (c *Client) GetMergeRequestChanges(ctx context.Context, projectID, mrID int) (int, error) {
	endpoint := fmt.Sprintf("/api/v4/projects/%d/merge_requests/%d/changes", projectID, mrID)

	resp, err := c.makeRequest(ctx, "GET", endpoint)
	if err != nil {
		return 0, fmt.Errorf("failed to get merge request changes %d for project %d: %w", mrID, projectID, err)
	}
	defer resp.Body.Close()

	var changes struct {
		Changes []struct {
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			Diff        string `json:"diff"`
		} `json:"changes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&changes); err != nil {
		return 0, fmt.Errorf("failed to decode merge request changes response: %w", err)
	}

	// Count actual file changes (not just empty diffs)
	actualChanges := 0
	for _, change := range changes.Changes {
		// Count as a real change if it's a new file, renamed, deleted, or has actual diff content
		if change.NewFile || change.RenamedFile || change.DeletedFile || (change.Diff != "" && len(strings.TrimSpace(change.Diff)) > 0) {
			actualChanges++
		}
	}

	return actualChanges, nil
}

// TestConnection verifies that the client can authenticate with GitLab
func (c *Client) TestConnection(ctx context.Context) error {
	endpoint := "/api/v4/user"

	resp, err := c.makeRequest(ctx, "GET", endpoint)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
