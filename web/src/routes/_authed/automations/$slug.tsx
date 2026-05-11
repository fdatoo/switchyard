/**
 * AutomationSlug — full automation editor (Plan 10).
 *
 * Replaces the Plan 11 stub. Wires the AutomationEditor with Plan 11's
 * edit-session infrastructure (useEditSession + ConflictBanner).
 *
 * The slug is extracted from the URL path by the routing layer in App.tsx.
 */

import { AutomationEditor } from "@/pages/automations/AutomationEditor";

interface Props {
  slug?: string;
}

export function AutomationSlug({ slug = "unknown" }: Props) {
  return <AutomationEditor slug={slug} />;
}
