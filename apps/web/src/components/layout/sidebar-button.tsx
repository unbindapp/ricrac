import { formatForDisplay } from "@tanstack/react-hotkeys";
import type { ReactElement } from "react";

import { PanelLeftCloseIcon, PanelLeftIcon } from "@/components/icons/symbols.gen";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";
import { SIDEBAR_HOTKEY, useSidebarStore } from "@/lib/stores/sidebar";

/** Opens the navigation drawer on narrow screens; wide ones always show the rail. */
export function SidebarButton() {
  const setOpen = useMobileNavStore((state) => state.setOpen);
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label="Open navigation"
      className="text-muted-foreground -ml-1 md:hidden"
      onClick={() => setOpen(true)}
    >
      <PanelLeftIcon className="size-4" />
    </Button>
  );
}

export function HideSidebarButton() {
  const toggle = useSidebarStore((state) => state.toggle);
  return (
    <SidebarToggleTooltip label="Collapse to rail">
      <Button
        variant="ghost"
        size="icon"
        aria-label="Collapse to rail"
        className="text-muted-foreground"
        onClick={toggle}
      >
        <PanelLeftCloseIcon className="size-4" />
      </Button>
    </SidebarToggleTooltip>
  );
}

export function SidebarToggleTooltip({
  label,
  children,
}: {
  label: string;
  children: ReactElement;
}) {
  return (
    <SimpleTooltip
      side="right"
      content={
        <span className="flex items-center gap-1.5">
          {label}
          <Kbd>{formatForDisplay(SIDEBAR_HOTKEY)}</Kbd>
        </span>
      }
    >
      {children}
    </SimpleTooltip>
  );
}
