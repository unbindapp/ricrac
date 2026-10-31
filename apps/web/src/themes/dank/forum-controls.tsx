import { useQuery } from "@tanstack/react-query";

import {
  forumSortOptions,
  forumStatusSearch,
  forumViewOptions,
  type TForumFilters,
  type TForumSort,
  type TForumView,
} from "@/components/forum/forum-views";
import {
  ArrowDownIcon,
  ArrowUpIcon,
  CalendarIcon,
  CheckIcon,
  HistoryIcon,
  LayoutGridIcon,
  LayoutListIcon,
  type LucideIcon,
  TagsIcon,
  TrendingUpIcon,
} from "@/components/icons";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { discussionTagsQuery } from "@/lib/queries/discussions";

export type TForumLayout = "grid" | "list";

type TProps = {
  filters: TForumFilters;
  layout: TForumLayout;
  onFilter: (next: TForumFilters) => void;
  onLayout: (layout: TForumLayout) => void;
};

const sortIcons: Record<TForumSort, LucideIcon> = {
  activity: HistoryIcon,
  newest: CalendarIcon,
  top: TrendingUpIcon,
};

/** Which discussions the forum lists on the left, and how on the right. */
export function ForumToolbar({ filters, layout, onFilter, onLayout }: TProps) {
  const status: TForumView = filters.status ?? "open";
  return (
    <div role="toolbar" aria-label="Forum" data-slot="forum-toolbar">
      <div data-slot="forum-toolbar-group">
        <StatusMenu status={status} onFilter={onFilter} />
        <TagMenu tag={filters.tag} onFilter={onFilter} />
      </div>
      {status !== "trash" && (
        <div data-slot="forum-toolbar-group">
          <LayoutButton layout={layout} onLayout={onLayout} />
          <SortButtons filters={filters} onFilter={onFilter} />
        </div>
      )}
    </div>
  );
}

function StatusMenu({
  status,
  onFilter,
}: {
  status: TForumView;
  onFilter: (next: TForumFilters) => void;
}) {
  const current = forumViewOptions.find((option) => option.value === status) ?? forumViewOptions[0];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            forceMinSize={false}
            data-slot="filter-chip"
            data-active={status !== "open" || undefined}
            aria-label={`Show: ${current.label}`}
          >
            <current.icon />
            <span className="truncate">{current.label}</span>
            <DropDownArrow />
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="min-w-48">
        <DropdownMenuGroup>
          {forumViewOptions.map((option) => (
            <DropdownMenuItem
              key={option.value}
              data-checked={option.value === status || undefined}
              onClick={() => onFilter({ status: forumStatusSearch(option.value) })}
            >
              <option.icon className="size-4" />
              <span className="min-w-0 flex-1 truncate">{option.label}</span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function TagMenu({ tag, onFilter }: { tag?: string; onFilter: (next: TForumFilters) => void }) {
  const { workspace } = useWorkspace();
  const { data: tags = [] } = useQuery(discussionTagsQuery(workspace.id));
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            forceMinSize={false}
            data-slot="filter-chip"
            data-active={tag ? true : undefined}
            aria-label={tag ? `Tag: #${tag}` : "Tags"}
          >
            <TagsIcon />
            <span className="truncate">{tag ? `#${tag}` : "Tags"}</span>
            <DropDownArrow />
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="max-h-80 min-w-48 overflow-y-auto">
        <DropdownMenuGroup>
          {tags.length === 0 && (
            <DropdownMenuItem disabled>
              <span className="min-w-0 flex-1 truncate">No tags yet</span>
            </DropdownMenuItem>
          )}
          {tags.map((option) => (
            <DropdownMenuItem
              key={option.name}
              onClick={() => onFilter({ tag: option.name === tag ? undefined : option.name })}
            >
              <TagsIcon className="size-4" />
              <span className="min-w-0 flex-1 truncate">#{option.name}</span>
              {option.name === tag && <CheckIcon className="size-4" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function LayoutButton({
  layout,
  onLayout,
}: {
  layout: TForumLayout;
  onLayout: (layout: TForumLayout) => void;
}) {
  const next: TForumLayout = layout === "grid" ? "list" : "grid";
  const label = `Switch to ${next} view`;
  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        forceMinSize={false}
        data-slot="layout-button"
        aria-label={label}
        onClick={() => onLayout(next)}
      >
        {next === "list" ? <LayoutListIcon /> : <LayoutGridIcon />}
      </Button>
    </SimpleTooltip>
  );
}

function SortButtons({
  filters,
  onFilter,
}: {
  filters: TForumFilters;
  onFilter: (next: TForumFilters) => void;
}) {
  const sort: TForumSort = filters.sort ?? "activity";
  const current = forumSortOptions.find((option) => option.value === sort) ?? forumSortOptions[0];
  const CurrentIcon = sortIcons[sort];
  const isAscending = filters.order === "asc";
  const direction = isAscending ? "Ascending" : "Descending";
  return (
    <div role="group" aria-label="Sort" data-slot="split-button">
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" forceMinSize={false} aria-label={`Sort by ${current.label}`}>
              <CurrentIcon />
              <span className="truncate">{current.label}</span>
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="min-w-48">
          <DropdownMenuGroup>
            {forumSortOptions.map((option) => {
              const Icon = sortIcons[option.value];
              return (
                <DropdownMenuItem
                  key={option.value}
                  data-checked={option.value === sort || undefined}
                  onClick={() =>
                    onFilter({ sort: option.value === "activity" ? undefined : option.value })
                  }
                >
                  <Icon className="size-4" />
                  <span className="min-w-0 flex-1 truncate">{option.label}</span>
                </DropdownMenuItem>
              );
            })}
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
      <SimpleTooltip content={direction}>
        <Button
          variant="ghost"
          forceMinSize={false}
          aria-label={`${direction}, turn it round`}
          onClick={() => onFilter({ order: isAscending ? undefined : "asc" })}
        >
          {isAscending ? <ArrowUpIcon /> : <ArrowDownIcon />}
        </Button>
      </SimpleTooltip>
    </div>
  );
}

/** Material's arrow_drop_down, which no lucide icon stands for. */
function DropDownArrow() {
  return (
    <svg aria-hidden viewBox="0 -960 960 960" fill="currentColor" data-slot="drop-down-arrow">
      <path d="M459-381 314-526q-3-3-4.5-6.5T308-540q0-8 5.5-14t14.5-6h304q9 0 14.5 6t5.5 14q0 2-6 14L501-381q-5 5-10 7t-11 2q-6 0-11-2t-10-7Z" />
    </svg>
  );
}
