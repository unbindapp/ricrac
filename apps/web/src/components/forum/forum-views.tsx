import type { ReactNode } from "react";

import type { Discussion } from "@/api/ops.gen";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import {
  LayersIcon,
  LockIcon,
  LockOpenIcon,
  type LucideIcon,
  MessagesSquareIcon,
  SearchIcon,
  Trash2Icon,
} from "@/components/icons";
import { TrashedRow } from "@/components/projects/entity-menu";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { discussionsQuery, useDiscussionLifecycleMutation } from "@/lib/queries/discussions";

export const FORUM_VIEWS = ["open", "closed", "all", "trash"] as const;
export type TForumView = (typeof FORUM_VIEWS)[number];

export const FORUM_SORTS = ["activity", "newest", "top"] as const;
export type TForumSort = (typeof FORUM_SORTS)[number];

/** What the forum lists, as its search params: open discussions by recent activity when unset. */
export type TForumFilters = {
  status?: Exclude<TForumView, "open">;
  tag?: string;
  sort?: Exclude<TForumSort, "activity">;
};

export const forumSortOptions: { value: TForumSort; label: string }[] = [
  { value: "activity", label: "Recent activity" },
  { value: "newest", label: "Newest" },
  { value: "top", label: "Most upvoted" },
];

const views: Record<TForumView, { label: string; icon: LucideIcon }> = {
  open: { label: "Open", icon: LockOpenIcon },
  closed: { label: "Closed", icon: LockIcon },
  all: { label: "All", icon: LayersIcon },
  trash: { label: "Trash", icon: Trash2Icon },
};

export const forumViewOptions = FORUM_VIEWS.map((value) => ({ value, ...views[value] }));

export function forumStatusSearch(view: TForumView): TForumFilters["status"] {
  return view !== "open" ? view : undefined;
}

export function forumDiscussionsQuery(
  workspaceId: string,
  {
    status = "open",
    tag,
    query,
    sort,
  }: { status?: TForumView; tag?: string; query?: string; sort?: TForumSort },
) {
  const isTrash = status === "trash";
  return discussionsQuery(workspaceId, {
    status: isTrash ? "all" : status,
    onlyTrashed: isTrash || undefined,
    tag,
    query: query || undefined,
    includeBody: !isTrash || undefined,
    sort: isTrash ? undefined : sort,
  });
}

export function TrashedDiscussionRow({ discussion }: { discussion: Discussion }) {
  const { workspace } = useWorkspace();
  const canRestore = useCanRewrite(discussion.creator);
  const { mutate: change, isPending } = useDiscussionLifecycleMutation(workspace.id);
  return (
    <TrashedRow
      icon={<MessagesSquareIcon className="size-4" />}
      name={discussion.title}
      trashedAt={discussion.trashedAt}
      isPending={isPending}
      onRestore={
        canRestore
          ? () =>
              change(
                { id: discussion.id, action: "restore" },
                { onSuccess: () => toast.add({ type: "success", title: "Discussion restored" }) },
              )
          : undefined
      }
    />
  );
}

export function forumEmptyState({
  status,
  isFiltered,
  action,
}: {
  status: TForumView;
  isFiltered: boolean;
  action: ReactNode;
}) {
  if (isFiltered) {
    return {
      icon: SearchIcon,
      title: "No matching discussions",
      description: "Nothing matches these filters.",
    };
  }
  if (status === "trash") {
    return {
      icon: Trash2Icon,
      title: "No trashed discussions yet",
      description: "Discussions moved to the trash stay here for 30 days.",
    };
  }
  if (status === "closed") {
    return {
      icon: MessagesSquareIcon,
      title: "No closed discussions yet",
      description: "Discussions closed once they are settled show up here.",
    };
  }
  return {
    icon: MessagesSquareIcon,
    title: "No discussions yet",
    description: "Start one to talk through an idea, a design or a problem together.",
    action,
  };
}
