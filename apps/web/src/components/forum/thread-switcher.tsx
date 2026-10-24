import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import type { DiscussionDetail, DiscussionThread } from "@/api/ops.gen";
import { NewThreadDialog } from "@/components/forum/new-thread-dialog";
import { threadKindOf, threadKinds } from "@/components/forum/thread-kinds";
import { MessagesSquareIcon, PlusIcon, SplitIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";

type TProps = { discussion: DiscussionDetail; threadId?: string };

/** Lists the discussion's main chat and its threads to switch between, and starts new ones. */
export function ThreadSwitcher({ discussion, threadId }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const [isStarting, setIsStarting] = useState(false);
  const open = discussion.threads.filter((thread) => !thread.resolvedAt);
  const resolved = discussion.threads.filter((thread) => thread.resolvedAt);
  const switchTo = (id?: string) =>
    void navigate({
      to: "/$workspace/discussion/$slug",
      params: { workspace: workspace.urlKey, slug: entitySlug(discussion) },
      search: id ? { thread: id } : {},
      replace: true,
    });

  return (
    <>
      <DropdownMenu>
        <SimpleTooltip content="Threads">
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" aria-label="Threads">
                <SplitIcon className="size-4" />
              </Button>
            }
          />
        </SimpleTooltip>
        <DropdownMenuContent align="end" className="w-72">
          <DropdownMenuGroup>
            <DropdownMenuItem
              data-checked={!threadId || undefined}
              aria-current={!threadId || undefined}
              onClick={() => switchTo()}
            >
              <MessagesSquareIcon className="size-4" />
              <span className="min-w-0 flex-1 truncate">Main chat</span>
              <MessageCount count={discussion.commentCount} />
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <ThreadGroup label="Open" threads={open} currentId={threadId} onSwitch={switchTo} />
          <ThreadGroup
            label="Resolved"
            threads={resolved}
            currentId={threadId}
            onSwitch={switchTo}
          />
          {canEdit && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem onClick={() => setIsStarting(true)}>
                  <PlusIcon className="size-4" />
                  New thread
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <NewThreadDialog discussion={discussion} open={isStarting} onOpenChange={setIsStarting} />
    </>
  );
}

function ThreadGroup({
  label,
  threads,
  currentId,
  onSwitch,
}: {
  label: string;
  threads: DiscussionThread[];
  currentId?: string;
  onSwitch: (id: string) => void;
}) {
  if (threads.length === 0) return null;
  return (
    <>
      <DropdownMenuSeparator />
      <DropdownMenuGroup>
        <DropdownMenuLabel>{label}</DropdownMenuLabel>
        {threads.map((thread) => {
          const { label: kind, icon: Icon } = threadKinds[threadKindOf(thread.kind)];
          const isCurrent = thread.id === currentId;
          return (
            <DropdownMenuItem
              key={thread.id}
              data-checked={isCurrent || undefined}
              aria-current={isCurrent || undefined}
              onClick={() => onSwitch(thread.id)}
            >
              <Icon aria-label={kind} role="img" className="size-4" />
              <span className="min-w-0 flex-1 truncate">{thread.title}</span>
              <MessageCount count={thread.commentCount} />
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuGroup>
    </>
  );
}

function MessageCount({ count }: { count: number }) {
  return <span className="text-muted-foreground shrink-0 text-xs tabular-nums">{count}</span>;
}
