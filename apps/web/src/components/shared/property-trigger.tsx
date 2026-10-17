import { type ComponentProps, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";

type TProps = Omit<ComponentProps<typeof Button>, "children"> & {
  label: string;
  icon: ReactNode;
  text: ReactNode;
  muted?: boolean;
};

/** The button a property picker opens from, in a properties panel; read-only ones look the same. */
export function PropertyTrigger({ label, icon, text, muted, className, ...props }: TProps) {
  return (
    <Button
      variant="ghost"
      size="sm"
      forceMinSize={false}
      fadeOnDisabled={false}
      aria-label={label}
      data-muted={muted || undefined}
      className={cn(
        "data-muted:text-muted-foreground w-full min-w-0 justify-start font-normal",
        className,
      )}
      {...props}
    >
      {icon}
      <span className="min-w-0 truncate">{text}</span>
    </Button>
  );
}
