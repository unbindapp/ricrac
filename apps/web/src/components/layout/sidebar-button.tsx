import { formatForDisplay } from "@tanstack/react-hotkeys";
import { PanelLeftCloseIcon, PanelLeftIcon } from "lucide-react";
import type { ReactElement } from "react";

import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";
import { SIDEBAR_HOTKEY, useSidebarStore } from "@/lib/stores/sidebar";

/** Opens the drawer on narrow screens, and shows the sidebar on wide ones once it is hidden. */
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

function SidebarToggleTooltip({ label, children }: { label: string; children: ReactElement }) {
  return (
    <SimpleTooltip
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
