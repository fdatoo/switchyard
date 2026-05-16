import * as monaco from "monaco-editor";
import {
  complete,
  definition,
  diagnose,
  hover,
  semanticTokens,
  type CompletionItem as DaemonCompletion,
  type Diagnostic as DaemonDiagnostic,
} from "@/data/pkl-ls";

let registered = false;
let armedModels = new WeakSet<monaco.editor.ITextModel>();

export function setupPklProviders(): void {
  if (registered) return;
  registered = true;

  monaco.languages.registerCompletionItemProvider("pkl", {
    triggerCharacters: [".", "\""],
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

  monaco.languages.registerHoverProvider("pkl", {
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

  monaco.languages.registerDefinitionProvider("pkl", {
    provideDefinition: async (model, position) => {
      try {
        const r = await definition({
          filePath: model.uri.path,
          source: model.getValue(),
          line: position.lineNumber,
          col: position.column - 1,
        });
        if (!r.filePath) return null;
        window.dispatchEvent(new CustomEvent("pkl-goto-definition", {
          detail: { filePath: r.filePath, line: r.line, col: r.col },
        }));
        return {
          uri: model.uri,
          range: {
            startLineNumber: position.lineNumber,
            endLineNumber: position.lineNumber,
            startColumn: position.column,
            endColumn: position.column,
          },
        };
      } catch {
        return null;
      }
    },
  });

  monaco.languages.registerDocumentSemanticTokensProvider("pkl", {
    getLegend: () => ({
      tokenTypes: [
        "keyword",
        "comment",
        "string",
        "number",
        "operator",
        "type",
        "class",
        "interface",
        "function",
        "method",
        "property",
        "parameter",
        "variable",
        "namespace",
      ],
      tokenModifiers: ["declaration", "readonly", "documentation"],
    }),
    provideDocumentSemanticTokens: async (model) => {
      try {
        return await semanticTokens({
          filePath: model.uri.path,
          source: model.getValue(),
        });
      } catch {
        return { data: new Uint32Array(0), resultId: undefined };
      }
    },
    releaseDocumentSemanticTokens: () => {},
  });

  for (const model of monaco.editor.getModels()) {
    if (model.getLanguageId() === "pkl") armDiagnostics(model);
  }
  monaco.editor.onDidCreateModel((model) => {
    if (model.getLanguageId() === "pkl") armDiagnostics(model);
  });
}

export function __resetPklProvidersForTest(): void {
  registered = false;
  armedModels = new WeakSet<monaco.editor.ITextModel>();
}

function completionKindOf(kind: string): monaco.languages.CompletionItemKind {
  switch (kind) {
    case "function":
      return monaco.languages.CompletionItemKind.Function;
    case "type":
      return monaco.languages.CompletionItemKind.Class;
    case "module":
      return monaco.languages.CompletionItemKind.Module;
    case "variable":
      return monaco.languages.CompletionItemKind.Variable;
    case "keyword":
      return monaco.languages.CompletionItemKind.Keyword;
    default:
      return monaco.languages.CompletionItemKind.Text;
  }
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
        monaco.editor.setModelMarkers(model, "pkl", r.diagnostics.map(diagToMonaco));
      } catch {
        monaco.editor.setModelMarkers(model, "pkl", []);
      }
    }, 300);
  };
  model.onDidChangeContent(fire);
  model.onWillDispose(() => {
    if (timer !== null) window.clearTimeout(timer);
  });
  fire();
}

function diagToMonaco(d: DaemonDiagnostic): monaco.editor.IMarkerData {
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
