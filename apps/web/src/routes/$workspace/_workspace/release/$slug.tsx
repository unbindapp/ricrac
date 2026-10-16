import { createFileRoute } from "@tanstack/react-router";

import { ReleaseDetail } from "@/components/releases/release-detail";

export const Route = createFileRoute("/$workspace/_workspace/release/$slug")({
  component: ReleasePage,
});

function ReleasePage() {
  const { slug } = Route.useParams();
  return <ReleaseDetail key={slug} slug={slug} />;
}
