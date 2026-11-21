import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

import type { DiscussionDetail } from "@/api/ops.gen";
import { DiscussionActions, DiscussionTitle } from "@/components/forum/discussion-detail";
import { DiscussionTagsPicker } from "@/components/forum/discussion-tags";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import {
  ArrowLeftIcon,
  LockIcon,
  MessagesSquareIcon,
  PinIcon,
  PlusIcon,
  XIcon,
} from "@/components/icons";
import { userName } from "@/components/issues/grouping";
import { ChatLeadContext } from "@/components/layout/chat-layout";
import { AvatarStack, PersonAvatar } from "@/components/projects/people";
import { ConfirmDeleteAction, CopyLinkAction } from "@/components/shared/detail-actions";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  discussionThreadQuery,
  useDeleteDiscussionThreadMutation,
  useSaveDiscussionMutation,
} from "@/lib/queries/discussions";
import { PostBody } from "@/themes/dux/post-body";
import { DuxThreadView } from "@/themes/dux/thread-view";
import { UpvoteChip } from "@/themes/dux/upvote-chip";

type TProps = {
  discussion: DiscussionDetail;
  /** The main chat, shown under the post. */
  chat: ReactNode;
  /** The thread open in place of the post and its chat. */
  threadId?: string;
  onBackToChat: () => void;
  onClose: () => void;
};

/** A discussion in the forum's panel: its post opens the chat and scrolls with it, or one of its threads. */
export function DiscussionPanel({ discussion, chat, threadId, onBackToChat, onClose }: TProps) {
  const backLabel = threadId ? `Back to ${discussion.title}` : "Back to forum";
  return (
    <section aria-label={discussion.title} data-slot="discussion-panel">
      <header data-slot="post-head">
        <SimpleTooltip content={backLabel}>
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            data-slot="post-back"
            aria-label={backLabel}
            onClick={threadId ? onBackToChat : onClose}
          >
            <ArrowLeftIcon />
          </Button>
        </SimpleTooltip>
        {threadId ? (
          <h1 data-slot="post-head-title">{discussion.title}</h1>
        ) : (
          <DiscussionTitle discussion={discussion} />
        )}
        <div role="group" aria-label="Actions" data-slot="post-actions">
          {threadId ? (
            <ThreadActions threadId={threadId} onDeleted={onBackToChat} />
          ) : (
            <DiscussionActions discussion={discussion} grouped />
          )}
        </div>
      </header>
      {threadId ? (
        <DuxThreadView key={threadId} threadId={threadId} onBack={onBackToChat} />
      ) : (
        <ChatLeadContext value={<Post discussion={discussion} />}>
          <div data-slot="discussion-chat">{chat}</div>
        </ChatLeadContext>
      )}
    </section>
  );
}

/** The post that opens the chat: its cover, who wrote it and when, its tags and text, then the chat. */
function Post({ discussion }: { discussion: DiscussionDetail }) {
  const creator = discussion.creator;
  const body = discussion.body ?? "";
  const cover =
    discussion.thumbnailUrl && !body.includes(discussion.thumbnailUrl)
      ? discussion.thumbnailUrl
      : null;
  return (
    <article aria-label="Post" data-slot="discussion-post">
      <div data-slot="discussion-cover" aria-hidden>
        {cover ? <img src={cover} alt="" /> : <MessagesSquareIcon />}
      </div>
      <div data-slot="post-by">
        {creator && <PersonAvatar user={creator} size="sm" />}
        <span data-slot="post-author">{creator ? userName(creator) : "A deleted user"}</span>
        <span data-slot="op-badge">OP</span>
        <RelativeTime value={discussion.createdAt} />
        {discussion.pinnedAt && (
          <span data-slot="status-chip">
            <PinIcon className="fill-current" />
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
      </div>
      <PostTags discussion={discussion} />
      <PostBody discussion={discussion} />
      <h3 data-slot="post-chat-title">Chat</h3>
    </article>
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
