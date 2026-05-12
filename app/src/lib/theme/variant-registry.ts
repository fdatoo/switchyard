/**
 * Variant registry: per-slot, per-language component map.
 *
 * Variant components are how the lib expresses per-language **shape** changes
 * (pill vs. sharp chip vs. glass capsule for `Button`) that pure-token theming
 * can't capture. Each variant-bearing primitive ships a dispatcher component
 * that reads the active language and renders the registered variant via
 * `<component :is="…">`.
 *
 * The registry is also the extension point for user-defined languages. The
 * API is intentionally the same one a third-party language pack would call
 * to install its own variants. The loader/isolation story for user packs is
 * future work, but this surface is the contract.
 *
 * Built-in registrations live in `./builtin-variants.ts` and run once at
 * app boot from `main.ts`.
 */

import { shallowReactive } from "vue";
import type { Component } from "vue";
import type { LanguageId, SlotName } from "./types";

/**
 * `shallowReactive` so that dispatchers (which read via `computed`) re-render
 * if a language pack lazily registers a new variant after first render. We
 * don't deep-watch because the inner `Map<LanguageId, Component>` is treated
 * as a flat lookup table.
 */
const registry = shallowReactive(new Map<SlotName, Map<LanguageId, Component>>());

/**
 * Fallback language used when a slot has no variant registered for the
 * active language. `friendly` is the canonical full-coverage language; if a
 * user-defined language only overrides some slots, we degrade to friendly
 * rather than rendering nothing.
 */
const FALLBACK_LANGUAGE: LanguageId = "friendly";

/**
 * Register a component as the variant for `slot` in `language`.
 *
 * Idempotent in practice: re-registering replaces the previous component for
 * that (slot, language). This lets a HMR reload or a language-pack hot-swap
 * work without restarting the app.
 */
export function registerVariant(
  slot: SlotName,
  language: LanguageId,
  component: Component,
): void {
  let slotMap = registry.get(slot);
  if (!slotMap) {
    slotMap = new Map();
    registry.set(slot, slotMap);
  }
  slotMap.set(language, component);
}

/**
 * Resolve which component should render the slot under the given language.
 *
 * Resolution order:
 *   1. Exact match for `language`.
 *   2. The `friendly` fallback (the canonical full-coverage language).
 *   3. The first variant registered for the slot, whatever its language.
 *
 * Throws if the slot has no registrations at all — a slot with no variants
 * is a bug, not a runtime fallback case.
 */
export function resolveVariant(slot: SlotName, language: LanguageId): Component {
  const slotMap = registry.get(slot);
  if (!slotMap) {
    throw new Error(`No variants registered for slot "${slot}"`);
  }
  const exact = slotMap.get(language);
  if (exact) return exact;
  const fallback = slotMap.get(FALLBACK_LANGUAGE);
  if (fallback) return fallback;
  const [first] = slotMap.values();
  if (first) return first;
  throw new Error(`No variants registered for slot "${slot}" and no fallback available`);
}

/**
 * List the languages that have registered a variant for `slot`. Mostly useful
 * for diagnostics / the /lab showcase / a future "which slots is my language
 * missing?" tool.
 */
export function listRegisteredLanguages(slot: SlotName): LanguageId[] {
  return Array.from(registry.get(slot)?.keys() ?? []);
}
