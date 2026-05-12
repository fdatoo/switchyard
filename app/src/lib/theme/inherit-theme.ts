/**
 * Walk up the DOM from `el` to find the nearest ancestor with
 * `data-language` and `data-mode` attributes, returning their values.
 *
 * Components that teleport content to `<body>` (tooltips, menus, sheets)
 * use this to find the trigger's effective theme. Without it, the
 * teleported content would always inherit the document-level theme,
 * which is wrong when a per-cell or per-region theme override is in
 * effect (e.g., the `/lab` showcase, embedded ambient displays).
 *
 * Language and mode are looked up independently — a parent might set
 * only one. Stops walking once both are resolved.
 */
export function getEffectiveTheme(el: Element | null): {
  language: string | null;
  mode: string | null;
} {
  let language: string | null = null;
  let mode: string | null = null;
  let cursor: Element | null = el;
  while (cursor && (!language || !mode)) {
    if (!language) language = cursor.getAttribute("data-language");
    if (!mode) mode = cursor.getAttribute("data-mode");
    cursor = cursor.parentElement;
  }
  return { language, mode };
}

/**
 * Convenience: apply the effective theme (from `anchor`) to `target` as
 * `data-language` / `data-mode` attributes. Used by teleported popovers
 * after they mount so their tokens match the anchor's region.
 */
export function applyAnchorTheme(target: Element, anchor: Element | null): void {
  const { language, mode } = getEffectiveTheme(anchor);
  if (language) target.setAttribute("data-language", language);
  if (mode) target.setAttribute("data-mode", mode);
}
