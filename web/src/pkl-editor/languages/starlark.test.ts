import { test, expect } from "vitest";
import { starlarkLanguageDefinition } from "./starlark";

test("starlark tokenizer has keywords and builtins", () => {
  expect(starlarkLanguageDefinition.keywords).toContain("def");
  expect(starlarkLanguageDefinition.keywords).toContain("return");
  expect(starlarkLanguageDefinition.keywords).toContain("load");
  expect(starlarkLanguageDefinition.builtins).toContain("len");
  expect(starlarkLanguageDefinition.builtins).toContain("range");
  expect(starlarkLanguageDefinition.builtins).toContain("print");
  expect(starlarkLanguageDefinition.builtins).toContain("int");
  expect(starlarkLanguageDefinition.builtins).toContain("str");
});
