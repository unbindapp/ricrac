import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";

import { CheckIcon } from "@/components/icons";
import { cn } from "@/components/ui/utils";

function Checkbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        "peer bg-border ring-foreground/6-10 focus-visible:ring-foreground/8-10 aria-invalid:ring-destructive/8-10 focus-visible:ring-offset-background dank:border-muted-foreground dank:data-checked:border-primary dank:aria-invalid:border-destructive dank:focus-visible:ring-primary dank:size-4 dank:rounded-xs dank:border-2 dank:bg-transparent dank:ring-0 dank:transition-colors dank:focus-visible:ring-2 size-3.75 shrink-0 rounded-sm ring-1 outline-none focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="data-checked:ring-foreground data-checked:bg-primary data-checked:text-primary-foreground dank:rounded-none dank:ring-0 flex size-full items-center justify-center rounded-sm p-px text-current ring-1 ring-transparent transition-none"
      >
        <CheckIcon className="size-full" strokeWidth={4} />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export { Checkbox };
