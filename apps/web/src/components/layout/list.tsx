import {
  type CSSProperties,
  Fragment,
  type HTMLAttributes,
  type ReactNode,
  type Ref,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { cn } from "@/components/ui/utils";
import { useListVirtualizer } from "@/lib/hooks/use-list-virtualizer";

type TListProps = {
  children: ReactNode;
  flush?: boolean;
  role?: "list" | "listbox" | "table";
  label?: string;
  ref?: Ref<HTMLDivElement>;
  style?: CSSProperties;
};

/** A card of rows, or with `flush` rows that run edge to edge across the page. */
export function List({ children, flush, role = "list", label, ref, style }: TListProps) {
  return (
    <div
      ref={ref}
      data-slot="list"
      data-flush={flush || undefined}
      role={role}
      aria-label={label}
      style={style}
      className={cn(
        "group/list flex flex-col",
        flush ? "-mx-4 w-auto" : "bg-card w-full overflow-hidden rounded-lg border",
      )}
    >
      {children}
    </div>
  );
}

const DEFAULT_ROW_HEIGHT = 44;
const END_REACHED_ROWS = 10;

type TVirtualListProps<T> = Omit<TListProps, "children" | "ref" | "style"> & {
  items: T[];
  getKey: (item: T) => string;
  /** Exactly one element per item, so rows stay the list's own children. */
  renderItem: (item: T) => ReactNode;
  estimateSize?: (item: T) => number;
  /** Called while the rendered rows reach the last few, to load the next page. */
  onEndReached?: () => void;
};

/** A List that renders only the rows near the screen; padding stands in for the rest. */
export function VirtualList<T>({
  items,
  getKey,
  renderItem,
  estimateSize,
  onEndReached,
  ...listProps
}: TVirtualListProps<T>) {
  const listRef = useRef<HTMLDivElement>(null);
  const { virtualizer, scrollMargin } = useListVirtualizer(listRef, {
    count: items.length,
    estimateSize: (index) => estimateSize?.(items[index]) ?? DEFAULT_ROW_HEIGHT,
    getItemKey: (index) => getKey(items[index]),
  });
  const virtualItems = virtualizer.getVirtualItems();
  const first = virtualItems[0];
  const last = virtualItems.at(-1);

  useLayoutEffect(() => {
    const rows = listRef.current?.children;
    if (!rows) return;
    virtualItems.forEach((item, position) => {
      const row = rows[position];
      if (!row) return;
      row.setAttribute("data-index", String(item.index));
      virtualizer.measureElement(row);
    });
  });

  const isNearEnd = Boolean(last && last.index >= items.length - END_REACHED_ROWS);
  useEffect(() => {
    if (isNearEnd) onEndReached?.();
  }, [isNearEnd, items.length, onEndReached]);

  return (
    <List
      {...listProps}
      ref={listRef}
      style={{
        paddingTop: first ? first.start - scrollMargin : 0,
        paddingBottom: last
          ? Math.max(0, virtualizer.getTotalSize() - (last.end - scrollMargin))
          : 0,
      }}
    >
      {virtualItems.map((item) => (
        <Fragment key={item.key}>{renderItem(items[item.index])}</Fragment>
      ))}
    </List>
  );
}

type TVirtualGridProps<T> = {
  items: T[];
  getKey: (item: T) => string;
  renderItem: (item: T) => ReactNode;
  label?: string;
  /** What lays out one row of cards, like the grid's classes; its columns decide the row length. */
  rowProps: HTMLAttributes<HTMLDivElement> & { [data: `data-${string}`]: string };
  estimateRowHeight: number;
  onEndReached?: () => void;
};

function chunk<T>(items: T[], size: number) {
  return Array.from({ length: Math.ceil(items.length / size) }, (_, row) =>
    items.slice(row * size, (row + 1) * size),
  );
}

/** Cards in a grid, rendered a row at a time near the screen. */
export function VirtualGrid<T>({
  items,
  getKey,
  renderItem,
  label,
  rowProps,
  estimateRowHeight,
  onEndReached,
}: TVirtualGridProps<T>) {
  const gridRef = useRef<HTMLDivElement>(null);
  const [columns, setColumns] = useState(1);
  const [rowGap, setRowGap] = useState(0);
  const rows = useMemo(() => chunk(items, columns), [items, columns]);
  const { virtualizer, scrollMargin } = useListVirtualizer(gridRef, {
    count: rows.length,
    estimateSize: () => estimateRowHeight,
    getItemKey: (index) => getKey(rows[index][0]),
    gap: rowGap,
  });
  const virtualItems = virtualizer.getVirtualItems();
  const first = virtualItems[0];
  const last = virtualItems.at(-1);
  const hasRows = rows.length > 0;

  useLayoutEffect(() => {
    const grid = gridRef.current;
    if (!grid || !hasRows) return;
    const measure = () => {
      const row = grid.firstElementChild;
      if (!row) return;
      const style = getComputedStyle(row);
      setColumns(Math.max(1, style.gridTemplateColumns.split(" ").length));
      setRowGap(Number.parseFloat(style.rowGap) || 0);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(grid);
    return () => observer.disconnect();
  }, [hasRows]);

  const isNearEnd = Boolean(last && last.index >= rows.length - END_REACHED_ROWS);
  useEffect(() => {
    if (isNearEnd) onEndReached?.();
  }, [isNearEnd, items.length, onEndReached]);

  return (
    <div
      ref={gridRef}
      role="list"
      aria-label={label}
      className="flex w-full flex-col"
      style={{
        rowGap,
        paddingTop: first ? first.start - scrollMargin : 0,
        paddingBottom: last
          ? Math.max(0, virtualizer.getTotalSize() - (last.end - scrollMargin))
          : 0,
      }}
    >
      {virtualItems.map((item) => (
        <div
          key={item.key}
          ref={virtualizer.measureElement}
          data-index={item.index}
          role="presentation"
          {...rowProps}
        >
          {rows[item.index].map((card) => (
            <Fragment key={getKey(card)}>{renderItem(card)}</Fragment>
          ))}
        </div>
      ))}
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
      <div className="dank:gap-3 flex min-w-0 flex-1 items-center gap-2">{children}</div>
      {actions && <div className="dank:gap-3 flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}

export function ListRowText({ primary, secondary }: { primary: ReactNode; secondary?: ReactNode }) {
  return (
    <div className="dank:flex-col dank:items-stretch dank:gap-0 flex min-w-0 flex-1 items-baseline gap-2">
      <span className="truncate">{primary}</span>
      {secondary && (
        <span data-slot="list-supporting" className="text-muted-foreground truncate text-xs">
          {secondary}
        </span>
      )}
    </div>
  );
}
