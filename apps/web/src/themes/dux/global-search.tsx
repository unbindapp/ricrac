import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { TriangleAlertIcon, XIcon } from "@/components/icons";
import { SearchHitGroups } from "@/components/search/search-hit-groups";
import { useSearch } from "@/components/search/use-search";
import { Button } from "@/components/ui/button";
import { Command, CommandEmpty, CommandInput, CommandList } from "@/components/ui/command";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useWorkspace } from "@/components/workspace/workspace-provider";

/** Searches the whole workspace from a bar that docks what matches under it while you type. */
export function GlobalSearch() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const search = useSearch(query);
  const isListShown = isOpen && Boolean(query.trim());

  const close = () => {
    setIsOpen(false);
    inputRef.current?.blur();
  };
  const open = (link: LinkProps) => {
    setQuery("");
    close();
    void navigate(link);
  };

  return (
    <Command
      shouldFilter={false}
      loop
      variant="searchView"
      wrapper="none"
      data-slot="global-search"
      data-open={isListShown || undefined}
      className="h-auto"
      onKeyDown={(event) => {
        if (event.key === "Escape") close();
      }}
    >
      <div data-slot="search-bar" className="flex h-14 shrink-0 items-center pr-1">
        <CommandInput
          ref={inputRef}
          value={query}
          onValueChange={(value) => {
            setQuery(value);
            setIsOpen(true);
          }}
          onFocus={() => setIsOpen(true)}
          onBlur={() => setIsOpen(false)}
          showSpinner={search.isPending}
          aria-label="Search"
          placeholder={`Search ${workspace.name}`}
          classNameWrapper="min-w-0 flex-1 border-0 [&>div]:left-5 [&>div]:size-6 [&>div]:text-m3-on-surface-variant [&_svg]:text-m3-on-surface-variant"
          className="text-m3-on-surface placeholder:text-m3-on-surface-variant h-14 pr-2 pl-14 text-base leading-6"
        />
        {query && (
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Clear search"
            className="text-m3-on-surface size-12 shrink-0 rounded-full active:rounded-full"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              setQuery("");
              inputRef.current?.focus();
            }}
          >
            <XIcon />
          </Button>
        )}
      </div>
      {isListShown && (
        <div
          className="border-m3-outline flex max-h-120 min-h-60 flex-col border-t"
          onMouseDown={(event) => event.preventDefault()}
        >
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
