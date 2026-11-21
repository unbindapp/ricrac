import { type ComponentProps, type ReactNode } from "react";

import { PropertyTrigger } from "@/components/shared/property-trigger";
import { Button } from "@/components/ui/button";

/** row: in a properties panel; compact: a chip in a create dialog. */
export type TPickerAppearance = "row" | "compact";

type TProps = Omit<ComponentProps<typeof Button>, "children"> & {
  appearance?: TPickerAppearance;
  label: string;
  icon: ReactNode;
  text: ReactNode;
  muted?: boolean;
};

export function PickerTrigger({ appearance = "row", label, icon, text, muted, ...props }: TProps) {
  if (appearance === "row") {
    return <PropertyTrigger label={label} icon={icon} text={text} muted={muted} {...props} />;
  }
  return (
    <Button
      variant="outline"
      size="sm"
      forceMinSize={false}
      aria-label={label}
      data-muted={muted || undefined}
      className="data-muted:text-muted-foreground dux:text-m3-on-surface dux:data-muted:text-m3-on-surface-variant dux:h-8 dux:gap-2 dux:rounded-sm dux:px-4 dux:active:rounded-sm dux:has-[>svg]:pl-2 dux:font-medium dux:[&>svg]:size-4.5 dux:[&>svg.text-muted-foreground]:text-m3-primary max-w-48 font-normal"
      {...props}
    >
      {icon}
      <span className="truncate">{text}</span>
    </Button>
  );
}
