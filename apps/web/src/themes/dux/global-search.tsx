import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { type FocusEvent, useRef, useState } from "react";

import { SearchIcon, TriangleAlertIcon, XIcon } from "@/components/icons";
import { SearchHitGroups } from "@/components/search/search-hit-groups";
import { useSearch } from "@/components/search/use-search";
import { Button } from "@/components/ui/button";
import { Command, CommandEmpty, CommandInput, CommandList } from "@/components/ui/command";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useWorkspace } from "@/components/workspace/workspace-provider";

/** A search button that opens into a field, listing what matches in the workspace under it. */
export function GlobalSearch() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement>(null);
  const toggleRef = useRef<HTMLButtonElement>(null);
  const [query, setQuery] = useState("");
  const [isExpanded, setIsExpanded] = useState(false);
  const [isListOpen, setIsListOpen] = useState(false);
  const search = useSearch(query);
  const isListShown = isExpanded && isListOpen && Boolean(query.trim());

  const expand = () => {
    setIsExpanded(true);
    inputRef.current?.focus();
  };
  const clear = () => {
    setQuery("");
    inputRef.current?.focus();
  };
  const collapse = () => {
    setQuery("");
    setIsListOpen(false);
    setIsExpanded(false);
    toggleRef.current?.focus();
  };
  const open = (link: LinkProps) => {
    setQuery("");
    setIsListOpen(false);
    setIsExpanded(false);
    inputRef.current?.blur();
    void navigate(link);
  };
  const collapseWhenLeft = (event: FocusEvent<HTMLDivElement>) => {
    if (event.currentTarget.contains(event.relatedTarget)) return;
    setIsListOpen(false);
    if (!query) setIsExpanded(false);
  };

  return (
    <Command
      shouldFilter={false}
      loop
      wrapper="none"
      role="search"
      data-slot="dux-search"
      data-expanded={isExpanded || undefined}
      onBlur={collapseWhenLeft}
      onKeyDown={(event) => {
        if (event.key === "Escape") collapse();
      }}
    >
      <div data-slot="dux-search-field">
        <Button
          ref={toggleRef}
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label="Search"
          aria-expanded={isExpanded}
          onClick={expand}
        >
          <SearchIcon />
        </Button>
        <CommandInput
          ref={inputRef}
          value={query}
          hideIcon
          tabIndex={isExpanded ? 0 : -1}
          onValueChange={(value) => {
            setQuery(value);
            setIsListOpen(true);
          }}
          onFocus={() => {
            setIsExpanded(true);
            setIsListOpen(true);
          }}
          showSpinner={false}
          aria-label="Search"
          placeholder={`Search ${workspace.name}`}
          classNameWrapper="contents"
        />
        {query && (
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Clear search"
            onMouseDown={(event) => event.preventDefault()}
            onClick={clear}
          >
            <XIcon />
          </Button>
        )}
      </div>
      {isListShown && (
        <div data-slot="dux-search-results" onMouseDown={(event) => event.preventDefault()}>
          <ScrollArea className="min-h-0">
            <CommandList className="p-2">
              <CommandEmpty>{search.isPending ? "Searching…" : "Nothing found"}</CommandEmpty>
              {search.error && (
                <div
                  role="alert"
                  className="text-destructive flex items-center gap-2 px-3 py-2 text-sm"
                >
                  <TriangleAlertIcon className="size-4" />
                  Search failed: {search.error.message}
                </div>
              )}
              <SearchHitGroups hits={search.hits} query={query} onOpen={open} />
            </CommandList>
          </ScrollArea>
        </div>
      )}
    </Command>
  );
}
