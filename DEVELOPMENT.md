# Development

## Prerequisites

- Go 1.25+
- Access to `go.mondoo.com/mql/v13` module

## Build

```bash
make build
make test
make lint
```

## Update MQL schemas

When the MQL OS provider changes (new resources or fields), update the embedded schemas:

```bash
make schemas
```

This copies `os.resources.json` and `core.resources.json` from the local `../mql` checkout.

## Architecture

```
skillcheck/
├── cmd/skillcheck/          # CLI entry point (cobra)
├── internal/
│   ├── engine/              # MQL runtime with OS + core providers compiled in
│   │   └── schemas/         # Embedded resource schema JSON
│   ├── hasher/              # SHA-256 content hashing
│   ├── mondoo/              # Mondoo API client (/api/v1/search/hash)
│   └── reporter/            # CLI (colored) + JSON output
├── .goreleaser.yaml         # Cross-platform builds + npm publishing
└── Makefile
```

## How it works

skillcheck embeds the [MQL](https://mondoo.com/docs/mql/home/) engine with the OS provider compiled in as a builtin. The same `claude.code` and `openai.codex` resources that power cnquery/cnspec are used for detection — one backend, consistent results.

## Release

Tag and push to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This builds binaries for 6 platforms, creates a GitHub Release, and publishes `@mondoohq/skillcheck` to npm.
