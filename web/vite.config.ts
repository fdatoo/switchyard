import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";
import type { VitePWAOptions } from "vite-plugin-pwa";
import path from "node:path";

const runtimeCaching: NonNullable<NonNullable<VitePWAOptions["workbox"]>["runtimeCaching"]> = [
  {
    urlPattern: ({ request }) => request.mode === "navigate",
    handler: "NetworkFirst",
    options: {
      cacheName: "switchyard-html-shell",
      networkTimeoutSeconds: 3,
      cacheableResponse: { statuses: [0, 200] },
      expiration: { maxEntries: 10 },
    },
  },
  {
    urlPattern: ({ request, sameOrigin }) =>
      sameOrigin && ["font", "image", "manifest", "script", "style"].includes(request.destination),
    handler: "CacheFirst",
    options: {
      cacheName: "switchyard-static-assets",
      cacheableResponse: { statuses: [0, 200] },
      expiration: {
        maxAgeSeconds: 30 * 24 * 60 * 60,
        maxEntries: 80,
      },
    },
  },
];

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      injectRegister: "script-defer",
      manifest: false,
      registerType: "autoUpdate",
      workbox: {
        cleanupOutdatedCaches: true,
        clientsClaim: true,
        globPatterns: ["**/*.{css,html,js,svg,webmanifest,woff,woff2}"],
        navigateFallback: "/index.html",
        runtimeCaching,
        skipWaiting: true,
      },
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
    target: "es2022",
    sourcemap: false,
    rollupOptions: {
      output: {
        assetFileNames: "assets/[name]-[hash][extname]",
        chunkFileNames: "assets/[name]-[hash].js",
        entryFileNames: "assets/[name]-[hash].js",
      },
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    setupFiles: "./src/test/setup.ts",
  },
  server: { port: 5173, strictPort: true },
});
