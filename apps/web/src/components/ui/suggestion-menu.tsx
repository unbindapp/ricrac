import * as React from "react";
import { createPortal } from "react-dom";

import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";

export type TSuggestionItem = {
  id: string;
  label: string;
  description?: string;
  icon?: React.ReactNode;
};

type TProps = {
  items: TSuggestionItem[];
  selectedIndex: number;
  rect: DOMRect | null;
  onSelect: (item: TSuggestionItem) => void;
  emptyText?: string;
};

const MENU_HEIGHT = 288;
const GAP = 4;

function SuggestionMenu({
  items,
  selectedIndex,
  rect,
  onSelect,
  emptyText = "No matches",
}: TProps) {
  const listRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    const selected = listRef.current?.querySelector("[aria-selected=true]");
    selected?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex, items]);
  if (!rect) return null;
  const opensUp = rect.bottom + GAP + MENU_HEIGHT > window.innerHeight && rect.top > MENU_HEIGHT;
  const position = opensUp
    ? { bottom: window.innerHeight - rect.top + GAP, left: rect.left }
    : { top: rect.bottom + GAP, left: rect.left };
  return createPortal(
    <div
      ref={listRef}
      role="listbox"
      data-slot="suggestion-menu"
      className="bg-popover text-popover-foreground shadow-shadow-color/shadow-opacity fixed z-[1100] flex max-h-72 w-64 flex-col overflow-y-auto rounded-lg border p-1 shadow-lg"
      style={{ ...position, left: Math.min(position.left, window.innerWidth - 264) }}
    >
      {items.length === 0 && (
        <p className="text-muted-foreground px-2 py-1.5 text-sm">{emptyText}</p>
      )}
      {items.map((item, index) => (
        <Button
          key={item.id}
          type="button"
          role="option"
          aria-selected={index === selectedIndex}
          variant="ghost"
          size="sm"
          forceMinSize={false}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => onSelect(item)}
          className={cn(
            "w-full shrink-0 justify-start font-normal",
            index === selectedIndex && "bg-accent text-accent-foreground",
          )}
        >
          {item.icon}
          <span className="truncate">{item.label}</span>
          {item.description && (
            <span className="text-muted-foreground ml-auto truncate text-xs">
              {item.description}
            </span>
          )}
        </Button>
      ))}
    </div>,
    document.body,
  );
}

export { SuggestionMenu };
