import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
  BellIcon,
  BellOffIcon,
  GlobeIcon,
  LockIcon,
  LockOpenIcon,
  MessagesSquareIcon,
  PinIcon,
  PinOffIcon,
  UsersIcon,
} from "lucide-react";

import type { DiscussionDetail as TDiscussionDetail } from "@/api/ops.gen";
import { CommentsSection } from "@/components/forum/comments-section";
import { DiscussionThreads } from "@/components/forum/discussion-threads";
import { DiscussionTeamPicker } from "@/components/forum/discussion-team-picker";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageBar } from "@/components/layout/page-header";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { EntityMenu, toastTrashed } from "@/components/projects/entity-menu";
import { AvatarStack } from "@/components/projects/people";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { CopyLinkAction, HeaderAction, HeaderActions } from "@/components/shared/detail-actions";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  DISCUSSION_TITLE_MAX_LENGTH,
  discussionQuery,
  useDiscussionLifecycleMutation,
  useSaveDiscussionMutation,
} from "@/lib/queries/discussions";
import { meQuery } from "@/lib/queries/me";
import { isMissing } from "@/lib/queries/projects";

type TProps = { slug: string; openThreadId?: string };

export function DiscussionDetail({ slug, openThreadId }: TProps) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(discussionQuery(workspace.id, slug));
  if (isPending) return <ListSkeleton rows={4} />;
  if (error) {
    if (isMissing(error)) {
      return (
        <EntityNotFound
          Icon={MessagesSquareIcon}
          description="Discussion not found. It was deleted, or you cannot see it."
        />
      );
    }
    return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  }
  return <DiscussionDetailContent key={data.id} discussion={data} openThreadId={openThreadId} />;
}

function DiscussionDetailContent({
  discussion,
  openThreadId,
}: {
  discussion: TDiscussionDetail;
  openThreadId?: string;
}) {
  const { workspace, canEdit } = useWorkspace();
  const canRewrite = useCanRewrite(discussion.creator);
  const { data: me } = useQuery(meQuery);
  const { mutate: save, mutateAsync: saveAsync } = useSaveDiscussionMutation(workspace.id);
  const { mutateAsync: changeLifecycle } = useDiscussionLifecycleMutation(workspace.id);
  const navigate = useNavigate();
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

  return (
    <div className="flex w-full flex-col gap-6">
      <PageBar>
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
                {isSubscribed ? (
                  <BellOffIcon className="size-4" />
                ) : (
                  <BellIcon className="size-4" />
                )}
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
          {canRewrite && <EntityMenu noun="discussion" onTrash={trash} />}
        </HeaderActions>
      </PageBar>
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
      <DiscussionThreads discussion={discussion} openThreadId={openThreadId} />
      <CommentsSection target={{ discussionId: discussion.id }} />
    </div>
  );
}
