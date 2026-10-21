import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { DiscussionDetail } from "@/components/forum/discussion-detail";
import { ThreadPanel } from "@/components/forum/thread-panel";
import { PageContainer } from "@/components/layout/page-container";
import { SplitView } from "@/components/layout/split-view";
import { useDeviceSize } from "@/components/providers/device-size-provider";

const searchSchema = z.object({
  /** The thread open beside the discussion. */
  thread: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/discussion/$slug")({
  validateSearch: zodValidator(searchSchema),
  component: DiscussionSlugPage,
});

function DiscussionSlugPage() {
  const { slug } = Route.useParams();
  const { thread } = Route.useSearch();
  const navigate = Route.useNavigate();
  const { size } = useDeviceSize();
  const hasPanel = size === "lg" || size === "xl";
  const closeThread = () =>
    void navigate({ search: (prev) => ({ ...prev, thread: undefined }), replace: true });

  if (thread && !hasPanel) {
    return (
      <PageContainer width="narrow">
        <ThreadPanel key={thread} threadId={thread} onClose={closeThread} />
      </PageContainer>
    );
  }
  return (
    <SplitView
      storageKey="discussion-thread-width"
      defaultAsideWidth={560}
      main={
        <PageContainer width="narrow">
          <DiscussionDetail key={slug} slug={slug} openThreadId={thread} />
        </PageContainer>
      }
      aside={
        thread ? <ThreadPanel key={thread} threadId={thread} onClose={closeThread} /> : undefined
      }
    />
  );
}
