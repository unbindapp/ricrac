import { useInfiniteQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import type { Issue, Project } from "@/api/ops.gen";
import { BoxIcon, CircleDotIcon } from "@/components/icons";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { VirtualList, isHeaderItem } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Avatar } from "@/components/shared/avatar";
import { EntityIcon } from "@/components/shared/entity-icon";
import { LabelChip } from "@/components/shared/label-chip";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { RelativeTime } from "@/components/shared/relative-time";
import { StateIcon, stateTypeLabels, type TStateType } from "@/components/shared/state-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useLoadMore } from "@/lib/hooks/use-load-more";
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

const HEADER_HEIGHT = 36;
const ROW_HEIGHT = 44;

type TProps = { filter: TFilter };

type TIssueItem =
  { kind: "header"; type: TStateType; count: number } | { kind: "row"; issue: Issue };

/** The issues a view's filter matches, by status type. */
export function ViewIssues({ filter }: TProps) {
  const { workspace } = useWorkspace();
  const query = useInfiniteQuery(viewIssuesQuery(workspace.id, filter));
  const loadMore = useLoadMore(query);
  if (query.isPending) return <ListSkeleton flush />;
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
  const items = TYPE_ORDER.flatMap((type): TIssueItem[] => {
    const matching = issues.filter((issue) => issue.state.type === type);
    if (matching.length === 0) return [];
    return [
      { kind: "header", type, count: matching.length },
      ...matching.map((issue) => ({ kind: "row" as const, issue })),
    ];
  });

  return (
    <div className="flex w-full flex-col gap-2">
      <VirtualList
        isSticky={isHeaderItem}
        flush
        items={items}
        getKey={(item) => (item.kind === "header" ? `type:${item.type}` : item.issue.id)}
        estimateSize={(item) => (item.kind === "header" ? HEADER_HEIGHT : ROW_HEIGHT)}
        onEndReached={loadMore}
        renderItem={(item) =>
          item.kind === "header" ? (
            <StatusTypeHeader type={item.type} count={item.count} />
          ) : (
            <IssueRow issue={item.issue} />
          )
        }
      />
      {query.isFetchingNextPage && <LoadingMore />}
    </div>
  );
}

function StatusTypeHeader({ type, count }: { type: TStateType; count: number }) {
  return (
    <div className="bg-muted dank:h-12 dank:bg-transparent dank:px-0 dank:group-data-flush/list:px-0 flex h-9 items-center gap-2 px-3">
      <StateIcon type={type} />
      <span className="dank:type-title-small text-sm font-medium">{stateTypeLabels[type]}</span>
      <span className="text-muted-foreground dank:type-label-large text-xs tabular-nums">
        {count}
      </span>
    </div>
  );
}

function LoadingMore() {
  return <p className="text-muted-foreground text-xs">Loading more…</p>;
}

function IssueRow({ issue }: { issue: Issue }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      to="/$workspace/issue/$identifier"
      params={{ workspace: workspace.urlKey, identifier: issue.identifier }}
      role="listitem"
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 last:border-b-0"
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
  const loadMore = useLoadMore(query);
  if (query.isPending) return <ListSkeleton flush />;
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
      <VirtualList
        flush
        items={projects}
        getKey={(project) => project.id}
        onEndReached={loadMore}
        renderItem={(project) => <ProjectRow project={project} />}
      />
      {query.isFetchingNextPage && <LoadingMore />}
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
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 last:border-b-0"
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
