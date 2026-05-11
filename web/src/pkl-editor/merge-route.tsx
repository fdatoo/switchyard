// web/src/pkl-editor/merge-route.tsx
// 3-way merge route: /_authed/pkl-editor/merge/<file>?session=<id>
import { useState } from "react";
import MergeView from "./merge-view";

const MERGE_PREFIX = "/_authed/pkl-editor/merge/";

function getFilePath(): string {
  const path = window.location.pathname;
  if (path.startsWith(MERGE_PREFIX)) {
    return path.slice(MERGE_PREFIX.length);
  }
  return "";
}

function getSessionId(): string {
  const params = new URLSearchParams(window.location.search);
  return params.get("session") ?? "";
}

export default function MergeRoute() {
  const filePath = getFilePath();
  const sessionId = getSessionId();

  // Placeholder content — real implementation fetches from ConfigService using sessionId.
  const [diskContent] = useState(`// On-disk version of ${filePath}`);
  const [ancestorContent] = useState(`// Common ancestor of ${filePath}`);
  const [yourContent] = useState(`// Your in-memory changes to ${filePath}`);

  const handleSave = (_merged: string) => {
    // Call ConfigService.CommitEdit(filePath, lockToken, merged, expectedHash, force=true)
    console.info("save merged result for session", sessionId);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh" }}>
      <div
        style={{
          padding: "var(--sy-space-2) var(--sy-space-3)",
          fontSize: 12,
          background: "var(--sy-color-warn)",
          color: "var(--sy-color-bg)",
          flexShrink: 0,
        }}
      >
        Merge conflict in <strong>{filePath}</strong> — pick changes then
        &ldquo;Save merged result&rdquo;.
      </div>
      <MergeView
        diskContent={diskContent}
        ancestorContent={ancestorContent}
        yourContent={yourContent}
        onPickLeft={() => {}}
        onPickRight={() => {}}
        onSave={handleSave}
      />
      <div
        style={{
          display: "flex",
          justifyContent: "flex-end",
          padding: "var(--sy-space-2) var(--sy-space-3)",
          borderTop: "1px solid var(--sy-color-line)",
          gap: "var(--sy-space-2)",
          flexShrink: 0,
        }}
      >
        <button onClick={() => history.back()}>Cancel</button>
        <button
          onClick={() => handleSave(yourContent)}
          style={{
            background: "var(--sy-color-accent)",
            color: "var(--sy-color-bg)",
            border: "none",
            padding: "4px 12px",
            borderRadius: "var(--sy-radius-sm)",
            cursor: "pointer",
          }}
        >
          Save merged result
        </button>
      </div>
    </div>
  );
}
