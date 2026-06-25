# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the DevWorkspace Operator.

## What is an ADR?

An ADR captures a significant design decision along with its context, alternatives considered, and trade-offs accepted. ADRs preserve the **why** behind decisions — information that is lost in code, commit messages, and PR descriptions over time.

## When to Write an ADR

Write an ADR when your change involves any of these:

1. **You rejected an alternative** — There were 2+ reasonable approaches and you picked one. The code shows what you chose but not what you didn't.
2. **You accepted a trade-off** — Something got worse (performance, complexity, a minor leak) in exchange for something more important.
3. **You changed a resource lifecycle or ownership model** — Who creates, owns, or cleans up a Kubernetes resource.
4. **You changed an external contract** — API shape, CRD fields, Secret/ConfigMap naming conventions, image paths.

**Don't** write an ADR for: bug fixes, dependency bumps, refactors preserving behavior, test additions, docs updates, or performance optimizations with no trade-offs.

## Lifecycle

| Status | Meaning |
|--------|---------|
| **Proposed** | Under discussion, not yet accepted |
| **Accepted** | Decision is in effect |
| **Deprecated** | Decision is outdated but not yet replaced |
| **Superseded** | Replaced by a newer ADR (link to it in the status line) |

To supersede an ADR, update its status to `Superseded by [new ADR title](new-adr-file.md)` and create a new ADR.

## Naming Convention

Files use date-prefixed slugs: `YYYY-MM-DD-short-description.md`

Example: `2026-05-11-backup-auth-secret-lifecycle.md`

## Creating a New ADR

Copy [TEMPLATE.md](TEMPLATE.md) and fill in the sections. See existing ADRs for examples.
