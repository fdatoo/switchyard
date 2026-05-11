// web/src/pkl-editor/languages/starlark.ts
import type { languages } from "monaco-editor";

export const STARLARK_LANGUAGE_ID = "starlark";

export const starlarkLanguageDefinition: languages.IMonarchLanguage & {
  keywords: string[];
  builtins: string[];
} = {
  defaultToken: "",
  tokenPostfix: ".star",

  keywords: [
    "and",
    "break",
    "continue",
    "def",
    "elif",
    "else",
    "for",
    "if",
    "in",
    "lambda",
    "load",
    "not",
    "or",
    "pass",
    "return",
    "True",
    "False",
    "None",
  ],

  builtins: [
    "abs",
    "all",
    "any",
    "bool",
    "dict",
    "dir",
    "enumerate",
    "fail",
    "float",
    "getattr",
    "hasattr",
    "hash",
    "int",
    "len",
    "list",
    "max",
    "min",
    "print",
    "range",
    "repr",
    "reversed",
    "set",
    "sorted",
    "str",
    "tuple",
    "type",
    "zip",
  ],

  tokenizer: {
    root: [
      // Line comment
      [/#.*$/, "comment.line"],
      // Strings
      [/"(?:[^"\\]|\\.)*"/, "string"],
      [/'(?:[^'\\]|\\.)*'/, "string"],
      [/"""[\s\S]*?"""/, "string.multiline"],
      [/'''[\s\S]*?'''/, "string.multiline"],
      // Numbers
      [/\b\d+(\.\d+)?\b/, "number"],
      // All-caps constants
      [/\b[A-Z_][A-Z0-9_]*\b/, "variable.constant"],
      // Keywords, builtins, identifiers
      [
        /\b[a-z_][a-zA-Z0-9_]*\b/,
        {
          cases: {
            "@keywords": "keyword",
            "@builtins": "support.function",
            "@default": "identifier",
          },
        },
      ],
      // Brackets
      [/[{}[\]()]/, "@brackets"],
      // Operators
      [/[=!<>+\-*/%&|^~]+/, "operator"],
    ],
  },
};

export function registerStarlarkLanguage(
  monaco: typeof import("monaco-editor")
): void {
  if (
    monaco.languages.getLanguages().some((l) => l.id === STARLARK_LANGUAGE_ID)
  ) {
    return;
  }
  monaco.languages.register({
    id: STARLARK_LANGUAGE_ID,
    extensions: [".star"],
  });
  monaco.languages.setMonarchTokensProvider(
    STARLARK_LANGUAGE_ID,
    starlarkLanguageDefinition
  );
  monaco.languages.setLanguageConfiguration(STARLARK_LANGUAGE_ID, {
    comments: { lineComment: "#" },
    brackets: [
      ["{", "}"],
      ["[", "]"],
      ["(", ")"],
    ],
    autoClosingPairs: [
      { open: "{", close: "}" },
      { open: "[", close: "]" },
      { open: "(", close: ")" },
      { open: '"', close: '"' },
      { open: "'", close: "'" },
    ],
  });
}
