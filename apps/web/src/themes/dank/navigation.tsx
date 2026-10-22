import { PanelLeftCloseIcon } from "@/components/icons";
import { SidebarToggleTooltip } from "@/components/layout/sidebar-button";
import { Button } from "@/components/ui/button";
import { Sidebar } from "@/components/workspace/sidebar";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useRememberSettingsReturn } from "@/lib/stores/settings-return";
import { SIDEBAR_HOTKEY } from "@/lib/stores/sidebar";
import { NavigationRail } from "@/themes/dank/navigation-rail";
import { useNavigationStore } from "@/themes/dank/navigation-store";

export function DankNavigation() {
  const isExpanded = useNavigationStore((state) => state.isExpanded);
  const toggle = useNavigationStore((state) => state.toggle);
  useHotkeyScope([{ hotkey: SIDEBAR_HOTKEY, callback: toggle }]);
  useRememberSettingsReturn();
  return (
    <aside className="hidden shrink-0 md:flex">
      {isExpanded ? (
        <div className="flex w-64">
          <Sidebar hideButton={<CollapseToRailButton />} />
        </div>
      ) : (
        <NavigationRail />
      )}
    </aside>
  );
}

function CollapseToRailButton() {
  const toggle = useNavigationStore((state) => state.toggle);
  return (
    <SidebarToggleTooltip label="Collapse to rail" side="right">
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
