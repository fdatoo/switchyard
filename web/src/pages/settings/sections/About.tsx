/**
 * About section — shows build-time information injected via Vite env vars.
 *
 * Env vars used:
 *   VITE_SY_VERSION         — binary version string
 *   VITE_SY_BUILD_SHA       — git commit SHA (first 8 chars displayed)
 *   VITE_SY_BUILD_DATE      — build date
 *   VITE_SY_LICENSE         — license identifier (e.g. "MIT")
 *   VITE_SY_BINARY_FINGERPRINT — binary integrity fingerprint
 */

const version = import.meta.env.VITE_SY_VERSION as string | undefined;
const buildSha = import.meta.env.VITE_SY_BUILD_SHA as string | undefined;
const buildDate = import.meta.env.VITE_SY_BUILD_DATE as string | undefined;
const license = import.meta.env.VITE_SY_LICENSE as string | undefined;
const fingerprint = import.meta.env.VITE_SY_BINARY_FINGERPRINT as string | undefined;

const ROW_STYLE = {
  display: "flex",
  alignItems: "flex-start",
  padding: "var(--sy-space-3) 0",
  borderBottom: "1px solid var(--sy-color-line)",
  gap: "var(--sy-space-4)",
} as const;

const LABEL_STYLE = {
  fontSize: "0.8125rem",
  fontWeight: 500,
  color: "var(--sy-color-fg-4)",
  minWidth: "140px",
  flexShrink: 0,
} as const;

const VALUE_STYLE = {
  fontSize: "0.8125rem",
  color: "var(--sy-color-fg)",
} as const;

const MONO_VALUE_STYLE = {
  ...VALUE_STYLE,
  fontFamily: "var(--sy-font-numeric)",
} as const;

interface InfoRowProps {
  label: string;
  value: string;
  mono?: boolean;
}

function InfoRow({ label, value, mono = false }: InfoRowProps) {
  return (
    <div style={ROW_STYLE}>
      <span style={LABEL_STYLE}>{label}</span>
      <span style={mono ? MONO_VALUE_STYLE : VALUE_STYLE}>{value}</span>
    </div>
  );
}

/**
 * About section — static build-time information about this Switchyard instance.
 */
export function About() {
  const sha8 = buildSha ? buildSha.slice(0, 8) : undefined;

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
        About
      </h1>

      <div
        style={{
          background: "var(--sy-color-surface-1)",
          borderRadius: "var(--sy-radius)",
          border: "1px solid var(--sy-color-line)",
          padding: "0 var(--sy-space-4)",
          marginBottom: "var(--sy-space-5)",
        }}
      >
        <InfoRow label="Version" value={version ?? "not available"} mono />
        <InfoRow label="Build SHA" value={sha8 ?? "not available"} mono />
        <InfoRow label="Build date" value={buildDate ?? "not available"} />
        <InfoRow label="License" value={license ?? "not available"} />
      </div>

      {/* Binary fingerprint */}
      <div style={{ marginBottom: "var(--sy-space-5)" }}>
        <p
          style={{
            margin: "0 0 var(--sy-space-2)",
            fontSize: "0.8125rem",
            fontWeight: 500,
            color: "var(--sy-color-fg-4)",
          }}
        >
          Binary fingerprint
        </p>
        <code
          style={{
            display: "block",
            padding: "var(--sy-space-3)",
            background: "var(--sy-color-surface-2)",
            borderRadius: "var(--sy-radius)",
            fontFamily: "var(--sy-font-numeric)",
            fontSize: "0.75rem",
            color: "var(--sy-color-fg)",
            wordBreak: "break-all",
            border: "1px solid var(--sy-color-line)",
          }}
        >
          {fingerprint || "not available"}
        </code>
      </div>

      {/* External links */}
      <div style={{ display: "flex", gap: "var(--sy-space-4)" }}>
        <a
          href="https://docs.switchyard.io"
          target="_blank"
          rel="noopener noreferrer"
          style={{
            fontSize: "0.875rem",
            color: "var(--sy-color-accent)",
            textDecoration: "none",
            fontWeight: 500,
          }}
        >
          Documentation →
        </a>
        <a
          href="https://github.com/fdatoo/switchyard/issues"
          target="_blank"
          rel="noopener noreferrer"
          style={{
            fontSize: "0.875rem",
            color: "var(--sy-color-accent)",
            textDecoration: "none",
            fontWeight: 500,
          }}
        >
          Issue tracker →
        </a>
      </div>
    </div>
  );
}
