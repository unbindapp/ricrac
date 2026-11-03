import type { ReactNode } from "react";

import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { PageBar } from "@/components/layout/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/components/ui/utils";

const BODY_LINES = ["w-full", "w-11/12", "w-4/5", "w-2/3"];

type TProps = {
  /** The crumbs above the thing, known before it loads; its own crumb pulses in their place. */
  breadcrumb: TBreadcrumbItem[];
  /** What sits under the bar regardless of the data, like a page's tabs. */
  children?: ReactNode;
};

/** A page's title while it loads: the title when it's known already, a block its size otherwise. */
export function PlaceholderTitle({ title }: { title?: string }) {
  return (
    <h1
      data-placeholder={title ? undefined : true}
      className="data-placeholder:animate-skeleton data-placeholder:bg-foreground w-fit text-2xl leading-tight font-semibold data-placeholder:rounded-md data-placeholder:text-transparent"
    >
      {title ?? "Loading the title"}
    </h1>
  );
}

/** A detail page while it loads: its bar and crumbs as they will be, its title and text to come. */
export function DetailPlaceholder({ breadcrumb, children }: TProps) {
  return (
    <>
      <div className="flex w-full flex-col gap-2">
        <PageBar>
          <div className="min-w-0 flex-1">
            <Breadcrumb items={[...breadcrumb, { label: "Loading title", isPlaceholder: true }]} />
          </div>
        </PageBar>
        {children}
      </div>
      <div aria-busy aria-label="Loading" className="flex w-full flex-col gap-2">
        <PlaceholderTitle />
        <div className="flex flex-col gap-2.5 py-1.5">
          {BODY_LINES.map((width) => (
            <Skeleton key={width} className={cn("h-3.5", width)} />
          ))}
        </div>
      </div>
    </>
  );
}
