import { type MouseEvent, type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import type { Issue } from "@/api/ops.gen";
import { ListTodoIcon, PlusIcon } from "@/components/icons";
import { BulkActionBar } from "@/components/issues/bulk-action-bar";
import type { DisplaySettings } from "@/components/issues/display";
import type { ListIssuesFilters } from "@/components/issues/filters";
import type { TIssueGroup } from "@/components/issues/grouping";
import {
  IssueGroupHeader,
  IssueGroupHeaderPlaceholder,
} from "@/components/issues/issue-group-header";
import { useIsPeekable, useIssuePeek } from "@/components/issues/issue-peek";
import { IssueRow, IssueRowPlaceholder } from "@/components/issues/issue-row";
import { nestSubIssues } from "@/components/issues/nesting";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { useOpenIssuePage } from "@/components/issues/use-issue-actions";
import {
  newIssueDefaultsForGroup,
  useIssueCollection,
} from "@/components/issues/use-issue-collection";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { isHeaderItem, List, type TVirtualListHandle, VirtualList } from "@/components/layout/list";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { LIST_MAX_ISSUES } from "@/lib/queries/issues";
import { useIssueSelectionStore } from "@/lib/stores/issue-selection";

export type TIssueListProps = {
  workspaceId: string;
  filters: ListIssuesFilters;
  display?: Partial<DisplaySettings>;
  fixedFilters?: Partial<ListIssuesFilters>;
  emptyState?: ReactNode;
  /** Quick actions shown on a hovered row, like triage's accept and decline. */
  rowActions?: (issue: Issue) => ReactNode;
};

const ROW_HEIGHT = 44;
const HEADER_HEIGHT = 36;

type TListItem =
  | { kind: "header"; key: string; group: TIssueGroup }
  | { kind: "row"; key: string; issue: Issue; depth: number; segment?: string };

function segmentOf(index: number, count: number) {
  const edges = [index === 0 && "first", index === count - 1 && "last"].filter(Boolean);
  return edges.length > 0 ? edges.join(" ") : undefined;
}

const PLACEHOLDER_ROWS = 12;

function IssueListPlaceholder({ showHeader }: { showHeader: boolean }) {
  return (
    <div aria-busy aria-label="Loading issues" className="flex w-full flex-col gap-2">
      <List flush>
        {showHeader && <IssueGroupHeaderPlaceholder />}
        {Array.from({ length: PLACEHOLDER_ROWS }, (_, index) => (
          <IssueRowPlaceholder key={index} index={index} />
        ))}
      </List>
    </div>
  );
}

export function IssueList(props: TIssueListProps) {
  const { query, issues, groups, display, isLoadingMore, isCapped } = useIssueCollection(props);
  const { canEdit } = useWorkspace();
  const openNewIssue = useNewIssueStore((state) => state.open);

  if (query.isPending) {
    return <IssueListPlaceholder showHeader={display.group_by !== "none"} />;
  }
  if (query.error && issues.length === 0) {
    return <ErrorCard message={query.error.message} onRetry={() => void query.refetch()} />;
  }
  if (issues.length === 0 && !isLoadingMore) {
    return (
      props.emptyState ?? (
        <EmptyState
          icon={ListTodoIcon}
          title="No matching issues"
          description="Issues that match these filters show up here."
          action={
            canEdit && (
              <Button size="sm" variant="outline" onClick={() => openNewIssue()}>
                <PlusIcon className="size-4" />
                New issue
              </Button>
            )
          }
        />
      )
    );
  }
  return (
    <div className="flex w-full flex-col gap-2">
      <VirtualIssueList
        groups={groups.filter((group) => group.issues.length > 0)}
        issues={issues}
        showHeaders={display.group_by !== "none"}
        nestSubIssues={display.nest_sub_issues}
        properties={display.properties}
        rowActions={props.rowActions}
      />
      {isLoadingMore && <p className="text-muted-foreground text-xs">Loading more issues…</p>}
      {isCapped && (
        <p className="text-muted-foreground text-xs">
          Showing the first {LIST_MAX_ISSUES} issues. Narrow the filters to see the rest.
        </p>
      )}
    </div>
  );
}

type TVirtualProps = {
  groups: TIssueGroup[];
  issues: Issue[];
  showHeaders: boolean;
  nestSubIssues: boolean;
  properties: DisplaySettings["properties"];
  rowActions?: (issue: Issue) => ReactNode;
};

function VirtualIssueList({
  groups,
  issues,
  showHeaders,
  nestSubIssues: shouldNest,
  properties,
  rowActions,
}: TVirtualProps) {
  const listRef = useRef<TVirtualListHandle>(null);
  const [collapsed, setCollapsed] = useState<string[]>([]);
  const { canEdit } = useWorkspace();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const peek = useIssuePeek();
  const isPeekable = useIsPeekable();
  const openPage = useOpenIssuePage();
  const selectedIds = useIssueSelectionStore((state) => state.selectedIds);
  const focusedId = useIssueSelectionStore((state) => state.focusedId);
  const toggle = useIssueSelectionStore((state) => state.toggle);
  const selectRange = useIssueSelectionStore((state) => state.selectRange);
  const setFocused = useIssueSelectionStore((state) => state.setFocused);
  const clear = useIssueSelectionStore((state) => state.clear);

  const items = useMemo<TListItem[]>(
    () =>
      groups.flatMap((group) => {
        const header: TListItem[] = showHeaders
          ? [{ kind: "header", key: `group:${group.key}`, group }]
          : [];
        if (showHeaders && collapsed.includes(group.key)) return header;
        const rows = shouldNest
          ? nestSubIssues(group.issues)
          : group.issues.map((issue) => ({ issue, depth: 0 }));
        return [
          ...header,
          ...rows.map(({ issue, depth }, index): TListItem => ({
            kind: "row",
            key: `${group.key}:${issue.id}`,
            issue,
            depth,
            segment: segmentOf(index, rows.length),
          })),
        ];
      }),
    [groups, showHeaders, shouldNest, collapsed],
  );
  const orderedIds = useMemo(
    () => items.flatMap((item) => (item.kind === "row" ? [item.issue.id] : [])),
    [items],
  );

  useEffect(
    () => () => {
      clear();
      setFocused(null);
    },
    [clear, setFocused],
  );

  const open = (issue: Issue) => {
    if (peek && isPeekable) {
      peek.openPeek(issue.identifier);
      return;
    }
    void openPage(issue.identifier);
  };

  const moveFocus = (step: number) => {
    if (orderedIds.length === 0) return;
    const current = focusedId ? orderedIds.indexOf(focusedId) : -1;
    const next = orderedIds[Math.max(0, Math.min(orderedIds.length - 1, current + step))];
    setFocused(next);
    const index = items.findIndex((item) => item.kind === "row" && item.issue.id === next);
    listRef.current?.scrollToIndex(index);
    requestAnimationFrame(() =>
      document.querySelector<HTMLElement>(`[data-issue-row="${next}"]`)?.focus(),
    );
  };

  const focusedIssue = issues.find((issue) => issue.id === focusedId);
  useHotkeyScope([
    { hotkey: "J", callback: () => moveFocus(1) },
    { hotkey: "K", callback: () => moveFocus(-1) },
    { hotkey: "ArrowDown", callback: () => moveFocus(1) },
    { hotkey: "ArrowUp", callback: () => moveFocus(-1) },
    {
      hotkey: "O",
      callback: () => focusedIssue && open(focusedIssue),
      enabled: Boolean(focusedIssue),
    },
    {
      hotkey: "X",
      callback: () => {
        if (focusedId && orderedIds.includes(focusedId)) toggle(focusedId);
      },
    },
    { hotkey: "Escape", callback: clear, enabled: selectedIds.length > 0, ignoreInputs: true },
  ]);

  const handleClick = (event: MouseEvent, issue: Issue) => {
    if (event.metaKey || event.ctrlKey) {
      void openPage(issue.identifier);
      return;
    }
    if (event.shiftKey) {
      selectRange(orderedIds, issue.id);
      return;
    }
    if (selectedIds.length > 0) {
      toggle(issue.id);
      return;
    }
    open(issue);
  };

  const selectedIssues = issues.filter((issue) => selectedIds.includes(issue.id));

  const renderItem = (item: TListItem) =>
    item.kind === "header" ? (
      <IssueGroupHeader
        group={item.group}
        isCollapsed={collapsed.includes(item.group.key)}
        onToggle={() =>
          setCollapsed((current) =>
            current.includes(item.group.key)
              ? current.filter((key) => key !== item.group.key)
              : [...current, item.group.key],
          )
        }
        onCreate={canEdit ? () => openNewIssue(newIssueDefaultsForGroup(item.group)) : undefined}
      />
    ) : (
      <IssueRow
        issue={item.issue}
        depth={item.depth}
        segment={item.segment}
        properties={properties}
        isSelected={selectedIds.includes(item.issue.id)}
        isActive={peek?.peek?.toUpperCase() === item.issue.identifier}
        isFocused={focusedId === item.issue.id}
        canEdit={canEdit}
        actions={rowActions?.(item.issue)}
        onClick={handleClick}
        onFocus={(issue) => setFocused(issue.id)}
      />
    );

  return (
    <>
      <VirtualList
        flush
        handle={listRef}
        items={items}
        getKey={(item) => item.key}
        estimateSize={(item) => (item.kind === "header" ? HEADER_HEIGHT : ROW_HEIGHT)}
        isSticky={isHeaderItem}
        renderItem={renderItem}
      />
      <BulkActionBar issues={selectedIssues} onClear={clear} />
    </>
  );
}
