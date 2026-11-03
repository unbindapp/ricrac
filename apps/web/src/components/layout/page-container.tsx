import { createContext, ReactNode, useContext, useState } from "react";

import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = {
  width?: "full" | "narrow";
  /** "window" lets phones scroll the whole document, for lists virtualized against the window. */
  scroll?: "pane" | "window";
  children: ReactNode;
};

type TPageSlots = { bar: HTMLElement | null; fab: HTMLElement | null };

/** Where the page bar and the floating create button land, when the theme places them apart. */
export const PageSlotsContext = createContext<TPageSlots>({ bar: null, fab: null });

export const usePageSlots = () => useContext(PageSlotsContext);

export function PageContainer({ width = "full", scroll = "pane", children }: TProps) {
  const { separatesBars, createButton } = useUiTheme();
  const [bar, setBar] = useState<HTMLElement | null>(null);
  const [fab, setFab] = useState<HTMLElement | null>(null);
  const isBarApart = separatesBars && width === "full";

  const page = (
    <div
      data-slot="page-container"
      data-width={width}
      data-scroll={scroll}
      data-bar={isBarApart ? "apart" : undefined}
      className={cn(
        "group/page flex min-h-0 w-full flex-1 flex-col",
        scroll === "window" ? "md:overflow-y-auto" : "overflow-y-auto",
        isBarApart && "bg-background rounded-lg",
      )}
    >
      <div
        className={cn(
          "dank:animate-in dank:fade-in-0 dank:slide-in-from-bottom-2 dank:ease-spring-default-spatial dank:duration-(--spring-default-spatial-duration) dank:motion-reduce:animate-none mx-auto flex w-full flex-1 flex-col gap-4 pb-8",
          "px-(--page-px) pt-(--page-pt) [--page-pt:--spacing(4)] [--page-px:--spacing(4)]",
          width === "narrow" &&
            "max-w-3xl sm:[--page-pt:--spacing(10)] sm:[--page-px:--spacing(6)]",
          width === "full" &&
            separatesBars &&
            "max-w-none md:[--page-pt:--spacing(8)] md:[--page-px:--spacing(10)]",
          width === "full" &&
            !separatesBars &&
            "max-w-none md:[--page-pt:--spacing(2)] md:[--page-px:--spacing(2)]",
        )}
      >
        {children}
        {createButton === "fab" && (
          <div
            ref={setFab}
            className="dank:md:bottom-6 dank:md:-mr-4 dank:md:-mb-2 pointer-events-none sticky bottom-4 z-20 mt-auto -mb-4 flex justify-end *:pointer-events-auto empty:hidden"
          />
        )}
      </div>
    </div>
  );

  return (
    <PageSlotsContext value={{ bar: isBarApart ? bar : null, fab }}>
      {isBarApart ? (
        <div className="bg-sidebar flex min-h-0 w-full flex-1 flex-col gap-3">
          <div
            ref={setBar}
            data-slot="page-bar-slot"
            data-sticky-bar
            className="bg-background shrink-0 overflow-hidden rounded-lg empty:hidden max-md:sticky max-md:top-0 max-md:z-10"
          />
          {page}
        </div>
      ) : (
        page
      )}
    </PageSlotsContext>
  );
}
