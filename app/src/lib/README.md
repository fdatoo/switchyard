# Switchyard component library

The Vue component library that backs the Switchyard UI. Three coordinated design
languages (`friendly`, `developer`, `ambient`) render the same component tree
fed by the same RPCs. Switching languages is a runtime preset change, not a
build-time decision.

This file documents the architecture and conventions. If you're adding or
modifying a component, read this first.

## Directory layout

```
lib/
  tokens/           CSS custom properties — one file per (language, mode).
  theme/            Language state, variant registry, builtin registrations.
  components/       The components themselves, grouped by primitive.
  index.ts          Public surface. Consumers import from "@/lib".
```

Outside the lib but related:

```
src/views/lab/      The /lab showcase. Renders every component in every
                    (language, mode) combination side-by-side.
```

## The two axes

Everything visual is governed by two attributes on the document root:

- `[data-language]` — `friendly` | `developer` | `ambient` | `<user-defined>`.
- `[data-mode]` — `light` | `dark`.

Only `friendly` honors `[data-mode]`. `developer` and `ambient` are dark-only
and the store refuses to switch their mode.

CSS selectors in `tokens/*.css` use **both** attributes via descendant matching,
so token overrides also apply to per-element scopes (e.g., a `/lab` cell that
sets `data-language="ambient"` on a `<div>` gets ambient tokens inside that
div, even when the page-level language is `friendly`).

## How a component picks up the theme

Every visual value in a component is a CSS custom property reference
(`var(--sy-color-fg)`, `var(--sy-radius-lg)`, etc.). No hex literals, no raw
px in component CSS — when the active language changes its tokens, every
component updates with no JS involvement.

The full token surface is documented in
`docs/design/specs/2026-05-11-ui-v2-vision-design.md` §20.

## When to use a variant component

Most components can express their per-language differences entirely through
tokens — that's the whole point of the token system. A few components have
**shape** differences that tokens can't capture (a pill versus a sharp chip is
not a color difference; a glassmorphic capsule is not a radius difference).
For those, the lib ships one component per language and a dispatcher.

Examples in the lib today:

- **`SyButton`** has three variants: friendly is a pill, developer is a 4px-corner
  chip, ambient is a tall glassmorphic capsule with backdrop blur.
- **`SyInput`** has three variants: friendly is rounded-lg with a soft accent ring
  on focus; developer is sharp with an inset 1px outline; ambient is rounded-xl
  with backdrop-blur and 44px touch height.

Components that do **not** need variants today:

- `SyText`, `SySurface`, `SyBadge` — token-driven; the per-language differences
  (radius, surface color, accent tint, ambient backdrop-blur) all flow through
  CSS custom properties.

The decision boundary: if the only difference between two languages is a value
that already lives in a token, no variant. If the difference is structural
(different layout, different children, different motion behavior, different
HTML shape), it's a variant.

## How variants are dispatched

`theme/variant-registry.ts` keeps a `Map<SlotName, Map<LanguageId, Component>>`.
A variant dispatcher component (e.g., `SyButton.vue`) reads the active
language from the Pinia language store, looks up the variant in the registry,
and renders it via `<component :is="…">`.

Registration happens once at app boot, in
`theme/builtin-variants.ts`, which is called from `main.ts` before mount.

To add a new variant-bearing primitive:

1. Add the slot name to the `SlotName` union in `theme/types.ts`.
2. Implement the per-language components.
3. Implement a dispatcher (`SyFoo.vue`) that calls `resolveVariant("Foo", language)`.
4. Register each variant in `installBuiltinVariants` (`theme/builtin-variants.ts`).

## User-defined languages

The variant registry's API (`registerVariant(slot, language, component)`) is
the same API a user-defined language pack will eventually use to install its
own variants — that's why it exists as a registry rather than a direct
component import.

For v1 we only register built-in languages. The infrastructure to load
user-supplied language packs (signed widget-pack-style bundles that ship token
CSS plus Vue components) is future work. The registration API is stable; the
loader, isolation, and signing story still need to be designed.

## Conventions

- **Component name prefix:** `Sy` (e.g., `SyButton`). User-defined language
  packs that ship their own primitives use a different prefix.
- **Single-file components** (`.vue`) with `<script setup lang="ts">` and
  `<style scoped>`.
- **Types live in `types.ts`** next to the component (not in the `.vue` file)
  when they need to be imported by sibling variant components — Vue can have
  flaky type re-export from SFCs.
- **`box-sizing: border-box`** is globally applied in `styles.css`. Don't fight
  it.
- **No hardcoded colors or sizes** in component CSS. Reference tokens. The one
  exception so far is the dot inside `SyBadge`, where 6px is the dot's
  intrinsic size, not a design-system spacing decision.

## The lab

`/lab` renders every component in every `(language, mode)` combination. This
is the load-bearing surface for design review — drift here means the lib has
drifted. Every new component must add a specimen to `views/lab/` and a section
to `views/LabView.vue` in the same pass.
