import { createFileRoute } from "@tanstack/react-router";

import { DocumentDetail } from "@/components/documents/document-detail";

export const Route = createFileRoute("/$workspace/_workspace/document/$slug")({
  component: DocumentSlugPage,
});

function DocumentSlugPage() {
  const { slug } = Route.useParams();
  return <DocumentDetail key={slug} slug={slug} />;
}
