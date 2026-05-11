import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  slug?: string;
}

export function AutomationSlug({ slug = "unknown" }: Props) {
  return <PlaceholderPage title={`Automation: ${slug}`} plan="Plan 10" />;
}
