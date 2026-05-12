/**
 * Shared prop types for SyButton and its per-language variants.
 *
 * Lives in a separate `.ts` file rather than being exported from a `.vue`
 * `<script setup>` because Vue's type re-export from SFCs is flaky and the
 * variant components need to import these types as siblings.
 */

/**
 * Semantic role of the button. Variants choose a color palette per intent:
 *   - `primary` — the default affirmative action; uses the accent color.
 *   - `secondary` — a quieter alternative; surface fill.
 *   - `ghost` — text-only until hovered; for low-emphasis actions in dense UI.
 *   - `danger` — destructive actions (delete, disconnect).
 */
export type ButtonIntent = "primary" | "secondary" | "ghost" | "danger";

/** Size scale. Per-language variants choose their own padding/height per size. */
export type ButtonSize = "sm" | "md" | "lg";
