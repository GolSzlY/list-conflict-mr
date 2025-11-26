package testing

import (
	"fmt"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"

	"mr-conflict-checker/internal/models"
)

// PropertyTestGenerators provides gopter generators for property-based testing
type PropertyTestGenerators struct{}

// NewPropertyTestGenerators creates a new property test generators instance
func NewPropertyTestGenerators() *PropertyTestGenerators {
	return &PropertyTestGenerators{}
}

// GenRepository generates random Repository instances
func (g *PropertyTestGenerators) GenRepository() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 10000),         // ID
		g.GenAlphaNumericString(5, 30), // Name
		g.GenGitLabURL(),               // WebURL
		g.GenRepositoryStatus(),        // Status
	).Map(func(values []interface{}) models.Repository {
		id := values[0].(int)
		name := values[1].(string)
		webURL := values[2].(string)
		status := values[3].(models.RepositoryStatus)

		return models.Repository{
			ID:     id,
			Name:   name,
			WebURL: webURL,
			Status: status,
			Error:  nil, // Keep error nil for most tests
		}
	})
}

// GenRepositorySlice generates slices of repositories
func (g *PropertyTestGenerators) GenRepositorySlice() gopter.Gen {
	return gen.SliceOfN(50, g.GenRepository()) // Max 50 repositories
}

// GenMergeRequest generates random MergeRequest instances
func (g *PropertyTestGenerators) GenMergeRequest() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 10000),           // ID
		g.GenAlphaNumericString(10, 100), // Title
		g.GenAuthor(),                    // Author
		g.GenGitLabMRURL(),               // WebURL
		g.GenBranchName(),                // SourceBranch
		g.GenBranchName(),                // TargetBranch
		gen.Bool(),                       // HasConflicts
		g.GenRecentTime(),                // CreatedAt
	).Map(func(values []interface{}) models.MergeRequest {
		return models.MergeRequest{
			ID:           values[0].(int),
			Title:        values[1].(string),
			Author:       values[2].(models.Author),
			WebURL:       values[3].(string),
			SourceBranch: values[4].(string),
			TargetBranch: values[5].(string),
			HasConflicts: values[6].(bool),
			CreatedAt:    values[7].(time.Time),
		}
	})
}

// GenConflictingMergeRequest generates MRs that should be detected as conflicts
func (g *PropertyTestGenerators) GenConflictingMergeRequest() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 10000),           // ID
		g.GenAlphaNumericString(10, 100), // Title
		g.GenAuthor(),                    // Author
		g.GenGitLabMRURL(),               // WebURL
		g.GenRecentTime(),                // CreatedAt
	).Map(func(values []interface{}) models.MergeRequest {
		return models.MergeRequest{
			ID:           values[0].(int),
			Title:        values[1].(string),
			Author:       values[2].(models.Author),
			WebURL:       values[3].(string),
			SourceBranch: "release", // Always release
			TargetBranch: "master",  // Always master
			HasConflicts: true,      // Always has conflicts
			CreatedAt:    values[4].(time.Time),
		}
	})
}

// GenMergeRequestSlice generates slices of merge requests
func (g *PropertyTestGenerators) GenMergeRequestSlice() gopter.Gen {
	return gen.SliceOfN(20, g.GenMergeRequest()) // Max 20 MRs
}

// GenConflictingMergeRequestSlice generates slices of conflicting merge requests
func (g *PropertyTestGenerators) GenConflictingMergeRequestSlice() gopter.Gen {
	return gen.SliceOfN(10, g.GenConflictingMergeRequest()) // Max 10 conflicting MRs
}

// GenAuthor generates random Author instances
func (g *PropertyTestGenerators) GenAuthor() gopter.Gen {
	return gopter.CombineGens(
		g.GenPersonName(), // Name
		g.GenUsername(),   // Username
		g.GenEmail(),      // Email
	).Map(func(values []interface{}) models.Author {
		return models.Author{
			Name:     values[0].(string),
			Username: values[1].(string),
			Email:    values[2].(string),
		}
	})
}

// GenRepositoryStatus generates random repository status values
func (g *PropertyTestGenerators) GenRepositoryStatus() gopter.Gen {
	return gen.OneConstOf(
		models.StatusAccessible,
		models.StatusError,
		models.StatusNoMRs,
		models.StatusConflicts,
	)
}

