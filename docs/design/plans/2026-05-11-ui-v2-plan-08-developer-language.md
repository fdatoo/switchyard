# Plan 08 — Developer Language

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the developer language — primitive variants, vocabulary swap, and table-shaped layouts — so the same React component tree that renders the friendly UI also renders a sharp, info-dense, Linear-grade technical interface, proving the theming engine is real.

**Architecture:** Plan 01's `LanguagePrimitives` provider ships `null` slots for developer variants; this plan fills them with sharper, denser, monospace-aware variants of Button, Chip, Pill, and Surface. A typed `useVocab()` hook drives the nav label swap. Rooms switches from a card grid to a sortable `RoomsTable` component when `data-language="developer"` is detected. All CSS-only affordances (keyboard shortcut labels, dense spacing) are driven by the `data-language` attribute Plan 01 writes — no extra React context subscribers.

**Tech Stack:** React 19, TypeScript strict, CSS custom properties (`--sy-*` tokens), Vitest, Playwright, TanStack Router (`useRouterState`).

**Spec refs:** §4.3 (developer aesthetic + vocabulary), §14 (developer language), §20.4 (dark-only token override).

**Mockup:** `.superpowers/brainstorm/71337-1778492716/screenshots/11-settings-and-developer-02.png`

**Branch:** `feat/ui-v2-plan-08-developer-language`
**Worktree:** `.claude/worktrees/plan-08-developer-language`
**Depends on:** Plan 01 merged to main

---

## Decisions (locked — no ambiguity for the implementer)

1. **`developer.css` token values are already authored in Plan 01.** This plan only registers React-layer artefacts: primitive variants, the vocab hook, and the `RoomsTable` component. Zero CSS changes to `developer.css`.
2. **Developer language is dark-only.** `LanguageProvider` already handles this (§20.4 / Plan 01 Decision 3). No mode axis exists for developer.
3. **`useVocab()` returns a typed lookup keyed by route ID.** No string interpolation, no i18n library, no runtime format strings. It is a plain object lookup: `vocabMap[language][routeId]`.
4. **`RoomsTable` is a new component.** `RoomGrid.tsx` (Plan 06) switches between `<RoomsGrid>` (friendly / ambient) and `<RoomsTable>` (developer) by reading `useLanguage().language`. No prop threading — the language is ambient context.
5. **Keyboard shortcut labels (⌘1, ⌘2, …) are already in the DOM** from Plan 01's Sidebar, hidden via CSS. This plan adds the single CSS rule `[data-language="developer"] .kbd-shortcut { display: inline; }` to reveal them. No Sidebar JSX changes needed.
6. **Primitive variants use only `--sy-*` tokens.** The `switchyard/no-raw-tokens` ESLint rule must accept every variant file without suppressions.
7. **The friendly Home page renders correctly in developer language.** The Playwright cross-language snapshot test in Task 8.7 is the regression gate — same route, four languages.

---

## File plan

### Created

```
web/src/theme/primitives/developer/
  button.tsx     ← sharp-cornered, monospace-label, dense padding variant
  chip.tsx       ← 3px radius, tight horizontal padding, mono font
  pill.tsx       ← flat rectangular status badge (no border-radius pill)
  surface.tsx    ← near-black surface, 4px radius, 1px line border

web/src/theme/vocab/
  index.ts       ← useVocab() hook + VocabMap type + routeId union
  friendly.ts    ← friendly vocab lookup (Home / Rooms / Activity / …)
  developer.ts   ← developer vocab lookup (Overview / Entities / Events / …)
  ambient.ts     ← ambient vocab lookup (same as friendly for now)

web/src/pages-system/widgets/sections/
  RoomsTable.tsx ← sortable table of rooms (developer language)
```

### Modified

```
web/src/theme/primitives-provider.tsx
  ← register the four developer primitive variants (currently null slots)

web/src/shell/Sidebar.tsx
  ← consume useVocab() for primary nav labels

web/src/shell/TopBar.tsx
  ← consume useVocab() for breadcrumb page name

web/src/pages-system/widgets/sections/RoomGrid.tsx
  ← switch between <RoomsGrid> and <RoomsTable> based on language

web/src/theme/languages/developer.css
  ← add single CSS rule revealing .kbd-shortcut in developer language
```

