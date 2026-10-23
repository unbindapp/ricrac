import { type ComponentProps } from "react";

import { CheckIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = Omit<ComponentProps<typeof Button>, "variant" | "size"> & { selected: boolean };

/** One choice of a few, toggled in place: a filter chip in Dank. */
export function FilterChip({ selected, className, children, ...props }: TProps) {
  const isDank = useUiTheme().id === "dank";
  return (
    <Button
      variant={selected ? "secondary" : "ghost"}
      size="sm"
      className={cn(
        "dank:h-8 dank:gap-2 dank:rounded-sm dank:px-4 dank:active:rounded-sm dank:has-[>svg]:pl-2 dank:[&>svg]:size-4.5",
        !selected && "dank:border-m3-outline-variant dank:text-m3-on-surface-variant dank:border",
        className,
      )}
      {...props}
    >
      {selected && isDank && <CheckIcon />}
      {children}
    </Button>
  );
}
