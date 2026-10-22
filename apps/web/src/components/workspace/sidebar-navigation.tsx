import { HideSidebarButton } from "@/components/layout/sidebar-button";
import { Sidebar } from "@/components/workspace/sidebar";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { SIDEBAR_HOTKEY, useSidebarStore } from "@/lib/stores/sidebar";

export function SidebarNavigation() {
  const isCollapsed = useSidebarStore((state) => state.isCollapsed);
  const toggle = useSidebarStore((state) => state.toggle);
  useHotkeyScope([{ hotkey: SIDEBAR_HOTKEY, callback: toggle }]);
  if (isCollapsed) return null;
  return (
    <aside className="hidden w-60 shrink-0 md:flex">
      <Sidebar hideButton={<HideSidebarButton />} />
    </aside>
  );
}

export function useIsSidebarHidden() {
  return useSidebarStore((state) => state.isCollapsed);
}
