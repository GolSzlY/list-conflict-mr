# MR Conflict Checker

A Golang application that automatically checks for conflicting merge requests across multiple GitLab repositories by reading configuration from a YAML file and generating comprehensive markdown reports for easy tracking and management.

## Features

- 🔍 **Automated Scanning**: Scans all accessible GitLab repositories for conflicting merge requests
- 🎯 **Targeted Analysis**: Focuses on merge requests from `release` branch to `master` branch
- 📊 **Detailed Reports**: Generates timestamped markdown reports with summary statistics
- 🔗 **Direct Links**: Provides clickable links to each conflicting merge request
- ⚡ **Rate Limiting**: Handles GitLab API rate limits gracefully
- 🛡️ **Error Resilience**: Continues processing even when individual repositories fail
- 📝 **Structured Logging**: Configurable logging levels for debugging and monitoring

## Installation

### Prerequisites

- Go 1.23.1 or later
- GitLab access token with appropriate permissions

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd mr-conflict-checker

# Install dependencies
make deps

# Build the application
make build

# Or build with version information
make build VERSION=v1.0.0
```

### Cross-Platform Builds

```bash
# Build for multiple platforms
make build-all
```

This creates binaries for:
- Linux (amd64)
- macOS (amd64, arm64)
- Windows (amd64)

## Configuration

Create a YAML configuration file with your GitLab credentials:

```yaml
# config.yaml
gitlab:
  token: "glpat-YOUR_GITLAB_ACCESS_TOKEN_HERE"
  url: "YOUR_GITLAB_URL_HERE"
  include_groups: [] # Optional: Only scan repositories from these group IDs (leave empty to scan all accessible repos)

output:
  directory: "./reports" # Optional: Default output directory for MR conflict reports
```

### Configuration Options

| Option | Description | Required | Default |
|--------|-------------|----------|---------|
| `gitlab.token` | GitLab access token (format: `glpat-xxx`) | Yes | - |
| `gitlab.url` | GitLab instance URL | Yes | - |
| `gitlab.include_groups` | Array of group IDs to scan (empty = scan all) | No | `[]` |
| `output.directory` | Default output directory for reports | No | `"."` |

### GitLab Token Requirements

Your GitLab access token needs the following scopes:
- `read_api` - To access repository and merge request information
- `read_repository` - To access repository metadata

### Group Filtering

Use the `include_groups` option to limit scanning to specific GitLab groups:

```yaml
gitlab:
  token: "glpat-YOUR_TOKEN"
  url: "https://gitlab.example.com"
  include_groups: [123, 456, 789] # Only scan repos from these group IDs
```

To find group IDs:
1. Navigate to your GitLab group
2. Check the URL or group settings for the numeric ID
3. Or use the GitLab API: `GET /groups?search=group-name`

### Output Directory Configuration

Configure where MR conflict reports are saved:

```yaml
output:
  directory: "./reports"  # Reports will be saved to ./reports/MR-conflict-*.md
```

Priority order for output directory:
1. Command line `--output` flag (highest priority)
2. Config file `output.directory` setting
3. Current directory `.` (default)

## Usage

### Basic Usage

```bash
# Use default configuration file
./mr-conflict-checker

# Specify custom configuration file
./mr-conflict-checker --config /path/to/config.yaml

# Enable verbose logging
./mr-conflict-checker --verbose

# Enable debug logging for troubleshooting
./mr-conflict-checker --debug

# Specify output directory for reports
./mr-conflict-checker --output /path/to/reports
```

### Command Line Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-c` | Path to YAML configuration file | `/Users/panupong.j/Workspace/panupong-project/list-conflict-mr/config.yaml` |
| `--verbose` | `-v` | Enable verbose logging output | `false` |
| `--debug` | `-d` | Enable debug logging with detailed trace | `false` |
| `--output` | `-o` | Directory for generated reports | `.` (current directory) |
| `--version` | | Show version information and exit | |
| `--help` | `-h` | Show detailed help and usage examples | |

### Examples

```bash
# Basic scan with default settings
./mr-conflict-checker

# Custom configuration with verbose output
./mr-conflict-checker -c ./my-config.yaml -v

# Debug mode with custom output directory
./mr-conflict-checker --debug --output ./reports

# Show version information
./mr-conflict-checker --version

# Show detailed help
./mr-conflict-checker --help
```

## Output

The application generates a markdown report named `MR-conflict-{timestamp}.md` with the following structure:

```markdown
# MR Conflict Report - 2024-01-15T10-30-45

## Summary
- Total Repositories Scanned: 25
- Repositories with Conflicts: 3
- Total Conflicting MRs: 7

## Repository Details

### [project-name](https://gitlab.example.com/group/project-name)
**Status**: Conflicts Found

#### Conflicting Merge Requests
- [Fix user authentication bug](https://gitlab.example.com/group/project-name/-/merge_requests/123) - Author: john.doe - Created: 2024-01-14T15:30:00Z
- [Update API documentation](https://gitlab.example.com/group/project-name/-/merge_requests/124) - Author: jane.smith - Created: 2024-01-15T09:15:00Z
```

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage
```

### Development Workflow

```bash
# Install dependencies
make deps

# Build and run with verbose logging
make dev

# Build and run with debug logging
make debug

# Clean build artifacts
make clean
```

### Project Structure

```
mr-conflict-checker/
├── analyzer/           # Merge request analysis logic
├── config/            # Configuration loading and validation
├── gitlab/            # GitLab API client
├── internal/
│   ├── models/        # Data structures and models
│   ├── errors/        # Error handling utilities
│   └── testing/       # Testing framework and utilities
├── reporter/          # Report generation
├── scanner/           # Repository scanning logic
├── main.go           # Application entry point
├── config.yaml       # Configuration file
├── Makefile          # Build automation
└── README.md         # This file
```

## Error Handling

The application handles various error conditions gracefully:

- **Configuration Errors**: Missing or invalid YAML files
- **Authentication Failures**: Invalid GitLab tokens
- **Network Issues**: API timeouts and connectivity problems
- **Rate Limiting**: Automatic backoff and retry logic
- **Permission Errors**: Inaccessible repositories are logged and skipped

## Logging

The application uses structured logging with configurable levels:

- **Info** (default): Basic operation information
- **Verbose** (`--verbose`): Detailed progress information
- **Debug** (`--debug`): Comprehensive trace information for troubleshooting

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | Error occurred during execution |

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## License

[Add your license information here]

## Support

For issues and questions:
- Create an issue in the repository
- Check the debug logs with `--debug` flag
- Verify your GitLab token permissions