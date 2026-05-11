import { PlaceholderPage } from "@/shell/PlaceholderPage";

interface Props {
  slug?: string;
}

export function PageSlug({ slug = "unknown" }: Props) {
  return <PlaceholderPage title={`Page: ${slug}`} plan="Plan 06" />;
}
