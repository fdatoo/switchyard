/**
 * Connect-ES typed wrappers for EditSessionService.
 *
 * These are thin adapters over the JSON/Connect protocol. They match the proto
 * RPC signatures but use plain TypeScript types so the hook layer stays
 * portable and testable without a real server.
 */

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

export type FileOnlyRegion = {
  startLine: number;
  endLine: number;
  reason: "starlark_call" | "import" | "let_binding" | "nondeterministic";
};

export type OpenForEditResult = {
  sessionId: string;
  lockToken: string;
  fileHash: string;
  ancestorPkl: string;
  astJson: string;
};

export type ListFilesEntry = {
  path: string;
  hasError: boolean;
};

export type CommitSuccess = {
  kind: "success";
  newFileHash: string;
};

export type CommitConflict = {
  kind: "conflict";
  diskHash: string;
  diskPkl: string;
  ancestorPkl: string;
};

export type CommitEditResult = CommitSuccess | CommitConflict;

export type SessionEventKind =
  | { type: "externalEdit"; filePath: string; newHash: string; modifiedAt: Date }
  | { type: "heartbeat"; serverTime: Date };

// ---------------------------------------------------------------------------
// EditSessionClient interface — implemented by RealEditSessionClient or mocks
// ---------------------------------------------------------------------------

export interface EditSessionClient {
  openForEdit(filePath: string): Promise<OpenForEditResult>;
  commitEdit(opts: {
    filePath: string;
    lockToken: string;
    regeneratedPkl: string;
    expectedFileHash: string;
    force: boolean;
  }): Promise<CommitEditResult>;
  abandonEdit(filePath: string, lockToken: string): Promise<void>;
  analyzeRegenerability(filePath: string): Promise<FileOnlyRegion[]>;
  sessionEvents(
    sessionId: string,
    onEvent: (event: SessionEventKind) => void,
    signal: AbortSignal,
  ): Promise<void>;
  listFiles(): Promise<ListFilesEntry[]>;
}

// ---------------------------------------------------------------------------
// HTTP/Connect implementation
// ---------------------------------------------------------------------------

async function postConnect<TRequest, TResponse>(
  procedure: string,
  body: TRequest,
): Promise<TResponse> {
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
    throw new Error(`connect request failed: ${response.status}`);
  }
  return response.json() as Promise<TResponse>;
}

export class RealEditSessionClient implements EditSessionClient {
  async openForEdit(filePath: string): Promise<OpenForEditResult> {
    const res = await postConnect<{ filePath: string }, {
      sessionId: string;
      lockToken: string;
      fileHash: string;
      ancestorPkl: string;
      astJson: string;
    }>("/switchyard.editsession.v1.EditSessionService/OpenForEdit", { filePath });
    return {
      sessionId: res.sessionId,
      lockToken: res.lockToken,
      fileHash: res.fileHash,
      ancestorPkl: res.ancestorPkl,
      astJson: res.astJson,
    };
  }

  async commitEdit(opts: {
    filePath: string;
    lockToken: string;
    regeneratedPkl: string;
    expectedFileHash: string;
    force: boolean;
  }): Promise<CommitEditResult> {
    const res = await postConnect<typeof opts, {
      success?: { newFileHash: string };
      conflict?: { diskHash: string; diskPkl: string; ancestorPkl: string };
    }>("/switchyard.editsession.v1.EditSessionService/CommitEdit", opts);

    if (res.success) {
      return { kind: "success", newFileHash: res.success.newFileHash };
    }
    if (res.conflict) {
      return {
        kind: "conflict",
        diskHash: res.conflict.diskHash,
        diskPkl: res.conflict.diskPkl,
        ancestorPkl: res.conflict.ancestorPkl,
      };
    }
    throw new Error("unexpected CommitEdit response: neither success nor conflict");
  }

  async abandonEdit(filePath: string, lockToken: string): Promise<void> {
    await postConnect("/switchyard.editsession.v1.EditSessionService/AbandonEdit", {
      filePath,
      lockToken,
    });
  }

  async analyzeRegenerability(filePath: string): Promise<FileOnlyRegion[]> {
    const res = await postConnect<{ filePath: string }, {
      fileOnlyRegions?: Array<{ startLine: number; endLine: number; reason: string }>;
    }>("/switchyard.editsession.v1.EditSessionService/AnalyzeRegenerability", { filePath });
    return (res.fileOnlyRegions ?? []).map((r) => ({
      startLine: r.startLine,
      endLine: r.endLine,
      reason: r.reason as FileOnlyRegion["reason"],
    }));
  }

  async listFiles(): Promise<ListFilesEntry[]> {
    const res = await postConnect<Record<string, never>, {
      files?: Array<{ path: string; hasError?: boolean }>;
    }>("/switchyard.editsession.v1.EditSessionService/ListFiles", {});
    return (res.files ?? []).map((f) => ({
      path: f.path,
      hasError: f.hasError ?? false,
    }));
  }

  async sessionEvents(
    sessionId: string,
    onEvent: (event: SessionEventKind) => void,
    signal: AbortSignal,
  ): Promise<void> {
    const response = await fetch(
      "/switchyard.editsession.v1.EditSessionService/SessionEvents",
      {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          "Connect-Protocol-Version": "1",
        },
        body: JSON.stringify({ sessionId }),
        signal,
      },
    );
    if (!response.ok) {
      throw new Error(`sessionEvents stream failed: ${response.status}`);
    }
    const reader = response.body?.getReader();
    if (!reader) return;
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop() ?? "";
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        try {
          const msg = JSON.parse(trimmed) as {
            externalEdit?: { filePath: string; newHash: string; modifiedAt: string };
            heartbeat?: { serverTime: string };
          };
          if (msg.externalEdit) {
            onEvent({
              type: "externalEdit",
              filePath: msg.externalEdit.filePath,
              newHash: msg.externalEdit.newHash,
              modifiedAt: new Date(msg.externalEdit.modifiedAt),
            });
          } else if (msg.heartbeat) {
            onEvent({
              type: "heartbeat",
              serverTime: new Date(msg.heartbeat.serverTime),
            });
          }
        } catch {
          // Skip malformed lines
        }
      }
    }
  }
}

// Singleton instance for production use.
export const editSessionClient: EditSessionClient = new RealEditSessionClient();
