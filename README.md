# hnkeep - Sync Hacker News bookmarks from Harmonic-HN to Karakeep

[![Release](https://img.shields.io/github/v/release/akhdanfadh/hnkeep)](https://github.com/akhdanfadh/hnkeep/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/akhdanfadh/hnkeep)](go.mod)
[![License](https://img.shields.io/github/license/akhdanfadh/hnkeep)](LICENSE)

hnkeep is a CLI tool that enables syncing [Hacker News](https://news.ycombinator.com) bookmarks from [Harmonic-HN](https://play.google.com/store/apps/details?id=com.simon.harmonichackernews) to [Karakeep](https://karakeep.app/). Harmonic-HN is an Android client for Hacker News while Karakeep is a self-hosted bookmark manager.

hnkeep is designed not for just a single one-time migration, but also for regular occasional exports. I hope others can find it useful.

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
# file to file (generates Karakeep import JSON)
hnkeep -i harmonic-export.txt -o karakeep-import.json

# piping and redirection (warnings/errors to stderr)
cat harmonic-export.txt | hnkeep > karakeep-import.json

# sync mode: push directly to Karakeep API
export KARAKEEP_API_URL=https://your-karakeep-server/api/v1
export KARAKEEP_API_KEY=your-api-key
hnkeep -i harmonic-export.txt -sync
```

| Flag               | Default                                        | Description                                          |
| ------------------ | ---------------------------------------------- | ---------------------------------------------------- |
| `-v, -version`     |                                                | Show version information                             |
| `-i, -input`       | stdin                                          | Input file (Harmonic export)                         |
| `-o, -output`      | stdout                                         | Output file (Karakeep JSON)                          |
| `-n, -limit`       | 0                                              | Max input bookmarks to process (0 = all)             |
| `-c, -concurrency` | 5                                              | Number of concurrent API calls                       |
| `-t, -tags`        | "src:hackernews,hnkeep:YYYYMMDD"               | Tags to apply to output bookmarks                    |
| `-note-template`   | "{{smart_url}}"                                | Template for output bookmark note field              |
| `-sync`            |                                                | Sync directly to Karakeep API (instead of JSON file) |
| `-api-url`         | env `KARAKEEP_API_URL`                         | Karakeep API base URL (required for sync)            |
| `-api-key`         | env `KARAKEEP_API_KEY`                         | Karakeep API key (required for sync)                 |
| -api-timeout       | 30s                                            | Karakeep API request timeout                         |
| `-before`          |                                                | Only include input bookmarks before this date        |
| `-after`           |                                                | Only include input bookmarks after this date         |
| `-dry-run`         |                                                | Preview conversion without API calls                 |
| `-verbose`         |                                                | Show progress messages during fetch/sync             |
| `-no-dedupe`       |                                                | Keep duplicate URLs in input instead of merging      |
| `-cache-dir`       | `${XDG_CACHE_DIR}/hnkeep` or `~/.cache/hnkeep` | HN API responses cache directory                     |
| `-no-cache`        |                                                | Disable caching of HN API responses                  |
| `-clear-cache`     |                                                | Clear the cache before running                       |

### Implementation notes

- By default, the JSON output is written to stdout, while warnings and errors are written to stderr.

- Date filters (`-before`, `-after`) accept: `YYYY-MM-DD`, [RFC3339](https://datatracker.ietf.org/doc/html/rfc3339), or [Unix timestamp](https://www.unixtimestamp.com/) (seconds). This could be useful for manually filtering bookmarks during periodic exports.

- Duplicate URLs (multiple HN submissions with the same URL) are merged into a single output bookmark by default. The first occurrence (by Harmonic save time, not HN submission time) is kept with its title and timestamp, and notes from duplicates are appended with a `---` separator. Use `-no-dedupe` to keep all duplicates.

- When syncing to Karakeep (if a bookmark URL already exists), notes are merged using content-based deduplication: if the existing Karakeep note already contains the incoming note text, no update is made. This ensures multiple sync runs are idempotent without adding timestamp markers or hashes to notes. The tradeoff is that if you manually edit a note in Karakeep to remove imported content, a subsequent sync may re-append it.

- Sync mode (`-sync`) and file output (`-output`) are mutually exclusive. When `-sync` is enabled, bookmarks are pushed directly to Karakeep and no JSON file is written.

- Sync mode performs a pre-flight connectivity check before any expensive operations. This validates both the API URL and key are correct. Use `-dry-run -sync` to verify your Karakeep configuration without actually syncing.

- For note template, the following variables are available (use `-note-template ""` to disable notes entirely):

  | Variable        | Description                                                    |
  | --------------- | -------------------------------------------------------------- |
  | `{{smart_url}}` | HN discussion URL if item has external link, empty otherwise   |
  | `{{item_url}}`  | Item's external URL (empty for text posts like Ask HN)         |
  | `{{hn_url}}`    | HN discussion URL (`https://news.ycombinator.com/item?id=...`) |
  | `{{id}}`        | HN item ID                                                     |
  | `{{title}}`     | Item title                                                     |
  | `{{author}}`    | Author username                                                |
  | `{{date}}`      | Post date (`YYYY-MM-DD`)                                       |

## Sync and Deduplication

The sync feature (`-sync`) is designed for reliability above all else. The primary use case is running hnkeep repeatedly over time with different or overlapping Harmonic export files, where each sync should produce the same result regardless of how many times it runs or what was synced before. You should be able to export your entire Harmonic bookmark history today, sync it, then export again six months later with new bookmarks accumulated, and sync that file without creating duplicates of the previously imported items.

**Why client-side deduplication is necessary?** Karakeep provides built-in deduplication for link bookmarks by checking if a URL already exists. However, when Karakeep's crawler processes certain URLs, it converts them from link bookmarks to asset bookmarks. This happens for:

- PDFs (`application/pdf`)
- Images (`image/gif`, `image/jpeg`, `image/png`, `image/webp`)

During this conversion, the bookmark is removed from link storage and moved to asset storage, but Karakeep's deduplication only checks link storage. Submitting the same PDF URL twice will create duplicates because the first one is no longer visible to the deduplication check.

**How hnkeep handles this?** At the start of each sync, hnkeep fetches all existing bookmarks from your Karakeep instance and builds a URL map. For link bookmarks, the URL is stored directly. For asset bookmarks, Karakeep preserves the original source URL in a separate field, which hnkeep extracts. When processing each Harmonic bookmark, hnkeep checks this map first and treats matching URLs as existing bookmarks rather than creating new ones.

The pre-fetch has minimal overhead: Karakeep returns 100 bookmarks per page, so 3,000 bookmarks requires only 30 API calls. The URL map consumes roughly 500KB–1MB of memory. This ensures every create/update/skip decision is made against the actual current state of Karakeep.

**Caveats:**

- If a bookmark is deleted from Karakeep between syncs, the next sync will recreate it. This is intentional as hnkeep treats Harmonic as the source of truth. To prevent recreation, remove the item from your Harmonic export or use date filters to exclude it.
- If the pre-fetch fails partway through due to network issues, hnkeep may create duplicates for items it failed to fetch. Warnings will be logged, and you can re-run once connectivity is restored.

## Contributing

Pull requests are very welcome. Feel free to open issues for bug reports or feature requests.

For reference, this tool was built against:

- Harmonic-HN [v2.2.5](https://github.com/SimonHalvdansson/Harmonic-HN/releases/tag/v2.2.5) (Dec 2025)
- Karakeep [v0.30.0](https://github.com/karakeep-app/karakeep/releases/tag/v0.30.0) (Jan 2026)
- Hacker News API [v0](https://github.com/HackerNews/API/tree/1fff41df2527fb24ece748acb928fa0cd6db048a) (Jan 2025)

It may require updates if any of these projects introduce breaking changes in the future.

## License

hnkeep is distributed under the [MIT](https://opensource.org/license/mit) license. See `LICENSE` for details.
