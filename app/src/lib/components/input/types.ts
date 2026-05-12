/**
 * Shared prop types for SyInput and its per-language variants.
 *
 * Kept in a separate `.ts` file (rather than exported from a `.vue` SFC) so
 * sibling variant components can import them without Vue type-export quirks.
 */

/** Native input type. Constrained to types where a single-line text input is appropriate. */
export type InputType = "text" | "email" | "password" | "number" | "search" | "tel" | "url";

/** Size scale. Variants choose their own height/padding/font-size per size. */
export type InputSize = "sm" | "md" | "lg";
