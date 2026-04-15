# skillcheck

AI agent skill security scanner by [Mondoo](https://mondoo.com/ai-agent-security).

Detects locally installed AI agent skills, computes SHA-256 checksums, and queries the Mondoo AI Agent Security database for known threats — prompt injection, credential theft, data exfiltration, and 25+ other threat categories.

## Supported Agents

| Agent | Config | What's Detected |
|-------|--------|-----------------|
| Claude Code | `~/.claude/` | skills, plugins, MCP servers |
| OpenAI Codex | `~/.codex/` | skills, plugins, MCP servers |

More agents (Cursor, GitHub Copilot, Goose, Gemini CLI, Windsurf, Zed) are coming soon.

## Install

```bash
# Build from source
make build

# Or with go install
go install go.mondoo.com/skillcheck/cmd/skillcheck@latest
```

## Usage

```bash
# Scan all detected agents
skillcheck

# JSON output for CI/CD
skillcheck --json

# Verbose output with hashes and Mondoo URLs
skillcheck --verbose

# Disable colored output
skillcheck --no-color
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No critical or high-risk skills found |
| 1 | At least one critical or high-risk finding |

## How It Works

skillcheck embeds the [MQL](https://mondoo.com/docs/mql/home/) engine with the OS provider compiled in. The same MQL resources that power cnquery/cnspec are used for detection — one backend, consistent results across standalone scanning and fleet-wide enterprise collection.

```
skillcheck CLI
  └── MQL engine (embedded)
        └── OS provider (builtin)
              ├── claude.code resource → ~/.claude/
              └── openai.codex resource → ~/.codex/
```

Each detected skill gets a SHA-256 content hash. The tool queries the [Mondoo AI Agent Security](https://mondoo.com/ai-agent-security) database for known threats.

## Development

```bash
# Build
make build

# Run tests
make test

# Update embedded MQL schemas from local mql checkout
make schemas
```

### Architecture

```
skillcheck/
├── cmd/skillcheck/          # CLI entry point
├── internal/
│   ├── engine/              # MQL runtime setup (OS + core providers)
│   │   └── schemas/         # Embedded resource schema JSON
│   ├── hasher/              # SHA-256 content hashing
│   ├── mondoo/              # Mondoo API client
│   └── reporter/            # CLI + JSON output
├── go.mod
└── Makefile
```

## Links

- [Mondoo AI Agent Security](https://mondoo.com/ai-agent-security)
- [Mondoo Skill Database](https://mondoo.com/ai-agent-security/skills)
- [ADR-001: Go + npm via GoReleaser](ADR-001-skillcheck-go-npm-goreleaser.md)
