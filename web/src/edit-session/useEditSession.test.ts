import { renderHook, act, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { EditSessionClient, FileOnlyRegion, ListFilesEntry, SessionEventKind } from "./client";
import { useEditSession } from "./useEditSession";

// ---------------------------------------------------------------------------
// Mock client factory
// ---------------------------------------------------------------------------

type MockEventEmitter = {
  emit: (evt: SessionEventKind) => void;
};

function makeMockClient(overrides?: Partial<EditSessionClient>): {
  client: EditSessionClient;
  events: MockEventEmitter;
} {
  const emitters: Array<(evt: SessionEventKind) => void> = [];
  const events: MockEventEmitter = {
    emit: (evt) => emitters.forEach((fn) => fn(evt)),
  };

  const client: EditSessionClient = {
    openForEdit: vi.fn().mockResolvedValue({
      sessionId: "sess-1",
      lockToken: "tok-1",
      fileHash: "hash-1",
      ancestorPkl: "id = \"orig\"\n",
      astJson: "{}",
    }),
    analyzeRegenerability: vi.fn().mockResolvedValue([] as FileOnlyRegion[]),
    commitEdit: vi.fn().mockResolvedValue({ kind: "success", newFileHash: "hash-2" }),
    abandonEdit: vi.fn().mockResolvedValue(undefined),
    listFiles: vi.fn().mockResolvedValue([] as ListFilesEntry[]),
    sessionEvents: vi.fn().mockImplementation(
      (_sessionId: string, onEvent: (evt: SessionEventKind) => void, signal: AbortSignal) => {
        emitters.push(onEvent);
        return new Promise<void>((_resolve, _reject) => {
          signal.addEventListener("abort", () => {
            const idx = emitters.indexOf(onEvent);
            if (idx >= 0) emitters.splice(idx, 1);
          });
        });
      },
    ),
    ...overrides,
  };

  return { client, events };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useEditSession", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("on mount: calls openForEdit + analyzeRegenerability in parallel", async () => {
    const { client } = makeMockClient();
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    expect(client.openForEdit).toHaveBeenCalledWith("/test.pkl");
    expect(client.analyzeRegenerability).toHaveBeenCalledWith("/test.pkl");
    expect(result.current.sessionId).toBe("sess-1");
    expect(result.current.fileHash).toBe("hash-1");
    expect(result.current.fileOnlyRegions).toHaveLength(0);
    expect(result.current.astJson).toBe("{}");
  });

  it("mutate increments dirtyCount", async () => {
    const { client } = makeMockClient();
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    act(() => result.current.mutate());
    act(() => result.current.mutate());
    expect(result.current.dirtyCount).toBe(2);
  });

  it("save (happy path) calls commitEdit + clears dirtyCount", async () => {
    const { client } = makeMockClient();
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));
    act(() => result.current.mutate());

    await act(async () => {
      await result.current.save("id = \"new\"\n");
    });

    expect(client.commitEdit).toHaveBeenCalledWith(
      expect.objectContaining({
        filePath: "/test.pkl",
        lockToken: "tok-1",
        expectedFileHash: "hash-1",
        force: false,
      }),
    );
    expect(result.current.dirtyCount).toBe(0);
    expect(result.current.conflict).toBeNull();
    expect(result.current.fileHash).toBe("hash-2");
  });

  it("save on CommitConflict response sets conflict state", async () => {
    const { client } = makeMockClient({
      commitEdit: vi.fn().mockResolvedValue({
        kind: "conflict",
        diskHash: "disk-hash",
        diskPkl: "id = \"disk\"\n",
        ancestorPkl: "id = \"orig\"\n",
      }),
    });
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    await act(async () => {
      await result.current.save("id = \"staged\"\n");
    });

    expect(result.current.conflict).not.toBeNull();
    expect(result.current.conflict?.diskHash).toBe("disk-hash");
    expect(result.current.conflict?.diskPkl).toBe("id = \"disk\"\n");
  });

  it("discard calls abandonEdit then openForEdit; resets dirtyCount to 0", async () => {
    const { client } = makeMockClient();
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));
    act(() => result.current.mutate());
    expect(result.current.dirtyCount).toBe(1);

    await act(async () => {
      await result.current.discard();
    });

    expect(client.abandonEdit).toHaveBeenCalledWith("/test.pkl", "tok-1");
    expect(result.current.dirtyCount).toBe(0);
    // openForEdit called again on discard
    expect(client.openForEdit).toHaveBeenCalledTimes(2);
  });

  it("resolveConflict({ kind: 'force' }) re-calls commitEdit with force=true", async () => {
    const { client } = makeMockClient({
      commitEdit: vi
        .fn()
        .mockResolvedValueOnce({ kind: "success", newFileHash: "hash-2" }) // initial save
        .mockResolvedValueOnce({ kind: "success", newFileHash: "hash-3" }), // force
    });
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    await act(async () => {
      const res = await result.current.resolveConflict(
        { kind: "force", stagedPkl: "id = \"forced\"\n" },
        "id = \"forced\"\n",
      );
      expect(res.kind).toBe("resolved");
    });

    expect(client.commitEdit).toHaveBeenCalledWith(
      expect.objectContaining({ force: true }),
    );
    expect(result.current.conflict).toBeNull();
  });

  it("resolveConflict({ kind: 'merge' }) returns merge context without calling any RPC", async () => {
    const { client } = makeMockClient();
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    let mergeCtx: ReturnType<typeof result.current.resolveConflict> extends Promise<infer T> ? T : never;
    await act(async () => {
      mergeCtx = await result.current.resolveConflict(
        { kind: "merge" },
        "id = \"staged\"\n",
      );
    });

    // @ts-expect-error assigned in act above
    expect(mergeCtx.kind).toBe("merge");
    // No additional RPC calls beyond the initial open
    expect(client.commitEdit).not.toHaveBeenCalled();
    expect(client.abandonEdit).not.toHaveBeenCalled();
  });

  it("beforeunload calls abandonEdit", async () => {
    const { client } = makeMockClient();
    const { result, unmount } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    // Fire the beforeunload event
    window.dispatchEvent(new Event("beforeunload"));
    expect(client.abandonEdit).toHaveBeenCalledWith("/test.pkl", "tok-1");

    unmount();
  });

  it("analyzeRegenerability result populates fileOnlyRegions", async () => {
    const region: FileOnlyRegion = { startLine: 3, endLine: 3, reason: "starlark_call" };
    const { client } = makeMockClient({
      analyzeRegenerability: vi.fn().mockResolvedValue([region]),
    });
    const { result } = renderHook(() => useEditSession("/test.pkl", client));

    await waitFor(() => expect(result.current.status).toBe("open"));

    expect(result.current.fileOnlyRegions).toHaveLength(1);
    expect(result.current.fileOnlyRegions[0].reason).toBe("starlark_call");
  });
});
