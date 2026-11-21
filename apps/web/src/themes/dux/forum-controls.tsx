import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useEffect, useRef } from "react";

import {
  forumSortOptions,
  forumStatusSearch,
  forumViewOptions,
  type TForumFilters,
  type TForumSort,
  type TForumView,
} from "@/components/forum/forum-views";
import { ArrowUpIcon, CheckIcon, ListFilterIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { discussionTagsQuery } from "@/lib/queries/discussions";

export const FORUM_CHIPS_ID = "dux-forum-chips";

/** How many of the forum's filters differ from open discussions by recent activity. */
export function activeFilterCount(filters: TForumFilters) {
  return [filters.status, filters.tag, filters.sort, filters.order].filter(Boolean).length;
}

/** Shows and hides the filter chips, with a badge counting the filters in use. */
export function ForumFilterButton({
  filters,
  isExpanded,
  onToggle,
}: {
  filters: TForumFilters;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const count = activeFilterCount(filters);
  return (
    <SimpleTooltip content="Filter" side="bottom">
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        data-slot="dux-filter-button"
        aria-label={count > 0 ? `Filter discussions, ${count} in use` : "Filter discussions"}
        aria-expanded={isExpanded}
        aria-controls={FORUM_CHIPS_ID}
        onClick={onToggle}
      >
        <ListFilterIcon />
        {count > 0 && (
          <span aria-hidden data-slot="dux-badge">
            {count}
          </span>
        )}
      </Button>
    </SimpleTooltip>
  );
}

/** Filter chips in one scrolling row: which discussions, in what order, and which tag. */
export function ForumChips({
  filters,
  onFilter,
}: {
  filters: TForumFilters;
  onFilter: (next: TForumFilters) => void;
}) {
  const { workspace } = useWorkspace();
  const { data: tags = [] } = useQuery(discussionTagsQuery(workspace.id));
  const status: TForumView = filters.status ?? "open";
  const sort: TForumSort = filters.sort ?? "activity";
  const isTrash = status === "trash";
  const rowRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const row = rowRef.current;
    if (!row) return;
    const scrollSideways = (event: WheelEvent) => {
      const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
      if (!delta || row.scrollWidth <= row.clientWidth) return;
      event.preventDefault();
      row.scrollLeft += delta;
    };
    row.addEventListener("wheel", scrollSideways, { passive: false });
    return () => row.removeEventListener("wheel", scrollSideways);
  }, []);

  return (
    <div
      ref={rowRef}
      id={FORUM_CHIPS_ID}
      role="group"
      aria-label="Filter discussions"
      data-slot="dux-chips"
    >
      <ChipGroup label="Show">
        {forumViewOptions.map((option) => (
          <Chip
            key={option.value}
            isPressed={option.value === status}
            onClick={() => onFilter({ status: forumStatusSearch(option.value) })}
          >
            {option.label}
          </Chip>
        ))}
      </ChipGroup>
      {!isTrash && (
        <ChipGroup label="Sort">
          {forumSortOptions.map((option) => (
            <Chip
              key={option.value}
              isPressed={option.value === sort}
              onClick={() =>
                onFilter({ sort: option.value === "activity" ? undefined : option.value })
              }
            >
              {option.label}
            </Chip>
          ))}
          <Chip
            isPressed={filters.order === "asc"}
            icon={<ArrowUpIcon />}
            onClick={() => onFilter({ order: filters.order === "asc" ? undefined : "asc" })}
          >
            Oldest first
          </Chip>
        </ChipGroup>
      )}
      {tags.length > 0 && (
        <ChipGroup label="Tag">
          {tags.map((tag) => (
            <Chip
              key={tag.name}
              isPressed={tag.name === filters.tag}
              onClick={() => onFilter({ tag: tag.name === filters.tag ? undefined : tag.name })}
            >
              {tag.name}
            </Chip>
          ))}
        </ChipGroup>
      )}
    </div>
  );
}

function ChipGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div role="group" aria-label={label} data-slot="dux-chip-group">
      {children}
    </div>
  );
}

function Chip({
  isPressed,
  icon,
  onClick,
  children,
}: {
  isPressed: boolean;
  icon?: ReactNode;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      data-slot="dux-chip"
      aria-pressed={isPressed}
      onClick={onClick}
    >
      {isPressed ? <CheckIcon /> : icon}
      <span>{children}</span>
    </Button>
  );
}
