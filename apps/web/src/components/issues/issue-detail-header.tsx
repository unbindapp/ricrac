import { useQuery } from "@tanstack/react-query";
import { addDays, addMonths, nextMonday, set } from "date-fns";

import { type ReactNode, useState } from "react";

import type { IssueDetail } from "@/api/ops.gen";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import {
  AlarmClockIcon,
  ArchiveIcon,
  ArchiveRestoreIcon,
  BellIcon,
  BellOffIcon,
  CalendarClockIcon,
  CalendarIcon,
  CopyIcon,
  EllipsisIcon,
  FlameIcon,
  GitBranchIcon,
  LinkIcon,
  Maximize2Icon,
  SunIcon,
  Trash2Icon,
  UndoIcon,
  XIcon,
} from "@/components/icons";
import {
  useCopyIssue,
  useIssueLifecycle,
  useOpenIssuePage,
} from "@/components/issues/use-issue-actions";
import { Breadcrumb } from "@/components/layout/breadcrumb";
import { PageBar } from "@/components/layout/page-header";
import { useNow } from "@/components/providers/now-provider";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { HeaderAction, HeaderActions } from "@/components/shared/detail-actions";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDateTime } from "@/lib/helpers/dates";
import { useIssueSubscriptionMutation, useUpdateIssueMutation } from "@/lib/queries/issues";
import { meQuery } from "@/lib/queries/me";

type TProps = {
  issue: IssueDetail;
  variant: "page" | "peek";
  onClose?: () => void;
  /** Opens the SLA row's date picker. */
  onSetSla: () => void;
};

const MORNING = { hours: 9, minutes: 0, seconds: 0, milliseconds: 0 };

function snoozeOptions(now: Date) {
  return [
    { label: "Tomorrow", icon: SunIcon, until: set(addDays(now, 1), MORNING) },
    { label: "Next week", icon: CalendarIcon, until: set(nextMonday(now), MORNING) },
    { label: "In a month", icon: CalendarClockIcon, until: set(addMonths(now, 1), MORNING) },
  ];
}

export function IssueDetailHeader({ issue, variant, onClose, onSetSla }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  const now = useNow();
  const { copyLink, copyBranch, copyIdentifier } = useCopyIssue();
  const lifecycle = useIssueLifecycle();
  const [isDeleting, setIsDeleting] = useState(false);
  const openPage = useOpenIssuePage();
  const { mutate: updateIssue } = useUpdateIssueMutation(workspace.id);
  const { mutate: setSubscribed, isPending: isSubscribing } = useIssueSubscriptionMutation(
    workspace.id,
  );
  const isSubscribed = Boolean(me && issue.subscribers.some((user) => user.id === me.id));
  const isSnoozed = Boolean(issue.snoozedUntil && new Date(issue.snoozedUntil).getTime() > now);
  const snooze = (until: string | null) =>
    updateIssue({
      id: issue.id,
      changes: { snoozedUntil: until },
      optimistic: { snoozedUntil: until },
    });

  const Bar = variant === "page" ? PageBar : PeekBar;
  return (
    <Bar>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <Breadcrumb
          items={[
            ...(variant === "page" ? [{ label: workspace.name }] : []),
            {
              label: "Issues",
              link: { to: "/$workspace/issues", params: { workspace: workspace.urlKey } },
            },
            { label: issue.identifier },
          ]}
        />
      </div>
      <HeaderActions>
        {variant === "page" && <FavoriteButton kind="issue" entityId={issue.id} />}
        <HeaderAction label="Copy link" onClick={() => copyLink(issue)}>
          <LinkIcon className="size-4" />
        </HeaderAction>
        <HeaderAction label="Copy branch name" onClick={() => copyBranch(issue)}>
          <GitBranchIcon className="size-4" />
        </HeaderAction>
        {canEdit && me && (
          <HeaderAction
            label={isSubscribed ? "Unsubscribe" : "Subscribe"}
            isPending={isSubscribing}
            onClick={() =>
              setSubscribed({ id: issue.id, users: [me.id], subscribed: !isSubscribed })
            }
          >
            {isSubscribed ? <BellOffIcon className="size-4" /> : <BellIcon className="size-4" />}
          </HeaderAction>
        )}
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" aria-label="More actions">
                <EllipsisIcon className="size-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuItem onClick={() => copyIdentifier(issue)}>
              <CopyIcon className="size-4" />
              Copy ID
            </DropdownMenuItem>
            {canEdit && (
              <>
                <DropdownMenuSeparator />
                {isSnoozed ? (
                  <DropdownMenuItem onClick={() => snooze(null)}>
                    <SunIcon className="size-4" />
                    Wake up
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                      <AlarmClockIcon className="size-4" />
                      Snooze
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent className="w-64">
                      {snoozeOptions(new Date(now)).map((option) => (
                        <DropdownMenuItem
                          key={option.label}
                          onClick={() => snooze(option.until.toISOString())}
                        >
                          <option.icon className="size-4" />
                          <span className="dank:min-w-0 dank:truncate flex-1">{option.label}</span>
                          <span className="text-muted-foreground dank:type-label-small dank:shrink-0 dank:whitespace-nowrap text-xs">
                            {formatDateTime(option.until)}
                          </span>
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                )}
                {!issue.slaBreachesAt && (
                  <DropdownMenuItem onClick={onSetSla}>
                    <FlameIcon className="size-4" />
                    Set SLA
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() =>
                    void lifecycle
                      .run([issue.id], issue.archivedAt ? "unarchive" : "archive")
                      .catch(() => undefined)
                  }
                >
                  {issue.archivedAt ? (
                    <ArchiveRestoreIcon className="size-4" />
                  ) : (
                    <ArchiveIcon className="size-4" />
                  )}
                  {issue.archivedAt ? "Unarchive" : "Archive"}
                </DropdownMenuItem>
                <DropdownMenuItem
                  className={issue.trashedAt ? undefined : "text-destructive"}
                  onClick={() => {
                    if (!issue.trashedAt) return setIsDeleting(true);
                    void lifecycle.run([issue.id], "restore").catch(() => undefined);
                  }}
                >
                  {issue.trashedAt ? (
                    <UndoIcon className="size-4" />
                  ) : (
                    <Trash2Icon className="size-4" />
                  )}
                  {issue.trashedAt ? "Restore from trash" : "Delete"}
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
        <ConfirmDialog
          open={isDeleting}
          onOpenChange={setIsDeleting}
          title={`Delete ${issue.identifier}`}
          description="It stays in the trash for 30 days, then it is deleted for good."
          confirmLabel="Delete issue"
          onConfirm={() => lifecycle.run([issue.id], "delete")}
        />
        {variant === "peek" && (
          <HeaderAction label="Open full page" onClick={() => void openPage(issue.identifier)}>
            <Maximize2Icon className="size-4" />
          </HeaderAction>
        )}
        {onClose && (
          <HeaderAction label="Close" onClick={onClose}>
            <XIcon className="size-4" />
          </HeaderAction>
        )}
      </HeaderActions>
    </Bar>
  );
}

function PeekBar({ children }: { children: ReactNode }) {
  return <div className="flex min-h-9 w-full min-w-0 items-center gap-2">{children}</div>;
}