// GenBranchName generates random branch names
func (g *PropertyTestGenerators) GenBranchName() gopter.Gen {
	return gen.OneConstOf(
		"master", "main", "develop", "release", "feature", "hotfix",
		"feature/new-feature", "hotfix/bug-fix", "release/v1.0.0",
	)
}

// GenAlphaNumericString generates alphanumeric strings within a length range
func (g *PropertyTestGenerators) GenAlphaNumericString(minLen, maxLen int) gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) >= minLen && len(s) <= maxLen
	}).Map(func(s string) string {
		if len(s) == 0 {
			return "test" // Fallback for empty strings
		}
		if len(s) > maxLen {
			return s[:maxLen]
		}
		return s
	})
}

// GenPersonName generates realistic person names
func (g *PropertyTestGenerators) GenPersonName() gopter.Gen {
	firstNames := []string{
		"John", "Jane", "Alice", "Bob", "Charlie", "Diana", "Eve", "Frank",
		"Grace", "Henry", "Ivy", "Jack", "Kate", "Liam", "Mia", "Noah",
	}
	lastNames := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
		"Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez",
	}

	firstNamesInterface := make([]interface{}, len(firstNames))
	for i, name := range firstNames {
		firstNamesInterface[i] = name
	}
	lastNamesInterface := make([]interface{}, len(lastNames))
	for i, name := range lastNames {
		lastNamesInterface[i] = name
	}

	return gopter.CombineGens(
		gen.OneConstOf(firstNamesInterface...),
		gen.OneConstOf(lastNamesInterface...),
	).Map(func(values []interface{}) string {
		return fmt.Sprintf("%s %s", values[0].(string), values[1].(string))
	})
}

// GenUsername generates realistic usernames
func (g *PropertyTestGenerators) GenUsername() gopter.Gen {
	prefixes := []string{
		"user", "dev", "admin", "test", "demo", "guest", "member",
	}

	prefixesInterface := make([]interface{}, len(prefixes))
	for i, prefix := range prefixes {
		prefixesInterface[i] = prefix
	}

	return gopter.CombineGens(
		gen.OneConstOf(prefixesInterface...),
		gen.IntRange(1, 9999),
	).Map(func(values []interface{}) string {
		return fmt.Sprintf("%s%d", values[0].(string), values[1].(int))
	})
}

// GenEmail generates realistic email addresses
func (g *PropertyTestGenerators) GenEmail() gopter.Gen {
	domains := []string{
		"example.com", "test.com", "demo.org", "sample.net", "gitlab.com",
		"github.com", "company.com", "enterprise.org",
	}

	domainsInterface := make([]interface{}, len(domains))
	for i, domain := range domains {
		domainsInterface[i] = domain
	}

	return gopter.CombineGens(
		g.GenUsername(),
		gen.OneConstOf(domainsInterface...),
	).Map(func(values []interface{}) string {
		return fmt.Sprintf("%s@%s", values[0].(string), values[1].(string))
	})
}

// GenGitLabURL generates realistic GitLab repository URLs
func (g *PropertyTestGenerators) GenGitLabURL() gopter.Gen {
	return gopter.CombineGens(
		g.GenUsername(),                // User/group name
		g.GenAlphaNumericString(5, 30), // Repository name
	).Map(func(values []interface{}) string {
		return fmt.Sprintf("https://gitlab.example.com/%s/%s", values[0].(string), values[1].(string))
	})
}

// GenGitLabMRURL generates realistic GitLab merge request URLs
func (g *PropertyTestGenerators) GenGitLabMRURL() gopter.Gen {
	return gopter.CombineGens(
		g.GenUsername(),                // User/group name
		g.GenAlphaNumericString(5, 30), // Repository name
		gen.IntRange(1, 10000),         // MR ID
	).Map(func(values []interface{}) string {
		return fmt.Sprintf("https://gitlab.example.com/%s/%s/-/merge_requests/%d",
			values[0].(string), values[1].(string), values[2].(int))
	})
}

