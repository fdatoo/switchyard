import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  slug?: string;
}

export function RoomSlug({ slug = "unknown" }: Props) {
  return <PlaceholderPage title={`Room: ${slug}`} plan="Plan 02" />;
}
