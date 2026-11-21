import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";

import { cn } from "@/components/ui/utils";

const duxChip =
  "dux:h-8 dux:gap-2 dux:rounded-sm dux:px-4 dux:type-label-large dux:has-[>svg]:pl-2 dux:[&_svg]:size-4.5";

const duxOutlinedChip = `${duxChip} dux:border-m3-outline-variant dux:bg-transparent`;

const badgeVariants = cva(
  "inline-flex h-5 max-w-full shrink-0 items-center gap-1 rounded-full border px-2 text-xs leading-none font-medium whitespace-nowrap [&_svg]:size-3.5 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: `bg-primary text-primary-foreground border-transparent ${duxChip} dux:bg-m3-secondary-container dux:text-m3-on-secondary-container`,
        outline: `text-foreground bg-background ${duxOutlinedChip} dux:text-m3-on-surface`,
        muted: `bg-muted text-muted-foreground border-transparent ${duxOutlinedChip} dux:text-m3-on-surface-variant`,
        destructive: `text-destructive bg-destructive/3-10 border-destructive/5-10 ${duxOutlinedChip}`,
        warning: `text-warning bg-warning/3-10 border-warning/5-10 ${duxOutlinedChip}`,
        success: `text-success bg-success/3-10 border-success/5-10 ${duxOutlinedChip}`,
        process: `text-process bg-process/3-10 border-process/5-10 ${duxOutlinedChip}`,
        count:
          "bg-muted text-muted-foreground dux:h-4 dux:min-w-4 dux:justify-center dux:border-0 dux:bg-m3-error dux:px-1 dux:type-label-small dux:text-m3-on-error border-transparent",
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
