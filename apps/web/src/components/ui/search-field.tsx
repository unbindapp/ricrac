import type { ComponentPropsWithRef } from "react";

import { SearchIcon } from "@/components/icons";
import { Input } from "@/components/ui/input";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = Omit<ComponentPropsWithRef<"input">, "className"> & {
  className?: string;
  /** Leads with a search icon in the default theme; Material's search bar always does. */
  icon?: boolean;
};

/** A field that filters what a page lists. Dank draws it as Material's search bar. */
export function SearchField({ className, icon = true, ...props }: TProps) {
  const isDank = useUiTheme().id === "dank";
  if (isDank) {
    return (
      <div className={cn("relative w-90 max-w-full shrink-0", className)}>
        <SearchIcon className="text-m3-on-surface pointer-events-none absolute top-1/2 left-4 size-6 -translate-y-1/2" />
        <input
          type="text"
          className="bg-m3-surface-container-high text-m3-on-surface placeholder:text-m3-on-surface-variant caret-m3-primary h-14 w-full rounded-full pr-4 pl-14 text-base leading-6 outline-none"
          {...props}
        />
      </div>
    );
  }
  if (!icon) return <Input className={cn("h-8 max-w-xs", className)} {...props} />;
  return (
    <div className={cn("relative w-full max-w-xs", className)}>
      <SearchIcon className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
      <Input className="h-8 pl-8" {...props} />
    </div>
  );
}
