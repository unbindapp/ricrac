import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";
import * as React from "react";

import { duxPopupMotionClassName } from "@/components/ui/dropdown-menu";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

function TooltipProvider({ delay = 400, ...props }: TooltipPrimitive.Provider.Props) {
  return <TooltipPrimitive.Provider delay={delay} {...props} />;
}

function Tooltip(props: TooltipPrimitive.Root.Props) {
  return <TooltipPrimitive.Root data-slot="tooltip" {...props} />;
}

function TooltipTrigger(props: TooltipPrimitive.Trigger.Props) {
  return <TooltipPrimitive.Trigger data-slot="tooltip-trigger" {...props} />;
}

function TooltipContent({
  className,
  side = "top",
  sideOffset,
  align = "center",
  children,
  ...props
}: TooltipPrimitive.Popup.Props &
  Pick<TooltipPrimitive.Positioner.Props, "side" | "sideOffset" | "align">) {
  const { id } = useUiTheme();
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Positioner
        side={side}
        sideOffset={sideOffset ?? (id === "dux" ? 4 : 6)}
        align={align}
        className="isolate z-[1500]"
      >
        <TooltipPrimitive.Popup
          data-slot="tooltip-content"
          className={cn(
            "bg-popover text-popover-foreground shadow-shadow-color/shadow-opacity data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 max-w-xs origin-(--transform-origin) rounded-md border px-2 py-1 text-xs leading-snug shadow-md",
            "dux:bg-m3-inverse-surface dux:text-m3-inverse-on-surface dux:type-body-small dux:min-h-6 dux:max-w-50 dux:min-w-10 dux:rounded-xs dux:border-0 dux:shadow-none",
            duxPopupMotionClassName,
            className,
          )}
          {...props}
        >
          {children}
        </TooltipPrimitive.Popup>
      </TooltipPrimitive.Positioner>
    </TooltipPrimitive.Portal>
  );
}

function SimpleTooltip({
  content,
  children,
  side,
}: {
  content: React.ReactNode;
  children: React.ReactElement;
  side?: TooltipPrimitive.Positioner.Props["side"];
}) {
  if (!content) return children;
  return (
    <Tooltip>
      <TooltipTrigger render={children} />
      <TooltipContent side={side}>{content}</TooltipContent>
    </Tooltip>
  );
}

export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider, SimpleTooltip };
