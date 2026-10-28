import type { Discussion } from "@/api/ops.gen";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useVoteDiscussionMutation } from "@/lib/queries/discussions";

/** An ⬆ chip with the discussion's upvotes that adds or takes back yours. */
export function UpvoteChip({ discussion }: { discussion: Discussion }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: vote, isPending } = useVoteDiscussionMutation(workspace.id);
  const count = discussion.upvoteCount;
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      data-slot="upvote-chip"
      aria-pressed={discussion.upvoted}
      aria-label={`${discussion.upvoted ? "Remove your upvote" : "Upvote"}, ${count} ${count === 1 ? "upvote" : "upvotes"}`}
      disabled={!canEdit || isPending}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        vote({ id: discussion.id, upvote: !discussion.upvoted });
      }}
    >
      <span aria-hidden data-slot="upvote-emoji">
        ⬆️
      </span>
      {count}
    </Button>
  );
}
