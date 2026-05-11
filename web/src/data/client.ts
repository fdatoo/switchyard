import { markDaemonReachable, markDaemonUnreachable } from "./daemon-connection";

export class ConnectHTTPError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "ConnectHTTPError";
  }
}

export type AuthRefreshOptions = {
  refresh: () => Promise<void>;
  redirectToLogin?: (path: string) => void;
};

export function isUnauthorized(error: unknown): boolean {
  return error instanceof ConnectHTTPError && error.status === 401;
}

export async function withAuthRefresh<T>(
  call: () => Promise<T>,
  options: AuthRefreshOptions,
): Promise<T> {
  try {
    return await call();
  } catch (error) {
    if (!isUnauthorized(error)) {
      throw error;
    }
  }

  try {
    await options.refresh();
  } catch (refreshError) {
    options.redirectToLogin?.("/login");
    throw refreshError;
  }

  return call();
}

/**
 * postConnect — issue a Connect-RPC JSON request against the daemon.
 * Use this for every Connect-protocol call from the web app; it handles
 * credentials (HttpOnly session cookie), the protocol-version header, and
 * daemon-reachability bookkeeping. Throws ConnectHTTPError on non-2xx,
 * including 401 which callers can detect via isUnauthorized().
 */
export async function postConnect<TRequest, TResponse>(
  procedure: string,
  body: TRequest,
): Promise<TResponse> {
  let response: Response;
  try {
    response = await fetch(procedure, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        "Connect-Protocol-Version": "1",
      },
      body: JSON.stringify(body),
    });
  } catch (error) {
    markDaemonUnreachable(error);
    throw error;
  }
  markDaemonReachable();
  if (!response.ok) {
    throw new ConnectHTTPError(`connect request failed: ${response.status}`, response.status);
  }
  return response.json() as Promise<TResponse>;
}

export async function loginWithPasswordRequest(username: string, password: string): Promise<void> {
  await postConnect<{ username: string; password: string }, { sessionToken?: string }>(
    "/switchyard.v1alpha1.AuthService/Login",
    { username, password },
  );
}

export async function refreshSessionRequest(): Promise<void> {
  await postConnect<Record<string, never>, { userSlug?: string; sessionId?: string }>(
    "/switchyard.v1alpha1.AuthService/Refresh",
    {},
  );
}

export const transport = {
  auth: {
    loginWithPassword: loginWithPasswordRequest,
    refresh: refreshSessionRequest,
  },
};
