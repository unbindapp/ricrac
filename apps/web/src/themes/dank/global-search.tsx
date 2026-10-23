import { type LinkProps, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useRef, useState } from "react";

import { TriangleAlertIcon } from "@/components/icons";
import { SearchHitGroups } from "@/components/search/search-hit-groups";
import { useSearch } from "@/components/search/use-search";
import { Command, CommandEmpty, CommandInput, CommandList } from "@/components/ui/command";
import { ScrollArea } from "@/components/ui/scroll-area";

/** Searches the whole workspace from a bar, listing what matches under it as you type. */
export function GlobalSearch({ trailing }: { trailing?: ReactNode }) {
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
      data-open={isListShown || undefined}
      className="group/search data-open:shadow-elevation-3 relative w-full max-w-180 min-w-90 overflow-visible rounded-[1.75rem] bg-transparent"
      onKeyDown={(event) => {
        if (event.key === "Escape") close();
      }}
    >
      <div
        data-slot="search-bar"
        className="bg-m3-surface-container-high flex h-14 items-center rounded-[1.75rem] pr-3 group-data-open/search:rounded-b-none"
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
          classNameWrapper="min-w-0 flex-1 border-0 [&>div]:left-4 [&>div]:size-6 [&>div]:text-m3-on-surface [&_svg]:text-m3-on-surface"
          className="text-m3-on-surface placeholder:text-m3-on-surface-variant h-14 pr-4 pl-14 text-base leading-6"
        />
        {trailing}
      </div>
      {isListShown && (
        <div
          className="bg-m3-surface-container-high text-m3-on-surface shadow-elevation-3 border-m3-outline absolute inset-x-0 top-full z-50 flex max-h-120 min-h-60 flex-col overflow-hidden rounded-b-[1.75rem] border-t"
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
