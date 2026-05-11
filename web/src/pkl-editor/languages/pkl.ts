// web/src/pkl-editor/languages/pkl.ts
// Ported from apple/pkl-vscode (Apache-2.0):
// https://github.com/apple/pkl-vscode/blob/main/syntaxes/pkl.tmLanguage.json
// This file adapts relevant token patterns to Monaco's IMonarchTokensProvider format.
import type { languages } from "monaco-editor";

export const PKL_LANGUAGE_ID = "pkl";

export const pklLanguageDefinition: languages.IMonarchLanguage & {
  keywords: string[];
  builtins: string[];
} = {
  defaultToken: "",
  tokenPostfix: ".pkl",

  keywords: [
    "amends",
    "module",
    "class",
    "typealias",
    "extends",
    "import",
    "import*",
    "function",
    "local",
    "hidden",
    "fixed",
    "const",
    "abstract",
    "open",
    "external",
    "let",
    "when",
    "is",
    "as",
    "new",
    "this",
    "outer",
    "super",
    "null",
    "true",
    "false",
    "nothing",
    "unknown",
    "if",
    "else",
    "for",
    "in",
    "read",
    "read?",
    "read*",
    "throw",
    "trace",
    "import@",
  ],

  builtins: [
    "String",
    "Int",
    "Float",
    "Boolean",
    "Duration",
    "DataSize",
    "Pair",
    "List",
    "Set",
    "Map",
    "Listing",
    "Mapping",
    "Dynamic",
    "Any",
    "Number",
    "Regex",
    "Resource",
  ],

  operators: [
    "=",
    "!=",
    "==",
    "<",
    ">",
    "<=",
    ">=",
    "&&",
    "||",
    "!",
    "?",
    "??",
    "|>",
    "->",
    "?.",
    "...",
    "@",
  ],

  tokenizer: {
    root: [
      // Doc comments (///)
      [/\/\/\/.*$/, "comment.doc"],
      // Line comments (//)
      [/\/\/.*$/, "comment.line"],
      // Block comment opening
      [/\/\*/, "comment.block", "@comment"],
      // Double-quoted strings
      [/"/, "string", "@string_double"],
      // Raw multiline strings (#""")
      [/#"/, "string", "@string_multiline"],
      // Language constants
      [/\b(true|false|null|nothing)\b/, "constant.language"],
      // Numbers
      [/\b\d+(\.\d+)?\b/, "number"],
      // Types (PascalCase identifiers)
      [
        /\b[A-Z][a-zA-Z0-9_]*\b/,
        { cases: { "@builtins": "type.builtin", "@default": "type" } },
      ],
      // Keywords and identifiers
      [
        /\b[a-z_$][a-zA-Z0-9_$]*\b/,
        { cases: { "@keywords": "keyword", "@default": "identifier" } },
      ],
      // Brackets
      [/[{}[\]()]/, "@brackets"],
      // Operators
      [/[=!<>|&?]+/, "operator"],
    ],

    comment: [
      [/[^/*]+/, "comment.block"],
      [/\*\//, "comment.block", "@pop"],
      [/[/*]/, "comment.block"],
    ],

    string_double: [
      [/[^"\\]+/, "string"],
      [/\\\\./, "string.escape"],
      [/"/, "string", "@pop"],
    ],

    string_multiline: [
      [/[^"#]+/, "string"],
      [/"(?!##)/, "string"],
      [/#"/, "string", "@pop"],
    ],
  },
};

export function registerPklLanguage(
  monaco: typeof import("monaco-editor")
): void {
  if (monaco.languages.getLanguages().some((l) => l.id === PKL_LANGUAGE_ID)) {
    return;
  }
  monaco.languages.register({ id: PKL_LANGUAGE_ID, extensions: [".pkl"] });
  monaco.languages.setMonarchTokensProvider(
    PKL_LANGUAGE_ID,
    pklLanguageDefinition
  );
  monaco.languages.setLanguageConfiguration(PKL_LANGUAGE_ID, {
    comments: { lineComment: "//", blockComment: ["/*", "*/"] },
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
    ],
    indentationRules: {
      increaseIndentPattern: /^.*\{[^}]*$/,
      decreaseIndentPattern: /^[^{]*\}/,
    },
  });
}
