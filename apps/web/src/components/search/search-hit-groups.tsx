import type { LinkProps } from "@tanstack/react-router";

import {
  searchHitContext,
  searchHitHeadings,
  searchHitLink,
  SearchHitIcon,
  Snippet,
} from "@/components/search/search-hits";
import { CommandGroup, CommandItem } from "@/components/ui/command";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import type { TSearchHit } from "@/lib/queries/search";

const HIT_ORDER: TSearchHit["type"][] = [
  "issue",
  "project",
  "initiative",
  "document",
  "discussion",
  "comment",
];

export function SearchHitGroups({
  hits,
  query,
  onOpen,
}: {
  hits: TSearchHit[];
  query: string;
  onOpen: (link: LinkProps) => void;
}) {
  const { workspace } = useWorkspace();
  return HIT_ORDER.map((type) => {
    const ofType = hits.filter((hit) => hit.type === type);
    if (ofType.length === 0) return null;
    return (
      <CommandGroup key={type} heading={searchHitHeadings[type]}>
        {ofType.map((hit) => {
          const context = searchHitContext(hit);
          return (
            <CommandItem
              key={`${hit.type}-${hit.id}`}
              value={`search-${hit.type}-${hit.id}`}
              keywords={[query]}
              onSelect={() => onOpen(searchHitLink(hit, workspace.urlKey))}
            >
              <SearchHitIcon hit={hit} />
              {(hit.type === "issue" || hit.type === "comment") && (
                <span className="text-muted-foreground shrink-0 font-mono text-xs">
                  {hit.identifier}
                </span>
              )}
              <span className="min-w-0 truncate">
                {hit.type === "comment" ? <Snippet text={hit.snippet} /> : hit.title}
              </span>
              {context && (
                <span className="text-muted-foreground ml-auto shrink-0 pl-2 text-xs">
                  {context}
                </span>
              )}
            </CommandItem>
          );
        })}
      </CommandGroup>
    );
  });
}
