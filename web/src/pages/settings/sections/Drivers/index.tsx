import { useEffect, useState } from "react";
import { driverClient } from "@/data/driver-management-client";
import type { DriverSummary, RegistryDriver } from "@/data/driver-management-client";
import { useAuthStore } from "@/data/auth-store";
import { DriverCard } from "./DriverCard";

/**
 * Drivers section — shows Running / Available split with expandable cards.
 *
 * Permission gating: the hasWriteScope flag is derived from the auth store roles.
 * The actual scope string is "settings.drivers.write"; auth store roles are
 * checked as a simple string contains until a structured scope API exists.
 */
function hasDriverWriteScope(roles: string[]): boolean {
  return roles.some(
    (r) => r === "settings.drivers.write" || r === "admin" || r === "superuser",
  );
}

export function Drivers() {
  const user = useAuthStore((s) => s.user);
  const [running, setRunning] = useState<DriverSummary[]>([]);
  const [available, setAvailable] = useState<RegistryDriver[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const hasWriteScope = hasDriverWriteScope(user?.roles ?? []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    driverClient
      .list()
      .then((res) => {
        if (cancelled) return;
        setRunning(res.running);
        setAvailable(res.available);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load drivers");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleToggle = (id: string) => {
    setExpanded((prev) => (prev === id ? null : id));
  };

  if (loading) {
    return (
      <p style={{ color: "var(--sy-color-fg-4)", fontStyle: "italic" }}>
        Loading drivers…
      </p>
    );
  }

  if (error) {
    return (
      <p style={{ color: "var(--sy-color-bad)" }}>
        Error loading drivers: {error}
      </p>
    );
  }

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
        Drivers
      </h1>

      {/* Running */}
      <section style={{ marginBottom: "var(--sy-space-6)" }}>
        <h2
          style={{
            margin: "0 0 var(--sy-space-3)",
            fontSize: "0.6875rem",
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.06em",
            color: "var(--sy-color-fg-4)",
          }}
        >
          Running
        </h2>
        {running.length === 0 ? (
          <p style={{ color: "var(--sy-color-fg-4)", fontStyle: "italic", fontSize: "0.8125rem" }}>
            No running drivers.
          </p>
        ) : (
          running.map((driver) => (
            <DriverCard
              key={driver.id}
              driver={driver}
              expanded={expanded === driver.id}
              onToggle={handleToggle}
              hasWriteScope={hasWriteScope}
            />
          ))
        )}
      </section>

      {/* Available */}
      {available.length > 0 && (
        <section>
          <h2
            style={{
              margin: "0 0 var(--sy-space-3)",
              fontSize: "0.6875rem",
              fontWeight: 600,
              textTransform: "uppercase",
              letterSpacing: "0.06em",
              color: "var(--sy-color-fg-4)",
            }}
          >
            Available
          </h2>
          {available.map((driver) => (
            <div
              key={driver.id}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "var(--sy-space-3) var(--sy-space-4)",
                border: "1px solid var(--sy-color-line)",
                borderRadius: "var(--sy-radius)",
                marginBottom: "var(--sy-space-2)",
                background: "var(--sy-color-bg)",
              }}
            >
              <div>
                <div
                  style={{
                    fontWeight: 500,
                    fontSize: "0.875rem",
                    color: "var(--sy-color-fg)",
                  }}
                >
                  {driver.pack}
                </div>
                <div style={{ fontSize: "0.75rem", color: "var(--sy-color-fg-4)" }}>
                  v{driver.version} · {driver.status}
                </div>
              </div>
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
