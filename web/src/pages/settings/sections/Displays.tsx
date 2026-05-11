import { PlaceholderPage } from "@/shell/PlaceholderPage";

/**
 * Displays section — placeholder for Plan 07 which ships the real list + pairing UI.
 */
export function Displays() {
  return (
    <div>
      <h1
        style={{
          margin: "0 0 var(--sy-space-5)",
          fontSize: "1.25rem",
          fontWeight: 600,
          color: "var(--sy-color-fg)",
        }}
      >
        Displays
      </h1>
      <PlaceholderPage title="Display list" plan="Plan 07" />
    </div>
  );
}
