import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import path from "node:path";
import os from "node:os";

/**
 * Build the proxy target for daemon-bound requests.
 *
 * The daemon listens on both a TCP port and a Unix domain socket. The
 * UDS path is special: it authenticates the local user automatically
 * via SO_PEERCRED, so dev requests are trusted as the daemon-owner
 * without needing a session cookie. The TCP listener requires a cookie
 * issued by AuthService, which the dev harness doesn't have.
 *
 * Default: connect to the UDS at the well-known path. Override with
 * `SY_DAEMON_URL` to point at TCP for production-style testing.
 *
 *   SY_DAEMON_URL=http://127.0.0.1:8080         → TCP, requires session.
 *   SY_DAEMON_URL=unix:/path/to/some.sock       → custom UDS path.
 *   (unset)                                     → ~/.local/share/switchyard/switchyardd.sock
 */
function daemonProxyTarget() {
  const override = process.env.SY_DAEMON_URL;
  if (override && !override.startsWith("unix:")) {
    return { target: override, changeOrigin: true };
  }
  const socketPath =
    override?.slice("unix:".length) ??
    path.join(os.homedir(), ".local/share/switchyard/switchyardd.sock");
  return {
    /* http-proxy accepts `target` as an object with `socketPath`. The
       protocol/host placeholders are required so the upstream sees a
       valid Host header — the daemon ignores Host but http-proxy needs
       it to build the proxied request. */
    target: { socketPath, protocol: "http:", host: "localhost" },
    changeOrigin: true,
  };
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    port: 5174,
    strictPort: true,
    proxy: (() => {
      const opts = daemonProxyTarget();
      return {
        "^/switchyard\\..+/.+$": opts,
        "^/healthz$": opts,
        "^/webhooks/": opts,
        "^/mcp(/.*)?$": opts,
        "^/widgets/": opts,
      };
    })(),
  },
});
