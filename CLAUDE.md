@AGENTS.md

## Claude Code specifics

- Skills referenced in AGENTS.md are installed at user level (`~/.claude/skills`, plus the plugins `superpowers`, `playwright`, `context7`). If a skill isn't listed in your session, tell the user rather than imitating it from memory.
- Use the Bash tool with POSIX syntax. The primary dev machine is Windows (Git Bash / PowerShell), so keep scripts in Bun/TypeScript or Go rather than bash where you can.
- Before committing, run `bun run lint && bun run test`.
