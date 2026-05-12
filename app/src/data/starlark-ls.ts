import { rpcCall, type RpcOptions } from "./rpc";

const SVC = "switchyard.starlarkls.v1.StarlarkLsService";

export interface TokenSpan {
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  tokenType: string;
}

export interface CompletionItem {
  label: string;
  kind: string;
  detail: string;
  insertText: string;
}

export interface Diagnostic {
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  severity: "error" | "warning";
  message: string;
  code: string;
}

interface RawTokenSpan {
  start_line?: number;
  startLine?: number;
  start_col?: number;
  startCol?: number;
  end_line?: number;
  endLine?: number;
  end_col?: number;
  endCol?: number;
  token_type?: string;
  tokenType?: string;
}

interface RawCompletionItem {
  label?: string;
  kind?: string;
  detail?: string;
  insert_text?: string;
  insertText?: string;
}

interface RawDiagnostic {
  start_line?: number;
  startLine?: number;
  start_col?: number;
  startCol?: number;
  end_line?: number;
  endLine?: number;
  end_col?: number;
  endCol?: number;
  severity?: string;
  message?: string;
  code?: string;
}

function decodeTokenSpan(r: RawTokenSpan): TokenSpan {
  return {
    startLine: r.startLine ?? r.start_line ?? 0,
    startCol: r.startCol ?? r.start_col ?? 0,
    endLine: r.endLine ?? r.end_line ?? 0,
    endCol: r.endCol ?? r.end_col ?? 0,
    tokenType: r.tokenType ?? r.token_type ?? "",
  };
}

function decodeCompletion(r: RawCompletionItem): CompletionItem {
  return {
    label: r.label ?? "",
    kind: r.kind ?? "",
    detail: r.detail ?? "",
    insertText: r.insertText ?? r.insert_text ?? r.label ?? "",
  };
}

function decodeDiagnostic(r: RawDiagnostic): Diagnostic {
  const rawSeverity = r.severity ?? "warning";
  return {
    startLine: r.startLine ?? r.start_line ?? 1,
    startCol: r.startCol ?? r.start_col ?? 0,
    endLine: r.endLine ?? r.end_line ?? 1,
    endCol: r.endCol ?? r.end_col ?? 1,
    severity: rawSeverity === "error" ? "error" : "warning",
    message: r.message ?? "",
    code: r.code ?? "",
  };
}

export async function tokenize(
  req: { filePath: string; source: string },
  opts: RpcOptions = {},
): Promise<{ spans: TokenSpan[] }> {
  const res = await rpcCall<unknown, { spans?: RawTokenSpan[] }>(
    `${SVC}/Tokenize`,
    { filePath: req.filePath, source: req.source },
    opts,
  );
  return { spans: (res.spans ?? []).map(decodeTokenSpan) };
}

export async function complete(
  req: { filePath: string; source: string; line: number; col: number },
  opts: RpcOptions = {},
): Promise<{ items: CompletionItem[] }> {
  const res = await rpcCall<unknown, { items?: RawCompletionItem[] }>(
    `${SVC}/Complete`,
    { filePath: req.filePath, source: req.source, line: req.line, col: req.col },
    opts,
  );
  return { items: (res.items ?? []).map(decodeCompletion) };
}

export async function hover(
  req: { filePath: string; source: string; line: number; col: number },
  opts: RpcOptions = {},
): Promise<{ markdown: string }> {
  const res = await rpcCall<unknown, { markdown?: string }>(
    `${SVC}/Hover`,
    { filePath: req.filePath, source: req.source, line: req.line, col: req.col },
    opts,
  );
  return { markdown: res.markdown ?? "" };
}

export async function lookupSymbol(
  req: { name: string },
  opts: RpcOptions = {},
): Promise<{ filePath: string; line: number; kind: string; doc: string }> {
  const res = await rpcCall<
    unknown,
    { file_path?: string; filePath?: string; line?: number; kind?: string; doc?: string }
  >(`${SVC}/LookupSymbol`, { name: req.name }, opts);
  return {
    filePath: res.filePath ?? res.file_path ?? "",
    line: res.line ?? 0,
    kind: res.kind ?? "",
    doc: res.doc ?? "",
  };
}

export async function diagnose(
  req: { filePath: string; source: string },
  opts: RpcOptions = {},
): Promise<{ diagnostics: Diagnostic[] }> {
  const res = await rpcCall<unknown, { diagnostics?: RawDiagnostic[] }>(
    `${SVC}/Diagnose`,
    { filePath: req.filePath, source: req.source },
    opts,
  );
  return { diagnostics: (res.diagnostics ?? []).map(decodeDiagnostic) };
}
