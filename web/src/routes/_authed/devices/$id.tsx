import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  id?: string;
}

export function DeviceDetail({ id = "unknown" }: Props) {
  return <PlaceholderPage title={`Device: ${id}`} plan="Plan 08" />;
}
