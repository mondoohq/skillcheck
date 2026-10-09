# AGENTS.md

Working instructions for an AI agent operating in this repository
([agents.md](https://agents.md/) convention).

## What this is

`skillcheck` is a Go CLI that (1) scans a machine for known-malicious installed AI agent
skills and (2) validates a skills *repository* against the Agent Skills and AGENTS.md specs
(`skillcheck validate`). It embeds the [MQL](https://mondoo.com/docs/mql/home/) engine with
the OS + core providers compiled in.

## Layout

- `cmd/skillcheck/` — the cobra CLI (default scan + the `validate` subcommand).
- `internal/engine/` — the embedded MQL runtime; `schemas/` holds the embedded resource JSON.
- `internal/hasher`, `internal/mondoo`, `internal/reporter` — hashing, the Mondoo API client, output.
- `internal/validate/` — `validate`; `policy/repo-contract.mql.yaml` is the embedded MQL policy
  whose checks it runs (the rules are data, not Go).

## Build, test, lint

```bash
make build    # CGO_ENABLED=0 go build
make install  # go install into $GOBIN (or $GOPATH/bin)
make test     # go test ./... -count=1
make lint     # golangci-lint run ./...
```

Run all three before opening a PR. `TestSearchByHash_LiveAPI` needs network to the Mondoo API
and is skipped under `go test -short`.

## Conventions

- Every Go and YAML source file carries the Apache-2.0 copyright header
  (`copywrite headers`); CI enforces it. `**/*.json` and `**/testdata/**` are exempt.
- After bumping mql in `go.mod`, regenerate the embedded MQL schemas with `make schemas`
  (generates them from the pinned mql module); CI checks they match.
- The `validate` policy is a standard cnspec bundle — keep it runnable by `cnspec` as well as
  by the embedded runner.

## Security

- The scan is **fail-open**: a skill not in the database shows as clean and never blocks a
  workflow. Preserve that.
- Don't commit secrets; the live-API test hits a public endpoint only.

## PRs

One focused change per PR. Run `make test` and `make lint` first; state what you verified.
