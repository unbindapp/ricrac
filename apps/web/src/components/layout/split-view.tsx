import { ReactNode } from "react";

import { cn } from "@/components/ui/utils";

type TProps = {
  main: ReactNode;
  aside?: ReactNode;
};

/** Two panes side by side, half the width each, apart on the shell; the aside only on wide screens. */
export function SplitView({ main, aside }: TProps) {
  return (
    <div data-slot="split-view" className="flex min-h-0 w-full flex-1 gap-2">
      <SplitPane>{main}</SplitPane>
      {aside && <SplitPane className="hidden lg:flex">{aside}</SplitPane>}
    </div>
  );
}

function SplitPane({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div
      data-slot="split-pane"
      className={cn(
        "bg-background flex min-h-0 min-w-0 flex-1 basis-0 flex-col overflow-hidden md:rounded-lg",
        className,
      )}
    >
      {children}
    </div>
  );
}
