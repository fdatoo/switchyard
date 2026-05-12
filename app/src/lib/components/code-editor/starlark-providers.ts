import * as monaco from "monaco-editor";
import {
  complete,
  diagnose,
  hover,
  lookupSymbol,
  tokenize,
  type CompletionItem as DaemonCompletion,
  type Diagnostic as DaemonDiagnostic,
  type TokenSpan,
} from "@/data/starlark-ls";

let registered = false;
let armedModels = new WeakSet<monaco.editor.ITextModel>();

export function setupStarlarkProviders(): void {
  if (registered) return;
  registered = true;

  monaco.languages.registerCompletionItemProvider("starlark", {
    triggerCharacters: ["."],
    provideCompletionItems: async (model, position) => {
      try {
        const r = await complete({
          filePath: model.uri.path,
          source: model.getValue(),
          line: position.lineNumber,
          col: position.column - 1,
        });
        const word = model.getWordUntilPosition(position);
        const range: monaco.IRange = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };
        return {
          suggestions: r.items.map((it: DaemonCompletion) => ({
            label: it.label,
            kind: completionKindOf(it.kind),
            detail: it.detail,
            insertText: it.insertText || it.label,
            range,
          })),
        };
      } catch {
        return { suggestions: [] };
      }
    },
  });

  monaco.languages.registerHoverProvider("starlark", {
    provideHover: async (model, position) => {
      try {
        const r = await hover({
          filePath: model.uri.path,
          source: model.getValue(),
          line: position.lineNumber,
          col: position.column - 1,
        });
        if (!r.markdown) return null;
        const word = model.getWordAtPosition(position);
        const range: monaco.IRange | undefined = word ? {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        } : undefined;
        return { contents: [{ value: r.markdown }], range };
      } catch {
        return null;
      }
    },
  });

  monaco.languages.registerDefinitionProvider("starlark", {
    provideDefinition: async (model, position) => {
      const word = model.getWordAtPosition(position);
      if (!word) return null;
      try {
        const r = await lookupSymbol({ name: word.word });
        if (!r.filePath) return null;
        window.dispatchEvent(new CustomEvent("starlark-goto-definition", {
          detail: { filePath: r.filePath, line: r.line },
        }));
        return {
          uri: model.uri,
          range: {
            startLineNumber: position.lineNumber,
            endLineNumber: position.lineNumber,
            startColumn: word.startColumn,
            endColumn: word.endColumn,
          },
        };
      } catch {
        return null;
      }
    },
  });

  monaco.languages.registerDocumentSemanticTokensProvider("starlark", {
    getLegend: () => ({
      tokenTypes: ["keyword", "identifier", "string", "number", "comment", "operator"],
      tokenModifiers: [],
    }),
    provideDocumentSemanticTokens: async (model) => {
      try {
        const r = await tokenize({
          filePath: model.uri.path,
          source: model.getValue(),
        });
        return { data: encodeSemanticTokens(r.spans), resultId: undefined };
      } catch {
        return { data: new Uint32Array(0), resultId: undefined };
      }
    },
    releaseDocumentSemanticTokens: () => {},
  });

  for (const model of monaco.editor.getModels()) {
    if (model.getLanguageId() === "starlark") armDiagnostics(model);
  }
  monaco.editor.onDidCreateModel((model) => {
    if (model.getLanguageId() === "starlark") armDiagnostics(model);
  });
}

export function __resetStarlarkProvidersForTest(): void {
  registered = false;
  armedModels = new WeakSet<monaco.editor.ITextModel>();
}

export function completionKindOf(kind: string): monaco.languages.CompletionItemKind {
  switch (kind) {
    case "function":
      return monaco.languages.CompletionItemKind.Function;
    case "variable":
      return monaco.languages.CompletionItemKind.Variable;
    case "keyword":
      return monaco.languages.CompletionItemKind.Keyword;
    default:
      return monaco.languages.CompletionItemKind.Text;
  }
}

const TOKEN_TYPE_INDEX: Record<string, number> = {
  keyword: 0,
  identifier: 1,
  string: 2,
  number: 3,
  comment: 4,
  operator: 5,
};

export function encodeSemanticTokens(spans: TokenSpan[]): Uint32Array {
  const sorted = [...spans].sort((a, b) => a.startLine - b.startLine || a.startCol - b.startCol);
  const out: number[] = [];
  let prevLine = 0;
  let prevCol = 0;
  for (const span of sorted) {
    const line = span.startLine - 1;
    const col = span.startCol;
    const length = span.endLine === span.startLine ? Math.max(1, span.endCol - span.startCol) : 1;
    const typeIdx = TOKEN_TYPE_INDEX[span.tokenType] ?? TOKEN_TYPE_INDEX.identifier;
    const deltaLine = line - prevLine;
    const deltaCol = deltaLine === 0 ? col - prevCol : col;
    out.push(deltaLine, deltaCol, length, typeIdx, 0);
    prevLine = line;
    prevCol = col;
  }
  return new Uint32Array(out);
}

function armDiagnostics(model: monaco.editor.ITextModel): void {
  if (armedModels.has(model)) return;
  armedModels.add(model);

  let timer: number | null = null;
  const fire = (): void => {
    if (timer !== null) window.clearTimeout(timer);
    timer = window.setTimeout(async () => {
      try {
        const r = await diagnose({ filePath: model.uri.path, source: model.getValue() });
        monaco.editor.setModelMarkers(model, "starlark", r.diagnostics.map(diagToMonaco));
      } catch {
        monaco.editor.setModelMarkers(model, "starlark", []);
      }
    }, 300);
  };
  model.onDidChangeContent(fire);
  model.onWillDispose(() => {
    if (timer !== null) window.clearTimeout(timer);
  });
  fire();
}

export function diagToMonaco(d: DaemonDiagnostic): monaco.editor.IMarkerData {
  return {
    severity: d.severity === "error" ? monaco.MarkerSeverity.Error : monaco.MarkerSeverity.Warning,
    message: d.message,
    code: d.code,
    startLineNumber: d.startLine,
    startColumn: d.startCol + 1,
    endLineNumber: d.endLine,
    endColumn: d.endCol + 1,
  };
}
