import { createContext, ReactNode, useContext, useState } from "react";

import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = {
  width?: "full" | "narrow";
  children: ReactNode;
};

type TPageSlots = { bar: HTMLElement | null; fab: HTMLElement | null };

/** Where the page bar and the floating create button land, when the theme places them apart. */
export const PageSlotsContext = createContext<TPageSlots>({ bar: null, fab: null });

export const usePageSlots = () => useContext(PageSlotsContext);

export function PageContainer({ width = "full", children }: TProps) {
  const { separatesBars, createButton } = useUiTheme();
  const [bar, setBar] = useState<HTMLElement | null>(null);
  const [fab, setFab] = useState<HTMLElement | null>(null);
  const isBarApart = separatesBars && width === "full";

  const page = (
    <div
      data-slot="page-container"
      data-width={width}
      data-bar={isBarApart ? "apart" : undefined}
      className={cn(
        "group/page flex min-h-0 w-full flex-1 flex-col overflow-y-auto",
        isBarApart && "bg-background rounded-lg",
      )}
    >
      <div
        className={cn(
          "dank:animate-in dank:fade-in-0 dank:slide-in-from-bottom-2 dank:ease-emphasized dank:duration-300 dank:motion-reduce:animate-none mx-auto flex w-full flex-1 flex-col gap-4 px-4 pt-4 pb-8",
          width === "narrow" ? "max-w-3xl sm:px-6 sm:pt-10" : "max-w-none",
        )}
      >
        {children}
        {createButton === "fab" && (
          <div
            ref={setFab}
            className="pointer-events-none sticky bottom-4 z-20 mt-auto -mb-4 flex justify-end *:pointer-events-auto empty:hidden"
          />
        )}
      </div>
    </div>
  );

  return (
    <PageSlotsContext value={{ bar: isBarApart ? bar : null, fab }}>
      {isBarApart ? (
        <div className="bg-sidebar flex min-h-0 w-full flex-1 flex-col gap-2">
          <div
            ref={setBar}
            data-slot="page-bar-slot"
            className="bg-background shrink-0 overflow-hidden rounded-lg empty:hidden"
          />
          {page}
        </div>
      ) : (
        page
      )}
    </PageSlotsContext>
  );
}
