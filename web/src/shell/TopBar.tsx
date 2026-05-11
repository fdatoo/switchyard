import { usePalette } from "@/palette/use-palette";
import { useVocab, type RouteId } from "../theme/vocab";

interface TopBarProps {
  currentPath?: string;
}

/**
 * Title-case a slug by splitting on `-` or `_`, capitalizing the first letter of each word.
 */
function titleCase(slug: string): string {
  return slug
    .split(/[-_]/)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(" ");
}

interface BreadcrumbSegment {
  label: string;
  isFinal: boolean;
}

/**
 * Derive breadcrumb segments from a pathname.
 * Respects useVocab for top-level segments and applies appropriate casing to nested segments.
 */
function pathToBreadcrumbSegments(path: string, vocabLabel: (routeId: RouteId) => string): BreadcrumbSegment[] {
  // Normalize path: remove /_authed prefix and leading slash
  const cleanPath = path.replace(/^\/_authed/, "").replace(/^\//, "");
  const segments = cleanPath.split("/").filter(Boolean);

  // Default to home if empty or explicitly "home"
  if (!segments.length) {
    return [{ label: vocabLabel("home"), isFinal: true }];
  }

  const firstSegment = segments[0];

  // Handle home explicitly
  if (firstSegment === "home") {
    return [{ label: vocabLabel("home"), isFinal: true }];
  }

  // Handle ask (no vocab, special case)
  if (firstSegment === "ask") {
    return [{ label: "Ask", isFinal: true }];
  }

  // Handle pages/slug
  if (firstSegment === "pages") {
    const breadcrumbs: BreadcrumbSegment[] = [{ label: "Pages", isFinal: false }];
    if (segments[1]) {
      breadcrumbs.push({ label: segments[1], isFinal: true });
    }
    return breadcrumbs;
  }

  // Handle displays[/slug]
  if (firstSegment === "displays") {
    const breadcrumbs: BreadcrumbSegment[] = [{ label: "Displays", isFinal: !segments[1] }];
    if (segments[1]) {
      breadcrumbs.push({ label: segments[1], isFinal: true });
    }
    return breadcrumbs;
  }

  // Handle pkl-editor (special routing case)
  if (firstSegment === "pkl-editor") {
    const breadcrumbs: BreadcrumbSegment[] = [{ label: "Pkl editor", isFinal: false }];
    const filePath = segments.slice(1);
    if (!filePath.length) {
      breadcrumbs.push({ label: "(root)", isFinal: true });
    } else {
      const lastSegment = filePath[filePath.length - 1];
      breadcrumbs.push({
        label: lastSegment.length > 30 ? lastSegment.substring(0, 27) + "..." : lastSegment,
        isFinal: true,
      });
    }
    return breadcrumbs;
  }

  // Handle time-machine (special routing case)
  if (firstSegment === "time-machine") {
    const breadcrumbs: BreadcrumbSegment[] = [{ label: "Time-machine", isFinal: false }];
    if (segments[1]) {
      breadcrumbs.push({ label: segments[1], isFinal: true });
    }
    return breadcrumbs;
  }

  // Handle known vocab routes: rooms, activity, automations, devices, settings
  const vocabRoutes: RouteId[] = ["rooms", "activity", "automations", "devices", "settings"];

  if (vocabRoutes.includes(firstSegment as RouteId)) {
    const routeId = firstSegment as RouteId;
    const breadcrumbs: BreadcrumbSegment[] = [
      { label: vocabLabel(routeId), isFinal: segments.length === 1 },
    ];

    // Handle nested segment
    if (segments.length > 1) {
      if (routeId === "settings") {
        // Special handling for settings sections
        const section = segments[1];
        let sectionLabel = "";

        if (section === "pkl-config") {
          sectionLabel = "Pkl config";
        } else if (section === "theme-language") {
          sectionLabel = "Theme & language";
        } else if (section === "widget-packs") {
          sectionLabel = "Widget packs";
        } else {
          sectionLabel = titleCase(section);
        }

        breadcrumbs.push({ label: sectionLabel, isFinal: true });
      } else {
        // Default nested segment handling: title-case the slug
        breadcrumbs.push({ label: titleCase(segments[1]), isFinal: true });
      }
    }

    return breadcrumbs;
  }

  // Unknown route: default to home
  return [{ label: vocabLabel("home"), isFinal: true }];
}

function isMac(): boolean {
  if (typeof navigator === "undefined") return false;
  return navigator.platform.toLowerCase().includes("mac");
}

export function TopBar({
  currentPath = typeof window !== "undefined" ? window.location.pathname : "/",
}: TopBarProps) {
  const vocab = useVocab();
  const breadcrumbs = pathToBreadcrumbSegments(currentPath, vocab.label);
  const { openPalette } = usePalette();
  const shortcutLabel = isMac() ? "⌘K" : "Ctrl+K";

  return (
    <header
      data-testid="topbar"
      style={{
        display: "flex",
        alignItems: "center",
        gap: "12px",
        padding: "14px 24px",
        borderBottom: "1px solid var(--sy-color-line)",
        background: "var(--sy-color-bg)",
      }}
    >
      {/* Breadcrumb */}
      <nav aria-label="Breadcrumb" data-testid="breadcrumb">
        <span
          style={{
            fontSize: "13px",
            color: "var(--sy-color-fg-3)",
          }}
        >
          {breadcrumbs.map((segment, index) => (
            <span key={index}>
              {index > 0 && (
                <span style={{ color: "var(--sy-color-fg-5)", margin: "0 4px" }}> › </span>
              )}
              <b
                style={{
                  color: segment.isFinal ? "var(--sy-color-fg)" : "var(--sy-color-fg-3)",
                  fontWeight: segment.isFinal ? 500 : 400,
                }}
              >
                {segment.label}
              </b>
            </span>
          ))}
        </span>
      </nav>

      {/* Spacer */}
      <div style={{ flex: 1 }} />

      {/* Status dot — placeholder (Plan 3 will wire to interestingness) */}
      <div
        aria-label="Status indicator"
        title="Status (coming in Plan 03)"
        style={{
          width: "8px",
          height: "8px",
          borderRadius: "var(--sy-radius-pill)",
          background: "var(--sy-color-good)",
        }}
      />

      {/* Command palette button */}
      <button
        data-testid="topbar-palette-btn"
        aria-label="Open command palette"
        onClick={openPalette}
        style={{
          display: "flex",
          alignItems: "center",
          gap: "8px",
          padding: "6px 12px",
          background: "var(--sy-color-surface-1)",
          border: "1px solid var(--sy-color-line)",
          borderRadius: "var(--sy-radius-pill)",
          color: "var(--sy-color-fg-4)",
          fontSize: "12.5px",
          cursor: "pointer",
          minWidth: "160px",
          boxShadow: "var(--sy-shadow)",
        }}
      >
        <span style={{ flex: 1, textAlign: "left" }}>Search...</span>
        <kbd
          style={{
            fontFamily: "var(--sy-font-numeric)",
            fontSize: "10.5px",
            padding: "1px 5px",
            background: "var(--sy-color-surface-2)",
            borderRadius: "var(--sy-radius-sm)",
            color: "var(--sy-color-fg-4)",
            border: "1px solid var(--sy-color-line)",
          }}
        >
          {shortcutLabel}
        </kbd>
      </button>
    </header>
  );
}
