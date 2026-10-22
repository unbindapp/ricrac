import { formatForDisplay } from "@tanstack/react-hotkeys";
import type { ReactElement } from "react";

import { PanelLeftCloseIcon, PanelLeftIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";
import { SIDEBAR_HOTKEY, useSidebarStore } from "@/lib/stores/sidebar";

export function SidebarButton() {
  const setOpen = useMobileNavStore((state) => state.setOpen);
  const toggle = useSidebarStore((state) => state.toggle);
  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        aria-label="Open navigation"
        className="text-muted-foreground -ml-1 md:hidden"
        onClick={() => setOpen(true)}
      >
        <PanelLeftIcon className="size-4" />
      </Button>
      <SidebarToggleTooltip label="Show sidebar">
        <Button
          variant="ghost"
          size="icon"
          aria-label="Show sidebar"
          className="text-muted-foreground -ml-1 hidden md:group-data-sidebar-collapsed/shell:inline-flex"
          onClick={toggle}
        >
          <PanelLeftIcon className="size-4" />
        </Button>
      </SidebarToggleTooltip>
    </>
  );
}

export function HideSidebarButton() {
  const toggle = useSidebarStore((state) => state.toggle);
  return (
    <SidebarToggleTooltip label="Hide sidebar">
      <Button
        variant="ghost"
        size="icon"
        aria-label="Hide sidebar"
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
  side,
  children,
}: {
  label: string;
  side?: "right";
  children: ReactElement;
}) {
  return (
    <SimpleTooltip
      side={side}
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
