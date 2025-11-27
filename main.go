package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mr-conflict-checker/analyzer"
	"mr-conflict-checker/config"
	"mr-conflict-checker/gitlab"
	"mr-conflict-checker/internal/models"
	"mr-conflict-checker/reporter"
	"mr-conflict-checker/scanner"
)

// Version information - can be set at build time using ldflags
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	var configPath string
	var verbose bool
	var debug bool
	var outputDir string
	var showVersion bool
	var showHelp bool

	// Define flags with detailed descriptions
	flag.StringVar(&configPath, "config", "/Users/panupong.j/Workspace/panupong-project/list-conflict-mr/config.yaml",
		"Path to YAML configuration file containing GitLab credentials")
	flag.StringVar(&configPath, "c", "/Users/panupong.j/Workspace/panupong-project/list-conflict-mr/config.yaml",
		"Path to YAML configuration file (shorthand)")

	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging output")
	flag.BoolVar(&verbose, "v", false, "Enable verbose logging output (shorthand)")

	flag.BoolVar(&debug, "debug", false, "Enable debug logging with detailed trace information")
	flag.BoolVar(&debug, "d", false, "Enable debug logging (shorthand)")

	flag.StringVar(&outputDir, "output", ".", "Directory where the markdown report will be generated")
	flag.StringVar(&outputDir, "o", ".", "Output directory for reports (shorthand)")

	flag.BoolVar(&showVersion, "version", false, "Show version information and exit")
	flag.BoolVar(&showHelp, "help", false, "Show detailed help information and usage examples")
	flag.BoolVar(&showHelp, "h", false, "Show help information (shorthand)")

	// Custom usage function
	flag.Usage = printUsage

	flag.Parse()

	// Handle version flag
	if showVersion {
		printVersion()
		return
	}

	// Handle help flag
	if showHelp {
		printDetailedHelp()
		return
	}

	// Set up structured logging
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	} else if verbose {
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run the main application
	if err := run(ctx, configPath, outputDir); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Application completed successfully")
}

func run(ctx context.Context, configPath, outputDir string) error {
	slog.Info("MR Conflict Checker starting", "config", configPath, "output", outputDir)

	// 1. Load configuration
	slog.Debug("Loading configuration")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	slog.Info("Configuration loaded successfully", "gitlab_url", cfg.GitLab.URL)

	// Use config output directory if command line output is default and config has output directory
	if outputDir == "." && cfg.Output.Directory != "" {
		outputDir = cfg.Output.Directory
		slog.Info("Using output directory from config", "output_dir", outputDir)
	}

	// 2. Initialize GitLab client
	slog.Debug("Initializing GitLab client")
	client := gitlab.NewClient(cfg.GitLab.URL, cfg.GitLab.Token)
	defer func() {
		slog.Debug("Cleaning up GitLab client")
		client.Close()
	}()

	// Test connection
	slog.Debug("Testing GitLab connection")
	if err := client.TestConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to GitLab: %w", err)
	}
	slog.Info("GitLab connection established successfully")

	// 3. Scan repositories
	slog.Info("Starting repository scan")
	repositoryScanner := scanner.NewRepositoryScanner(client, cfg.GitLab.IncludeGroups)

	repositories, err := repositoryScanner.ScanRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to scan repositories: %w", err)
	}
	slog.Info("Repository scan completed", "total_repositories", len(repositories))

	// Check for context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 4. Analyze merge requests
	slog.Info("Starting merge request analysis")
	analyzedRepos, err := analyzer.AnalyzeMRs(ctx, client, repositories)
	if err != nil {
		return fmt.Errorf("failed to analyze merge requests: %w", err)
	}

	// Get conflicting MRs for report generation
	conflictingMRs, err := analyzer.GetConflictingMRs(ctx, client, analyzedRepos)
	if err != nil {
		return fmt.Errorf("failed to get conflicting merge requests: %w", err)
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 5. Generate report
	slog.Info("Generating report")
	report := buildReport(analyzedRepos, conflictingMRs)

	reportPath, err := reporter.GenerateReport(report, outputDir)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Log summary statistics
	totalRepos, reposWithConflicts, totalConflicts := report.GetSummaryStats()
	slog.Info("Report generated successfully",
		"report_path", reportPath,
		"total_repositories", totalRepos,
		"repositories_with_conflicts", reposWithConflicts,
		"total_conflicting_mrs", totalConflicts)

	return nil
}

