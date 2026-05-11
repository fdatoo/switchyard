import { RuleTester } from "eslint";
import { describe, it } from "vitest";
import rule from "./no-raw-tokens";

const tester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

describe("switchyard/no-raw-tokens", () => {
  it("fails on raw color, radius, and spacing utilities", () => {
    tester.run("no-raw-tokens", rule, {
      valid: [
        // Non-color Tailwind classes pass
        { code: '<div className="surface-panel control-compact" />' },
        // Token-aliased utilities pass
        { code: '<div className="bg-surface bg-token-danger text-[var(--sy-color-fg)] rounded-token-md p-token-sm gap-[var(--sy-space-2)]" />' },
        // --sy-* var() refs in style prop are always valid
        { code: '<div style={{ color: "var(--sy-color-fg)", backgroundColor: "var(--sy-color-bg)" }} />' },
        { code: '<div style={{ borderRadius: "var(--sy-radius)", padding: "var(--sy-space-3)" }} />' },
        { code: '<div style={{ boxShadow: "var(--sy-shadow)", font: "var(--sy-font-body)" }} />' },
        { code: '<div style={{ transition: "var(--sy-motion)", background: "var(--sy-gradient-tod)" }} />' },
        // Full --sy-* token surface in style prop
        {
          code: `<div style={{
            color: "var(--sy-color-fg)",
            background: "var(--sy-color-bg)",
            borderColor: "var(--sy-color-line)",
            accentColor: "var(--sy-color-accent)",
          }} />`,
        },
      ],
      invalid: [
        {
          code: '<div className="hover:bg-red-500 rounded-lg p-4" />',
          errors: [
            { messageId: "rawToken" },
            { messageId: "rawToken" },
            { messageId: "rawToken" },
          ],
        },
        {
          code: '<div className={`md:text-blue-400 ring-rose-300 divide-gray-200 gap-x-2 -mt-1`} />',
          errors: [
            { messageId: "rawToken" },
            { messageId: "rawToken" },
            { messageId: "rawToken" },
            { messageId: "rawToken" },
            { messageId: "rawToken" },
          ],
        },
        {
          code: '<div className="bg-[#fff] p-[8px] rounded-[6px]" />',
          errors: [
            { messageId: "rawToken" },
            { messageId: "rawToken" },
            { messageId: "rawToken" },
          ],
        },
        {
          code: '<div style={{ color: "#ff0000", borderColor: "rgb(255 0 0)" }} />',
          errors: [
            { messageId: "rawStyleColor" },
            { messageId: "rawStyleColor" },
          ],
        },
      ],
    });
  });
});
