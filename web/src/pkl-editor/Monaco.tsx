// web/src/pkl-editor/Monaco.tsx
// Monaco Editor wrapper — loaded lazily via dynamic import() so that
// monaco-editor NEVER appears in the base bundle.
import { lazy, Suspense, useEffect, useRef } from "react";
import type * as monacoTypes from "monaco-editor";

export interface MonacoProps {
  language: "pkl" | "starlark" | "plaintext";
  value: string;
  onChange?: (value: string) => void;
  options?: Record<string, unknown>;
  onEditorMount?: (
    editor: monacoTypes.editor.IStandaloneCodeEditor,
    monaco: typeof monacoTypes
  ) => void;
  "data-testid"?: string;
}

// The inner component lives in a separate module-scoped function to avoid
// TS issues with the lazy factory closure. Types come from the `type` import
// above (erased at runtime — zero cost).
function createMonacoImpl(m: typeof monacoTypes) {
  return function MonacoEditorImpl({
    language,
    value,
    onChange,
    options,
    onEditorMount,
  }: MonacoProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const editorRef =
      useRef<monacoTypes.editor.IStandaloneCodeEditor | null>(null);

    useEffect(() => {
      if (!containerRef.current) return;
      const editor = m.editor.create(containerRef.current, {
        value,
        language,
        theme: "vs-dark",
        automaticLayout: true,
        fontSize: 13,
        lineNumbers: "on",
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        ...(options as monacoTypes.editor.IStandaloneEditorConstructionOptions),
      });
      editorRef.current = editor;
      onEditorMount?.(editor, m);
      const sub = editor.onDidChangeModelContent(() => {
        onChange?.(editor.getValue());
      });
      return () => {
        sub.dispose();
        editor.dispose();
      };
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    return (
      <div
        ref={containerRef}
        style={{ width: "100%", height: "100%" }}
      />
    );
  };
}

// Dynamic import — Monaco must never appear in the base chunk.
const MonacoEditorInner = lazy(() =>
  import("monaco-editor").then((m) => ({
    default: createMonacoImpl(m),
  }))
);

/**
 * Monaco editor with Suspense boundary.
 *
 * Monaco is loaded lazily — the base bundle will never include monaco-editor.
 * Verify with: task web:build && grep -r "monaco" internal/web/dist/assets/index-*.js | wc -l
 * (should be 0)
 */
export default function Monaco(props: MonacoProps) {
  return (
    <Suspense
      fallback={
        <div
          className="editor-loading"
          data-testid="editor-loading"
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            width: "100%",
            height: "100%",
            color: "var(--sy-color-fg-3)",
            fontSize: 13,
            background: "var(--sy-color-surface-1)",
          }}
        >
          Loading editor…
        </div>
      }
    >
      <MonacoEditorInner {...props} />
    </Suspense>
  );
}
