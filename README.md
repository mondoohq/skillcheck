# skillcheck

Scan your machine for malicious AI agent skills in seconds.

```bash
npx @mondoohq/skillcheck
```

![skillcheck demo](demo.gif)

skillcheck detects locally installed AI agent skills, computes SHA-256 checksums, and checks them against the [Mondoo AI Agent Security](https://mondoo.com/ai-agent-security) database — covering prompt injection, credential theft, data exfiltration, and 25+ other threat categories across 1,200+ known skills.

## Supported Agents

| Agent | Config | Skills | What's Detected |
|-------|--------|--------|-----------------|
| Antigravity | `~/.gemini/antigravity/` | `~/.gemini/antigravity/skills/` | skills |
| Augment | `~/.augment/` | `~/.augment/skills/` | skills |
| Claude Code | `~/.claude/` | `~/.claude/skills/` | skills, plugins, MCP servers |
| Cline | `~/.cline/` | `~/.cline/skills/` | skills |
| Continue | `~/.continue/` | `~/.continue/skills/` | skills |
| Cursor | `~/.cursor/` | `~/.cursor/skills/` | skills, MCP servers, rules |
| Gemini CLI | `~/.gemini/` | `~/.gemini/skills/` | skills, MCP servers |
| GitHub Copilot | `~/.config/github-copilot/` | `~/.config/github-copilot/skills/` | skills, MCP servers |
| Goose | `~/.config/goose/` | `~/.config/goose/skills/` | skills, extensions |
| IBM Bob | `~/.bob/` | `~/.bob/skills/` | skills |
| Junie | `~/.junie/` | `~/.junie/skills/` | skills |
| Kilo Code | `~/.kilocode/` | `~/.kilocode/skills/` | skills |
| Kiro | `~/.kiro/` | `~/.kiro/skills/` | skills |
| Mistral Vibe | `~/.vibe/` | `~/.vibe/skills/` | skills |
| OpenAI Codex | `~/.codex/` | `~/.codex/skills/` | skills, plugins, MCP servers |
| OpenClaw | `~/.openclaw/` | `~/.openclaw/skills/` | skills |
| OpenCode | `~/.config/opencode/` | `~/.config/opencode/skills/` | skills |
| OpenHands | `~/.openhands/` | `~/.openhands/skills/` | skills |
| Pi | `~/.pi/agent/` | `~/.pi/agent/skills/` | skills |
| Qwen Code | `~/.qwen/` | `~/.qwen/skills/` | skills |
| Roo | `~/.roo/` | `~/.roo/skills/` | skills |
| Snowflake Cortex | `~/.snowflake/cortex/` | `~/.snowflake/cortex/skills/` | skills |
| Trae | `~/.trae/` | `~/.trae/skills/` | skills |
| Warp | `~/.warp/` | `~/.warp/skills/` | skills |
| Windsurf | `~/.codeium/windsurf/` | `~/.codeium/windsurf/skills/` | skills, MCP servers, rules |

## Usage

```bash
# Scan all detected agents
npx @mondoohq/skillcheck

# JSON output for CI/CD pipelines
npx @mondoohq/skillcheck --json

# Verbose output with full hashes and report URLs
npx @mondoohq/skillcheck --verbose
```

### CI/CD Integration

skillcheck exits with code **1** when critical or high-risk skills are found, making it easy to use as a gate:

```yaml
# GitHub Actions
- run: npx @mondoohq/skillcheck
```

```bash
# Any CI pipeline
npx @mondoohq/skillcheck --json --no-color
```

### Other Install Methods

```bash
# Install globally via npm
npm i -g @mondoohq/skillcheck
```

Binaries for macOS, Linux, and Windows are also available on [GitHub Releases](https://github.com/mondoohq/skillcheck/releases).

## Validate a skills repository

Beyond scanning *installed* skills for threats, skillcheck can validate a skills
**repository** you author against the [Agent Skills](https://agentskills.io/specification)
and [AGENTS.md](https://agents.md/) specifications — handy as a CI gate for a skills repo:

```bash
# Validate the current directory (or pass a path)
npx @mondoohq/skillcheck validate .
npx @mondoohq/skillcheck validate ./my-skills-repo --json
```

```text
Validating /path/to/repo

Agent Skills spec (SKILL.md)
  ✓ Every skill name is a lowercase slug (a-z0-9, single hyphens)
  ✓ Every skill name is 1-64 characters
  ✓ Every skill name equals its directory name
  ✓ Every skill description is 1-1024 characters
Repository contract (agents.md + marketplace)
  ✓ A root AGENTS.md exists (agents.md convention)
  ✓ .claude-plugin/marketplace.json exists and lists plugins

PASS 6 passed, 0 failed
```

The rules are an embedded [MQL](https://mondoo.com/docs/mql/home/) policy
(`internal/validate/policy/repo-contract.mql.yaml`), run by the same compiled-in engine
as the scan — no external policy file or cnspec install. It exits **1** if any check
fails. Because the policy is a standard cnspec bundle, `cnspec scan filesystem <repo> -f
repo-contract.mql.yaml` runs the identical checks.

## What Gets Checked

For each detected agent, skillcheck:

1. Discovers installed skills, plugins, MCP servers, and rules
2. Computes a SHA-256 content hash for each skill
3. Queries the [Mondoo skill database](https://mondoo.com/ai-agent-security/skills) for known threats
4. Reports findings with severity, summary, and a link to the full security report

Skills that aren't in the database yet show as clean — skillcheck fails open, never blocks your workflow.

## Links

- [Mondoo AI Agent Security](https://mondoo.com/ai-agent-security)
- [Skill Database](https://mondoo.com/ai-agent-security/skills) — browse 1,200+ analyzed skills
- [Security Checks](https://mondoo.com/ai-agent-security/checks) — 25+ threat categories
