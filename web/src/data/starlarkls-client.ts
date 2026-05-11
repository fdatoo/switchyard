// web/src/data/starlarkls-client.ts
// Typed wrapper around StarlarkLsService Connect-RPC calls.
// Uses the same plain-JSON Connect transport as the rest of the web app.

const BASE_PATH = "/switchyard.starlarkls.v1.StarlarkLsService";

async function postConnect<TReq, TRes>(
  procedure: string,
  body: TReq
): Promise<TRes> {
  const response = await fetch(procedure, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new Error(`starlarkls: ${procedure} failed: ${response.status}`);
  }
  return response.json() as Promise<TRes>;
}

export interface CompletionItem {
  label: string;
  kind: string;
  detail: string;
  insertText: string;
}

export interface CompleteResponse {
  items: CompletionItem[];
}

export interface HoverResponse {
  markdown: string;
}

export interface LookupSymbolResponse {
  filePath: string;
  line: number;
  kind: string;
  doc: string;
}

export function buildStarlarkLsClient(_baseUrl?: string) {
  return {
    complete: (
      filePath: string,
      source: string,
      line: number,
      col: number
    ): Promise<CompleteResponse> =>
      postConnect(`${BASE_PATH}/Complete`, { filePath, source, line, col }),

    hover: (
      filePath: string,
      source: string,
      line: number,
      col: number
    ): Promise<HoverResponse> =>
      postConnect(`${BASE_PATH}/Hover`, { filePath, source, line, col }),

    lookupSymbol: (name: string): Promise<LookupSymbolResponse> =>
      postConnect(`${BASE_PATH}/LookupSymbol`, { name }),
  };
}

export type StarlarkLsClient = ReturnType<typeof buildStarlarkLsClient>;

// Singleton used by the editor route.
export const starlarkLsClient = buildStarlarkLsClient();
