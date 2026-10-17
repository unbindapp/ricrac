import { useQuery } from "@tanstack/react-query";
import { addDays, addMonths, nextMonday, set } from "date-fns";
import {
  AlarmClockIcon,
  ArchiveIcon,
  ArchiveRestoreIcon,
  BellIcon,
  BellOffIcon,
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
} from "lucide-react";

import type { ReactNode } from "react";

import type { IssueDetail } from "@/api/ops.gen";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import {
  useCopyIssue,
  useIssueLifecycle,
  useOpenIssuePage,
} from "@/components/issues/use-issue-actions";
import { Breadcrumb } from "@/components/layout/breadcrumb";
import { PageBar } from "@/components/layout/page-header";
import { useNow } from "@/components/providers/now-provider";
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
import { teamCrumb } from "@/components/workspace/team-crumb";
import { useTeams } from "@/components/workspace/teams-provider";
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
    { label: "Tomorrow", until: set(addDays(now, 1), MORNING) },
    { label: "Next week", until: set(nextMonday(now), MORNING) },
    { label: "In a month", until: set(addMonths(now, 1), MORNING) },
  ];
}

export function IssueDetailHeader({ issue, variant, onClose, onSetSla }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const { teams } = useTeams();
  const { data: me } = useQuery(meQuery);
  const now = useNow();
  const team = teams.find((item) => item.id === issue.team.id);
  const { copyLink, copyBranch, copyIdentifier } = useCopyIssue();
  const lifecycle = useIssueLifecycle();
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
            team ? teamCrumb(team, workspace.urlKey) : { label: issue.team.name },
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
                          <span className="flex-1">{option.label}</span>
                          <span className="text-muted-foreground text-xs">
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
                  onClick={() =>
                    void lifecycle
                      .run([issue.id], issue.trashedAt ? "restore" : "delete")
                      .catch(() => undefined)
                  }
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
