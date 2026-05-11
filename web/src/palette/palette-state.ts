/**
 * palette-state.ts
 * Pure client-side state machine for the command palette input parser.
 * No server round-trips during typing — all computation is synchronous
 * from the locally cached verb catalog.
 *
 * UI v2 Plan 05.
 */

export type ArgType = "string" | "int" | "bool" | "duration" | "string_list";

export interface ArgSchema {
  name: string;
  type: ArgType;
  required: boolean;
  cliFlag: string;
  hint: string;
}

export interface Verb {
  name: string;
  description: string;
  cliForm: string;
  handlerRef: string;
  args: ArgSchema[];
}

export type ParsedPaletteState =
  | { kind: "empty" }
  | { kind: "partial"; verbCandidates: Verb[]; rawInput: string }
  | {
      kind: "resolved";
      verb: Verb;
      filledArgs: Record<string, string>;
      missingRequired: ArgSchema[];
      missingOptional: ArgSchema[];
      cliPreview: string;
    };

/**
 * parsePaletteInput is the core tokenizer + matcher.
 *
 * Tokenization rules (from plan decisions §9):
 * 1. Input split on whitespace.
 * 2. Try two-word verb match first, then one-word.
 * 3. Remaining tokens of the form key:value fill the matching arg by name.
 * 4. Remaining tokens that are NOT key:value fill positional args in schema order.
 * 5. cliPreview = verb.cliForm + filled args as --flag=value in schema order.
 */
export function parsePaletteInput(
  input: string,
  catalog: Verb[],
): ParsedPaletteState {
  const trimmed = input.trim();
  if (trimmed === "") {
    return { kind: "empty" };
  }

  const tokens = trimmed.split(/\s+/);

  // --- Attempt two-word verb match ---
  if (tokens.length >= 2) {
    const candidate = `${tokens[0]} ${tokens[1]}`;
    const verb = catalog.find((v) => v.name === candidate);
    if (verb) {
      return buildResolved(verb, tokens.slice(2));
    }
  }

  // --- Attempt one-word verb exact match (e.g. "entity get") ---
  const oneWordExact = catalog.find((v) => v.name === tokens[0]);
  if (oneWordExact) {
    return buildResolved(oneWordExact, tokens.slice(1));
  }

  // --- Partial matching: any verb whose name or any word in the name starts with
  //     or contains the prefix tokens.
  //
  //     When all remaining tokens (beyond the first) are key:value pairs, and
  //     only one verb candidate matches the first token, treat it as resolved
  //     (e.g. "tail source:z2m kind:err" → resolves to "events tail").
  // ---
  const firstTok = tokens[0].toLowerCase();
  // Key:value tokens: colonIdx > 0 means it's a named arg.
  const allRestAreKV = tokens.slice(1).every((t) => t.indexOf(":") > 0);

  const candidates = catalog.filter((v) => {
    const lc = v.name.toLowerCase();
    // Match if any word in the verb name starts with any token.
    return tokens.some((tok) => {
      const t = tok.toLowerCase();
      return lc.includes(t);
    });
  });

  // If exactly one candidate, tokens.length > 1, and all remaining tokens are
  // key:value pairs, treat as resolved (e.g. "tail source:z2m" → events tail).
  if (candidates.length === 1 && tokens.length > 1 && allRestAreKV) {
    const onlyCandidate = candidates[0];
    // Only resolve if the first token is a substring match of the verb name.
    if (onlyCandidate.name.toLowerCase().includes(firstTok)) {
      return buildResolved(onlyCandidate, tokens.slice(1));
    }
  }

  if (candidates.length > 0) {
    return { kind: "partial", verbCandidates: candidates, rawInput: trimmed };
  }

  // No candidates at all — treat as partial with empty candidates.
  return { kind: "partial", verbCandidates: [], rawInput: trimmed };
}

function buildResolved(
  verb: Verb,
  remainingTokens: string[],
): Extract<ParsedPaletteState, { kind: "resolved" }> {
  const filledArgs: Record<string, string> = {};

  // Separate key:value tokens from positional tokens.
  const positional: string[] = [];
  for (const tok of remainingTokens) {
    const colonIdx = tok.indexOf(":");
    if (colonIdx > 0) {
      const key = tok.slice(0, colonIdx);
      const value = tok.slice(colonIdx + 1);
      // Only fill if the key matches a known arg name.
      if (verb.args.some((a) => a.name === key)) {
        filledArgs[key] = value;
      } else {
        // Treat as positional if key doesn't match an arg name.
        positional.push(tok);
      }
    } else {
      positional.push(tok);
    }
  }

  // Fill positional args into the first unfilled arg in schema order.
  let posIdx = 0;
  for (const arg of verb.args) {
    if (posIdx >= positional.length) break;
    if (!(arg.name in filledArgs)) {
      filledArgs[arg.name] = positional[posIdx++];
    }
  }

  // Compute missing args.
  const missingRequired = verb.args.filter(
    (a) => a.required && !(a.name in filledArgs),
  );
  const missingOptional = verb.args.filter(
    (a) => !a.required && !(a.name in filledArgs),
  );

  // Build CLI preview string from cliForm + filled args in schema order.
  const flagParts = verb.args
    .filter((a) => a.name in filledArgs)
    .map((a) => `${a.cliFlag}=${filledArgs[a.name]}`);
  const cliPreview =
    flagParts.length > 0
      ? `${verb.cliForm} ${flagParts.join(" ")}`
      : verb.cliForm;

  return {
    kind: "resolved",
    verb,
    filledArgs,
    missingRequired,
    missingOptional,
    cliPreview,
  };
}