// GenRecentTime generates timestamps within the last year
func (g *PropertyTestGenerators) GenRecentTime() gopter.Gen {
	baseTime := time.Now().AddDate(-1, 0, 0) // One year ago
	return gen.Int64Range(0, 365*24*3600).Map(func(seconds int64) time.Time {
		return baseTime.Add(time.Duration(seconds) * time.Second)
	})
}

// GenYAMLConfig generates YAML configuration content
func (g *PropertyTestGenerators) GenYAMLConfig() gopter.Gen {
	return gopter.CombineGens(
		g.GenAlphaNumericString(10, 50), // Token
		g.GenGitLabBaseURL(),            // URL
	).Map(func(values []interface{}) string {
		return fmt.Sprintf(`gitlab:
  token: %s
  url: %s
`, values[0].(string), values[1].(string))
	})
}

// GenGitLabBaseURL generates GitLab base URLs
func (g *PropertyTestGenerators) GenGitLabBaseURL() gopter.Gen {
	hosts := []string{
		"gitlab.com", "gitlab.example.com", "git.company.com",
		"gitlab.enterprise.org", "source.internal.net",
	}

	hostsInterface := make([]interface{}, len(hosts))
	for i, host := range hosts {
		hostsInterface[i] = host
	}

	return gen.OneConstOf(hostsInterface...).Map(func(host string) string {
		return fmt.Sprintf("https://%s", host)
	})
}

// GenInvalidYAMLConfig generates invalid YAML configurations for error testing
func (g *PropertyTestGenerators) GenInvalidYAMLConfig() gopter.Gen {
	invalidConfigs := []string{
		"gitlab:\n  token: test\n  url: [unclosed",
		"gitlab\n  token: test\n  url: https://gitlab.com", // Missing colon
		"gitlab:\n  token: test\n  url: https://gitlab.com\n  invalid: {unclosed",
		"gitlab:\n  token: \"unclosed string\n  url: https://gitlab.com",
		"gitlab:\n  token: test\n  url: https://gitlab.com\n    invalid_indent: value",
	}

	invalidConfigsInterface := make([]interface{}, len(invalidConfigs))
	for i, config := range invalidConfigs {
		invalidConfigsInterface[i] = config
	}

	return gen.OneConstOf(invalidConfigsInterface...)
}

// GenConfigErrorType generates different types of configuration errors for testing
func (g *PropertyTestGenerators) GenConfigErrorType() gopter.Gen {
	return gen.IntRange(0, 3) // 0: file not found, 1: invalid YAML, 2: missing token, 3: missing URL
}

// GenReportData generates test data for report generation
func (g *PropertyTestGenerators) GenReportData() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(0, 50),    // Total repositories
		gen.IntRange(0, 20),    // Repositories with conflicts
		gen.IntRange(0, 100),   // Total conflicting MRs
		g.GenRepositorySlice(), // Repository list
	).Map(func(values []interface{}) map[string]interface{} {
		return map[string]interface{}{
			"totalRepos":          values[0].(int),
			"conflictRepos":       values[1].(int),
			"totalConflictingMRs": values[2].(int),
			"repositories":        values[3].([]models.Repository),
		}
	})
}

// GenPaginationParams generates pagination parameters for testing
func (g *PropertyTestGenerators) GenPaginationParams() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 10),   // Page number
		gen.IntRange(10, 100), // Per page
	).Map(func(values []interface{}) map[string]int {
		return map[string]int{
			"page":    values[0].(int),
			"perPage": values[1].(int),
		}
	})
}

// GenHTTPErrorCode generates HTTP error codes for testing error scenarios
func (g *PropertyTestGenerators) GenHTTPErrorCode() gopter.Gen {
	errorCodes := []int{400, 401, 403, 404, 429, 500, 502, 503, 504}
	errorCodesInterface := make([]interface{}, len(errorCodes))
	for i, code := range errorCodes {
		errorCodesInterface[i] = code
	}

	return gen.OneConstOf(errorCodesInterface...)
}

// GenNetworkErrorType generates different types of network errors for testing
func (g *PropertyTestGenerators) GenNetworkErrorType() gopter.Gen {
	return gen.IntRange(0, 4) // 0: timeout, 1: connection refused, 2: DNS error, 3: SSL error, 4: rate limit
}
