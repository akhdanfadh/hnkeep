# hnkeep - Sync Hacker News bookmarks from Harmonic-HN to Karakeep

[![Release](https://img.shields.io/github/v/release/akhdanfadh/hnkeep)](https://github.com/akhdanfadh/hnkeep/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/akhdanfadh/hnkeep)](go.mod)
[![License](https://img.shields.io/github/license/akhdanfadh/hnkeep)](LICENSE)

hnkeep is a CLI tool that enables syncing [Hacker News](https://news.ycombinator.com) bookmarks from [Harmonic-HN](https://play.google.com/store/apps/details?id=com.simon.harmonichackernews) to [Karakeep](https://karakeep.app/). Harmonic-HN is an Android client for Hacker News while Karakeep is a self-hosted bookmark manager.

hnkeep is designed not for just a single one-time migration, but also for regular occasional exports. It is built in Go without any external dependencies. I hope others can find this tool useful.

I built this because I have been using Harmonic to read HN articles for years and occasionally bookmark posts either to read them later (_uhm..._) or to keep track of interesting content. After 1500+ saved articles, I want to manage and backup these bookmarks somewhere centralized. Karakeep's features (mainly the auto tagging and link rot protection) and its self-hosted nature made it seems like an ideal choice for me.

![A demonstration of hnkeep.](https://vhs.charm.sh/vhs-5QpmugcHeIi0IlBUAFeVOp.gif)

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

Assuming you have the export file from Harmonic-HN (e.g. `HarmonicBookmarks2026-1-17.txt`), you can run either of the following:

```sh
# 1a. Generate JSON file for manual import to Karakeep
hnkeep -i HarmonicBookmarks2026-1-17.txt -o karakeep-import.json
# 1b. Same but using pipes
cat HarmonicBookmarks2026-1-17.txt | hnkeep > karakeep-import.json

# 2. Sync mode: push directly with Karakeep API
export KARAKEEP_API_URL=https://your-karakeep-server/api/v1
export KARAKEEP_API_KEY=your-api-key
hnkeep -i HarmonicBookmarks2026-1-17.txt -sync
```

| Flag               | Description                                          | Default                                        |
| ------------------ | ---------------------------------------------------- | ---------------------------------------------- |
| `-v, -version`     | Show version information                             |                                                |
| `-i, -input`       | Input file (Harmonic export)                         | stdin                                          |
| `-o, -output`      | Output file (Karakeep JSON)                          | stdout                                         |
| `-n, -limit`       | Max input bookmarks to process (0 = all)             | 0                                              |
| `-c, -concurrency` | Number of concurrent API calls                       | 5                                              |
| `-t, -tags`        | Tags to apply to output bookmarks                    | "src:hackernews, hnkeep:YYYYMMDD"              |
| `-note-template`   | Template for output bookmark note field              | "{{smart_url}}"                                |
| `-sync`            | Sync directly to Karakeep API (instead of JSON file) |                                                |
| `-api-url`         | Karakeep API base URL (required for sync)            | env `KARAKEEP_API_URL`                         |
| `-api-key`         | Karakeep API key (required for sync)                 | env `KARAKEEP_API_KEY`                         |
| `-api-timeout`     | Karakeep API request timeout                         | 30s                                            |
| `-before`          | Only include input bookmarks before this date        |                                                |
| `-after`           | Only include input bookmarks after this date         |                                                |
| `-dry-run`         | Preview conversion without API calls                 |                                                |
| `-verbose`         | Show progress messages during fetch/sync             |                                                |
| `-cache-dir`       | HN API responses cache directory                     | `${XDG_CACHE_DIR}/hnkeep` or `~/.cache/hnkeep` |
| `-no-cache`        | Disable caching of HN API responses                  |                                                |
| `-clear-cache`     | Clear the cache before running                       |                                                |

For note template, the following variables are available (use `-note-template ""` to disable notes entirely):

- `{{smart_url}}`: HN discussion URL if item has external link, empty otherwise
- `{{item_url}}`: Item's external URL (empty for text posts like Ask HN)
- `{{hn_url}}`: HN discussion URL (`https://news.ycombinator.com/item?id=...`)
- `{{id}}`: HN item ID
- `{{title}}`: Item title
- `{{author}}`: Author username
- `{{date}}`: Post date (`YYYY-MM-DD`)

## Implementation notes

- Output is written to stdout by default, while warnings and errors go to stderr.

- Date filters (`-before`, `-after`) accept `YYYY-MM-DD`, [RFC3339](https://datatracker.ietf.org/doc/html/rfc3339), or [Unix timestamp](https://www.unixtimestamp.com/) (seconds). Useful for filtering bookmarks during periodic exports.

- Duplicate URLs (multiple HN submissions pointing to the same URL) are merged into a single bookmark. The first occurrence by Harmonic save time is kept, and notes from duplicates are appended with a `---` separator.

- Sync mode (`-sync`) and file output (`-output`) are mutually exclusive. When syncing, bookmarks are pushed directly to Karakeep without writing a JSON file.

- Sync mode performs a pre-flight connectivity check to validate the API URL and key before processing. Use `-dry-run -sync` to verify your Karakeep configuration.

- Sync is designed for idempotency: running multiple times with the same or overlapping exports won't create duplicates. If a bookmark is deleted from Karakeep between syncs, it will be recreated (use date filters or remove from Harmonic export to prevent this).

- When syncing existing bookmarks, notes are merged using content-based deduplication. If the Karakeep note already contains the incoming text, no update is made. This means manually removing imported content from Karakeep may result in it being re-appended on the next sync.

## Contributing

Pull requests are very welcome. Feel free to open issues for bug reports or feature requests.

For reference, this tool was built against:

- Harmonic-HN [v2.2.5](https://github.com/SimonHalvdansson/Harmonic-HN/releases/tag/v2.2.5) (Dec 2025)
- Karakeep [v0.30.0](https://github.com/karakeep-app/karakeep/releases/tag/v0.30.0) (Jan 2026)
- Hacker News API [v0](https://github.com/HackerNews/API/tree/1fff41df2527fb24ece748acb928fa0cd6db048a) (Jan 2025)

It may require updates if any of these projects introduce breaking changes in the future.

## License

hnkeep is distributed under the [MIT](https://opensource.org/license/mit) license. See `LICENSE` for details.
