import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";

import { cn } from "@/components/ui/utils";

const dankChip =
  "dank:h-8 dank:gap-2 dank:rounded-sm dank:px-4 dank:type-label-large dank:has-[>svg]:pl-2 dank:[&_svg]:size-4.5";

const dankOutlinedChip = `${dankChip} dank:border-m3-outline-variant dank:bg-transparent`;

const badgeVariants = cva(
  "inline-flex h-5 max-w-full shrink-0 items-center gap-1 rounded-full border px-2 text-xs leading-none font-medium whitespace-nowrap [&_svg]:size-3.5 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: `bg-primary text-primary-foreground border-transparent ${dankChip} dank:bg-m3-secondary-container dank:text-m3-on-secondary-container`,
        outline: `text-foreground bg-background ${dankOutlinedChip} dank:text-m3-on-surface`,
        muted: `bg-muted text-muted-foreground border-transparent ${dankOutlinedChip} dank:text-m3-on-surface-variant`,
        destructive: `text-destructive bg-destructive/3-10 border-destructive/5-10 ${dankOutlinedChip}`,
        warning: `text-warning bg-warning/3-10 border-warning/5-10 ${dankOutlinedChip}`,
        success: `text-success bg-success/3-10 border-success/5-10 ${dankOutlinedChip}`,
        process: `text-process bg-process/3-10 border-process/5-10 ${dankOutlinedChip}`,
        count:
          "bg-muted text-muted-foreground dank:h-4 dank:min-w-4 dank:justify-center dank:border-0 dank:bg-m3-error dank:px-1 dank:type-label-small dank:text-m3-on-error border-transparent",
      },
    },
    defaultVariants: { variant: "outline" },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return (
    <span data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}

export { Badge, badgeVariants };
