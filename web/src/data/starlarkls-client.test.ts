import { test, expect } from "vitest";
import { buildStarlarkLsClient } from "./starlarkls-client";

test("buildStarlarkLsClient returns object with complete, hover, lookupSymbol", () => {
  const c = buildStarlarkLsClient("http://localhost:8080");
  expect(typeof c.complete).toBe("function");
  expect(typeof c.hover).toBe("function");
  expect(typeof c.lookupSymbol).toBe("function");
});
