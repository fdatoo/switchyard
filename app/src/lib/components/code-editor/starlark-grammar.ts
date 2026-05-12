import type { languages } from "monaco-editor";

export const starlarkLanguageId = "starlark";

export const starlarkLanguageConfig: languages.LanguageConfiguration = {
  comments: { lineComment: "#" },
  brackets: [
    ["(", ")"],
    ["[", "]"],
    ["{", "}"],
  ],
  autoClosingPairs: [
    { open: "(", close: ")" },
    { open: "[", close: "]" },
    { open: "{", close: "}" },
    { open: '"', close: '"', notIn: ["string"] },
    { open: "'", close: "'", notIn: ["string"] },
  ],
  surroundingPairs: [
    { open: "(", close: ")" },
    { open: "[", close: "]" },
    { open: "{", close: "}" },
    { open: '"', close: '"' },
    { open: "'", close: "'" },
  ],
  wordPattern: /(-?\d*\.\d\w*)|([^`~!@#%^&*()\-=+[{\]}\\|;:'",.<>/?\s]+)/,
};

export const starlarkMonarchTokens: languages.IMonarchLanguage = {
  defaultToken: "",
  tokenPostfix: ".star",

  keywords: [
    "def", "if", "elif", "else", "for", "in", "while", "return",
    "and", "or", "not", "True", "False", "None",
    "load", "lambda", "pass", "break", "continue",
  ],

  builtins: [
    "print", "len", "range", "list", "dict", "tuple", "set",
    "str", "int", "float", "bool", "type",
    "state", "now", "time", "log", "call_service", "random",
    "sleep", "notify", "scene", "event",
  ],

  brackets: [
    { open: "(", close: ")", token: "delimiter.parenthesis" },
    { open: "[", close: "]", token: "delimiter.square" },
    { open: "{", close: "}", token: "delimiter.curly" },
  ],

  tokenizer: {
    root: [
      [/#.*$/, "comment"],

      [/"""/, { token: "string.quote", bracket: "@open", next: "@tripleString" }],
      [/'''/, { token: "string.quote", bracket: "@open", next: "@tripleStringSingle" }],

      [/"([^"\\]|\\.)*$/, "string.invalid"],
      [/'([^'\\]|\\.)*$/, "string.invalid"],
      [/"/, { token: "string.quote", bracket: "@open", next: "@string" }],
      [/'/, { token: "string.quote", bracket: "@open", next: "@stringSingle" }],

      [/0[xX][0-9a-fA-F]+/, "number.hex"],
      [/0[oO][0-7]+/, "number.octal"],
      [/0[bB][01]+/, "number.binary"],
      [/\d+(\.\d+)?([eE][+-]?\d+)?/, "number"],

      [/[A-Za-z_][A-Za-z0-9_]*/, {
        cases: {
          "@keywords": "keyword",
          "@builtins": "type.identifier",
          "@default": "identifier",
        },
      }],

      [/[=+\-*/%<>!]+/, "operator"],
      [/[(){}[\]]/, "@brackets"],
      [/[,;]/, "delimiter"],
      [/\s+/, "white"],
    ],

    string: [
      [/[^\\"]+/, "string"],
      [/\\./, "string.escape"],
      [/"/, { token: "string.quote", bracket: "@close", next: "@pop" }],
    ],
    stringSingle: [
      [/[^\\']+/, "string"],
      [/\\./, "string.escape"],
      [/'/, { token: "string.quote", bracket: "@close", next: "@pop" }],
    ],
    tripleString: [
      [/[^"]+/, "string"],
      [/"""/, { token: "string.quote", bracket: "@close", next: "@pop" }],
      [/"/, "string"],
    ],
    tripleStringSingle: [
      [/[^']+/, "string"],
      [/'''/, { token: "string.quote", bracket: "@close", next: "@pop" }],
      [/'/, "string"],
    ],
  },
};
