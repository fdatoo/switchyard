/**
 * auth-client.ts — typed client wrappers for auth-related settings operations.
 *
 * Passkeys, sessions, and issued tokens are not yet exposed via dedicated RPCs
 * in the AuthService proto (auth.proto has RegisterPasskey and RevokeToken but
 * no list operations). This client provides stub data shapes so the Account
 * section renders correctly; the real RPC calls can be wired when the backend
 * adds the corresponding proto RPCs.
 *
 * TODO: add ListPasskeys, ListSessions, and ListTokens RPCs to auth.proto and
 * implement them in internal/api/service_auth.go, then replace these stubs.
 */

export interface PasskeyRow {
  id: string;
  name: string;
  createdAt: string;
}

export interface SessionRow {
  id: string;
  userAgent: string;
  lastSeen: string;
}

export interface TokenRow {
  id: string;
  label: string;
  expiresAt: string;
}

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
    throw new Error(`auth-client: ${procedure} failed: ${response.status}`);
  }
  return response.json() as Promise<TResponse>;
}

/**
 * usePasskeys — stub returning empty list.
 * TODO: call ListPasskeys RPC when proto + server support it.
 */
export function usePasskeys(): {
  passkeys: PasskeyRow[];
  loading: boolean;
  revokePasskey: (id: string) => Promise<void>;
} {
  return {
    passkeys: [],
    loading: false,
    revokePasskey: async (_id: string) => {
      // TODO: call RevokePasskey RPC
    },
  };
}

/**
 * useSessions — stub returning empty list.
 * TODO: call ListSessions RPC when proto + server support it.
 */
export function useSessions(): {
  sessions: SessionRow[];
  loading: boolean;
  revokeSession: (id: string) => Promise<void>;
} {
  return {
    sessions: [],
    loading: false,
    revokeSession: async (_id: string) => {
      // TODO: call RevokeSession RPC
    },
  };
}

/**
 * useTokens — stub returning empty list.
 * TODO: call ListTokens RPC when proto + server support it.
 */
export function useTokens(): {
  tokens: TokenRow[];
  loading: boolean;
  revokeToken: (id: string) => Promise<void>;
} {
  return {
    tokens: [],
    loading: false,
    revokeToken: async (id: string) => {
      await postConnect<{ token_id: string }, Record<string, never>>(
        "/switchyard.v1alpha1.AuthService/RevokeToken",
        { token_id: id },
      );
    },
  };
}
