import { ReactNode } from "react";

import { cn } from "@/components/ui/utils";

/** A card of rows, or with `flush` rows that run edge to edge across the page. */
export function List({ children, flush }: { children: ReactNode; flush?: boolean }) {
  return (
    <div
      data-slot="list"
      data-flush={flush || undefined}
      role="list"
      className={cn(
        "group/list flex flex-col",
        flush ? "-mx-4 w-auto" : "bg-card w-full overflow-hidden rounded-lg border",
      )}
    >
      {children}
    </div>
  );
}

type TListRowProps = {
  children: ReactNode;
  actions?: ReactNode;
  muted?: boolean;
};

export function ListRow({ children, actions, muted }: TListRowProps) {
  return (
    <div
      role="listitem"
      data-slot="list-row"
      className={cn(
        "flex h-9 w-full items-center gap-2 border-b px-3 text-sm group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0",
        muted && "text-muted-foreground",
      )}
    >
      <div className="flex min-w-0 flex-1 items-center gap-2">{children}</div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}

export function ListRowText({ primary, secondary }: { primary: ReactNode; secondary?: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-1 items-baseline gap-2">
      <span className="truncate">{primary}</span>
      {secondary && <span className="text-muted-foreground truncate text-xs">{secondary}</span>}
    </div>
  );
}
