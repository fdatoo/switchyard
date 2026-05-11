# Edit Sessions — Protocol Reference

> **For implementers of Plan 10 (automation editor) and Plan 12 (Pkl editor).**
> This document describes the five RPCs, lock semantics, reconnect contract,
> and the file-only regions contract.

## Five RPCs

### `OpenForEdit(file_path) → OpenForEditResponse`
Opens a file for editing. Returns:
- `session_id` — unique identifier for this session.
- `lock_token` — opaque token required for Commit/Abandon. Multiple sessions on the same file get distinct tokens; first-to-commit wins.
- `file_hash` — SHA-256 hex of the file at open time. Pass back in `CommitEdit.expected_file_hash`.
- `ancestor_pkl` — full Pkl text at open time. Passed to the 3-way merge surface.
- `ast_json` — JSON-encoded AST for client hydration (currently `{}`; will expand).

### `CommitEdit(file_path, lock_token, regenerated_pkl, expected_file_hash, force) → CommitEditResponse`
Writes the regenerated Pkl to disk. Two possible responses:
- **CommitSuccess** — lock valid + hashes match (or `force=true`). File written.
- **CommitConflict** — lock valid but `expected_file_hash` ≠ current disk hash and `force=false`. Contains `disk_hash`, `disk_pkl`, and `ancestor_pkl` for the merge surface.

Error codes:
- `FAILED_PRECONDITION` with message `LOCK_EXPIRED` — session TTL elapsed; re-open.
- `PERMISSION_DENIED` — invalid lock token.

### `AbandonEdit(file_path, lock_token) → AbandonEditResponse`
Releases the session. Idempotent; safe to fire-and-forget on page unload.

### `SessionEvents(session_id) → stream SessionEvent`
Long-lived SSE stream. Delivers:
- `ExternalEditDetected { file_path, new_hash, modified_at }` — the watched file changed externally.
- `SessionHeartbeat { server_time }` — sent every 5 minutes to reset the server-side TTL.

**Reconnect contract:** the server buffers events for 60 seconds after a stream disconnects. Reconnect with the same `session_id` to drain buffered events.

### `AnalyzeRegenerability(file_path) → RegenerabilityReport`
Stateless; no open session required. Returns `FileOnlyRegion` entries with:
- `start_line`, `end_line` — 1-indexed, inclusive.
- `reason` — one of `starlark_call | import | let_binding | nondeterministic`.

## Lock Semantics

- **Soft lock, not exclusive.** Multiple sessions on the same file are permitted.
- **TTL: 30 minutes.** Heartbeat (via `SessionEvents` stream) resets the TTL.
- **First-to-commit wins.** Subsequent sessions receive `CommitConflict`.
- **In-memory only.** Server restart clears all sessions; clients must handle `LOCK_EXPIRED`.

## Consuming `useEditSession` (Plan 10 / Plan 12)

```tsx
const session = useEditSession(filePath, editSessionClient);

// Render conflict banner when session.conflict is non-null:
{session.conflict && (
  <ConflictBanner
    filePath={filePath}
    dirtyCount={session.dirtyCount}
    onDiscard={() => session.discard()}
    onForceOverwrite={() =>
      session.resolveConflict({ kind: "force", stagedPkl: staged }, staged)
    }
    onOpenMerge={() => {
      // TODO(plan-12): real Monaco merge view
      navigate(`/_authed/pkl-editor/merge/${filePath}?session=${session.sessionId}`);
    }}
  />
)}
```

## File-Only Regions

Regions where the regenerator cannot round-trip the AST. The web client should:
- Disable edit affordances for those line ranges.
- Render a "View source" link pointing to the relevant line range in the Pkl editor (Plan 12).

```tsx
const { fileOnlyRegions } = useEditSession(filePath, client);
// fileOnlyRegions: Array<{ startLine, endLine, reason }>
```
