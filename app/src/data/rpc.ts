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
