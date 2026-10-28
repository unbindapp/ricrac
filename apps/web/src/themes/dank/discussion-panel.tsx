import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useEffect, useState } from "react";

import type { DiscussionDetail } from "@/api/ops.gen";
import {
  DiscussionActions,
  DiscussionTitle,
} from "@/components/forum/discussion-detail";
import { DiscussionTagsPicker } from "@/components/forum/discussion-tags";
import { ArrowLeftIcon, LockIcon, PinIcon, PlusIcon, XIcon } from "@/components/icons";
import { ChatLeadContext } from "@/components/layout/chat-layout";
import { AvatarStack } from "@/components/projects/people";
import { ConfirmDeleteAction, CopyLinkAction } from "@/components/shared/detail-actions";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  discussionThreadQuery,
  useDeleteDiscussionThreadMutation,
  useSaveDiscussionMutation,
} from "@/lib/queries/discussions";
import { Byline } from "@/themes/dank/byline";
import { PostBody } from "@/themes/dank/post-body";
import { UpvoteChip } from "@/themes/dank/upvote-chip";
import { DankThreadView } from "@/themes/dank/thread-view";

type TProps = {
  discussion: DiscussionDetail;
  /** The main chat, shown under the post. */
  chat: ReactNode;
  /** The thread open in place of the post and its chat. */
  threadId?: string;
  onBackToChat: () => void;
  onClose: () => void;
};

/** A discussion in one panel: its post opens the chat and scrolls with it, or one of its threads. */
export function DiscussionPanel({ discussion, chat, threadId, onBackToChat, onClose }: TProps) {
  const [title, setTitle] = useState<HTMLElement | null>(null);
  const isTitleOut = useIsScrolledAway(title);
  const body = discussion.body ?? "";
  const cover =
    discussion.thumbnailUrl && !body.includes(discussion.thumbnailUrl)
      ? discussion.thumbnailUrl
      : null;

  const post = (
    <article aria-label="Post" data-slot="discussion-post">
      <div ref={setTitle}>
        <DiscussionTitle discussion={discussion} />
      </div>
      <Byline user={discussion.creator} at={discussion.createdAt}>
        {discussion.pinnedAt && (
          <span data-slot="status-chip">
            <PinIcon />
            Pinned
          </span>
        )}
        {discussion.closedAt && (
          <span data-slot="status-chip">
            <LockIcon />
            Closed
          </span>
        )}
        <UpvoteChip discussion={discussion} />
        {discussion.subscribers.length > 0 && (
          <span data-slot="byline-people" role="group" aria-label="Participants">
            <AvatarStack users={discussion.subscribers} max={5} />
          </span>
        )}
      </Byline>
      <PostTags discussion={discussion} />
      {cover && <img data-slot="discussion-cover" src={cover} alt="" />}
      <PostBody discussion={discussion} />
    </article>
  );

  return (
    <section aria-label={discussion.title} data-slot="discussion-panel">
      <div data-slot="discussion-bar">
        <div data-slot="discussion-bar-start">
          {threadId ? (
            <Button
              variant="ghost"
              forceMinSize={false}
              data-slot="back-button"
              aria-label={`Back to ${discussion.title}`}
              onClick={onBackToChat}
            >
              <ArrowLeftIcon />
              <span className="truncate">{discussion.title}</span>
            </Button>
          ) : (
            isTitleOut && <span data-slot="discussion-bar-title">{discussion.title}</span>
          )}
        </div>
        <div role="group" aria-label="Actions" data-slot="button-group">
          {threadId ? (
            <ThreadActions threadId={threadId} onDeleted={onBackToChat} />
          ) : (
            <DiscussionActions discussion={discussion} grouped />
          )}
        </div>
        <SimpleTooltip content="Close">
          <Button
            variant="ghost"
            size="icon"
            data-slot="bar-close"
            aria-label="Close"
            onClick={onClose}
          >
            <XIcon />
          </Button>
        </SimpleTooltip>
      </div>
      {threadId ? (
        <DankThreadView key={threadId} threadId={threadId} onBack={onBackToChat} />
      ) : (
        <ChatLeadContext value={post}>
          <div data-slot="discussion-chat">{chat}</div>
        </ChatLeadContext>
      )}
    </section>
  );
}

function PostTags({ discussion }: { discussion: DiscussionDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save } = useSaveDiscussionMutation(workspace.id);
  const setTags = (tags: string[]) => save({ id: discussion.id, tags });
  if (!canEdit && discussion.tags.length === 0) return null;
  return (
    <div data-slot="post-tags">
      {discussion.tags.map((tag) => (
        <span key={tag} data-slot="tag-chip">
          <span aria-hidden data-slot="tag-hash">
            #
          </span>
          {tag}
          {canEdit && (
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              data-slot="tag-remove"
              aria-label={`Remove tag ${tag}`}
              onClick={() => setTags(discussion.tags.filter((other) => other !== tag))}
            >
              <XIcon />
            </Button>
          )}
        </span>
      ))}
      {canEdit && (
        <DiscussionTagsPicker
          value={discussion.tags}
          onChange={setTags}
          trigger={
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              data-slot="tag-add"
              aria-label="Add tag"
            >
              <PlusIcon />
            </Button>
          }
        />
      )}
    </div>
  );
}

function ThreadActions({ threadId, onDeleted }: { threadId: string; onDeleted: () => void }) {
  const { workspace } = useWorkspace();
  const { data: thread } = useQuery(discussionThreadQuery(workspace.id, threadId));
  const canRewrite = useCanRewrite(thread?.creator ?? null);
  const { mutateAsync: deleteThread } = useDeleteDiscussionThreadMutation(workspace.id);
  if (!thread) return null;
  const remove = async () => {
    onDeleted();
    await deleteThread(thread.id);
    toast.add({ type: "success", title: "Thread deleted" });
  };
  return (
    <>
      <CopyLinkAction url={thread.url} label="Copy link to thread" />
      {canRewrite && <ConfirmDeleteAction label="Delete thread" onDelete={() => void remove()} />}
    </>
  );
}

/** True once the element has scrolled out of view, like a title under a bar. */
function useIsScrolledAway(element: HTMLElement | null) {
  const [isAway, setIsAway] = useState(false);
  useEffect(() => {
    if (!element) return;
    const observer = new IntersectionObserver(([entry]) => setIsAway(!entry.isIntersecting));
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);
  return isAway;
}
