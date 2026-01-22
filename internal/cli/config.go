package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	Version = "dev"
	Commit  = "none"
)

type Config struct {
	InputPath    string        // Input file path (default: stdin)
	OutputPath   string        // Output file path (default: stdout)
	Verbose      bool          // Show progress messages during fetch/sync
	DryRun       bool          // Preview conversion without API calls
	Before       int64         // Process only bookmarks before this timestamp (0 = all)
	After        int64         // Process only bookmarks after this timestamp (0 = all)
	Limit        int           // Process only first N bookmarks (0 = all)
	Concurrency  int           // Number of concurrent API calls
	Tags         []string      // Tags to add to all imported bookmarks
	NoteTemplate string        // Template for note field in bookmarks
	Dedupe       bool          // Merge duplicate URLs (default: true)
	CacheDir     string        // HN API responses cache directory path
	ClearCache   bool          // Clear the cache before running
	Sync         bool          // Export directly using Karakeep's API
	APIBaseURL   string        // Karakeep API URL for direct sync
	APIKey       string        // Karakeep API key for direct sync
	APITimeout   time.Duration // Karakeep API request timeout duration
}

// parseFlags parses command-line flags and returns a Config struct.
func parseFlags() (*Config, error) {
	showVersion := flag.Bool("version", false, "Show version information and exit")
	flag.BoolVar(showVersion, "v", false, "alias for -version")

	inputPath := flag.String("input", "", "Input file path, e.g., harmonic-export.txt (default to stdin)")
	flag.StringVar(inputPath, "i", "", "alias for -input (default stdin)")

	outputPath := flag.String("output", "", "Output file path, e.g., karakeep-import.json (default stdout)")
	flag.StringVar(outputPath, "o", "", "alias for -output (default stdout)")

	verbose := flag.Bool("verbose", false, "Show progress messages during fetch/sync")

	dryRun := flag.Bool("dry-run", false, "Preview conversion without API calls")

	before := flag.String("before", "", "Only include Harmonic bookmarks before this timestamp")
	after := flag.String("after", "", "Only include Harmonic bookmarks after this timestamp")
	limit := flag.Int("limit", 0, "Number of bookmarks to process (0 = all)")
	flag.IntVar(limit, "n", 0, "alias for -limit")

	concurrency := flag.Int("concurrency", 5, "Number of concurrent API calls.")
	flag.IntVar(concurrency, "c", 5, "alias for -concurrency")

	defaultTags := "src:hackernews,hnkeep:" + time.Now().Format("20060102")
	tags := flag.String("tags", defaultTags, "Comma-separated list of tags to add to all imported bookmarks")
	flag.StringVar(tags, "t", defaultTags, "alias for -tags")

	noteTemplate := flag.String("note-template", "{{smart_url}}",
		"Template for note field in bookmarks (empty = no note). "+
			"Variables: {{smart_url}}, {{item_url}}, {{hn_url}}, "+
			"{{id}}, {{title}}, {{author}}, {{date}}")
	noDedupe := flag.Bool("no-dedupe", false, "Keep duplicate URLs instead of merging them")

	defaultCacheDir := getDefaultCacheDir()
	cacheDir := flag.String("cache-dir", defaultCacheDir, "HN API responses cache directory path")
	noCache := flag.Bool("no-cache", false, "Disable caching of HN API responses")
	clearCache := flag.Bool("clear-cache", false, "Clear the cache before running")

	sync := flag.Bool("sync", false, "Enable sync mode (push to Karakeep API directly)")
	apiBaseURL := flag.String("api-url", "", "Karakeep API URL (env: KARAKEEP_API_URL)")
	apiKey := flag.String("api-key", "", "Karakeep API key (env: KARAKEEP_API_KEY)")
	apiTimeout := flag.Duration("api-timeout", 30*time.Second, "Karakeep API request timeout duration")

	flag.Parse()

	if *showVersion {
		_, _ = fmt.Fprintf(os.Stdout, "hnkeep version %s, build %s\n", Version, Commit)
		os.Exit(0)
	}

	// parse date filters
	var beforeTS, afterTS int64
	if *before != "" {
		t, err := parseDate(*before)
		if err != nil {
			return nil, fmt.Errorf("parsing -before date: %w", err)
		}
		beforeTS = t.Unix()
	}
	if *after != "" {
		t, err := parseDate(*after)
		if err != nil {
			return nil, fmt.Errorf("parsing -after date: %w", err)
		}
		afterTS = t.Unix()
	}

	// parse tags
	var tagsSlice []string
	if *tags != "" {
		for split := range strings.SplitSeq(*tags, ",") {
			if tag := strings.TrimSpace(split); tag != "" {
				tagsSlice = append(tagsSlice, tag)
			}
		}
	}

	// resolve cache dir
	resolvedCacheDir := *cacheDir
	if *noCache {
		resolvedCacheDir = ""
	}

	// handle sync env vars
	resolvedAPIBaseURL := *apiBaseURL
	if resolvedAPIBaseURL == "" {
		resolvedAPIBaseURL = os.Getenv("KARAKEEP_API_URL")
	}
	resolvedAPIKey := *apiKey
	if resolvedAPIKey == "" {
		resolvedAPIKey = os.Getenv("KARAKEEP_API_KEY")
	}
	if *sync {
		if resolvedAPIBaseURL == "" {
			return nil, fmt.Errorf("--sync requires --api-url or KARAKEEP_API_URL to be set")
		}
		if resolvedAPIKey == "" {
			return nil, fmt.Errorf("--sync requires --api-key or KARAKEEP_API_KEY to be set")
		}
	}

	return &Config{
		InputPath:    *inputPath,
		OutputPath:   *outputPath,
		Verbose:      *verbose,
		DryRun:       *dryRun,
		Before:       beforeTS,
		After:        afterTS,
		Limit:        *limit,
		Concurrency:  *concurrency,
		Tags:         tagsSlice,
		NoteTemplate: *noteTemplate,
		Dedupe:       !*noDedupe,
		CacheDir:     resolvedCacheDir,
		ClearCache:   *clearCache,
		Sync:         *sync,
		APIBaseURL:   resolvedAPIBaseURL,
		APIKey:       resolvedAPIKey,
		APITimeout:   *apiTimeout,
	}, nil
}

// getDefaultCacheDir returns the default cache directory following platform conventions.
// Returns empty string if home directory cannot be determined.
func getDefaultCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "hnkeep")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "hnkeep")
	}
	return ""
}

// parseDate attempts to parse a date string in various formats.
// Supported formats are "2006-01-02", RFC3339, and Unix timestamp (seconds since epoch).
func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		time.RFC3339,
	}

	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(ts, 0), nil
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date format: %s", s)
}
