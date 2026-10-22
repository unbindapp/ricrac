import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { TriangleAlertIcon } from "@/components/icons";
import { SearchHitGroups } from "@/components/search/search-hit-groups";
import { useSearch } from "@/components/search/use-search";
import { Command, CommandEmpty, CommandInput, CommandList } from "@/components/ui/command";
import { ScrollArea } from "@/components/ui/scroll-area";

/** Searches the whole workspace from a bar, listing what matches under it as you type. */
export function GlobalSearch() {
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
      wrapper="none"
      className="relative w-full max-w-2xl overflow-visible bg-transparent"
      onKeyDown={(event) => {
        if (event.key === "Escape") close();
      }}
    >
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
        placeholder="Search issues, projects, documents, discussions…"
        classNameWrapper="bg-surface-container-highest has-hover:hover:bg-surface-container-highest/80 h-12 rounded-full border-0 transition-colors [&>div]:left-5 [&>div]:size-5"
        className="h-12 rounded-full pr-5 pl-13 text-base"
      />
      {isListShown && (
        <div
          className="bg-popover text-popover-foreground shadow-shadow-color/shadow-opacity absolute inset-x-0 top-full z-50 mt-2 flex max-h-120 flex-col overflow-hidden rounded-lg shadow-xl"
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
