# ADR-001: Build `skillcheck` CLI in Go, Publish to npm via GoReleaser

| Field       | Value                                      |
|-------------|--------------------------------------------|
| Status      | Proposed                                   |
| Date        | 2026-04-15                                 |
| Authors     | —                                          |
| Deciders    | —                                          |
| Tags        | cli, security, ai-agents, npm, goreleaser  |

---

## Context

We are building **skillcheck**, a CLI tool that detects locally installed AI agent skills (Claude Code, Cursor, Codex, Gemini CLI, Windsurf, Cline, Copilot, Goose, Amp, Kilo, and others), computes SHA-256 checksums, and queries the [Mondoo AI Agent Security](https://mondoo.com/ai-agent-security) database for known threats — prompt injection, credential theft, data exfiltration, and 25+ other threat categories.

The tool must ship as a **single installable command** that works on macOS (Intel + Apple Silicon), Linux (x64 + ARM), and Windows (x64). The primary distribution channel is **npm** (`npx @mondoo/skillcheck` or `npm i -g @mondoo/skillcheck`), because our target audience — developers using AI coding agents — overwhelmingly has Node.js installed already. We also want GitHub Releases, Homebrew, and potential future channels.

Two candidate languages: **Go** or **Node.js (TypeScript)**.

---

## Decision

**Write skillcheck in Go.** Publish the Go binary to npm using GoReleaser's native `npms:` publisher, which ships platform-specific npm packages that download the correct prebuilt binary on install.

---

## Evaluation

### Why Go wins for this tool

**Single binary, zero runtime.** Go compiles to a static binary with no dependencies. The user never encounters Node version mismatches, missing native modules, or `node_modules` bloat. The installed binary is 5–10 MB and starts instantly.

**Cross-platform filesystem hashing is trivial.** The core work — walking directories, computing SHA-256, making HTTP requests — is Go's bread and butter. The standard library handles all of it with no third-party dependencies for core functionality.

**Mondoo ecosystem alignment.** Mondoo's own tooling (cnspec, cnquery, the mondoo-go API client) is written in Go. A Go skillcheck can directly import `go.mondoo.com/mondoo-go` for future API integration and shares the same build and release infrastructure.

**Precedent in the agent security space.** Cisco's skill-scanner is Python, Snyk's agent-scan is Python — but both suffer from slow startup and complex install steps. The Go-based tools in this space (cnspec, agentguard) have notably smoother install experiences.

**CI/CD exit codes and no runtime.** For pipeline integration (`skillcheck --json --no-color`), a Go binary has deterministic behavior with no runtime overhead, no garbage collection pauses during large scans, and predictable memory usage.

### Why not Node.js

**Runtime dependency.** A Node.js CLI requires Node ≥18 at minimum. While our audience likely has Node, version fragmentation is real — especially in CI environments with pinned Node versions.

**Startup time.** A non-trivial Node CLI with dependencies takes 200–500ms to start. A Go binary starts in under 10ms. For a security scanning tool that might run on every commit, this adds up.

**Native module risk.** If we ever need to do binary analysis (inspecting skill archives, checking for embedded executables), Node requires native addons or shelling out, while Go handles it natively.

**Dependency supply chain.** A Go binary has zero runtime dependencies. A Node.js tool inherits the npm dependency tree — ironic for a tool that *detects supply chain attacks in AI agent skills*.

### Trade-off acknowledged

**Contributor accessibility.** More developers know TypeScript than Go. This is a real cost. We mitigate it by keeping the Go code simple (no generics, no channels, straightforward struct-based design) and providing thorough documentation.

---

## npm Distribution via GoReleaser

GoReleaser v2.13+ has **native npm publishing** via the `npms:` configuration block. This is the recommended approach over third-party wrappers like `goreleaser-npm-publisher` or `go-npm`.

### How it works

GoReleaser builds platform-specific archives (e.g., `skillcheck_darwin_arm64.tar.gz`). The `npms:` publisher creates:

1. **Platform-specific packages** (`@mondoo/skillcheck-darwin-arm64`, `@mondoo/skillcheck-linux-x64`, etc.) — each containing the binary for one OS/arch.
2. **A root package** (`@mondoo/skillcheck`) — a thin wrapper with an `optionalDependencies` entry for each platform package, plus a `bin` entry pointing to a small JS script that runs the correct binary.

When a user runs `npm i -g @mondoo/skillcheck`, npm installs only the platform package matching their OS/arch, and the root package's bin script delegates to the binary. Total overhead: one 3-line JS file.

### GoReleaser configuration

```yaml
# .goreleaser.yaml

version: 2

builds:
  - id: skillcheck
    main: ./cmd/skillcheck
    binary: skillcheck
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .ShortCommit }}
      - -X main.date={{ .CommitDate }}
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}

checksum:
  name_template: "checksums.txt"

npms:
  - name: "@mondoo/skillcheck"
    description: >
      AI agent skill security scanner. Detects locally installed
      agent skills, computes SHA-256 checksums, and queries the
      Mondoo AI Agent Security database for known threats.
    license: Apache-2.0
    homepage: https://mondoo.com/ai-agent-security
    keywords:
      - ai-security
      - agent-skills
      - skillcheck
      - mondoo
      - cli
      - devsecops
      - prompt-injection
      - supply-chain
    author: Mondoo Inc. <hello@mondoo.com>
    repository: https://github.com/mondoohq/skillcheck
    bugs: https://github.com/mondoohq/skillcheck/issues
    access: public
    extra_files:
      - glob: README.md
      - glob: LICENSE

brews:
  - name: skillcheck
    repository:
      owner: mondoohq
      name: homebrew-mondoo
    homepage: https://mondoo.com/ai-agent-security
    description: AI agent skill security scanner by Mondoo
    license: Apache-2.0

release:
  github:
    owner: mondoohq
    name: skillcheck
  prerelease: auto

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
```

### GitHub Actions release workflow

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - "v*.*.*"

permissions:
  contents: write
  id-token: write  # npm provenance

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - uses: actions/setup-node@v4
        with:
          node-version: "lts/*"
          registry-url: "https://registry.npmjs.org"

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser-pro  # native npm requires Pro
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
          GORELEASER_KEY: ${{ secrets.GORELEASER_KEY }}
```

> **Note:** GoReleaser's native `npms:` publisher is a **Pro** feature. If using GoReleaser OSS, the alternative is `goreleaser-npm-publisher-action` by evg4b, which reads GoReleaser's `dist/` output and generates the same platform-specific npm package structure:
>
> ```yaml
> - name: Publish to npm
>   uses: evg4b/goreleaser-npm-publisher-action@v1.2.0
>   with:
>     prefix: "@mondoo"
>     token: ${{ secrets.NPM_TOKEN }}
> ```

---

## User experience

```bash
# Install globally
npm i -g @mondoo/skillcheck

# Or run directly without installing
npx @mondoo/skillcheck

# Or via Homebrew
brew install mondoohq/mondoo/skillcheck

# Or download binary from GitHub Releases
curl -sSL https://github.com/mondoohq/skillcheck/releases/latest/download/skillcheck_Linux_x86_64.tar.gz | tar xz
```

After install, usage is identical regardless of distribution channel:

```bash
# Scan all agent skill directories
skillcheck

# Scan specific path
skillcheck --path ~/.cursor/skills

# JSON for CI/CD
skillcheck --json --no-color

# Verbose file-level hashes
skillcheck --verbose
```

---

## Architecture overview

```
skillcheck/
├── cmd/skillcheck/
│   └── main.go              # CLI entry point, flag parsing
├── internal/
│   ├── detector/
│   │   └── detector.go      # Discovers skills across 13+ agent platforms
│   ├── hasher/
│   │   └── hasher.go        # SHA-256 per-file and content-hash
│   ├── mondoo/
│   │   └── client.go        # REST API + web fallback to mondoo.com/ai-agent-security
│   └── reporter/
│       └── reporter.go      # CLI (colored) and JSON output
├── .goreleaser.yaml
├── .github/workflows/
│   └── release.yml
├── go.mod
├── Makefile
└── README.md
```

### Detection targets

The detector walks these directories (relative to `$HOME`):

| Agent            | Paths                                                |
|------------------|------------------------------------------------------|
| Claude Code      | `.claude/skills`, `.claude/commands`                 |
| Cursor           | `.cursor/skills`, `.cursor/rules`                    |
| OpenAI Codex     | `.codex/skills`                                      |
| Gemini CLI       | `.gemini/skills`, `.gemini/commands`                 |
| Windsurf         | `.windsurf/skills`, `.windsurf/rules`                |
| Cline            | `.cline/skills`, `.cline/rules`                      |
| GitHub Copilot   | `.github/copilot-skills`, `.github/skills`           |
| Goose            | `.goose/skills`                                      |
| Amp              | `.amp/skills`                                        |
| Kilo             | `.kilo/skills`                                       |
| Roo              | `.roo/skills`, `.roo/rules`                          |
| OpenCode         | `.opencode/skills`                                   |
| VS Code          | `.vscode/skills`                                     |

Plus the current working directory for project-scoped skills.

### Mondoo integration

Each detected skill gets a SHA-256 content hash (sorted concatenation of all files). The tool queries:

1. `https://mondoo.com/ai-agent-security/api/v1/lookup?name=<skill>&hash=<sha>` — REST API (when available)
2. `https://mondoo.com/ai-agent-security/skills?q=<skill>` — web search fallback
3. Always emits a clickable Mondoo URL so the user can verify in the browser

The Mondoo Skill Check database covers **1,200+ skills** across ClawHub, Skills.sh, GitHub, and Claude Marketplace, with a 6-layer analysis pipeline (static analysis → ML classification → LLM threat analysis → deep inspection → false positive filtering → MITRE ATLAS / OWASP LLM Top 10 mapping).

### Exit codes

| Code | Meaning                                    |
|------|--------------------------------------------|
| 0    | No critical or high-risk skills found      |
| 1    | At least one critical or high-risk finding |

This makes it usable as a CI/CD gate.

---

## Consequences

### Positive

- **Zero-dependency install** via npm, Homebrew, or direct binary download
- **Fast execution** — sub-second for typical skill sets (< 50 skills)
- **Mondoo ecosystem compatibility** — can import `go.mondoo.com/mondoo-go` for future deep integration
- **Single release pipeline** — one `git tag` triggers GitHub Release + npm publish + Homebrew tap via GoReleaser
- **Supply chain integrity** — npm `--provenance` + GoReleaser checksums + cosign signatures

### Negative

- **GoReleaser Pro required** for native `npms:` (or use the free `goreleaser-npm-publisher-action` as fallback)
- **Go contributor barrier** — slightly higher than TypeScript, mitigated by simple code structure
- **npm install is a two-step** — npm downloads the wrapper, which downloads the binary from GitHub Releases. If GitHub is unreachable, install fails. (Can be mitigated by embedding binaries directly in platform packages at the cost of npm package size.)

### Neutral

- New agent platforms will require adding paths to `detector.go` — a one-line change per platform, easily contributed
- The Mondoo REST API is aspirational; the web scrape fallback handles the current state

---

## Links

- Mondoo AI Agent Security: https://mondoo.com/ai-agent-security
- Mondoo Skill Database: https://mondoo.com/ai-agent-security/skills
- Mondoo Security Checks: https://mondoo.com/ai-agent-security/checks
- GoReleaser npm docs: https://goreleaser.com/customization/npm/
- GoReleaser npm publisher (OSS alternative): https://github.com/evg4b/goreleaser-npm-publisher
- Cisco skill-scanner (prior art): https://github.com/cisco-ai-defense/skill-scanner
- Snyk agent-scan (prior art): https://github.com/snyk/agent-scan
- Mondoo Go client library: https://github.com/mondoohq/mondoo-go