---

## Tasks

### Task 8.1 — Developer primitive variants

**Files:**
- Create: `web/src/theme/primitives/developer/button.tsx`
- Create: `web/src/theme/primitives/developer/chip.tsx`
- Create: `web/src/theme/primitives/developer/pill.tsx`
- Create: `web/src/theme/primitives/developer/surface.tsx`
- Modify: `web/src/theme/primitives-provider.tsx`

- [ ] **Step 1: Write failing test for developer Surface variant**

Create `web/src/theme/primitives/developer/surface.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { LanguageProvider } from "../../language-provider";
import { LanguagePrimitives, usePrimitive } from "../../primitives-provider";

function SurfaceConsumer() {
  const Surface = usePrimitive("Surface");
  return <Surface data-testid="surface">content</Surface>;
}

test("developer Surface renders with data-variant=developer-surface", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <LanguagePrimitives>
        <SurfaceConsumer />
      </LanguagePrimitives>
    </LanguageProvider>
  );
  expect(screen.getByTestId("surface")).toHaveAttribute(
    "data-variant",
    "developer-surface"
  );
});

test("developer Surface renders children", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <LanguagePrimitives>
        <SurfaceConsumer />
      </LanguagePrimitives>
    </LanguageProvider>
  );
  expect(screen.getByTestId("surface")).toHaveTextContent("content");
});
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd web && npx vitest run src/theme/primitives/developer/surface.test.tsx
```

Expected: FAIL — `developer-surface` variant not registered yet.

- [ ] **Step 3: Create `developer/surface.tsx`**

```tsx
import type { HTMLAttributes } from "react";

interface DeveloperSurfaceProps extends HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
}

export function DeveloperSurface({ children, ...props }: DeveloperSurfaceProps) {
  return (
    <div data-variant="developer-surface" {...props}>
      {children}
    </div>
  );
}
```

- [ ] **Step 4: Create `developer/button.tsx`**

```tsx
import type { ButtonHTMLAttributes } from "react";

interface DeveloperButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children?: React.ReactNode;
}

export function DeveloperButton({ children, ...props }: DeveloperButtonProps) {
  return (
    <button data-variant="developer-button" {...props}>
      {children}
    </button>
  );
}
```

- [ ] **Step 5: Create `developer/chip.tsx`**

```tsx
import type { HTMLAttributes } from "react";

interface DeveloperChipProps extends HTMLAttributes<HTMLSpanElement> {
  children?: React.ReactNode;
}

export function DeveloperChip({ children, ...props }: DeveloperChipProps) {
  return (
    <span data-variant="developer-chip" {...props}>
      {children}
    </span>
  );
}
```

- [ ] **Step 6: Create `developer/pill.tsx`**

```tsx
import type { HTMLAttributes } from "react";

interface DeveloperPillProps extends HTMLAttributes<HTMLSpanElement> {
  children?: React.ReactNode;
}

export function DeveloperPill({ children, ...props }: DeveloperPillProps) {
  return (
    <span data-variant="developer-pill" {...props}>
      {children}
    </span>
  );
}
```

- [ ] **Step 7: Register developer variants in `primitives-provider.tsx`**

Find the `null` slots for the developer language in `primitives-provider.tsx` and replace with the actual components. The registry object should look like:

```tsx
import { DeveloperButton } from "./primitives/developer/button";
import { DeveloperChip } from "./primitives/developer/chip";
import { DeveloperPill } from "./primitives/developer/pill";
import { DeveloperSurface } from "./primitives/developer/surface";

// inside the registry constant:
developer: {
  Button: DeveloperButton,
  Chip: DeveloperChip,
  Pill: DeveloperPill,
  Surface: DeveloperSurface,
},
```

- [ ] **Step 8: Run the test — verify it passes**

```bash
cd web && npx vitest run src/theme/primitives/developer/surface.test.tsx
```

Expected: PASS (2 tests).

- [ ] **Step 9: Run lint to confirm no raw tokens**

```bash
cd web && npx eslint src/theme/primitives/developer/
```

Expected: no errors. (The components currently use no inline styles; CSS for the variants lives in `developer.css` which Plan 01 already authored.)

- [ ] **Step 10: Commit**

