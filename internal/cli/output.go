package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/harmonic"
)

// stats tracks bookmark counts at each pipeline stage and timing statistics.
type stats struct {
	found       int
	afterFilter int
	afterLimit  int
	skipped     int
	converted   int
	deduped     int
	cacheHits   int

	totalStart time.Time
	fetchStart time.Time
	fetchEnd   time.Time
}

func (s *stats) totalDuration() time.Duration {
	return time.Since(s.totalStart)
}

func (s *stats) fetchDuration() time.Duration {
	return s.fetchEnd.Sub(s.fetchStart)
}

func (s *stats) avgFetchTime() time.Duration {
	if s.afterLimit == 0 {
		return 0
	}
	return s.fetchDuration() / time.Duration(s.afterLimit)
}

// printPipelineStats prints the common pipeline statistics (found, filtered, limited)
func printPipelineStats(stats stats) {
	fmt.Fprintf(os.Stderr, "Bookmarks found : %d\n", stats.found)

	dateFiltered := stats.found - stats.afterFilter
	if dateFiltered > 0 {
		fmt.Fprintf(os.Stderr, "  Date filtered : -%d\n", dateFiltered)
	}

	limited := stats.afterFilter - stats.afterLimit
	if limited > 0 {
		fmt.Fprintf(os.Stderr, "  Limited       : -%d\n", limited)
	}
}

func printSummary(stats stats) {
	fmt.Fprintf(os.Stderr, "\n=== Summary ===\n")
	printPipelineStats(stats)

	if stats.skipped > 0 {
		fmt.Fprintf(os.Stderr, "  Fetch skipped : -%d   (deleted/dead/not found)\n", stats.skipped)
	}

	if stats.deduped > 0 {
		fmt.Fprintf(os.Stderr, "  Deduplicated  : -%d   (merged duplicate URLs)\n", stats.deduped)
	}

	fmt.Fprintf(os.Stderr, "Converted       : %d\n", stats.converted)

	if stats.cacheHits > 0 || stats.afterLimit > stats.cacheHits {
		fromAPI := stats.afterLimit - stats.cacheHits
		fmt.Fprintf(os.Stderr, "  From cache    : %d\n", stats.cacheHits)
		fmt.Fprintf(os.Stderr, "  From API      : %d\n", fromAPI)
	}

	fmt.Fprintf(os.Stderr, "\nTiming:\n")
	fmt.Fprintf(os.Stderr, "  Total time    : %.2fs\n", stats.totalDuration().Seconds())
	fmt.Fprintf(os.Stderr, "  Fetch time    : %.2fs\n", stats.fetchDuration().Seconds())
	if stats.afterLimit > 0 {
		fmt.Fprintf(os.Stderr, "  Avg per fetch : %dms\n", stats.avgFetchTime().Milliseconds())
	}
}

// printDryRunMode prints statistics about the bookmarks without making any API calls.
func printDryRunMode(stats stats, bookmarks []harmonic.Bookmark) {
	fmt.Fprintf(os.Stderr, "=== Dry Run ===\n")
	printPipelineStats(stats)
	fmt.Fprintf(os.Stderr, "To process      : %d\n", stats.afterLimit)

	if len(bookmarks) > 0 {
		// find date range
		minTS, maxTS := bookmarks[0].Timestamp, bookmarks[0].Timestamp
		for _, b := range bookmarks[1:] {
			if b.Timestamp < minTS {
				minTS = b.Timestamp
			}
			if b.Timestamp > maxTS {
				maxTS = b.Timestamp
			}
		}

		fmt.Fprintf(os.Stderr, "\nDate range:\n")
		fmt.Fprintf(os.Stderr, "  Oldest        : %s\n", time.Unix(minTS, 0).UTC().Format("2006-01-02"))
		fmt.Fprintf(os.Stderr, "  Newest        : %s\n", time.Unix(maxTS, 0).UTC().Format("2006-01-02"))
	}

	fmt.Fprintf(os.Stderr, "\nNo API calls made.\n")
}
