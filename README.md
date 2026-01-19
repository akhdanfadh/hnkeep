# hnkeep

hnkeep is a CLI tool that enables exporting [Hacker News](https://news.ycombinator.com) bookmarks from [Harmonic-HN](https://play.google.com/store/apps/details?id=com.simon.harmonichackernews) to [Karakeep](https://karakeep.app/).

## Installation

```sh
go install github.com/akhdanfadh/hnkeep/cmd/hnkeep@latest
```

Or build from source:

```sh
git clone https://github.com/akhdanfadh/hnkeep.git
cd hnkeep
make build

# or manually
go build -o hnkeep ./cmd/hnkeep
```

## Usage

By default, the JSON output is written to stdout, while warnings and errors are written to stderr.

```sh
# file to file
hnkeep -i harmonic-export.txt -o karakeep-import.json

# piping and redirection (warnings/errors to stderr)
cat harmonic-export.txt | hnkeep > karakeep-import.json
```

| Flag                | Default | Description                          |
| ------------------- | ------- | ------------------------------------ |
| `-i, --input`       | stdin   | Input file (Harmonic export)         |
| `-o, --output`      | stdout  | Output file (Karakeep JSON)          |
| `-c, --concurrency` | 5       | Number of concurrent HN API requests |

## Contributing

Pull requests are very welcome. Feel free to open issues for bug reports or feature requests.

For reference, this tool was built against:

- Harmonic-HN [v2.2.5](https://github.com/SimonHalvdansson/Harmonic-HN/releases/tag/v2.2.5) (Dec 2025)
- Karakeep [v0.30.0](https://github.com/karakeep-app/karakeep/releases/tag/v0.30.0) (Jan 2026)
- Hacker News API [v0](https://github.com/HackerNews/API/tree/1fff41df2527fb24ece748acb928fa0cd6db048a) (Jan 2025)

It may require updates if any of these projects introduce breaking changes in the future.

## License

hnkeep is distributed under the [MIT](https://opensource.org/license/mit) license. See `LICENSE` for details.