```bash
git add web/src/theme/primitives/developer/ web/src/theme/primitives-provider.tsx
git commit -m "feat(web): developer primitive variants (UI v2 plan 08)"
```

---

### Task 8.2 — `useVocab` hook + three language lookup tables

**Files:**
- Create: `web/src/theme/vocab/friendly.ts`
- Create: `web/src/theme/vocab/developer.ts`
- Create: `web/src/theme/vocab/ambient.ts`
- Create: `web/src/theme/vocab/index.ts`

- [ ] **Step 1: Write failing tests for `useVocab`**

Create `web/src/theme/vocab/index.test.ts`:

```ts
import { renderHook } from "@testing-library/react";
import { LanguageProvider } from "../language-provider";
import { useVocab } from "./index";
import React from "react";

function wrapper(language: "friendly" | "developer" | "ambient") {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(LanguageProvider, { initialLanguage: language }, children);
}

test("friendly vocab: home route returns 'Home'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("friendly"),
  });
  expect(result.current.label("home")).toBe("Home");
});

test("friendly vocab: rooms route returns 'Rooms'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("friendly"),
  });
  expect(result.current.label("rooms")).toBe("Rooms");
});

test("developer vocab: home route returns 'Overview'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(result.current.label("home")).toBe("Overview");
});

test("developer vocab: rooms route returns 'Entities'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(result.current.label("rooms")).toBe("Entities");
});

test("developer vocab: activity route returns 'Events'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(result.current.label("activity")).toBe("Events");
});

test("ambient vocab: home route returns 'Home'", () => {
  const { result } = renderHook(() => useVocab(), {
    wrapper: wrapper("ambient"),
  });
  expect(result.current.label("home")).toBe("Home");
});
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd web && npx vitest run src/theme/vocab/index.test.ts
```

Expected: FAIL — module not found.

- [ ] **Step 3: Create `vocab/friendly.ts`**

```ts
export const friendlyVocab = {
  home: "Home",
  rooms: "Rooms",
  activity: "Activity",
  automations: "Automations",
  devices: "Devices",
  settings: "Settings",
} as const;
```

- [ ] **Step 4: Create `vocab/developer.ts`**

```ts
export const developerVocab = {
  home: "Overview",
  rooms: "Entities",
  activity: "Events",
  automations: "Automations",
  devices: "Devices",
  settings: "Settings",
} as const;
```

- [ ] **Step 5: Create `vocab/ambient.ts`**

```ts
// Ambient uses friendly labels — the surface is curated, not technical.
export const ambientVocab = {
  home: "Home",
  rooms: "Rooms",
  activity: "Activity",
  automations: "Automations",
  devices: "Devices",
  settings: "Settings",
} as const;
```

- [ ] **Step 6: Create `vocab/index.ts`**

```ts
import { useLanguage } from "../language-provider";
import { friendlyVocab } from "./friendly";
import { developerVocab } from "./developer";
import { ambientVocab } from "./ambient";

export type RouteId =
  | "home"
  | "rooms"
  | "activity"
  | "automations"
  | "devices"
  | "settings";

type VocabMap = Record<RouteId, string>;

const vocabByLanguage: Record<"friendly" | "developer" | "ambient", VocabMap> = {
  friendly: friendlyVocab,
  developer: developerVocab,
  ambient: ambientVocab,
};

export interface VocabHandle {
  label: (routeId: RouteId) => string;
}

export function useVocab(): VocabHandle {
  const { language } = useLanguage();
  const map = vocabByLanguage[language];
  return {
    label: (routeId) => map[routeId],
  };
}
```

- [ ] **Step 7: Run test — verify it passes**

```bash
cd web && npx vitest run src/theme/vocab/index.test.ts
```

Expected: PASS (6 tests).

- [ ] **Step 8: Commit**

```bash
git add web/src/theme/vocab/
git commit -m "feat(web): useVocab hook + three-language lookup tables (UI v2 plan 08)"
```

---

### Task 8.3 — Apply `useVocab` to Sidebar + TopBar breadcrumb

**Files:**
- Modify: `web/src/shell/Sidebar.tsx`
- Modify: `web/src/shell/TopBar.tsx`
- Modify (tests): `web/src/shell/Sidebar.test.tsx`, `web/src/shell/TopBar.test.tsx`

