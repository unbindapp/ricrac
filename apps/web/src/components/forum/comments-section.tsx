import { useInfiniteQuery } from "@tanstack/react-query";
import { MessageSquareIcon } from "lucide-react";

import { CommentComposer, CommentThread } from "@/components/issues/comment-thread";
import { ListStates } from "@/components/layout/list-states";
import { SectionTitle } from "@/components/layout/section-title";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { commentsQuery, type TCommentTarget } from "@/lib/queries/comments";

export function CommentsSection({ target }: { target: TCommentTarget }) {
  const { workspace, canEdit } = useWorkspace();
  const comments = useInfiniteQuery(commentsQuery(workspace.id, target));
  const threads = comments.data?.pages.flatMap((page) => page.nodes) ?? [];
  return (
    <section aria-label="Comments" className="flex w-full flex-col gap-4">
      <SectionTitle title="Comments" />
      <ListStates
        isPending={comments.isPending}
        error={comments.error}
        onRetry={() => void comments.refetch()}
        isEmpty={threads.length === 0 && !canEdit}
        empty={{ icon: MessageSquareIcon, title: "No comments yet" }}
        skeletonRows={2}
      >
        {threads.length > 0 && (
          <ol className="flex flex-col gap-3">
            {threads.map((thread) => (
              <li key={thread.id} className="flex flex-col">
                <CommentThread thread={thread} target={target} />
              </li>
            ))}
          </ol>
        )}
      </ListStates>
      {comments.hasNextPage && (
        <div className="flex">
          <Button
            variant="outline"
            size="sm"
            isPending={comments.isFetchingNextPage}
            onClick={() => void comments.fetchNextPage()}
          >
            Load more
          </Button>
        </div>
      )}
      {canEdit && <CommentComposer target={target} />}
    </section>
  );
}
