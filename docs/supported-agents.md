# Supported agents

skillcheck looks for each agent below in your home directory and scans whatever it finds.
Agents that aren't installed are skipped.

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

Missing an agent? [Open an issue](https://github.com/mondoohq/skillcheck/issues) with
where it keeps its config and skills.