- [ ] **Step 1: Add failing test — developer Sidebar shows 'Overview' not 'Home'**

In `web/src/shell/Sidebar.test.tsx`, add:

```tsx
import { LanguageProvider } from "../theme/language-provider";

test("developer language: sidebar nav item reads 'Overview' for home", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <MemoryRouter initialEntries={["/_authed/home"]}>
        <Sidebar />
      </MemoryRouter>
    </LanguageProvider>
  );
  expect(screen.getByRole("link", { name: /overview/i })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /^home$/i })).not.toBeInTheDocument();
});

test("developer language: sidebar nav item reads 'Events' for activity", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <MemoryRouter initialEntries={["/_authed/activity"]}>
        <Sidebar />
      </MemoryRouter>
    </LanguageProvider>
  );
  expect(screen.getByRole("link", { name: /events/i })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd web && npx vitest run src/shell/Sidebar.test.tsx
```

Expected: FAIL — Sidebar still shows hardcoded "Home".

- [ ] **Step 3: Update `Sidebar.tsx` to consume `useVocab`**

Add the import at the top of `Sidebar.tsx`:

```tsx
import { useVocab } from "../theme/vocab";
```

Inside the Sidebar component body, before the nav items, add:

```tsx
const vocab = useVocab();
```

Replace each hardcoded nav label string with `vocab.label(routeId)`. The six primary nav items map to route IDs as follows:

```tsx
const navItems = [
  { routeId: "home" as const,        href: "/_authed/home" },
  { routeId: "rooms" as const,       href: "/_authed/rooms" },
  { routeId: "activity" as const,    href: "/_authed/activity" },
  { routeId: "automations" as const, href: "/_authed/automations" },
  { routeId: "devices" as const,     href: "/_authed/devices" },
  { routeId: "settings" as const,    href: "/_authed/settings" },
];
```

Render each as:

```tsx
{navItems.map(({ routeId, href }) => (
  <NavLink key={routeId} to={href}>
    {vocab.label(routeId)}
  </NavLink>
))}
```

- [ ] **Step 4: Update `TopBar.tsx` to consume `useVocab` for breadcrumb**

Add the import:

```tsx
import { useVocab, type RouteId } from "../theme/vocab";
```

Inside `TopBar`, derive the current route ID from TanStack Router state and look up the breadcrumb label:

```tsx
const vocab = useVocab();
const routerState = useRouterState();

// Extract the first non-root segment as the route ID for breadcrumb.
// e.g. "/_authed/activity" → "activity"
const segments = routerState.location.pathname.split("/").filter(Boolean);
const routeId = (segments[1] ?? "home") as RouteId;
const breadcrumb = vocab.label(routeId);
```

Replace any hardcoded breadcrumb string with `{breadcrumb}`.

- [ ] **Step 5: Add failing TopBar breadcrumb test for developer language**

In `web/src/shell/TopBar.test.tsx`, add:

```tsx
test("developer language: breadcrumb reads 'Events' at /activity", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <MemoryRouter initialEntries={["/_authed/activity"]}>
        <TopBar />
      </MemoryRouter>
    </LanguageProvider>
  );
  expect(screen.getByText("Events")).toBeInTheDocument();
});
```

- [ ] **Step 6: Run all shell tests — verify they pass**

```bash
cd web && npx vitest run src/shell/
```

Expected: all existing tests still pass; new tests pass.

- [ ] **Step 7: Commit**

```bash
git add web/src/shell/Sidebar.tsx web/src/shell/TopBar.tsx \
        web/src/shell/Sidebar.test.tsx web/src/shell/TopBar.test.tsx
git commit -m "feat(web): apply useVocab to Sidebar and TopBar breadcrumb (UI v2 plan 08)"
```

---

### Task 8.4 — `RoomsTable` component

**Files:**
- Create: `web/src/pages-system/widgets/sections/RoomsTable.tsx`
- Create: `web/src/pages-system/widgets/sections/RoomsTable.test.tsx`

- [ ] **Step 1: Write failing tests**

