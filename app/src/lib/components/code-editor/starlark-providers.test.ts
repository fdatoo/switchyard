import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";

type CompletionProvider = {
  provideCompletionItems: (model: MockModel, position: { lineNumber: number; column: number }) => Promise<{ suggestions: unknown[] }>;
};

type SemanticProvider = {
  provideDocumentSemanticTokens: (model: MockModel) => Promise<{ data: Uint32Array; resultId: undefined }>;
};

type MockModel = {
  uri: { path: string };
  getValue: () => string;
  getWordUntilPosition: () => { startColumn: number; endColumn: number };
};

let completionProvider: CompletionProvider | null = null;
let semanticProvider: SemanticProvider | null = null;

vi.mock("monaco-editor", () => ({
  languages: {
    CompletionItemKind: {
      Function: 1,
      Variable: 2,
      Keyword: 3,
      Text: 4,
    },
    registerCompletionItemProvider: vi.fn((_language: string, provider: CompletionProvider) => {
      completionProvider = provider;
      return { dispose: vi.fn() };
    }),
    registerHoverProvider: vi.fn(),
    registerDefinitionProvider: vi.fn(),
    registerDocumentSemanticTokensProvider: vi.fn((_language: string, provider: SemanticProvider) => {
      semanticProvider = provider;
      return { dispose: vi.fn() };
    }),
  },
  editor: {
    getModels: vi.fn(() => []),
    onDidCreateModel: vi.fn(),
    setModelMarkers: vi.fn(),
  },
  MarkerSeverity: {
    Error: 8,
    Warning: 4,
  },
}));

vi.mock("@/data/starlark-ls", () => ({
  complete: vi.fn(),
  diagnose: vi.fn(),
  hover: vi.fn(),
  lookupSymbol: vi.fn(),
  tokenize: vi.fn(),
}));

import { complete, tokenize } from "@/data/starlark-ls";
import {
  __resetStarlarkProvidersForTest,
  completionKindOf,
  diagToMonaco,
  encodeSemanticTokens,
  setupStarlarkProviders,
} from "./starlark-providers";

const completeMock = complete as unknown as Mock;
const tokenizeMock = tokenize as unknown as Mock;

function model(source = "hel"): MockModel {
  return {
    uri: { path: "/config/scripts/test.star" },
    getValue: () => source,
    getWordUntilPosition: () => ({ startColumn: 1, endColumn: 4 }),
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  completionProvider = null;
  semanticProvider = null;
  __resetStarlarkProvidersForTest();
});

describe("Starlark Monaco provider helpers", () => {
  it("encodes semantic tokens in sorted LSP delta format", () => {
    const encoded = encodeSemanticTokens([
      { startLine: 2, startCol: 4, endLine: 2, endCol: 10, tokenType: "string" },
      { startLine: 1, startCol: 0, endLine: 1, endCol: 4, tokenType: "keyword" },
      { startLine: 2, startCol: 11, endLine: 2, endCol: 17, tokenType: "unknown" },
    ]);

    expect([...encoded]).toEqual([
      0, 0, 4, 0, 0,
      1, 4, 6, 2, 0,
      0, 7, 6, 1, 0,
    ]);
  });

  it("maps daemon diagnostics to Monaco markers", () => {
    expect(diagToMonaco({
      startLine: 3,
      startCol: 2,
      endLine: 3,
      endCol: 9,
      severity: "error",
      message: "load failed",
      code: "load_not_found",
    })).toEqual({
      severity: 8,
      message: "load failed",
      code: "load_not_found",
      startLineNumber: 3,
      startColumn: 3,
      endLineNumber: 3,
      endColumn: 10,
    });
  });

  it("maps completion kinds conservatively", () => {
    expect(completionKindOf("function")).toBe(1);
    expect(completionKindOf("variable")).toBe(2);
    expect(completionKindOf("keyword")).toBe(3);
    expect(completionKindOf("other")).toBe(4);
  });
});

describe("Starlark Monaco provider fallbacks", () => {
  it("returns no completion suggestions when the RPC fails", async () => {
    completeMock.mockRejectedValueOnce(new Error("daemon down"));
    setupStarlarkProviders();

    await expect(completionProvider?.provideCompletionItems(model(), {
      lineNumber: 1,
      column: 4,
    })).resolves.toEqual({ suggestions: [] });
  });

  it("returns an empty semantic token set when tokenize fails", async () => {
    tokenizeMock.mockRejectedValueOnce(new Error("daemon down"));
    setupStarlarkProviders();

    await expect(semanticProvider?.provideDocumentSemanticTokens(model()))
      .resolves.toEqual({ data: new Uint32Array(0), resultId: undefined });
  });
});
