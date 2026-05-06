# dewey

Terminal-native access to the Dewey API. See `docs/dewey-cli-plan.md` for the
full project brief.

## Build

```sh
go build -ldflags="-X github.com/lambdabaa/dewey/apps/cli/internal/version.Version=$(git describe --tags --always) \
                   -X github.com/lambdabaa/dewey/apps/cli/internal/version.Commit=$(git rev-parse --short HEAD) \
                   -s -w" -trimpath -o dewey ./cmd/dewey
```

## Run

```sh
export DEWEY_API_KEY=dwy_live_…
./dewey doctor
./dewey collections list
./dewey upload ./papers/*.pdf -c research-papers --watch
./dewey research research-papers "what are the key findings?" --depth deep
```

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

## Conventions

- Diagnostic noise (status, progress) → stderr
- Data the user asked for → stdout
- `--json` is a stable contract; human mode is allowed to be lossy
- `DEWEY_API_KEY` is the only auth surface in v1; `DEWEY_BASE_URL` overrides
  the default endpoint
- Exit codes follow `sysexits.h` (see `internal/cmd/root.go`)
