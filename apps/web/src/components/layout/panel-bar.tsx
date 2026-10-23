import { ReactNode } from "react";

/** The bar across the top of a panel beside the page. */
export function PanelBar({ children }: { children: ReactNode }) {
  return (
    <div
      data-slot="panel-bar"
      className="bg-background dank:h-16 dank:rounded-lg dank:border-0 dank:pr-1 dank:pl-4 dank:has-[>button:first-child]:pl-1 flex h-12 shrink-0 items-center gap-2 border-b px-3"
    >
      {children}
    </div>
  );
}
