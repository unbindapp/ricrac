import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
  BellIcon,
  BellOffIcon,
  GlobeIcon,
  ImageIcon,
  LockIcon,
  LockOpenIcon,
  MessagesSquareIcon,
  PinIcon,
  PinOffIcon,
  UsersIcon,
} from "lucide-react";
import { useState } from "react";

import type { Comment, DiscussionDetail as TDiscussionDetail } from "@/api/ops.gen";
import { ChatComposer } from "@/components/forum/chat-message";
import { ChatMessages } from "@/components/forum/chat-messages";
import { DiscussionTagBadges, DiscussionTagsPicker } from "@/components/forum/discussion-tags";
import { DiscussionTeamPicker } from "@/components/forum/discussion-team-picker";
import { DiscussionThreads } from "@/components/forum/discussion-threads";
import { NewThreadDialog } from "@/components/forum/new-thread-dialog";
import { ThumbnailPicker } from "@/components/forum/thumbnail-picker";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { commentExcerpt } from "@/components/inbox/notifications";
import { userName } from "@/components/issues/grouping";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ChatLayout } from "@/components/layout/chat-layout";
import { EntityMenu, toastTrashed } from "@/components/projects/entity-menu";
import { AvatarStack } from "@/components/projects/people";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { CopyLinkAction, HeaderAction, HeaderActions } from "@/components/shared/detail-actions";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  DISCUSSION_TITLE_MAX_LENGTH,
  useDiscussionLifecycleMutation,
  useSaveDiscussionMutation,
} from "@/lib/queries/discussions";
import { messagePlaceholder, shorten } from "@/lib/helpers/chat";
import { meQuery } from "@/lib/queries/me";

const THREAD_TITLE_FROM_MESSAGE_LENGTH = 80;

type TProps = {
  discussion: TDiscussionDetail;
  /** On narrow screens the threads list sits in the page instead of beside it. */
  showThreads: boolean;
  openMessageId?: string;
  onOpenMessage: (messageId: string) => void;
};

