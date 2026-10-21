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
          "animate-in fade-in-0 slide-in-from-bottom-2 ease-emphasized mx-auto flex w-full flex-1 flex-col gap-4 px-4 pt-4 pb-8 duration-300 motion-reduce:animate-none",
          width === "narrow" ? "max-w-3xl sm:px-6 sm:pt-10" : "max-w-none",
        )}
      >
        {children}
      </div>
    </div>
  );
}