// printVersion displays version information
func printVersion() {
	fmt.Printf("MR Conflict Checker %s\n", Version)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Go Version: %s\n", "go1.23.1")
}

// printUsage displays basic usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "MR Conflict Checker - Automatically detect conflicting merge requests across GitLab repositories\n\n")
	fmt.Fprintf(os.Stderr, "OPTIONS:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nFor detailed help and examples, use: %s --help\n", os.Args[0])
}

// printDetailedHelp displays comprehensive help information with examples
func printDetailedHelp() {
	fmt.Printf("MR Conflict Checker %s\n", Version)
	fmt.Printf("=====================================\n\n")

	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("  Automatically scans GitLab repositories for conflicting merge requests\n")
	fmt.Printf("  from 'release' branch to 'master' branch and generates a detailed\n")
	fmt.Printf("  markdown report for tracking and resolution.\n\n")

	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [OPTIONS]\n\n", os.Args[0])

	fmt.Printf("OPTIONS:\n")
	flag.PrintDefaults()

	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("  # Basic usage with default config file\n")
	fmt.Printf("  %s\n\n", os.Args[0])

	fmt.Printf("  # Use custom config file\n")
	fmt.Printf("  %s --config /path/to/my-config.yaml\n\n", os.Args[0])

	fmt.Printf("  # Enable verbose logging and custom output directory\n")
	fmt.Printf("  %s --verbose --output /tmp/reports\n\n", os.Args[0])

	fmt.Printf("  # Enable debug logging for troubleshooting\n")
	fmt.Printf("  %s --debug --config ./config.yaml\n\n", os.Args[0])

	fmt.Printf("  # Using short flags\n")
	fmt.Printf("  %s -c ./config.yaml -v -o ./reports\n\n", os.Args[0])

	fmt.Printf("CONFIGURATION FILE:\n")
	fmt.Printf("  The configuration file should be in YAML format:\n\n")
	fmt.Printf("  gitlab:\n")
	fmt.Printf("    token: \"your-gitlab-access-token\"\n")
	fmt.Printf("    url: \"https://gitlab.example.com\"\n\n")

	fmt.Printf("OUTPUT:\n")
	fmt.Printf("  Generates a markdown report named 'MR-conflict-{timestamp}.md'\n")
	fmt.Printf("  containing:\n")
	fmt.Printf("  - Summary statistics of scanned repositories\n")
	fmt.Printf("  - List of repositories with conflicting merge requests\n")
	fmt.Printf("  - Direct links to each conflicting MR for easy access\n\n")

	fmt.Printf("EXIT CODES:\n")
	fmt.Printf("  0  Success\n")
	fmt.Printf("  1  Error occurred during execution\n\n")

	fmt.Printf("For more information, visit: https://github.com/your-org/mr-conflict-checker\n")
}

// buildReport constructs a Report from analyzed repositories and conflicting MRs
func buildReport(repositories []models.Repository, conflictingMRs map[int][]models.MergeRequest) *models.Report {
	report := &models.Report{
		Timestamp:    time.Now().UTC().Format("2006-01-02T15-04-05"),
		Repositories: make([]models.RepositoryReport, 0, len(repositories)),
	}

	for _, repo := range repositories {
		var errorMsg string
		if repo.Error != nil {
			errorMsg = repo.Error.Error()
		}

		// Get conflicting MRs for this repository
		mrs, exists := conflictingMRs[repo.ID]
		if !exists {
			mrs = []models.MergeRequest{}
		}

		report.AddRepository(repo, mrs, repo.Status, errorMsg)
	}

	return report
}
