package cli

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	InputPath   string
	OutputPath  string
	Concurrency int
	Tags        []string
	CacheDir    string
	ClearCache  bool
}

// parseFlags parses command-line flags and returns a Config struct.
func parseFlags() *Config {
	// NOTE: go flag package does not support alias natively.
	// - https://github.com/golang/go/issues/35761

	inputPath := flag.String("input", "", "Input file path, e.g., harmonic-export.txt (default to stdin)")
	flag.StringVar(inputPath, "i", "", "alias for -input (default stdin)")

	outputPath := flag.String("output", "", "Output file path, e.g.., karakeep-import.json (default stdout)")
	flag.StringVar(outputPath, "o", "", "alias for -output (default stdout)")

	concurrency := flag.Int("concurrency", 5, "Number of concurrent Hacker News fetches.")
	flag.IntVar(concurrency, "c", 5, "alias for -concurrency")

	tags := flag.String("tags", "src:hackernews", "Comma-separated list of tags to add to all imported bookmarks")
	flag.StringVar(tags, "t", "src:hackernews", "alias for -tags")

	defaultCacheDir := getDefaultCacheDir()
	cacheDir := flag.String("cache-dir", defaultCacheDir, "HN API responses cache directory path")
	noCache := flag.Bool("no-cache", false, "Disable caching of HN API responses")
	clearCache := flag.Bool("clear-cache", false, "Clear the cache before running")

	flag.Parse()

	// resolve cache dir
	resolvedCacheDir := *cacheDir
	if *noCache {
		resolvedCacheDir = ""
	}

	return &Config{
		InputPath:   *inputPath,
		OutputPath:  *outputPath,
		Concurrency: *concurrency,
		Tags:        parseTags(*tags),
		CacheDir:    resolvedCacheDir,
		ClearCache:  *clearCache,
	}
}

// parseTags parses a comma-separated string of tags into a slice of strings.
func parseTags(tags string) []string {
	var slice []string
	if tags == "" {
		for split := range strings.SplitSeq(tags, ",") {
			if tag := strings.TrimSpace(split); tag != "" {
				slice = append(slice, tag)
			}
		}
	}
	return slice
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
