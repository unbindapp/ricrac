import { List } from "@/components/layout/list";
import { Skeleton } from "@/components/ui/skeleton";

const widths = ["w-1/2", "w-2/3", "w-1/3", "w-3/5", "w-2/5", "w-1/2", "w-3/4", "w-1/4"];

/** Rows while a list loads; `flush` for lists whose rows run edge to edge, as they will. */
export function ListSkeleton({ rows = 6, flush }: { rows?: number; flush?: boolean }) {
  if (flush) {
    return (
      <div aria-busy aria-label="Loading" className="flex w-full flex-col">
        <List flush>
          {Array.from({ length: rows }, (_, index) => (
            <div
              key={index}
              aria-hidden
              data-slot="list-item"
              className="flex h-11 w-full items-center gap-3 px-3"
            >
              <Skeleton className="size-4 rounded-full" />
              <Skeleton className={`h-3.5 ${widths[index % widths.length]}`} />
            </div>
          ))}
        </List>
      </div>
    );
  }
  return (
    <div
      data-slot="list-skeleton"
      aria-busy
      aria-label="Loading"
      className="dux:gap-0.5 dux:border-0 flex w-full flex-col overflow-hidden rounded-lg border"
    >
      {Array.from({ length: rows }, (_, index) => (
        <div
          key={index}
          className="dux:h-14 dux:gap-3 dux:rounded-xs dux:border-0 dux:bg-card dux:px-4 dux:first:rounded-t-lg dux:last:rounded-b-lg flex h-9 items-center gap-2 border-b px-3 last:border-b-0"
        >
          <Skeleton className="dux:size-6 size-4 rounded-full" />
          <Skeleton className={`dux:h-4 h-3 ${widths[index % widths.length]}`} />
        </div>
      ))}
    </div>
  );
}
