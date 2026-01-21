# hnkeep

[![Release](https://img.shields.io/github/v/release/akhdanfadh/hnkeep)](https://github.com/akhdanfadh/hnkeep/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/akhdanfadh/hnkeep)](go.mod)
[![License](https://img.shields.io/github/license/akhdanfadh/hnkeep)](LICENSE)

hnkeep is a CLI tool that enables exporting [Hacker News](https://news.ycombinator.com) bookmarks from [Harmonic-HN](https://play.google.com/store/apps/details?id=com.simon.harmonichackernews) to [Karakeep](https://karakeep.app/). Harmonic-HN is an Android client for Hacker News while Karakeep is a self-hosted bookmark manager.

hnkeep is designed not for just a single one-time migration, but also for regular occasional exports with the ability to filter Harmonic bookmarks by date. I hope others can find it useful.

I built this because I have been using Harmonic to read HN articles for years and occasionally bookmark posts either to read them later (_uhm..._) or to keep track of interesting content. If you have Android phone and like doom-scrolling HN, I really recommend this app. After 1500+ saved articles, I want to manage and backup these bookmarks somewhere centralized. Karakeep's features (mainly the auto tagging and link rot protection) and its self-hosted nature made it seems like an ideal choice for me.

## Installation

### Pre-built binaries

Download the latest release for your platform from the [releases page](https://github.com/akhdanfadh/hnkeep/releases/latest). Available for Linux, macOS, and Windows (amd64/arm64).

### Go install

Requires Go 1.25.6 or later.

```sh
go install github.com/akhdanfadh/hnkeep/cmd/hnkeep@latest
```

### Build from source

Requires Go 1.25.6 or later.

```sh
git clone https://github.com/akhdanfadh/hnkeep.git
cd hnkeep
make build

# or manually
go build -o hnkeep ./cmd/hnkeep
```

## Usage

```sh
# file to file
hnkeep -i harmonic-export.txt -o karakeep-import.json

# piping and redirection (warnings/errors to stderr)
cat harmonic-export.txt | hnkeep > karakeep-import.json
```

| Flag                | Default                                        | Description                                        |
| ------------------- | ---------------------------------------------- | -------------------------------------------------- |
| `-v, --version`     |                                                | Show version information                           |
| `-i, --input`       | stdin                                          | Input file (Harmonic export)                       |
| `-o, --output`      | stdout                                         | Output file (Karakeep JSON)                        |
| `-q, --quiet`       |                                                | Suppress info messages (show warnings/errors only) |
| `--dry-run`         |                                                | Preview conversion without HN API calls            |
| `--before`          |                                                | Only include bookmarks before this date            |
| `--after`           |                                                | Only include bookmarks after this date             |
| `-n, --limit`       | 0                                              | Number of bookmarks to process (0 = all)           |
| `-c, --concurrency` | 5                                              | Number of concurrent HN API requests               |
| `-t, --tags`        | "src:hackernews"                               | Comma-separated tags for all bookmarks             |
| `--note-template`   | "{{smart_url}}"                                | Template for bookmark note field                   |
| `--no-dedupe`       |                                                | Keep duplicate URLs instead of merging them        |
| `--cache-dir`       | `${XDG_CACHE_DIR}/hnkeep` or `~/.cache/hnkeep` | HN API responses cache directory                   |
| `--no-cache`        |                                                | Disable caching of HN API responses                |
| `--clear-cache`     |                                                | Clear the cache before running                     |

### Implementation notes

- By default, the JSON output is written to stdout, while warnings and errors are written to stderr.

- Date filters (`--before`, `--after`) accept: `YYYY-MM-DD`, [RFC3339](https://datatracker.ietf.org/doc/html/rfc3339), or [Unix timestamp](https://www.unixtimestamp.com/) (seconds).

- Duplicate URLs (multiple HN submissions with the same URL) are merged into a single bookmark by default. The first occurrence (by bookmark save time, not HN submission time) is kept with its title and timestamp, and notes from duplicates are appended with a `---` separator. Use `--no-dedupe` to keep all duplicates.

- When syncing to Karakeep (if a bookmark URL already exists), notes are merged using content-based deduplication: if the existing note already contains the incoming note text, no update is made. This ensures multiple sync runs are idempotent without adding timestamp markers or hashes to notes. The tradeoff is that if you manually edit a note in Karakeep to remove imported content, a subsequent sync may re-append it.

- For note template, the following variables are available (use `--note-template ""` to disable notes entirely):

  | Variable        | Description                                                    |
  | --------------- | -------------------------------------------------------------- |
  | `{{smart_url}}` | HN discussion URL if item has external link, empty otherwise   |
  | `{{item_url}}`  | Item's external URL (empty for text posts like Ask HN)         |
  | `{{hn_url}}`    | HN discussion URL (`https://news.ycombinator.com/item?id=...`) |
  | `{{id}}`        | HN item ID                                                     |
  | `{{title}}`     | Item title                                                     |
  | `{{author}}`    | Author username                                                |
  | `{{date}}`      | Post date (`YYYY-MM-DD`)                                       |

## Contributing

Pull requests are very welcome. Feel free to open issues for bug reports or feature requests.

For reference, this tool was built against:

- Harmonic-HN [v2.2.5](https://github.com/SimonHalvdansson/Harmonic-HN/releases/tag/v2.2.5) (Dec 2025)
- Karakeep [v0.30.0](https://github.com/karakeep-app/karakeep/releases/tag/v0.30.0) (Jan 2026)
- Hacker News API [v0](https://github.com/HackerNews/API/tree/1fff41df2527fb24ece748acb928fa0cd6db048a) (Jan 2025)

It may require updates if any of these projects introduce breaking changes in the future.

## License

hnkeep is distributed under the [MIT](https://opensource.org/license/mit) license. See `LICENSE` for details.
