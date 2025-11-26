# MR Conflict Checker Testing Framework

This comprehensive testing framework provides utilities for testing the MR Conflict Checker application using both unit tests and property-based tests.

## Components

### TestHelper (`helpers.go`)
Provides utilities for creating temporary files, directories, and configuration files for testing.

**Key Features:**
- Temporary directory management with automatic cleanup
- Configuration file creation (valid, invalid, missing fields)
- File and directory utilities

**Usage:**
```go
helper := NewTestHelper(t)
configPath := helper.CreateValidConfigFile("token", "https://gitlab.com")
tempDir := helper.GetTempDir()
```

### MockGitLabServer (`helpers.go`)
A configurable HTTP server that mimics GitLab API behavior for testing.

**Key Features:**
- Repository listing with pagination
- Merge request endpoints
- Authentication simulation
- Error scenario simulation
- Configurable responses

**Usage:**
```go
server := NewMockGitLabServer()
defer server.Close()

server.SetRepositories(repos)
server.SetMergeRequests(repoID, mrs)
server.SetRepositoryError(repoID, true)
```

### TestDataGenerator (`helpers.go`)
Generates realistic test data for repositories, merge requests, and other entities.

**Key Features:**
- Repository generation with configurable IDs and counts
- Merge request generation (conflicting and non-conflicting)
- Author and metadata generation
- Realistic URLs and timestamps

**Usage:**
```go
generator := NewTestDataGenerator()
repos := generator.GenerateRepositories(5, 1)
mrs := generator.GenerateMergeRequests(3, repoID, hasConflicts)
```

### PropertyTestGenerators (`generators.go`)
Provides gopter generators for property-based testing.

**Key Features:**
- Repository generators with random properties
- Merge request generators with various branch combinations
- Configuration generators (valid and invalid YAML)
- Network error simulation generators

**Usage:**
```go
generators := NewPropertyTestGenerators()
repoGen := generators.GenRepository()
mrGen := generators.GenConflictingMergeRequest()
```

### ValidationHelper (`helpers.go`)
Utilities for validating test results and ensuring correctness.

**Key Features:**
- Repository completeness validation
- Merge request filtering validation
- Sorting consistency validation
- Report structure validation

**Usage:**
```go
validator := NewValidationHelper()
isComplete := validator.ValidateRepositoryCompleteness(expected, actual)
isFiltered := validator.ValidateMergeRequestFiltering(input, filtered)
```

### IntegrationTestSuite (`integration.go`)
Complete integration testing environment with all components wired together.

**Key Features:**
- End-to-end workflow testing
- Error handling scenario testing
- Authentication failure testing
- Configuration loading testing
- Performance benchmarking

**Usage:**
```go
integrationTest := NewIntegrationTest(t)
integrationTest.TestFullWorkflow()
integrationTest.TestErrorHandling()
```

### TestSuite (`suite.go`)
Comprehensive test suite using testify/suite for organized testing.

**Key Features:**
- Organized test structure with setup/teardown
- Property-based test integration
- Component integration testing
- Performance benchmarks

**Usage:**
```go
suite.Run(t, new(TestSuite))
```

## Property-Based Tests

The framework includes property-based tests for all major correctness properties:

1. **Configuration Parsing Completeness** - Valid YAML configurations parse successfully
2. **Repository Access Completeness** - All accessible repositories are retrieved
3. **Branch Filtering Accuracy** - Only release->master conflicts are identified
4. **Error Resilience** - System handles errors gracefully
5. **Report Inclusion Completeness** - All repositories appear in reports
6. **File Naming Consistency** - Reports follow ISO 8601 naming pattern
7. **Report Structure Consistency** - Reports contain required markdown elements
8. **Summary Statistics Accuracy** - Statistics accurately reflect scan results
9. **MR Sorting Consistency** - MRs are sorted by creation date (newest first)

## Running Tests

### All Tests
```bash
go test ./...
```

### Property-Based Tests Only
```bash
go test ./... -v | grep Property
```

### Specific Component Tests
```bash
go test ./internal/testing/ -v
go test ./config/ -v
go test ./gitlab/ -v
```

### Integration Tests
```bash
go test ./internal/testing/ -run TestIntegration -v
```

### Performance Benchmarks
```bash
go test ./internal/testing/ -bench=. -v
```

## Test Configuration

Property-based tests run with the following configuration:
- **Minimum successful tests**: 100 iterations per property
- **Test timeout**: 30 seconds for integration tests
- **Mock server**: Automatic setup and teardown
- **Temporary files**: Automatic cleanup after tests

## Adding New Tests

### Unit Tests
```go
func TestNewFeature(t *testing.T) {
    helper := NewTestHelper(t)
    // Test implementation
}
```

### Property-Based Tests
```go
func TestProperty_NewProperty(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("description", prop.ForAll(
        func(input InputType) bool {
            // Property implementation
            return result
        },
        generators.GenInputType(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

### Integration Tests
```go
func TestNewIntegration(t *testing.T) {
    integrationTest := NewIntegrationTest(t)
    // Integration test implementation
}
```

## Best Practices

1. **Use TestHelper** for all temporary file operations
2. **Mock external dependencies** using MockGitLabServer
3. **Generate realistic test data** using TestDataGenerator
4. **Validate results** using ValidationHelper
5. **Write property-based tests** for universal properties
6. **Write unit tests** for specific examples and edge cases
7. **Use integration tests** for end-to-end workflows
8. **Clean up resources** automatically with t.Cleanup()

## Dependencies

- `github.com/stretchr/testify` - Assertions and test suites
- `github.com/leanovate/gopter` - Property-based testing
- `net/http/httptest` - HTTP server mocking
- Standard Go testing package