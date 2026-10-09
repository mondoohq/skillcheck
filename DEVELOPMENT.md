# Development

## Prerequisites

- Go 1.25+
- Access to `go.mondoo.com/mql (v14)` module

## Build

```bash
make build
make install   # into $GOBIN, or $GOPATH/bin
make test
make lint
```

## Update MQL schemas

The embedded schemas (`internal/engine/schemas/`) must match the mql version in `go.mod`.
After bumping mql, regenerate them:

```bash
go get go.mondoo.com/mql@<version or commit> && go mod tidy
make schemas
```

`make schemas` runs `scripts/gen-schemas.sh`, which builds mql's `mqlr` tool from the pinned
module and generates `os.resources.json` and `core.resources.json` from its `.lr` files (mql
does not commit the JSON). No local mql checkout is needed. CI fails when the committed
schemas differ from what the pinned version generates.

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

### Slack notifications

When `SLACK_RELEASE_CHANNEL_ID` is set, the release posts a Slack
message when it starts (every stage pending) and updates it in place with the result of each
stage when it ends. `scripts/slack-release-message.mjs` builds both messages.

| Setting | Kind | Purpose |
|---|---|---|
| `SLACK_BOT_TOKEN` | secret | Bot token with `chat:write`; the bot must be in the channel |
| `SLACK_RELEASE_CHANNEL_ID` | secret or variable | Channel to post to; unset disables notifications |
| `SLACK_RELEASE_ALERT_GROUP_ID` | secret or variable | Optional Slack user group mentioned when a release needs attention |

Each can be an organization or repository secret; the channel and alert group can also be
repository variables. A secret wins when both are set.

Notifications never block or fail a release.

### Testing the npm packaging

```bash
make test-npm   # packaging, publish ordering, launcher exit codes
node scripts/npm-package.mjs --version 0.3.0 --from-release   # lay out packages locally, no publish
```
