import { ReactNode } from "react";

import { cn } from "@/components/ui/utils";

type TProps = {
  width?: "full" | "narrow";
  children: ReactNode;
};

export function PageContainer({ width = "full", children }: TProps) {
  return (
    <div
      data-slot="page-container"
      data-width={width}
      className="group/page flex min-h-0 w-full flex-1 flex-col overflow-y-auto"
    >
      <div
        className={cn(
          "mx-auto flex w-full flex-1 flex-col gap-4 px-4 pt-4 pb-8",
          width === "narrow" ? "max-w-3xl pt-10 sm:px-6" : "max-w-none",
        )}
      >
        {children}
      </div>
    </div>
  );
}
