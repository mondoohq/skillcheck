# Development

## Prerequisites

- Go 1.25+
- Access to `go.mondoo.com/mql (v14)` module

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
│   ├── reporter/            # CLI (colored) + JSON output
│   └── validate/            # `validate` subcommand: embedded MQL repo-contract policy
│       └── policy/          #   repo-contract.mql.yaml (Agent Skills + AGENTS.md rules)
├── npm/                     # npm wrapper package (@mondoohq/skillcheck) + launcher
├── scripts/                 # npm packaging/publish + post-release verification
├── .goreleaser.yaml         # Cross-platform builds + GitHub release
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

`.github/workflows/release.yml` then runs three jobs:

1. **release**: goreleaser builds binaries for 6 platforms and creates the GitHub release.
2. **publish-npm**: `scripts/npm-package.mjs` publishes the six
   `@mondoohq/skillcheck_<os>_<arch>` platform packages, waits until they are live on the
   registry, and only then publishes the `@mondoohq/skillcheck` wrapper, which pins them to
   the exact version. Packages already on the registry are skipped, so re-running is safe.
3. **verify-npm**: `scripts/verify-npm-release.sh` waits until all seven packages are on the
   registry, installs the wrapper into a clean directory, and checks the exit codes of
   `skillcheck validate` through the npm launcher.

### npm authentication

Publishing uses [npm trusted publishing](https://docs.npmjs.com/trusted-publishers) (OIDC)
with provenance; there is no npm token. Each of the seven packages has a trusted publisher on
npmjs.com for repository `mondoohq/skillcheck` and workflow `release.yml`. Renaming the
workflow file breaks publishing until those entries are updated.

### Republishing a version

If a release's npm publish failed or is incomplete, run the **Release** workflow manually
(`workflow_dispatch`) with the version, e.g. `0.3.0`. It repackages the binaries attached to
that GitHub release and publishes only the packages missing from the registry; nothing is
rebuilt and no new tag is needed.

### Testing the npm packaging

```bash
make test-npm   # packaging, publish ordering, launcher exit codes
node scripts/npm-package.mjs --version 0.3.0 --from-release   # lay out packages locally, no publish
```
