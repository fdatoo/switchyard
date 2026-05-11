import { describe, it, expect } from "vitest";
import { parsePaletteInput } from "./palette-state";
import type { Verb } from "./palette-state";

// Minimal catalog for testing — mirrors the plan's built-in verb catalog.
const catalog: Verb[] = [
  {
    name: "events tail",
    description: "Stream live events",
    cliForm: "switchyard event tail",
    handlerRef: "events.tail",
    args: [
      { name: "source", type: "string", required: false, cliFlag: "--source", hint: "driver name" },
      { name: "kind", type: "string", required: false, cliFlag: "--kind", hint: "event kind" },
      { name: "entity", type: "string", required: false, cliFlag: "--entity", hint: "entity id" },
      { name: "since", type: "duration", required: false, cliFlag: "--since", hint: "e.g. 1h" },
    ],
  },
  {
    name: "events query",
    description: "Query event store",
    cliForm: "switchyard event query",
    handlerRef: "events.query",
    args: [
      { name: "kind", type: "string", required: false, cliFlag: "--kind", hint: "" },
      { name: "source", type: "string", required: false, cliFlag: "--source", hint: "" },
    ],
  },
  {
    name: "entity get",
    description: "Fetch an entity",
    cliForm: "switchyard entity get <id>",
    handlerRef: "entity.get",
    args: [
      { name: "id", type: "string", required: true, cliFlag: "--id", hint: "entity id" },
    ],
  },
  {
    name: "driver list",
    description: "List drivers",
    cliForm: "switchyard driver list",
    handlerRef: "driver.list",
    args: [],
  },
];

describe("parsePaletteInput", () => {
  it('empty string → kind: "empty"', () => {
    const result = parsePaletteInput("", catalog);
    expect(result.kind).toBe("empty");
  });

  it('"tai" → kind: partial; candidates include events tail', () => {
    const result = parsePaletteInput("tai", catalog);
    expect(result.kind).toBe("partial");
    if (result.kind === "partial") {
      const names = result.verbCandidates.map((v) => v.name);
      expect(names).toContain("events tail");
    }
  });

  it('"tail" → kind: partial; candidates include events tail', () => {
    const result = parsePaletteInput("tail", catalog);
    expect(result.kind).toBe("partial");
    if (result.kind === "partial") {
      const names = result.verbCandidates.map((v) => v.name);
      expect(names).toContain("events tail");
    }
  });

  it('"events tail" → kind: resolved; verb.name = events tail; missingRequired empty; 4 missingOptional', () => {
    const result = parsePaletteInput("events tail", catalog);
    expect(result.kind).toBe("resolved");
    if (result.kind === "resolved") {
      expect(result.verb.name).toBe("events tail");
      expect(Object.keys(result.filledArgs)).toHaveLength(0);
      expect(result.missingRequired).toHaveLength(0);
      expect(result.missingOptional.map((a) => a.name)).toEqual(
        expect.arrayContaining(["source", "kind", "entity", "since"])
      );
    }
  });

  it('"events tail z2m" → resolved; filledArgs = { source: "z2m" }', () => {
    const result = parsePaletteInput("events tail z2m", catalog);
    expect(result.kind).toBe("resolved");
    if (result.kind === "resolved") {
      expect(result.filledArgs).toEqual(expect.objectContaining({ source: "z2m" }));
    }
  });

  it('"tail source:z2m kind:err" → resolved; verb matches events tail; filledArgs = { source: "z2m", kind: "err" }', () => {
    const result = parsePaletteInput("tail source:z2m kind:err", catalog);
    expect(result.kind).toBe("resolved");
    if (result.kind === "resolved") {
      expect(result.verb.name).toBe("events tail");
      expect(result.filledArgs).toEqual(expect.objectContaining({ source: "z2m", kind: "err" }));
    }
  });

  it('"entity get abc-123" → resolved; filledArgs = { id: "abc-123" }; missingRequired empty', () => {
    const result = parsePaletteInput("entity get abc-123", catalog);
    expect(result.kind).toBe("resolved");
    if (result.kind === "resolved") {
      expect(result.filledArgs).toEqual({ id: "abc-123" });
      expect(result.missingRequired).toHaveLength(0);
    }
  });

  it('"entity get" → resolved; missingRequired = [{ name: "id" }]', () => {
    const result = parsePaletteInput("entity get", catalog);
    expect(result.kind).toBe("resolved");
    if (result.kind === "resolved") {
      expect(result.missingRequired).toHaveLength(1);
      expect(result.missingRequired[0].name).toBe("id");
    }
  });

  it("cliPreview assembled correctly for resolved verb with args", () => {
    const result = parsePaletteInput("events tail source:z2m", catalog);
    expect(result.kind).toBe("resolved");
    if (result.kind === "resolved") {
      expect(result.cliPreview).toContain("switchyard event tail");
      expect(result.cliPreview).toContain("--source=z2m");
    }
  });
});
