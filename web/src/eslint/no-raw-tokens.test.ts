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
        { code: '<div className="surface-panel control-compact" />' },
        { code: '<div className="bg-surface bg-token-danger text-[var(--gh-color-fg)] rounded-token-md p-token-sm gap-[var(--gh-pad-normal)]" />' },
        { code: '<div style={{ color: "var(--gh-color-fg)", backgroundColor: "var(--gh-color-bg)" }} />' },
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
