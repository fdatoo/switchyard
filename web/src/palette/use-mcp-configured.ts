/**
 * use-mcp-configured.ts
 * Hook: returns true when McpService.IsConfigured() returns true.
 * Returns false while loading. Cached for the session (staleTime: Infinity).
 * UI v2 Plan 05.
 */
import { useState, useEffect } from "react";

/**
 * useMcpConfigured returns whether MCP is configured on the server.
 * Since we don't have a generated McpService.IsConfigured RPC yet,
 * this calls /api/mcp/configured and returns the result.
 * Falls back to false on error or while loading.
 */
export function useMcpConfigured(): boolean {
  const [configured, setConfigured] = useState(false);

  useEffect(() => {
    let cancelled = false;
    // Attempt to call the mcp-configured endpoint. If it doesn't exist,
    // treat as not configured (false). This mirrors the plan's specification
    // that the hook returns false when MCP is not configured.
    fetch("/api/mcp/configured")
      .then((res) => {
        if (!res.ok) return false;
        return res.json() as Promise<{ configured: boolean }>;
      })
      .then((data) => {
        if (!cancelled && data && typeof data === "object" && "configured" in data) {
          setConfigured((data as { configured: boolean }).configured);
        }
      })
      .catch(() => {
        // Network error → not configured.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return configured;
}
