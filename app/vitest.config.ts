import { defineConfig } from "vitest/config";
import path from "node:path";

export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
      "monaco-editor": path.resolve(__dirname, "src/test/monaco-editor.ts"),
    },
  },
});
