import { type ComponentProps } from "react";

import { CheckIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = Omit<ComponentProps<typeof Button>, "variant" | "size"> & { selected: boolean };

/** One choice of a few, toggled in place: a filter chip in Dux. */
export function FilterChip({ selected, className, children, ...props }: TProps) {
  const isDux = useUiTheme().id === "dux";
  return (
    <Button
      variant={selected ? "secondary" : "ghost"}
      size="sm"
      className={cn(
        "dux:h-8 dux:gap-2 dux:rounded-sm dux:px-4 dux:active:rounded-sm dux:has-[>svg]:pl-2 dux:[&>svg]:size-4.5",
        !selected && "dux:border-m3-outline-variant dux:text-m3-on-surface-variant dux:border",
        className,
      )}
      {...props}
    >
      {selected && isDux && <CheckIcon />}
      {children}
    </Button>
  );
}
