# ADR Maintenance Guide for AI Agents

This guide covers how to create and maintain ADRs. For when to create an ADR, see the root [AGENTS.md](../AGENTS.md).

## Creating a New ADR

1. Copy [TEMPLATE.md](TEMPLATE.md) to a new file named `YYYY-MM-DD-short-slug.md`
2. Use today's date and a descriptive lowercase slug with hyphens
3. Fill in all sections — the "Considered Alternatives" section is the most valuable part
4. Set status to **Proposed** if seeking feedback, or **Accepted** if the decision is final

## Naming Convention

`YYYY-MM-DD-short-description.md`

- Date prefix for chronological ordering
- Lowercase, hyphens only, no special characters
- Keep the slug short but descriptive

## Superseding an ADR

When a decision is replaced:

1. Update the old ADR's status line: `**Status**: Superseded by [New Title](YYYY-MM-DD-new-slug.md)`
2. Create the new ADR with its own context explaining why the previous decision changed
3. Reference the old ADR in the new one's Context section

## Format Rules

- Follow the structure in [TEMPLATE.md](TEMPLATE.md)
- End every file with a trailing newline
- Use the existing ADRs as examples for tone and level of detail
