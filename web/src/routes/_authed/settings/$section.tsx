import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  section?: string;
}

export function SettingsSection({ section = "general" }: Props) {
  return <PlaceholderPage title={`Settings: ${section}`} plan="Plan 09" />;
}
