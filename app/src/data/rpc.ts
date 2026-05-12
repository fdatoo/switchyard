/**
 * Minimal Connect-RPC helper. Wraps `fetch` for the unary request pattern
 * the daemon's Connect-ES services use.
 *
 * Connect's unary HTTP shape:
 *   POST /<package>.<service>/<method>
 *   Content-Type: application/json
 *   Connect-Protocol-Version: 1
 *   Body: JSON-encoded request message
 *
 * Response: JSON-encoded response message on 2xx, or a Connect error on
 * non-2xx (which we surface as `RpcError`). For now we go raw fetch
 * rather than pulling in @bufbuild/connect — keeps the bundle small and
 * matches the pattern the React app already uses in its lighter clients.
 *
 * Auth/session: cookies are sent via `credentials: "include"`; the
 * daemon's auth interceptor reads the session cookie and rejects 401 if
 * absent. Consumers don't need to thread tokens manually.
 */

const PROTOCOL_VERSION = "1";

export interface RpcOptions {
  /** AbortSignal for cancellation. */
  signal?: AbortSignal;
}

export class RpcError extends Error {
  constructor(
    public readonly status: number,
    public readonly serviceMethod: string,
    message: string,
  ) {
    super(message);
    this.name = "RpcError";
  }
}

/**
 * Call a unary Connect RPC.
 *
 * @param serviceMethod Path: `<package>.<service>/<method>` without leading `/`,
 *   e.g. `"switchyard.driver.v1.DriverManagementService/List"`.
 * @param request Request body — will be JSON-encoded.
 */
export async function rpcCall<TReq, TRes>(
  serviceMethod: string,
  request: TReq,
  opts: RpcOptions = {},
): Promise<TRes> {
  const res = await fetch(`/${serviceMethod}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": PROTOCOL_VERSION,
    },
    body: JSON.stringify(request ?? {}),
    signal: opts.signal,
  });
  if (!res.ok) {
    let detail = `${res.status} ${res.statusText}`;
    try {
      const text = await res.text();
      if (text) detail = `${detail}: ${text.slice(0, 240)}`;
    } catch { /* ignore */ }
    throw new RpcError(res.status, serviceMethod, detail);
  }
  return (await res.json()) as TRes;
}

const STREAM_PROTOCOL_CT = "application/connect+json";
const FLAG_END_STREAM = 0x02;

/**
 * Call a Connect server-streaming RPC. Returns an async generator that
 * yields each `TItem` as it arrives. The trailer envelope ends the
 * stream; if it carries an error, it's thrown as `RpcError`.
 *
 * Wire format: each envelope is `[1 byte flags][4 bytes length BE][N
 * bytes JSON payload]`. The request body is a single envelope (flags=0)
 * wrapping the request message; the response is a stream of envelopes
 * with the trailer marked by `flags & 0x02`.
 *
 * Aborting `opts.signal` causes the underlying fetch to reject and the
 * generator to throw the abort reason.
 */
export async function* rpcStream<TReq, TItem>(
  serviceMethod: string,
  request: TReq,
  opts: RpcOptions = {},
): AsyncGenerator<TItem, void, void> {
  // Encode the single client message into a length-prefixed envelope.
  const reqBytes = new TextEncoder().encode(JSON.stringify(request ?? {}));
  const envelope = new Uint8Array(5 + reqBytes.length);
  envelope[0] = 0;
  new DataView(envelope.buffer).setUint32(1, reqBytes.length, false);
  envelope.set(reqBytes, 5);

  const res = await fetch(`/${serviceMethod}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": STREAM_PROTOCOL_CT,
      "Connect-Protocol-Version": PROTOCOL_VERSION,
    },
    body: envelope,
    signal: opts.signal,
  });
  if (!res.ok) {
    let detail = `${res.status} ${res.statusText}`;
    try {
      const text = await res.text();
      if (text) detail = `${detail}: ${text.slice(0, 240)}`;
    } catch { /* ignore */ }
    throw new RpcError(res.status, serviceMethod, detail);
  }
  if (!res.body) {
    throw new RpcError(0, serviceMethod, "stream response missing body");
  }

  const reader = res.body.getReader();
  let buffer = new Uint8Array(0);
  const decoder = new TextDecoder();

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (value && value.length) {
        const next = new Uint8Array(buffer.length + value.length);
        next.set(buffer, 0);
        next.set(value, buffer.length);
        buffer = next;
      }
      // Drain whole envelopes from the buffer.
      while (buffer.length >= 5) {
        const flags = buffer[0];
        const len = new DataView(
          buffer.buffer, buffer.byteOffset + 1, 4,
        ).getUint32(0, false);
        if (buffer.length < 5 + len) break;
        const payload = buffer.subarray(5, 5 + len);
        buffer = buffer.subarray(5 + len);
        const text = decoder.decode(payload);
        if (flags & FLAG_END_STREAM) {
          if (text.length > 0) {
            try {
              const parsed = JSON.parse(text) as { error?: { code?: string; message?: string } };
              if (parsed.error) {
                throw new RpcError(
                  0,
                  serviceMethod,
                  `${parsed.error.code ?? "stream_error"}: ${parsed.error.message ?? ""}`,
                );
              }
            } catch (err) {
              if (err instanceof RpcError) throw err;
              // Malformed trailer JSON — treat as benign EOS.
            }
          }
          return;
        }
        yield JSON.parse(text) as TItem;
      }
      if (done) {
        // Stream ended without an explicit trailer envelope. Treat as EOS.
        return;
      }
    }
  } finally {
    try { await reader.cancel(); } catch { /* ignore */ }
  }
}
