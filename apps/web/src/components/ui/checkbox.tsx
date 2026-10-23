import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";

import { CheckIcon } from "@/components/icons";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

function Checkbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  const isDank = useUiTheme().id === "dank";
  if (isDank) return <DankCheckbox className={className} {...props} />;
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        "peer bg-border ring-foreground/6-10 focus-visible:ring-foreground/8-10 aria-invalid:ring-destructive/8-10 focus-visible:ring-offset-background size-3.75 shrink-0 rounded-sm ring-1 outline-none focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="data-checked:ring-foreground data-checked:bg-primary data-checked:text-primary-foreground flex size-full items-center justify-center rounded-sm p-px text-current ring-1 ring-transparent transition-none"
      >
        <CheckIcon className="size-full" strokeWidth={4} />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

// Box, check color and state layer come from [data-slot="checkbox"] in themes/dank/inputs.css
function DankCheckbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      data-focus-indicator="child"
      className={cn("peer cursor-pointer data-disabled:cursor-not-allowed", className)}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="absolute -inset-0.5">
        <svg viewBox="0 0 18 18" fill="none" aria-hidden className="size-full">
          <path
            data-slot="checkbox-mark"
            d="M4.5 9l2.7 2.7 6.3-6.3"
            pathLength={1}
            stroke="var(--checkbox-icon)"
            strokeWidth={2}
            strokeLinecap="square"
          />
        </svg>
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export { Checkbox };
