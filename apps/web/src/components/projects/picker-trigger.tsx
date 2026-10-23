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
      className="data-muted:text-muted-foreground dank:text-m3-on-surface dank:data-muted:text-m3-on-surface-variant dank:h-8 dank:gap-2 dank:rounded-sm dank:px-4 dank:active:rounded-sm dank:has-[>svg]:pl-2 dank:font-medium dank:[&>svg]:size-4.5 dank:[&>svg.text-muted-foreground]:text-m3-primary max-w-48 font-normal"
      {...props}
    >
      {icon}
      <span className="truncate">{text}</span>
    </Button>
  );
}