export function DiscussionDetail({
  discussion,
  showThreads,
  openMessageId,
  onOpenMessage,
}: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const canRewrite = useCanRewrite(discussion.creator);
  const { data: me } = useQuery(meQuery);
  const { mutate: save, mutateAsync: saveAsync } = useSaveDiscussionMutation(workspace.id);
  const { mutateAsync: changeLifecycle } = useDiscussionLifecycleMutation(workspace.id);
  const navigate = useNavigate();
  const [splitFrom, setSplitFrom] = useState<Comment | null>(null);
  const [isEditingThumbnail, setIsEditingThumbnail] = useState(false);
  const isPinned = Boolean(discussion.pinnedAt);
  const isClosed = Boolean(discussion.closedAt);
  const isSubscribed = discussion.subscribers.some((user) => user.id === me?.id);
  const forumLink = { to: "/$workspace/forum", params: { workspace: workspace.urlKey } } as const;

  const trash = () =>
    void changeLifecycle({ id: discussion.id, action: "trash" })
      .then(async () => {
        await navigate(forumLink);
        toastTrashed(
          "Discussion moved to trash",
          () =>
            void changeLifecycle({ id: discussion.id, action: "restore" })
              .then(() => toast.add({ type: "success", title: "Discussion restored" }))
              .catch(() => undefined),
        );
      })
      .catch(() => undefined);

  const breadcrumb: TBreadcrumbItem[] = [
    {
      label: "Forum",
      icon: <MessagesSquareIcon className="size-4" />,
      link: forumLink,
    },
    { label: discussion.title },
  ];

  const bar = (
    <>
      <div className="min-w-0 flex-1">
        <Breadcrumb items={breadcrumb} />
      </div>
      <HeaderActions>
        <CopyLinkAction url={discussion.url} />
        {canEdit && (
          <>
            <HeaderAction
              label={isSubscribed ? "Unsubscribe" : "Subscribe"}
              onClick={() =>
                save(
                  isSubscribed
                    ? { id: discussion.id, removeSubscribers: ["me"] }
                    : { id: discussion.id, addSubscribers: ["me"] },
                )
              }
            >
              {isSubscribed ? <BellOffIcon className="size-4" /> : <BellIcon className="size-4" />}
            </HeaderAction>
            <HeaderAction
              label={isPinned ? "Unpin" : "Pin to the top"}
              onClick={() => save({ id: discussion.id, pinned: !isPinned })}
            >
              {isPinned ? <PinOffIcon className="size-4" /> : <PinIcon className="size-4" />}
            </HeaderAction>
            <HeaderAction
              label={isClosed ? "Reopen discussion" : "Close discussion"}
              onClick={() => save({ id: discussion.id, closed: !isClosed })}
            >
              {isClosed ? <LockOpenIcon className="size-4" /> : <LockIcon className="size-4" />}
            </HeaderAction>
          </>
        )}
        {canRewrite && (
          <EntityMenu noun="discussion" onTrash={trash}>
            <DropdownMenuItem onClick={() => setIsEditingThumbnail(true)}>
              <ImageIcon className="size-4" />
              Change thumbnail
            </DropdownMenuItem>
          </EntityMenu>
        )}
      </HeaderActions>
    </>
  );

  return (
    <ChatLayout
      bar={bar}
      width="narrow"
      composer={
        canEdit && (
          <ChatComposer
            target={{ discussionId: discussion.id }}
            placeholder={messagePlaceholder(discussion.title)}
          />
        )
      }
    >
      <div className="flex flex-col gap-2">
        <InlineText
          value={discussion.title}
          label="Discussion title"
          maxLength={DISCUSSION_TITLE_MAX_LENGTH}
          readOnly={!canRewrite}
          onSave={(title) => save({ id: discussion.id, title })}
        />
        <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
          {canRewrite ? (
            <DiscussionTeamPicker
              label="Visible to"
              noneLabel="Whole workspace"
              value={discussion.team?.id ?? null}
              onChange={(team) => save({ id: discussion.id, team })}
            />
          ) : (
            <span className="flex items-center gap-1.5">
              {discussion.team ? (
                <UsersIcon className="size-3.5" />
              ) : (
                <GlobeIcon className="size-3.5" />
              )}
              {discussion.team?.name ?? "Whole workspace"}
            </span>
          )}
          {canEdit ? (
            <DiscussionTagsPicker
              value={discussion.tags}
              onChange={(tags) => save({ id: discussion.id, tags })}
            />
          ) : (
            <DiscussionTagBadges tags={discussion.tags} />
          )}
          {discussion.creator && (
            <span>
              Started by {discussion.creator.name} <RelativeTime value={discussion.createdAt} />
            </span>
          )}
          {isPinned && (
            <Badge variant="process">
              <PinIcon />
              Pinned
            </Badge>
          )}
          {isClosed && (
            <Badge variant="muted">
              <LockIcon />
              Closed
            </Badge>
          )}
          {discussion.subscribers.length > 0 && (
            <SimpleTooltip
              content={`Subscribed: ${discussion.subscribers.map((user) => user.name).join(", ")}`}
            >
              <span className="ml-auto flex" aria-label="Subscribers">
                <AvatarStack users={discussion.subscribers} max={5} />
              </span>
            </SimpleTooltip>
          )}
        </div>
      </div>
      <AutosaveMarkdown
        label="Discussion body"
        placeholder="Write it up…"
        value={discussion.body ?? ""}
        readOnly={!canRewrite}
        onSave={async (after, before) => {
          const saved = await saveAsync({ id: discussion.id, edit: { before, after } });
          return saved.body ?? "";
        }}
      />
      {showThreads && <DiscussionThreads discussion={discussion} variant="section" />}
      <ChatMessages
        target={{ discussionId: discussion.id }}
        threads={{ openId: openMessageId, onOpen: onOpenMessage }}
        onNewThread={setSplitFrom}
        empty={{
          title: "No messages yet",
          description: "Say what you think. Reply in a thread to keep a side topic together.",
        }}
      />
      <NewThreadDialog
        key={splitFrom?.id}
        discussion={discussion}
        open={Boolean(splitFrom)}
        onOpenChange={(open) => {
          if (!open) setSplitFrom(null);
        }}
        defaults={splitFrom ? threadFromMessage(splitFrom) : undefined}
      />
      <Dialog open={isEditingThumbnail} onOpenChange={setIsEditingThumbnail}>
        <DialogContent className="w-xl max-w-full">
          <DialogHeader>
            <DialogTitle>Thumbnail</DialogTitle>
          </DialogHeader>
          <ThumbnailPicker
            value={discussion.thumbnailUrl}
            body={discussion.body ?? ""}
            onChange={(thumbnailUrl) => save({ id: discussion.id, thumbnailUrl })}
          />
        </DialogContent>
      </Dialog>
    </ChatLayout>
  );
}

function threadFromMessage(message: Comment) {
  const title = shorten(commentExcerpt(message.body), THREAD_TITLE_FROM_MESSAGE_LENGTH);
  const author = message.user ? userName(message.user) : "A deleted user";
  const quoted = message.body
    .split("\n")
    .map((line) => `> ${line}`.trimEnd())
    .join("\n");
  return { title, body: `${author} wrote:\n\n${quoted}\n\n` };
}
