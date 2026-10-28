import { useQuery } from "@tanstack/react-query";

import {
  forumSortOptions,
  forumStatusSearch,
  forumViewOptions,
  type TForumFilters,
  type TForumSort,
  type TForumView,
} from "@/components/forum/forum-views";
import { NONE_KEY } from "@/components/issues/grouping";
import { ChevronDownIcon, SlidersHorizontalIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { discussionTagsQuery } from "@/lib/queries/discussions";

export type TForumLayout = "grid" | "list";

type TProps = {
  filters: TForumFilters;
  layout: TForumLayout;
  onFilter: (next: TForumFilters) => void;
  onLayout: (layout: TForumLayout) => void;
};

/** One button for what the forum shows and how: status, tag and layout. */
export function ForumControls({ filters, layout, onFilter, onLayout }: TProps) {
  const { workspace } = useWorkspace();
  const { data: tags = [] } = useQuery(discussionTagsQuery(workspace.id));
  const status: TForumView = filters.status ?? "open";
  const sort: TForumSort = filters.sort ?? "activity";
  const sortLabel = forumSortOptions.find((option) => option.value === sort)?.label;
  const statusLabel = forumViewOptions.find((option) => option.value === status)?.label;
  const summary = [statusLabel, filters.tag && `#${filters.tag}`, status !== "trash" && sortLabel]
    .filter(Boolean)
    .join(" · ");

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="tonal"
            forceMinSize={false}
            data-slot="forum-controls"
            aria-label={`Filter and layout: ${summary}`}
          >
            <SlidersHorizontalIcon />
            <span className="truncate">{summary}</span>
            <ChevronDownIcon />
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="w-64">
        {status !== "trash" && (
          <>
            <DropdownMenuGroup>
              <DropdownMenuLabel>Sort by</DropdownMenuLabel>
              <DropdownMenuRadioGroup
                value={sort}
                onValueChange={(value: TForumSort) =>
                  onFilter({ sort: value === "activity" ? undefined : value })
                }
              >
                {forumSortOptions.map((option) => (
                  <DropdownMenuRadioItem key={option.value} value={option.value}>
                    {option.label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
          </>
        )}
        <DropdownMenuGroup>
          <DropdownMenuLabel>Show</DropdownMenuLabel>
          <DropdownMenuRadioGroup
            value={status}
            onValueChange={(value: TForumView) => onFilter({ status: forumStatusSearch(value) })}
          >
            {forumViewOptions.map((option) => (
              <DropdownMenuRadioItem key={option.value} value={option.value}>
                {option.label}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>
            <span className="min-w-0 flex-1 truncate">
              Tag: {filters.tag ? `#${filters.tag}` : "All tags"}
            </span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="max-h-80 w-56 overflow-y-auto">
            <DropdownMenuRadioGroup
              value={filters.tag ?? NONE_KEY}
              onValueChange={(value: string) =>
                onFilter({ tag: value === NONE_KEY ? undefined : value })
              }
            >
              <DropdownMenuRadioItem value={NONE_KEY}>All tags</DropdownMenuRadioItem>
              {tags.map((tag) => (
                <DropdownMenuRadioItem key={tag.name} value={tag.name}>
                  <span className="min-w-0 flex-1 truncate">#{tag.name}</span>
                  <span className="text-muted-foreground text-xs tabular-nums">{tag.count}</span>
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        {status !== "trash" && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuLabel>Layout</DropdownMenuLabel>
              <DropdownMenuRadioGroup
                value={layout}
                onValueChange={(value: TForumLayout) => onLayout(value)}
              >
                <DropdownMenuRadioItem value="list">List</DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="grid">Grid</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
