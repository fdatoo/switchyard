import { usePasskeys, useSessions, useTokens } from "@/data/auth-client";
import type { PasskeyRow, SessionRow, TokenRow } from "@/data/auth-client";
import { Button } from "@/theme/primitives/button";
import { Surface } from "@/theme/primitives/surface";

const CARD_STYLE = {
  padding: "var(--sy-space-5)",
  marginBottom: "var(--sy-space-4)",
  border: "1px solid var(--sy-color-line)",
} as const;

const HEADING_STYLE = {
  margin: "0 0 var(--sy-space-4)",
  fontSize: "1rem",
  fontWeight: 600,
  color: "var(--sy-color-fg)",
} as const;

const ROW_STYLE = {
  display: "flex",
  alignItems: "center",
  justifyContent: "space-between",
  padding: "var(--sy-space-3) 0",
  borderBottom: "1px solid var(--sy-color-line)",
  gap: "var(--sy-space-4)",
} as const;

const LABEL_STYLE = {
  fontSize: "0.875rem",
  color: "var(--sy-color-fg)",
} as const;

const META_STYLE = {
  fontSize: "0.75rem",
  color: "var(--sy-color-fg-4)",
} as const;

const EMPTY_STYLE = {
  fontSize: "0.8125rem",
  color: "var(--sy-color-fg-4)",
  fontStyle: "italic",
  padding: "var(--sy-space-3) 0",
} as const;

function PasskeysCard() {
  const { passkeys, revokePasskey } = usePasskeys();
  return (
    <Surface style={CARD_STYLE}>
      <h2 style={HEADING_STYLE}>Passkeys</h2>
      {passkeys.length === 0 ? (
        <p style={EMPTY_STYLE}>No passkeys registered.</p>
      ) : (
        passkeys.map((pk: PasskeyRow) => (
          <div key={pk.id} style={ROW_STYLE}>
            <div>
              <div style={LABEL_STYLE}>{pk.name}</div>
              <div style={META_STYLE}>Added {pk.createdAt}</div>
            </div>
            <Button
              variant="secondary"
              onClick={() => void revokePasskey(pk.id)}
            >
              Revoke
            </Button>
          </div>
        ))
      )}
    </Surface>
  );
}

function SessionsCard() {
  const { sessions, revokeSession } = useSessions();
  return (
    <Surface style={CARD_STYLE}>
      <h2 style={HEADING_STYLE}>Active sessions</h2>
      {sessions.length === 0 ? (
        <p style={EMPTY_STYLE}>No active sessions.</p>
      ) : (
        sessions.map((s: SessionRow) => (
          <div key={s.id} style={ROW_STYLE}>
            <div>
              <div style={LABEL_STYLE}>{s.userAgent}</div>
              <div style={META_STYLE}>Last seen {s.lastSeen}</div>
            </div>
            <Button
              variant="secondary"
              onClick={() => void revokeSession(s.id)}
            >
              Revoke
            </Button>
          </div>
        ))
      )}
    </Surface>
  );
}

function TokensCard() {
  const { tokens, revokeToken } = useTokens();
  return (
    <Surface style={CARD_STYLE}>
      <h2 style={HEADING_STYLE}>Issued tokens</h2>
      {tokens.length === 0 ? (
        <p style={EMPTY_STYLE}>No issued tokens.</p>
      ) : (
        tokens.map((t: TokenRow) => (
          <div key={t.id} style={ROW_STYLE}>
            <div>
              <div style={LABEL_STYLE}>{t.label}</div>
              <div style={META_STYLE}>Expires {t.expiresAt}</div>
            </div>
            <Button
              variant="secondary"
              onClick={() => void revokeToken(t.id)}
            >
              Revoke
            </Button>
          </div>
        ))
      )}
    </Surface>
  );
}

/**
 * Account section — passkeys, active sessions, issued tokens.
 * Each is a bordered surface card.
 */
export function Account() {
  return (
    <div>
      <h1
        style={{
          margin: "0 0 var(--sy-space-5)",
          fontSize: "1.25rem",
          fontWeight: 600,
          color: "var(--sy-color-fg)",
        }}
      >
        Account
      </h1>
      <PasskeysCard />
      <SessionsCard />
      <TokensCard />
    </div>
  );
}
