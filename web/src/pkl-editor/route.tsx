// web/src/pkl-editor/route.tsx
// Main editor route: /_authed/pkl-editor/*
// The file path is read from the URL after /pkl-editor/.
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type * as monacoTypes from "monaco-editor";
import Monaco from "./Monaco";
import FileTree, { type FileEntry } from "./FileTree";
import Inspector from "./Inspector";
import AstBreadcrumb from "./AstBreadcrumb";
import StatusBar from "./StatusBar";
import { type FormBoundRegion } from "./form-bound-decorations";
import { findStarlarkRegions } from "./embedded";
import { registerPklLanguage } from "./languages/pkl";
import { registerStarlarkLanguage, STARLARK_LANGUAGE_ID } from "./languages/starlark";
import { AppRail } from "../shell/AppRail";
import "./pkl-editor.css";

export interface PklEditorRouteProps {
  /** File path relative to ~/.switchyard/. Read from the URL splat in production. */
  filePath?: string;
}

export default function PklEditorRoute({ filePath: propFilePath }: PklEditorRouteProps) {
  // Derive filePath from URL when not supplied as a prop (production use).
  const [filePath] = useState(() => {
    if (propFilePath) return propFilePath;
    const prefix = "/_authed/pkl-editor/";
    const path = window.location.pathname;
    return path.startsWith(prefix) ? path.slice(prefix.length) : "";
  });

  const [files, setFiles] = useState<FileEntry[]>([]);
  const [content, setContent] = useState("");
  const [astPath, setAstPath] = useState<string[]>([]);
  const [formBoundRegions] = useState<FormBoundRegion[]>([]);
  const [problems, setProblems] = useState<
    Array<{ line: number; message: string; severity: "error" | "warning" }>
  >([]);
  const [cursorLine, setCursorLine] = useState(1);
  const [cursorCol, setCursorCol] = useState(1);
  const editorRef =
    useRef<monacoTypes.editor.IStandaloneCodeEditor | null>(null);

  const starlarkRegions = useMemo(() => findStarlarkRegions(content), [content]);

  // Register languages once Monaco loads.
  const handleEditorMount = useCallback(
    (
      editor: monacoTypes.editor.IStandaloneCodeEditor,
      monaco: typeof monacoTypes
    ) => {
      registerPklLanguage(monaco);
      registerStarlarkLanguage(monaco);
      editorRef.current = editor;

      editor.onDidChangeCursorPosition((e) => {
        setCursorLine(e.position.lineNumber);
        setCursorCol(e.position.column);
      });

      // Register Starlark autocomplete provider (lazy import to avoid circular dep)
      import("../data/starlarkls-client").then(({ starlarkLsClient }) => {
        monaco.languages.registerCompletionItemProvider(STARLARK_LANGUAGE_ID, {
          triggerCharacters: [".", "_"],
          provideCompletionItems: async (model, position) => {
            const source = model.getValue();
            try {
              const resp = await starlarkLsClient.complete(
                filePath,
                source,
                position.lineNumber,
                position.column
              );
              return {
                suggestions: resp.items.map((item) => ({
                  label: item.label,
                  kind:
                    item.kind === "function"
                      ? monaco.languages.CompletionItemKind.Function
                      : monaco.languages.CompletionItemKind.Variable,
                  detail: item.detail,
                  insertText: item.insertText,
                  range: {
                    startLineNumber: position.lineNumber,
                    startColumn: position.column,
                    endLineNumber: position.lineNumber,
                    endColumn: position.column,
                  },
                })),
              };
            } catch {
              return { suggestions: [] };
            }
          },
        });

        monaco.languages.registerHoverProvider(STARLARK_LANGUAGE_ID, {
          provideHover: async (model, position) => {
            const source = model.getValue();
            try {
              const resp = await starlarkLsClient.hover(
                filePath,
                source,
                position.lineNumber,
                position.column
              );
              if (!resp.markdown) return null;
              return { contents: [{ value: resp.markdown }] };
            } catch {
              return null;
            }
          },
        });
      });
    },
    [filePath]
  );

  // Load file content — in production, calls ConfigService.OpenForEdit(filePath).
  useEffect(() => {
    if (!filePath) return;
    // Placeholder: real content comes from ConfigService.OpenForEdit
    setContent(`// ${filePath}\n`);
    setFiles([{ path: filePath, dirty: false, hasError: false }]);
    // Derive a simple breadcrumb from the file path
    const parts = filePath.split("/");
    setAstPath(parts);
  }, [filePath]);

  const handleFormat = () => {
    // ConfigService.FormatFile(filePath, content) → formatted string
  };

  const handleValidate = () => {
    // ConfigService.ValidateFile(filePath, content) → problems
    // setProblems(result.problems);
    void setProblems;
  };

  const handleApply = () => {
    // Plan 11 CommitEdit
  };

  const handleRevealFormEditor = (editorId: string) => {
    const slug = editorId.replace(/\.pkl$/, "").replace(/\//g, "/");
    window.location.href = `/_authed/${slug}`;
  };

  const handleSelectFile = (path: string) => {
    window.history.pushState(null, "", `/_authed/pkl-editor/${path}`);
    window.location.reload();
  };

  const unsavedCount = files.filter((f) => f.dirty).length;

  return (
    <div
      data-testid="pkl-editor-root"
      style={{ display: "flex", height: "100vh", overflow: "hidden" }}
    >
      {/* AppRail — rendered on pkl-editor routes per plan decision 3 */}
      <AppRail />
      <FileTree
        files={files}
        activePath={filePath}
        onSelect={handleSelectFile}
        onSearch={() => {
          /* open ⌘P palette scoped to files */
        }}
      />
      <div
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <AstBreadcrumb path={astPath} />
        <div style={{ flex: 1, overflow: "hidden" }}>
          <Monaco
            language={filePath.endsWith(".star") ? "starlark" : "pkl"}
            value={content}
            onChange={setContent}
            onEditorMount={handleEditorMount}
            data-testid="editor"
          />
        </div>
        <StatusBar
          pklVersion="0.27"
          unsavedCount={unsavedCount}
          errorCount={problems.filter((p) => p.severity === "error").length}
          formBoundCount={formBoundRegions.length}
          line={cursorLine}
          col={cursorCol}
          onFormat={handleFormat}
          onValidate={handleValidate}
          onApply={handleApply}
        />
      </div>
      <Inspector
        filePath={filePath}
        cursorLine={cursorLine}
        cursorCol={cursorCol}
        formBoundRegions={formBoundRegions}
        starlarkRegions={starlarkRegions}
        problems={problems}
        onRevealFormEditor={handleRevealFormEditor}
      />
    </div>
  );
}
