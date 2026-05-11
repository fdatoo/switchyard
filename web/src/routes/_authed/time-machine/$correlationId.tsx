/**
 * Time Machine run detail — placeholder for Plan 04.
 *
 * This route is navigated to by the "Run now" button in the automation editor.
 * Plan 04 will replace this with the full execution timeline view.
 */

import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  correlationId?: string;
}

export function TimeMachineRun({ correlationId = "unknown" }: Props) {
  return (
    <PlaceholderPage
      title={`Run: ${correlationId}`}
      plan="Plan 04"
    />
  );
}
