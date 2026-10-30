import { Link } from "@tanstack/react-router";
import { type ComponentProps, useState } from "react";

import type { IssueChild, StateRef } from "@/api/ops.gen";
import { pointsText } from "@/components/cycles/cycle-status";
import { ChevronRightIcon } from "@/components/icons";
import { EmptyState } from "@/components/layout/empty-state";
import { List } from "@/components/layout/list";
import { Avatar } from "@/components/shared/avatar";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { StateIcon, type TStateType } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";

const TYPE_ORDER: TStateType[] = [
  "started",
  "unstarted",
  "backlog",
  "triage",
  "completed",
  "canceled",
];

type TStateIssue = IssueChild & { estimate?: number | null };
type TGroup = { state: StateRef; issues: TStateIssue[] };

function groupByState(issues: TStateIssue[]): TGroup[] {
  const groups = new Map<string, TGroup>();
  for (const issue of issues) {
    const group = groups.get(issue.state.id) ?? { state: issue.state, issues: [] };
    group.issues.push(issue);
    groups.set(issue.state.id, group);
  }
  return [...groups.values()].sort(
    (a, b) => TYPE_ORDER.indexOf(a.state.type) - TYPE_ORDER.indexOf(b.state.type),
  );
}

/** Issues by status, like a cycle's or a milestone's; finished ones start collapsed. */
export function IssuesByState({
  issues,
  empty,
}: {
  issues: TStateIssue[];
  empty: ComponentProps<typeof EmptyState>;
}) {
  if (issues.length === 0) return <EmptyState {...empty} />;
  return (
    <List>
      {groupByState(issues).map((group) => (
        <StateGroup key={group.state.id} group={group} />
      ))}
    </List>
  );
}

function StateGroup({ group }: { group: TGroup }) {
  const isFinished = group.state.type === "completed" || group.state.type === "canceled";
  const [isOpen, setIsOpen] = useState(!isFinished);
  return (
    <>
      <div className="bg-muted/50 dank:h-12 dank:gap-1 dank:border-0 dank:bg-transparent dank:px-0 flex h-9 items-center gap-2 border-b px-3 last:border-b-0">
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-expanded={isOpen}
          aria-label={isOpen ? `Collapse ${group.state.name}` : `Expand ${group.state.name}`}
          onClick={() => setIsOpen((open) => !open)}
          className="text-muted-foreground dank:ml-0 dank:size-10 -ml-1 size-6"
        >
          <ChevronRightIcon
            data-open={isOpen || undefined}
            className="size-4 transition-transform data-open:rotate-90"
          />
        </Button>
        <StateIcon type={group.state.type} color={group.state.color} />
        <span className="dank:type-title-small text-sm font-medium">{group.state.name}</span>
        <span className="text-muted-foreground dank:type-label-large text-xs tabular-nums">
          {group.issues.length}
        </span>
      </div>
      {isOpen && group.issues.map((issue) => <IssueRow key={issue.id} issue={issue} />)}
    </>
  );
}

function IssueRow({ issue }: { issue: TStateIssue }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      to="/$workspace/issue/$identifier"
      params={{ workspace: workspace.urlKey, identifier: issue.identifier }}
      role="listitem"
      className="has-hover:hover:bg-accent focus-visible:bg-accent flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0"
    >
      <PriorityIcon priority={issue.priority} />
      <span className="text-muted-foreground w-16 shrink-0 font-mono text-xs">
        {issue.identifier}
      </span>
      <StateIcon type={issue.state.type} color={issue.state.color} name={issue.state.name} />
      <span className="min-w-0 flex-1 truncate">{issue.title}</span>
      {issue.estimate != null && (
        <span className="text-muted-foreground shrink-0 text-xs tabular-nums">
          {pointsText(issue.estimate)}
        </span>
      )}
      {issue.assignee ? (
        <Avatar id={issue.assignee.id} name={issue.assignee.displayName} size="sm" />
      ) : (
        <span aria-hidden className="size-5 shrink-0" />
      )}
    </Link>
  );
}
