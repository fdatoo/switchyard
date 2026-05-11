import { test, expect } from "vitest";
import { pklLanguageDefinition } from "./pkl";

test("pklLanguageDefinition has required fields", () => {
  expect(pklLanguageDefinition.tokenizer).toBeDefined();
  expect(pklLanguageDefinition.keywords).toContain("amends");
  expect(pklLanguageDefinition.keywords).toContain("module");
  expect(pklLanguageDefinition.keywords).toContain("class");
  expect(pklLanguageDefinition.keywords).toContain("extends");
  expect(pklLanguageDefinition.keywords).toContain("import");
  expect(pklLanguageDefinition.keywords).toContain("function");
  expect(pklLanguageDefinition.keywords).toContain("let");
  expect(pklLanguageDefinition.keywords).toContain("when");
  expect(pklLanguageDefinition.keywords).toContain("is");
  expect(pklLanguageDefinition.keywords).toContain("as");
  expect(pklLanguageDefinition.keywords).toContain("new");
  expect(pklLanguageDefinition.keywords).toContain("this");
  expect(pklLanguageDefinition.keywords).toContain("outer");
  expect(pklLanguageDefinition.keywords).toContain("super");
  expect(pklLanguageDefinition.keywords).toContain("null");
  expect(pklLanguageDefinition.keywords).toContain("true");
  expect(pklLanguageDefinition.keywords).toContain("false");
});
