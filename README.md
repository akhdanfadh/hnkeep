# hnkeep

`hnkeep` is a CLI tool that enables exporting Hacker News bookmarks from [Harmonic-HN](https://github.com/nicnocquee/Harmonic-HN) to [Karakeep](https://github.com/karakeep/karakeep).

## Installation

```sh
go install github.com/akhdanfadh/hnkeep/cmd/hnkeep@latest
```

Or build from source:

```sh
git clone https:/github.com/akhdanfadh/hnkeep.git
cd hnkeep
make build

# or manually
go build -o hnkeep ./cmd/hnkeep
```

## Usage

```sh
# file to file
hnkeep -i input.txt -o output.json

# unix piping
cat input.txt | hnkeep > output.json
```

| Flag           | Default | Description                  |
| -------------- | ------- | ---------------------------- |
| `-i, --input`  | stdin   | Input file (Harmonic export) |
| `-o, --output` | stdout  | Output file (Karakeep JSON)  |
