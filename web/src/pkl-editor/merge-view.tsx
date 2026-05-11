// web/src/pkl-editor/merge-view.tsx
import Monaco from "./Monaco";

interface MergeViewProps {
  diskContent: string;
  ancestorContent: string;
  yourContent: string;
  onPickLeft: () => void;
  onPickRight: () => void;
  onSave: (mergedContent: string) => void;
}

export default function MergeView({
  diskContent,
  ancestorContent,
  yourContent,
}: MergeViewProps) {
  return (
    <div
      style={{
        display: "flex",
        flex: 1,
        overflow: "hidden",
        gap: 1,
        background: "var(--sy-color-line)",
      }}
    >
      {(
        [
          { label: "On disk now", content: diskContent, readOnly: true },
          {
            label: "Common ancestor — when you opened it",
            content: ancestorContent,
            readOnly: true,
          },
          { label: "Your changes", content: yourContent, readOnly: false },
        ] as const
      ).map(({ label, content, readOnly }) => (
        <div
          key={label}
          style={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              padding: "4px var(--sy-space-2)",
              fontSize: 11,
              background: "var(--sy-color-surface-1)",
              color: "var(--sy-color-fg-3)",
              borderBottom: "1px solid var(--sy-color-line)",
            }}
          >
            {label}
          </div>
          <div style={{ flex: 1, overflow: "hidden" }}>
            <Monaco
              language="pkl"
              value={content}
              onChange={readOnly ? undefined : () => {}}
              options={{ readOnly }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
