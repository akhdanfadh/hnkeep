package cli

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	InputPath    string
	OutputPath   string
	Quiet        bool
	DryRun       bool
	Concurrency  int
	Tags         []string
	NoteTemplate string
	CacheDir     string
	ClearCache   bool
}

// parseFlags parses command-line flags and returns a Config struct.
func parseFlags() *Config {
	// NOTE: go flag package does not support alias natively.
	// - https://github.com/golang/go/issues/35761

	inputPath := flag.String("input", "",
		"Input file path, e.g., harmonic-export.txt (default to stdin)")
	flag.StringVar(inputPath, "i", "",
		"alias for -input (default stdin)")

	outputPath := flag.String("output", "",
		"Output file path, e.g.., karakeep-import.json (default stdout)")
	flag.StringVar(outputPath, "o", "",
		"alias for -output (default stdout)")

	quiet := flag.Bool("quiet", false,
		"Suppress informational messages (warnings and errors are always shown)")
	flag.BoolVar(quiet, "q", false,
		"alias for -quiet")

	dryRun := flag.Bool("dry-run", false,
		"Preview conversion without API calls")

	concurrency := flag.Int("concurrency", 5,
		"Number of concurrent Hacker News fetches.")
	flag.IntVar(concurrency, "c", 5,
		"alias for -concurrency")

	tags := flag.String("tags", "src:hackernews",
		"Comma-separated list of tags to add to all imported bookmarks")
	flag.StringVar(tags, "t", "src:hackernews",
		"alias for -tags")

	noteTemplate := flag.String("note-template", "{{smart_url}}",
		"Template for note field in bookmarks (empty = no note). "+
			"Variables: {{smart_url}}, {{item_url}}, {{hn_url}}, "+
			"{{id}}, {{title}}, {{author}}, {{date}}")

	defaultCacheDir := getDefaultCacheDir()
	cacheDir := flag.String("cache-dir", defaultCacheDir,
		"HN API responses cache directory path")
	noCache := flag.Bool("no-cache", false,
		"Disable caching of HN API responses")
	clearCache := flag.Bool("clear-cache", false,
		"Clear the cache before running")

	flag.Parse()

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

	return &Config{
		InputPath:    *inputPath,
		OutputPath:   *outputPath,
		Quiet:        *quiet,
		DryRun:       *dryRun,
		Concurrency:  *concurrency,
		Tags:         tagsSlice,
		NoteTemplate: *noteTemplate,
		CacheDir:     resolvedCacheDir,
		ClearCache:   *clearCache,
	}
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
