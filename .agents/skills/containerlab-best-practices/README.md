# Containerlab Best Practices Skill

Engineering rules for AI agents writing, reviewing, or refactoring Containerlab. Structured after the [vercel-labs react-best-practices](https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices) skill: many small, single-concern rules grouped into impact-ordered sections.

## Layout

```
SKILL.md            Entry point: when to apply, category table, quick reference.
AGENTS.md           Compiled guide: subsystem map + every rule, in section order.
metadata.json       Version and abstract.
rules/
  _sections.md      Section definitions: prefix -> title, impact, ordering.
  _template.md      Template for a new rule.
  <prefix>-<name>.md One focused rule each (title, explanation, Incorrect, Correct).
```

## Conventions

- The filename prefix (`cli`, `lifecycle`, `architecture`, `contracts`, `topology`, `go`, `tests`) selects the section from `_sections.md`.
- Each rule is one concern: a short explanation plus an Incorrect and a Correct example.
- Examples are grounded in real packages (`links`, `nodes`, `core`, `runtime`, `cmd`, `types`). Reference anchors omit line numbers, which drift.
- `AGENTS.md` is compiled from `rules/` in section order — edit the rule files, then regenerate `AGENTS.md`, not the reverse.

## Adding a Rule

1. Copy `rules/_template.md` to `rules/<prefix>-<short-name>.md`.
2. Keep it to one concern with an Incorrect and a Correct example.
3. Add it to the Quick Reference in `SKILL.md` and to the matching section in `AGENTS.md`.