Create `web/src/pages-system/widgets/sections/RoomsTable.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { RoomsTable } from "./RoomsTable";

const rooms = [
  { id: "r1", name: "Living Room", state: "on",  scene: "Evening", brightness: 72, sinceMs: 120_000 },
  { id: "r2", name: "Kitchen",     state: "off", scene: "—",       brightness: 0,  sinceMs: 600_000 },
  { id: "r3", name: "Bedroom",     state: "on",  scene: "Night",   brightness: 20, sinceMs: 30_000 },
];

test("renders one row per room", () => {
  render(<RoomsTable rooms={rooms} />);
  expect(screen.getAllByRole("row")).toHaveLength(rooms.length + 1); // +1 header
});

test("renders column headers: Name, State, Scene, Brightness, Since", () => {
  render(<RoomsTable rooms={rooms} />);
  ["Name", "State", "Scene", "Brightness", "Since"].forEach((header) =>
    expect(screen.getByRole("columnheader", { name: header })).toBeInTheDocument()
  );
});

test("clicking Name header sorts ascending then descending", () => {
  render(<RoomsTable rooms={rooms} />);
  const nameHeader = screen.getByRole("columnheader", { name: "Name" });
  fireEvent.click(nameHeader);
  const rowsAsc = screen.getAllByRole("row").slice(1).map((r) => r.textContent ?? "");
  expect(rowsAsc[0]).toContain("Bedroom");

  fireEvent.click(nameHeader);
  const rowsDesc = screen.getAllByRole("row").slice(1).map((r) => r.textContent ?? "");
  expect(rowsDesc[0]).toContain("Living Room");
});

test("brightness column renders numeric value", () => {
  render(<RoomsTable rooms={rooms} />);
  expect(screen.getByText("72")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd web && npx vitest run src/pages-system/widgets/sections/RoomsTable.test.tsx
```

Expected: FAIL — module not found.

- [ ] **Step 3: Create `RoomsTable.tsx`**

```tsx
import { useState } from "react";

export interface RoomRow {
  id: string;
  name: string;
  state: "on" | "off";
  scene: string;
  brightness: number; // 0–100
  sinceMs: number;    // milliseconds since last state change
}

type SortKey = keyof Pick<RoomRow, "name" | "state" | "scene" | "brightness" | "sinceMs">;
type SortDir = "asc" | "desc";

function formatSince(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h`;
}

interface Props {
  rooms: RoomRow[];
}

