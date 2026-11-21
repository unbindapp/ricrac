import { ReactNode } from "react";

/** The bar across the top of a panel beside the page. */
export function PanelBar({ children }: { children: ReactNode }) {
  return (
    <div
      data-slot="panel-bar"
      className="bg-background dux:h-16 dux:rounded-lg dux:border-0 dux:pr-1 dux:pl-4 dux:has-[>button:first-child]:pl-1 flex h-12 shrink-0 items-center gap-2 border-b px-3"
    >
      {children}
    </div>
  );
}
