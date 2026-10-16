import { createFileRoute } from "@tanstack/react-router";

import { ReleaseNoteDetail } from "@/components/releases/release-note-detail";

export const Route = createFileRoute("/$workspace/_workspace/release-note/$slug")({
  component: ReleaseNotePage,
});

function ReleaseNotePage() {
  const { slug } = Route.useParams();
  return <ReleaseNoteDetail key={slug} slug={slug} />;
}
