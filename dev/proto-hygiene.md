# Proto Hygiene Rules

Applies to every `.proto` file in this repo.

## Rules

1. **Grouped numbering.** In any `oneof` or message with 3+ fields, group fields by semantic
   category using tens-aligned blocks: identity (1-9), primary domain payload (10-19), metadata
   (20-29, 30-39, ...), transport/health (90-99). Intermediate blocks (40-49, 50-59, 60-69,
   70-79, 80-89) follow the same pattern and can be used for additional categories.

2. **Range-comment headers.** Every group gets a one-line comment above its first member:
   `// 10-19: state-plane events`. The header is the contract for future additions — new fields
   in a category pick the next free tag *within that block*.

3. **`reserved` on removal, forever.** Any field number OR oneof tag removed from a message
   MUST be added to a `reserved N;` statement in the same message (and the old name in
   `reserved "old_name";`). Field numbers are never reused, even across breaking package
   versions.

4. **Tag stability is part of the wire contract.** Field numbers and oneof tags do not change
   within a protocol version (`v1alpha1`). Breaking changes move to a new package suffix
   (`v1alpha2`, or eventually `v1`).

5. **`v1alpha*` vs `v1`.** Packages suffixed `v1alpha*` may make wire-breaking changes between
   releases with a migration note. Graduating to `v1` is a one-way door and requires a decision
   record entry in the relevant design doc.
