# Agent Artifacts

This directory holds agent-authored planning artifacts so they do not pollute
the product and architecture docs in `docs/`.

| Directory | Purpose |
|-----------|---------|
| `specs/`  | Design specs, one per non-trivial issue or epic. The what and why. |
| `plans/`  | Implementation plans for a spec. The how. |

## Naming

Use `YYYY-MM-DD-short-slug.md` with the UTC date the file was authored.

## Lifecycle

Specs and plans are working documents. They capture intent at the time they
were written and are not kept perfectly in sync with the code afterwards.
Architectural decisions that need to outlive the issue belong in `docs/adrs/`.

## Boundary

- `docs/`: product, user, operations, and long-lived architecture material.
- `docs/agents/`: agent-authored working docs tied to a specific task.
