import { useInfiniteQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { BoxIcon, CircleDotIcon } from "lucide-react";

import type { Issue, Project } from "@/api/ops.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Avatar } from "@/components/shared/avatar";
import { EntityIcon } from "@/components/shared/entity-icon";
import { LabelChip } from "@/components/shared/label-chip";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { RelativeTime } from "@/components/shared/relative-time";
import { StateIcon, stateTypeLabels, type TStateType } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDate } from "@/lib/helpers/dates";
import { type TFilter, viewIssuesQuery, viewProjectsQuery } from "@/lib/queries/views";

const TYPE_ORDER: TStateType[] = [
  "triage",
  "started",
  "unstarted",
  "backlog",
  "completed",
  "canceled",
];

type TProps = { filter: TFilter };

/** The issues a view's filter matches, by status type. */
export function ViewIssues({ filter }: TProps) {
  const { workspace } = useWorkspace();
  const query = useInfiniteQuery(viewIssuesQuery(workspace.id, filter));
  if (query.isPending) return <ListSkeleton />;
  if (query.error) {
    return <ErrorCard message={query.error.message} onRetry={() => void query.refetch()} />;
  }
  const issues = query.data.pages.flatMap((page) => page.nodes);
  if (issues.length === 0) {
    return (
      <EmptyState
        icon={CircleDotIcon}
        title="No matching issues"
        description="Nothing matches these filters right now."
      />
    );
  }
  const groups = TYPE_ORDER.map((type) => ({
    type,
    issues: issues.filter((issue) => issue.state.type === type),
  })).filter((group) => group.issues.length > 0);

  return (
    <div className="flex w-full flex-col gap-2">
      <List flush>
        {groups.map((group) => (
          <div
            key={group.type}
            role="group"
            aria-label={stateTypeLabels[group.type]}
            className="border-b last:border-b-0"
          >
            <div className="bg-muted flex h-9 items-center gap-2 px-3 group-data-flush/list:px-6">
              <StateIcon type={group.type} />
              <span className="text-sm font-medium">{stateTypeLabels[group.type]}</span>
              <span className="text-muted-foreground text-xs tabular-nums">
                {group.issues.length}
              </span>
            </div>
            {group.issues.map((issue) => (
              <IssueRow key={issue.id} issue={issue} />
            ))}
          </div>
        ))}
      </List>
      {query.hasNextPage && (
        <Button
          variant="ghost"
          size="sm"
          isPending={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
          className="self-center"
        >
          Show more
        </Button>
      )}
    </div>
  );
}

function IssueRow({ issue }: { issue: Issue }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      to="/$workspace/issue/$identifier"
      params={{ workspace: workspace.urlKey, identifier: issue.identifier }}
      role="listitem"
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <PriorityIcon priority={issue.priority} />
      <span className="text-muted-foreground w-16 shrink-0 font-mono text-xs">
        {issue.identifier}
      </span>
      <StateIcon type={issue.state.type} color={issue.state.color} name={issue.state.name} />
      <span className="min-w-0 flex-1 truncate">{issue.title}</span>
      <span className="hidden shrink-0 items-center gap-1 lg:flex">
        {issue.labels.slice(0, 2).map((label) => (
          <LabelChip key={label.id} name={label.name} color={label.color} />
        ))}
      </span>
      {issue.project && (
        <span className="text-muted-foreground hidden max-w-40 shrink-0 items-center gap-1.5 truncate text-xs md:flex">
          <BoxIcon className="size-3.5 shrink-0" />
          <span className="truncate">{issue.project.name}</span>
        </span>
      )}
      <span className="text-muted-foreground hidden w-16 shrink-0 text-right text-xs sm:inline">
        <RelativeTime value={issue.updatedAt} />
      </span>
      {issue.assignee ? (
        <Avatar id={issue.assignee.id} name={issue.assignee.displayName} size="sm" />
      ) : (
        <span aria-hidden className="size-5 shrink-0" />
      )}
    </Link>
  );
}

/** The projects a project view's filter matches. */
export function ViewProjects({ filter }: TProps) {
  const { workspace } = useWorkspace();
  const query = useInfiniteQuery(viewProjectsQuery(workspace.id, filter));
  if (query.isPending) return <ListSkeleton />;
  if (query.error) {
    return <ErrorCard message={query.error.message} onRetry={() => void query.refetch()} />;
  }
  const projects = query.data.pages.flatMap((page) => page.nodes);
  if (projects.length === 0) {
    return (
      <EmptyState
        icon={BoxIcon}
        title="No matching projects"
        description="Nothing matches these filters right now."
      />
    );
  }
  return (
    <div className="flex w-full flex-col gap-2">
      <List flush>
        {projects.map((project) => (
          <ProjectRow key={project.id} project={project} />
        ))}
      </List>
      {query.hasNextPage && (
        <Button
          variant="ghost"
          size="sm"
          isPending={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
          className="self-center"
        >
          Show more
        </Button>
      )}
    </div>
  );
}

function ProjectRow({ project }: { project: Project }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      to="/$workspace/project/$slug"
      params={{ workspace: workspace.urlKey, slug: project.slugId }}
      role="listitem"
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <EntityIcon icon={project.icon} color={project.color} fallback={BoxIcon} />
      <span className="min-w-0 flex-1 truncate">{project.name}</span>
      <span className="text-muted-foreground flex shrink-0 items-center gap-1.5 text-xs">
        <span
          aria-hidden
          className="size-2 rounded-full"
          style={{ backgroundColor: project.status.color }}
        />
        {project.status.name}
      </span>
      <span className="text-muted-foreground hidden w-24 shrink-0 text-right text-xs sm:inline">
        {project.targetDate ? formatDate(project.targetDate) : ""}
      </span>
      <span className="text-muted-foreground w-9 shrink-0 text-right text-xs tabular-nums">
        {Math.round(project.progress * 100)}%
      </span>
      {project.lead ? (
        <Avatar id={project.lead.id} name={project.lead.displayName} size="sm" />
      ) : (
        <span aria-hidden className="size-5 shrink-0" />
      )}
    </Link>
  );
}
