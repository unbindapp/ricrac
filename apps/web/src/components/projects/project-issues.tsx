import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import type { Issue, ProjectDetail, StateRef } from "@/api/ops.gen";
import { CircleDotIcon, DiamondIcon, PlusIcon } from "@/components/icons";
import { useNewIssueContext, useNewIssueStore } from "@/components/issues/new-issue-store";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PersonAvatar } from "@/components/projects/people";
import { FilterChip } from "@/components/shared/filter-chip";
import { GroupToggle } from "@/components/shared/group-toggle";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { StateIcon, stateTypeLabels } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { projectIssuesQuery } from "@/lib/queries/projects";

const NO_MILESTONE = "none";

const groupOrder: StateRef["type"][] = [
  "started",
  "unstarted",
  "triage",
  "backlog",
  "completed",
  "canceled",
];

type TProps = {
  project: ProjectDetail;
  milestone: string | undefined;
  onMilestoneChange: (milestone: string | undefined) => void;
};

/** The project's issues by where they stand, narrowed to a milestone. */
export function ProjectIssues({ project, milestone, onMilestoneChange }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(
    projectIssuesQuery(workspace.id, project.id),
  );
  const openNewIssue = useNewIssueStore((state) => state.open);
  const [toggled, setToggled] = useState<string[]>([]);
  const projectRef = { id: project.id, name: project.name, slugId: project.slugId };
  useNewIssueContext({ draft: { project: projectRef } });

  const issues = (data ?? []).filter((issue) => {
    if (!milestone) return true;
    if (milestone === NO_MILESTONE) return !issue.projectMilestone;
    return issue.projectMilestone?.id === milestone;
  });

  const newIssue = canEdit && (
    <Button
      size="sm"
      variant="outline"
      onClick={() => openNewIssue({ draft: { project: projectRef } })}
    >
      <PlusIcon className="size-4" />
      New issue
    </Button>
  );

  return (
    <div className="flex w-full flex-col gap-3">
      {(project.milestones.length > 0 || issues.length > 0) && (
        <div className="flex items-start gap-2">
          <div
            role="radiogroup"
            aria-label="Milestone"
            className="flex min-w-0 flex-1 flex-wrap items-center gap-1"
          >
            {project.milestones.length > 0 && (
              <>
                <MilestoneChip
                  label="All"
                  isActive={!milestone}
                  onClick={() => onMilestoneChange(undefined)}
                />
                {project.milestones.map((item) => (
                  <MilestoneChip
                    key={item.id}
                    label={item.name}
                    icon
                    isActive={milestone === item.id}
                    onClick={() => onMilestoneChange(item.id)}
                  />
                ))}
                <MilestoneChip
                  label="No milestone"
                  isActive={milestone === NO_MILESTONE}
                  onClick={() => onMilestoneChange(NO_MILESTONE)}
                />
              </>
            )}
          </div>
          {issues.length > 0 && newIssue}
        </div>
      )}
      {isPending && <ListSkeleton />}
      {error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && issues.length === 0 && (
        <EmptyState
          icon={CircleDotIcon}
          title={milestone ? "No matching issues" : "No issues yet"}
          description={milestone ? "No issues in this milestone." : "Add issues to the project."}
          action={newIssue}
        />
      )}
      {!isPending && !error && issues.length > 0 && (
        <List>
          {groupOrder.map((type) => {
            const group = issues.filter((issue) => issue.state.type === type);
            if (group.length === 0) return null;
            const closedByDefault = type === "completed" || type === "canceled";
            const isCollapsed = closedByDefault !== toggled.includes(type);
            return (
              <div key={type} role="presentation" data-slot="list-group" className="contents">
                <div className="bg-muted dank:h-12 dank:gap-1 dank:border-0 dank:bg-transparent dank:px-0 flex h-9 items-center border-b px-1.5">
                  <GroupToggle
                    icon={<StateIcon type={type} />}
                    label={stateTypeLabels[type]}
                    count={group.length}
                    isCollapsed={isCollapsed}
                    onToggle={() =>
                      setToggled((current) =>
                        current.includes(type)
                          ? current.filter((item) => item !== type)
                          : [...current, type],
                      )
                    }
                  />
                </div>
                {!isCollapsed &&
                  group.map((issue) => (
                    <IssueRow key={issue.id} issue={issue} showMilestone={!milestone} />
                  ))}
              </div>
            );
          })}
        </List>
      )}
    </div>
  );
}

function MilestoneChip({
  label,
  icon,
  isActive,
  onClick,
}: {
  label: string;
  icon?: boolean;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <FilterChip
      role="radio"
      aria-checked={isActive}
      selected={isActive}
      forceMinSize={false}
      onClick={onClick}
    >
      {icon && (
        <DiamondIcon
          className={cn(
            "text-muted-foreground dank:text-m3-primary size-3.5",
            isActive && "dank:hidden",
          )}
        />
      )}
      {label}
    </FilterChip>
  );
}

function IssueRow({ issue, showMilestone }: { issue: Issue; showMilestone: boolean }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      role="listitem"
      to="/$workspace/issue/$identifier"
      params={{ workspace: workspace.urlKey, identifier: issue.identifier }}
      className="has-hover:hover:bg-accent focus-visible:bg-accent flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden last:border-b-0"
    >
      <span className="flex size-4 shrink-0">
        {issue.priority !== 0 && <PriorityIcon priority={issue.priority} />}
      </span>
      <span className="text-muted-foreground w-16 shrink-0 font-mono text-xs">
        {issue.identifier}
      </span>
      <StateIcon type={issue.state.type} color={issue.state.color} name={issue.state.name} />
      <span className="min-w-0 flex-1 truncate">{issue.title}</span>
      {showMilestone && issue.projectMilestone && (
        <span className="text-muted-foreground hidden max-w-40 shrink-0 items-center gap-1.5 text-xs md:flex">
          <DiamondIcon className="size-3.5 shrink-0" />
          <span className="truncate">{issue.projectMilestone.name}</span>
        </span>
      )}
      <span className="flex w-6 shrink-0 justify-center">
        {issue.assignee && (
          <SimpleTooltip content={issue.assignee.name}>
            <span className="flex">
              <PersonAvatar user={issue.assignee} />
            </span>
          </SimpleTooltip>
        )}
      </span>
    </Link>
  );
}