export function RoomsTable({ rooms }: Props) {
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  }

  const sorted = [...rooms].sort((a, b) => {
    const av = a[sortKey];
    const bv = b[sortKey];
    const cmp = av < bv ? -1 : av > bv ? 1 : 0;
    return sortDir === "asc" ? cmp : -cmp;
  });

  const columns: { key: SortKey; label: string }[] = [
    { key: "name",       label: "Name" },
    { key: "state",      label: "State" },
    { key: "scene",      label: "Scene" },
    { key: "brightness", label: "Brightness" },
    { key: "sinceMs",    label: "Since" },
  ];

  return (
    <table data-variant="rooms-table">
      <thead>
        <tr>
          {columns.map(({ key, label }) => (
            <th
              key={key}
              role="columnheader"
              aria-sort={sortKey === key ? (sortDir === "asc" ? "ascending" : "descending") : "none"}
              onClick={() => handleSort(key)}
              style={{ cursor: "pointer" }}
            >
              {label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {sorted.map((room) => (
          <tr key={room.id}>
            <td>{room.name}</td>
            <td data-variant={`state-${room.state}`}>{room.state}</td>
            <td>{room.scene}</td>
            <td data-variant="numeric">{room.brightness}</td>
            <td data-variant="numeric">{formatSince(room.sinceMs)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
```

- [ ] **Step 4: Run test — verify it passes**

```bash
cd web && npx vitest run src/pages-system/widgets/sections/RoomsTable.test.tsx
```

Expected: PASS (4 tests).

- [ ] **Step 5: Run lint**

```bash
cd web && npx eslint src/pages-system/widgets/sections/RoomsTable.tsx
```

Expected: no errors. (Inline `style` with `cursor` is structural, not a visual token — acceptable.)

- [ ] **Step 6: Commit**

```bash
git add web/src/pages-system/widgets/sections/RoomsTable.tsx \
        web/src/pages-system/widgets/sections/RoomsTable.test.tsx
git commit -m "feat(web): RoomsTable component for developer language (UI v2 plan 08)"
```

---

### Task 8.5 — `RoomGrid` section: switch on language

**Files:**
- Modify: `web/src/pages-system/widgets/sections/RoomGrid.tsx`

- [ ] **Step 1: Write failing test — RoomGrid renders RoomsTable in developer language**

Add to the existing `RoomGrid.test.tsx` (or create if missing):

```tsx
import { LanguageProvider } from "../../../theme/language-provider";

const sampleRooms = [
  { id: "r1", name: "Living Room", state: "on" as const, scene: "Evening", brightness: 72, sinceMs: 120_000 },
];

test("renders RoomsTable when language=developer", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <RoomGrid rooms={sampleRooms} />
    </LanguageProvider>
  );
  expect(screen.getByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("room-card")).not.toBeInTheDocument();
});

test("renders card grid when language=friendly", () => {
  render(
    <LanguageProvider initialLanguage="friendly">
      <RoomGrid rooms={sampleRooms} />
    </LanguageProvider>
  );
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd web && npx vitest run src/pages-system/widgets/sections/RoomGrid.test.tsx
```

Expected: FAIL — RoomGrid always renders the card grid regardless of language.

- [ ] **Step 3: Update `RoomGrid.tsx`**

Add these imports at the top of `RoomGrid.tsx`:

```tsx
import { useLanguage } from "../../../theme/language-provider";
import { RoomsTable, type RoomRow } from "./RoomsTable";
```

Inside the `RoomGrid` component, read the language and branch:

```tsx
const { language } = useLanguage();

if (language === "developer") {
  return <RoomsTable rooms={rooms} />;
}

// ... existing card grid render unchanged below
```

Note: `rooms` must be the prop that `RoomGrid` already accepts. Its type must be compatible with `RoomRow`. If it isn't, add the `sinceMs` and `scene` fields with defaults at the call site:

```tsx
// mapping helper if needed at the top of the branch:
const tableRows: RoomRow[] = rooms.map((r) => ({
  id: r.id,
  name: r.name,
  state: r.state as "on" | "off",
  scene: r.scene ?? "—",
  brightness: r.brightness ?? 0,
  sinceMs: r.sinceMs ?? 0,
}));
return <RoomsTable rooms={tableRows} />;
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd web && npx vitest run src/pages-system/widgets/sections/RoomGrid.test.tsx
```

Expected: PASS (all tests including pre-existing ones).

- [ ] **Step 5: Commit**

```bash
git add web/src/pages-system/widgets/sections/RoomGrid.tsx
git commit -m "feat(web): RoomGrid switches to RoomsTable in developer language (UI v2 plan 08)"
```

---

### Task 8.6 — CSS rule revealing `⌘N` keyboard shortcuts in developer language

**Files:**
- Modify: `web/src/theme/languages/developer.css`

- [ ] **Step 1: Confirm Plan 01's Sidebar emits `.kbd-shortcut` elements**

Check `web/src/shell/Sidebar.tsx` for `kbd-shortcut` class usage. If present, skip to Step 3. If not, add the elements now:

In `Sidebar.tsx`, update each nav item render to emit a sibling `<kbd>` after the label:

```tsx
{navItems.map(({ routeId, href }, index) => (
  <NavLink key={routeId} to={href}>
    {vocab.label(routeId)}
    <kbd className="kbd-shortcut" aria-hidden="true">⌘{index + 1}</kbd>
  </NavLink>
))}
```

- [ ] **Step 2: Write failing CSS test (Vitest + jsdom)**

Add to `web/src/shell/Sidebar.test.tsx`:

```tsx
test("kbd-shortcut elements are present in DOM for all six nav items", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <MemoryRouter initialEntries={["/_authed/home"]}>
        <Sidebar />
      </MemoryRouter>
    </LanguageProvider>
  );
  const shortcuts = document.querySelectorAll(".kbd-shortcut");
  expect(shortcuts).toHaveLength(6);
});
```

- [ ] **Step 3: Run test — verify it passes (DOM presence, not visibility)**

```bash
cd web && npx vitest run src/shell/Sidebar.test.tsx
```

Expected: PASS. (CSS visibility is tested by Playwright in Task 8.7.)

- [ ] **Step 4: Add CSS rule to `developer.css`**

Open `web/src/theme/languages/developer.css` and append at the end of the file:

```css
/* Keyboard shortcut labels: hidden by default, visible in developer language */
.kbd-shortcut {
  display: none;
}

:root[data-language="developer"] .kbd-shortcut {
  display: inline;
}
```

Note: The `.kbd-shortcut { display: none }` default rule should live in a shared base file if one exists; place it in `developer.css` only if no shared base is yet established. Either way, the `:root[data-language="developer"]` override goes in `developer.css`.

- [ ] **Step 5: Commit**

```bash
git add web/src/theme/languages/developer.css web/src/shell/Sidebar.tsx \
        web/src/shell/Sidebar.test.tsx
git commit -m "feat(web): reveal kbd shortcuts in developer language via CSS (UI v2 plan 08)"
```

---

### Task 8.7 — Playwright cross-language snapshot: Home in four languages

**Files:**
- Create: `web/e2e/developer-language-snapshot.spec.ts`
- Create (auto): `web/e2e/__screenshots__/developer-language-snapshot/` (committed reference images)

- [ ] **Step 1: Write the Playwright snapshot test**

Create `web/e2e/developer-language-snapshot.spec.ts`:

```ts
import { test, expect } from "@playwright/test";

const languages = [
  { id: "friendly-light", language: "friendly", mode: "light" },
  { id: "friendly-dark",  language: "friendly", mode: "dark" },
  { id: "ambient",        language: "ambient",  mode: null },
  { id: "developer",      language: "developer", mode: null },
] as const;

test.describe("Home page in all four languages", () => {
  for (const { id, language, mode } of languages) {
    test(`Home renders correctly in ${id}`, async ({ page }) => {
      await page.goto("/_authed/home");

      // Set language via localStorage (mirrors LanguageProvider persistence key)
      await page.evaluate(
        ({ language, mode }) => {
          const value = mode ? { language, mode } : { language };
          localStorage.setItem("sy.theme.v2", JSON.stringify(value));
        },
        { language, mode }
      );

      // Reload so LanguageProvider picks up the persisted value
      await page.reload();
      await page.waitForLoadState("networkidle");

      await expect(page).toHaveScreenshot(`home-${id}.png`, {
        fullPage: true,
        threshold: 0.02,
      });
    });
  }
});
```

- [ ] **Step 2: Run once to generate reference screenshots**

```bash
cd web && npx playwright test e2e/developer-language-snapshot.spec.ts --update-snapshots
```

Expected: test runs, four screenshots created under `web/e2e/__screenshots__/developer-language-snapshot/`.

- [ ] **Step 3: Run again to verify the snapshots are stable**

```bash
cd web && npx playwright test e2e/developer-language-snapshot.spec.ts
```

Expected: PASS (all four screenshot comparisons within the 2% threshold).

- [ ] **Step 4: Commit screenshots + test**

```bash
git add web/e2e/developer-language-snapshot.spec.ts \
        web/e2e/__screenshots__/developer-language-snapshot/
git commit -m "test(web): Playwright cross-language snapshot of Home (UI v2 plan 08)"
```

---

### Task 8.8 — Vitest unit tests: `useVocab` edge cases + primitive variant selection

**Files:**
- Modify: `web/src/theme/vocab/index.test.ts` (extend)
- Create: `web/src/theme/primitives-provider.test.tsx` (or extend existing)

- [ ] **Step 1: Add `useVocab` edge-case tests**

Extend `web/src/theme/vocab/index.test.ts`:

```ts
test("all six route IDs return a non-empty string in every language", () => {
  const routeIds = ["home", "rooms", "activity", "automations", "devices", "settings"] as const;
  const languages = ["friendly", "developer", "ambient"] as const;

  for (const language of languages) {
    const { result } = renderHook(() => useVocab(), {
      wrapper: wrapper(language),
    });
    for (const routeId of routeIds) {
      const label = result.current.label(routeId);
      expect(typeof label).toBe("string");
      expect(label.length).toBeGreaterThan(0);
    }
  }
});

test("friendly and developer differ for home and rooms", () => {
  const { result: friendly } = renderHook(() => useVocab(), {
    wrapper: wrapper("friendly"),
  });
  const { result: developer } = renderHook(() => useVocab(), {
    wrapper: wrapper("developer"),
  });
  expect(friendly.result.current.label("home")).not.toBe(
    developer.result.current.label("home")
  );
  expect(friendly.result.current.label("rooms")).not.toBe(
    developer.result.current.label("rooms")
  );
});
```

- [ ] **Step 2: Add primitive variant selection tests**

Create (or extend) `web/src/theme/primitives-provider.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { LanguageProvider } from "./language-provider";
import { LanguagePrimitives, usePrimitive } from "./primitives-provider";

function ButtonConsumer({ testId }: { testId: string }) {
  const Button = usePrimitive("Button");
  return <Button data-testid={testId}>click</Button>;
}

test("friendly language: Button renders without developer variant attribute", () => {
  render(
    <LanguageProvider initialLanguage="friendly">
      <LanguagePrimitives>
        <ButtonConsumer testId="btn" />
      </LanguagePrimitives>
    </LanguageProvider>
  );
  expect(screen.getByTestId("btn")).not.toHaveAttribute(
    "data-variant",
    "developer-button"
  );
});

test("developer language: Button renders with data-variant=developer-button", () => {
  render(
    <LanguageProvider initialLanguage="developer">
      <LanguagePrimitives>
        <ButtonConsumer testId="btn" />
      </LanguagePrimitives>
    </LanguageProvider>
  );
  expect(screen.getByTestId("btn")).toHaveAttribute(
    "data-variant",
    "developer-button"
  );
});

test("developer language: Chip renders with data-variant=developer-chip", () => {
  function ChipConsumer() {
    const Chip = usePrimitive("Chip");
    return <Chip data-testid="chip">label</Chip>;
  }
  render(
    <LanguageProvider initialLanguage="developer">
      <LanguagePrimitives>
        <ChipConsumer />
      </LanguagePrimitives>
    </LanguageProvider>
  );
  expect(screen.getByTestId("chip")).toHaveAttribute(
    "data-variant",
    "developer-chip"
  );
});
```

- [ ] **Step 3: Run all new tests**

```bash
cd web && npx vitest run src/theme/vocab/index.test.ts src/theme/primitives-provider.test.tsx
```

Expected: PASS (all tests).

- [ ] **Step 4: Run the full test suite**

```bash
cd web && npx vitest run
```

Expected: green across the board. If anything regresses, fix before committing.

- [ ] **Step 5: Run typecheck**

```bash
cd web && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 6: Run lint**

```bash
cd web && npx eslint src/
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add web/src/theme/vocab/index.test.ts web/src/theme/primitives-provider.test.tsx
git commit -m "test(web): useVocab edge cases + primitive variant selection (UI v2 plan 08)"
```

---

## Test plan

| Command | What it checks |
|---|---|
| `task web:lint` | No raw tokens in developer variant files; ESLint rule green |
| `task web:test` | All 8.1–8.8 Vitest tests pass |
| `task web:build` | TypeScript strict + Vite bundle succeeds |
| `task web:e2e` | Playwright snapshot: Home in all four languages within 2% threshold |
| Manual smoke | Set `localStorage["sy.theme.v2"] = '{"language":"developer"}'`, reload; sidebar shows "Overview / Entities / Events / …", kbd shortcuts `⌘1–⌘6` visible, Rooms renders as a sortable table |

## Acceptance criteria for merging

- All tests + typecheck + lint green locally and in CI.
- Developer primitive variants (`button`, `chip`, `pill`, `surface`) are registered in `LanguagePrimitives` and carry `data-variant="developer-*"` attributes.
- `useVocab("developer").label("home")` returns `"Overview"`; `"rooms"` returns `"Entities"`; `"activity"` returns `"Events"`.
- Sidebar and TopBar breadcrumb render developer vocabulary when `data-language="developer"` is set.
- Navigating to `/rooms` in developer language renders a sortable `<table>` instead of a card grid.
- `⌘1–⌘6` labels are visible in the Sidebar under `data-language="developer"` and hidden otherwise.
- Playwright snapshots for Home in all four languages are committed and stable.
- No `--gh-*` token references introduced. No hardcoded colors or radii.
- Linear sub-tasks for this plan transition to `Done`.
- Branch merged via `git merge --no-ff` into main.
