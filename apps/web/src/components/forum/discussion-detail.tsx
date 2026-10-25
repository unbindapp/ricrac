import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import type { DiscussionDetail as TDiscussionDetail } from "@/api/ops.gen";
import { DiscussionTagBadges, DiscussionTagsPicker } from "@/components/forum/discussion-tags";
import { MainChatComposer, MainChatMessages } from "@/components/forum/main-chat";
import { ThreadBranches } from "@/components/forum/thread-branches";
import { ThumbnailPicker } from "@/components/forum/thumbnail-picker";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import {
  BellIcon,
  BellOffIcon,
  ImageIcon,
  LockIcon,
  LockOpenIcon,
  MessagesSquareIcon,
  PinIcon,
  PinOffIcon,
} from "@/components/icons";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ChatLayout } from "@/components/layout/chat-layout";
import { PageContainer } from "@/components/layout/page-container";
import { PageBar } from "@/components/layout/page-header";
import { SectionTitle } from "@/components/layout/section-title";
import { EntityMenu, toastTrashed } from "@/components/projects/entity-menu";
import { AvatarStack } from "@/components/projects/people";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { CopyLinkAction, HeaderAction, HeaderActions } from "@/components/shared/detail-actions";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
import { meQuery } from "@/lib/queries/me";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = {
  discussion: TDiscussionDetail;
  /** The main chat follows the topic on narrow screens; on wide ones it sits beside it or is hidden. */
  chat: "inline" | "aside" | "hidden";
  onToggleChat: () => void;
  threadSwitcher?: ReactNode;
  openThreadId?: string;
  openMessageId?: string;
  onOpenMessage: (messageId: string) => void;
};

export function DiscussionDetail({
  discussion,
  chat,
  onToggleChat,
  threadSwitcher,
  openThreadId,
  openMessageId,
  onOpenMessage,
}: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const { threadsInChat } = useUiTheme();

  const breadcrumb: TBreadcrumbItem[] = [
    {
      label: "Forum",
      icon: <MessagesSquareIcon className="size-4" />,
      link: { to: "/$workspace/forum", params: { workspace: workspace.urlKey } },
    },
    { label: discussion.title },
  ];

  const bar = (
    <>
      <div className="min-w-0 flex-1">
        <Breadcrumb items={breadcrumb} />
      </div>
      <DiscussionActions discussion={discussion}>
        {chat !== "inline" && <ChatToggle isShown={chat === "aside"} onToggle={onToggleChat} />}
      </DiscussionActions>
    </>
  );

  const topic = (
    <>
      <DiscussionHeading discussion={discussion} />
      {!threadsInChat && (
        <ThreadBranches
          discussion={discussion}
          openThreadId={openThreadId}
          isMainChatOpen={!openThreadId && !openMessageId}
        />
      )}
      <DiscussionBody discussion={discussion} />
    </>
  );

  if (chat !== "inline") {
    return (
      <PageContainer width="narrow">
        <div className="flex w-full flex-col gap-6">
          <PageBar>{bar}</PageBar>
          {topic}
        </div>
      </PageContainer>
    );
  }
  return (
    <ChatLayout
      bar={bar}
      width="narrow"
      startAtEnd={false}
      composer={canEdit && <MainChatComposer discussion={discussion} />}
    >
      {topic}
      <SectionTitle title="Main chat" actions={threadSwitcher} />
      <MainChatMessages
        discussion={discussion}
        openMessageId={openMessageId}
        onOpenMessage={onOpenMessage}
      />
    </ChatLayout>
  );
}

/** Copying, subscribing, pinning, closing and trashing the discussion, after the buttons passed in. */
export function DiscussionActions({
  discussion,
  children,
}: {
  discussion: TDiscussionDetail;
  children?: ReactNode;
}) {
  const { workspace, canEdit } = useWorkspace();
  const canRewrite = useCanRewrite(discussion.creator);
  const { data: me } = useQuery(meQuery);
  const { mutate: save } = useSaveDiscussionMutation(workspace.id);
  const { mutateAsync: changeLifecycle } = useDiscussionLifecycleMutation(workspace.id);
  const navigate = useNavigate();
  const [isEditingThumbnail, setIsEditingThumbnail] = useState(false);
  const isPinned = Boolean(discussion.pinnedAt);
  const isClosed = Boolean(discussion.closedAt);
  const isSubscribed = discussion.subscribers.some((user) => user.id === me?.id);

  const trash = () =>
    void changeLifecycle({ id: discussion.id, action: "trash" })
      .then(async () => {
        await navigate({ to: "/$workspace/forum", params: { workspace: workspace.urlKey } });
        toastTrashed(
          "Discussion moved to trash",
          () =>
            void changeLifecycle({ id: discussion.id, action: "restore" })
              .then(() => toast.add({ type: "success", title: "Discussion restored" }))
              .catch(() => undefined),
        );
      })
      .catch(() => undefined);

  return (
    <HeaderActions>
      {children}
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
    </HeaderActions>
  );
}

/** The editable title over the tags, who started it and when, and who follows it. */
export function DiscussionHeading({ discussion }: { discussion: TDiscussionDetail }) {
  return (
    <div className="flex flex-col gap-2">
      <DiscussionTitle discussion={discussion} />
      <DiscussionMeta discussion={discussion} />
    </div>
  );
}

export function DiscussionTitle({ discussion }: { discussion: TDiscussionDetail }) {
  const { workspace } = useWorkspace();
  const canRewrite = useCanRewrite(discussion.creator);
  const { mutate: save } = useSaveDiscussionMutation(workspace.id);
  return (
    <div data-m3-title>
      <InlineText
        value={discussion.title}
        label="Discussion title"
        maxLength={DISCUSSION_TITLE_MAX_LENGTH}
        readOnly={!canRewrite}
        onSave={(title) => save({ id: discussion.id, title })}
      />
    </div>
  );
}

/** Its tags, who started it and when, whether it is pinned or closed, and who follows it. */
export function DiscussionMeta({
  discussion,
  tagChips = false,
}: {
  discussion: TDiscussionDetail;
  /** Each tag in its own chip, instead of all of them in the picker's button. */
  tagChips?: boolean;
}) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save } = useSaveDiscussionMutation(workspace.id);
  return (
    <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
      {canEdit ? (
        <DiscussionTagsPicker
          value={discussion.tags}
          chips={tagChips}
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
      {discussion.pinnedAt && (
        <Badge variant="process" data-m3-chip="assist">
          <PinIcon />
          Pinned
        </Badge>
      )}
      {discussion.closedAt && (
        <Badge variant="muted" data-m3-chip="assist">
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
  );
}

export function DiscussionBody({ discussion }: { discussion: TDiscussionDetail }) {
  const { workspace } = useWorkspace();
  const canRewrite = useCanRewrite(discussion.creator);
  const { mutateAsync: saveAsync } = useSaveDiscussionMutation(workspace.id);
  return (
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
  );
}

function ChatToggle({ isShown, onToggle }: { isShown: boolean; onToggle: () => void }) {
  return (
    <SimpleTooltip content={isShown ? "Hide chat" : "Show chat"}>
      <Button
        variant={isShown ? "secondary" : "ghost"}
        size="icon"
        aria-label="Chat"
        aria-pressed={isShown}
        onClick={onToggle}
      >
        <MessagesSquareIcon className="size-4" />
      </Button>
    </SimpleTooltip>
  );
}
