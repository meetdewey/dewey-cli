# dewey

[![CI](https://github.com/meetdewey/dewey-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/meetdewey/dewey-cli/actions/workflows/ci.yml)

Terminal-native access to the [Dewey](https://meetdewey.com) document backend API. Upload documents, run hybrid search, stream cited research answers — all from the command line.

```sh
export DEWEY_API_KEY=dwy_live_…

dewey collections create research-papers
dewey upload ./papers/*.pdf -c research-papers --watch
dewey research research-papers "what are the key findings?" --depth deep
```

## Install

### macOS / Linux

```sh
curl -fsSL https://raw.githubusercontent.com/meetdewey/dewey-cli/main/install.sh | sh
```

The script detects your OS and arch, downloads the correct binary, verifies the SHA-256 checksum, and installs to `~/.local/bin` (no sudo required). Override with `INSTALL_DIR=/usr/local/bin sh install.sh`.

Or download a specific release directly — archives are named `dewey_<version>_<os>_<arch>.tar.gz`:

```sh
# Example: v0.1.0 on macOS arm64
curl -Lo dewey.tar.gz \
  https://github.com/meetdewey/dewey-cli/releases/download/v0.1.0/dewey_0.1.0_darwin_arm64.tar.gz
tar -xzf dewey.tar.gz && mv dewey /usr/local/bin/
```

### Windows

Download `dewey_<version>_windows_amd64.zip` from [Releases](https://github.com/meetdewey/dewey-cli/releases/latest) and extract `dewey.exe` to a directory in your `PATH`.

## Authentication

All commands require a project API key:

```sh
export DEWEY_API_KEY=dwy_live_…
```

The `--api-key` flag is also accepted (prefer the env var). Run `dewey doctor` to validate your setup.

## Commands

```
dewey upload <files...>         Upload files (--watch for live status)
dewey query  <collection> <q>   Hybrid retrieval
dewey scan   <collection> <q>   Section-summary scan
dewey research <col> <question> Streaming cited answer
dewey watch  [collection]       Tail document-status events
dewey doctor                    Validate environment

dewey collections list|get|create|update|delete|stats
dewey docs        list|get|markdown|sections|chunks|images|delete|wait
dewey duplicates  detect|list|resolve|dismiss
dewey contradictions detect|list|apply|dismiss
dewey claims      list|get
dewey provider-keys list|set|delete
dewey config      get|set|reset|path
dewey version
```

Every command supports `--json` for machine-readable output. See `dewey <command> --help` for full flag details.

## Build from source

```sh
git clone https://github.com/meetdewey/dewey-cli.git
cd dewey-cli
go build -trimpath -o dewey ./cmd/dewey
```

Requires Go 1.23+.

## Layout

```
cmd/dewey/main.go             entrypoint
internal/
  api/                        hand-written HTTP client
  cmd/                        cobra subcommands
  config/                     ~/.dewey/config.toml + state.toml
  output/                     human + JSON renderers
  version/                    ldflags-injected build metadata
```

## Output conventions

- Diagnostic noise (status, progress bars) → **stderr**
- Data (query results, research answers, JSON) → **stdout**
- `--json` is a stable contract across minor versions; human output is allowed to be lossy

## Configuration

`~/.dewey/config.toml`:
```toml
default_collection = "research-papers"
output = "human"   # "human" | "json"
color  = "auto"    # "auto" | "always" | "never"
```

Override the API endpoint (staging, self-hosted, local dev):
```sh
export DEWEY_BASE_URL=http://localhost:3000/v1
```
