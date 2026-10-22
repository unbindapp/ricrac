import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";

import type { DiscussionDetail, DiscussionThread } from "@/api/ops.gen";
import { THREAD_KINDS, threadKindOf, threadKinds } from "@/components/forum/thread-kinds";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { CircleCheckIcon, LinkIcon, SplitIcon, Trash2Icon, UndoIcon } from "@/components/icons";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "@/components/ui/context-menu";
import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { useCopyToClipboard } from "@/lib/hooks/use-copy";
import {
  useDeleteDiscussionThreadMutation,
  useSaveDiscussionThreadMutation,
} from "@/lib/queries/discussions";

type TProps = {
  discussion: DiscussionDetail;
  thread: DiscussionThread;
  isOpen: boolean;
  /** The row the menu opens on with a right click or a long press. */
  children: ReactElement;
};

export function ThreadContextMenu({ discussion, thread, isOpen, children }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const canRewrite = useCanRewrite(thread.creator);
  const navigate = useNavigate();
  const { copyToClipboard } = useCopyToClipboard();
  const [isDeleting, setIsDeleting] = useState(false);
  const { mutate: save } = useSaveDiscussionThreadMutation(workspace.id);
  const { mutateAsync: deleteThread } = useDeleteDiscussionThreadMutation(workspace.id);
  const isResolved = Boolean(thread.resolvedAt);
  const kind = threadKindOf(thread.kind);
  const KindIcon = threadKinds[kind].icon;
  const discussionLink = {
    to: "/$workspace/discussion/$slug",
    params: { workspace: workspace.urlKey, slug: entitySlug(discussion) },
    replace: true,
  } as const;

  const remove = async () => {
    if (isOpen) await navigate({ ...discussionLink, search: {} });
    await deleteThread(thread.id);
    toast.add({ type: "success", title: "Thread deleted" });
  };

  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger render={children} />
        <ContextMenuContent className="w-56">
          <DropdownMenuGroup>
            <DropdownMenuItem
              onClick={() => void navigate({ ...discussionLink, search: { thread: thread.id } })}
            >
              <SplitIcon className="size-4" />
              Open thread
            </DropdownMenuItem>
            {canEdit && (
              <DropdownMenuItem onClick={() => save({ id: thread.id, resolved: !isResolved })}>
                {isResolved ? (
                  <UndoIcon className="size-4" />
                ) : (
                  <CircleCheckIcon className="size-4" />
                )}
                {isResolved ? "Reopen" : "Resolve"}
              </DropdownMenuItem>
            )}
            {canEdit && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <KindIcon className="size-4" />
                  Kind
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  <DropdownMenuRadioGroup
                    value={kind}
                    className="border-0 bg-transparent p-0"
                    onValueChange={(next) => {
                      const picked = THREAD_KINDS.find((known) => known === next);
                      if (picked && picked !== kind) save({ id: thread.id, kind: picked });
                    }}
                  >
                    {THREAD_KINDS.map((known) => (
                      <DropdownMenuRadioItem key={known} value={known}>
                        {threadKinds[known].label}
                      </DropdownMenuRadioItem>
                    ))}
                  </DropdownMenuRadioGroup>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            <DropdownMenuItem
              onClick={() => {
                copyToClipboard(thread.url);
                toast.add({ type: "success", title: "Link copied" });
              }}
            >
              <LinkIcon className="size-4" />
              Copy link
            </DropdownMenuItem>
            {canRewrite && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
                  <Trash2Icon className="size-4" />
                  Delete thread
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuGroup>
        </ContextMenuContent>
      </ContextMenu>
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title="Delete thread"
        description="The thread and its comments are deleted for everyone."
        confirmLabel="Delete thread"
        onConfirm={remove}
      />
    </>
  );
}
