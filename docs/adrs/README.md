# Architecture Decision Records

This directory holds Architecture Decision Records for Switchyard. An ADR
captures one cross-cutting technical decision: the context that forced it, the
choice that was made, and the consequences that follow.

## When To Write One

Write an ADR when the decision:

- spans more than one package, module, driver, API surface, or runtime process;
- is something a future contributor is likely to ask "why?" about;
- is hard or expensive to reverse, such as wire formats, persistence strategy,
  config language semantics, auth model, or generated-code policy.

Do not write an ADR for routine implementation choices that live inside one
package and can be changed without coordination. Those belong in code, in a
design spec under `docs/agents/specs/`, or in the PR description.

## Format

Use a short Nygard-style shape:

- **Status:** `Proposed`, `Accepted`, or `Superseded by ADR-NNNN`.
- **Date:** ISO date the decision was accepted.
- **Context:** what forced the decision and the constraints.
- **Decision:** the choice, stated plainly.
- **Consequences:** what follows from the choice.

Keep ADRs short. Long ADRs are usually two ADRs.

## Naming

Use `NNNN-short-slug.md`, where `NNNN` is a zero-padded four-digit number and
the slug describes the decision.
