import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  slug?: string;
}

export function DisplaySlug({ slug = "unknown" }: Props) {
  return <PlaceholderPage title={`Display: ${slug}`} plan="Plan 07" />;
}
