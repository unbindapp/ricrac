import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { useRememberSettingsReturn } from "@/lib/stores/settings-return";
import { SIDEBAR_HOTKEY } from "@/lib/stores/sidebar";
import { NavigationRail } from "@/themes/dux/navigation-rail";
import { useNavigationStore } from "@/themes/dux/navigation-store";

export function DuxNavigation() {
  const isExpanded = useNavigationStore((state) => state.isExpanded);
  const toggle = useNavigationStore((state) => state.toggle);
  useHotkeyScope([{ hotkey: SIDEBAR_HOTKEY, callback: toggle }]);
  useRememberSettingsReturn();
  return (
    <aside className="hidden shrink-0 overflow-hidden md:flex">
      <NavigationRail
        isExpanded={isExpanded}
        menuLabel={isExpanded ? "Collapse navigation" : "Expand navigation"}
        onMenu={toggle}
      />
    </aside>
  );
}

/** The expanded rail, modal over the page on narrow screens. */
export function DuxNavigationDrawer({ onNavigate }: { onNavigate: () => void }) {
  return (
    <NavigationRail
      isExpanded
      isModal
      menuLabel="Close navigation"
      onMenu={onNavigate}
      onNavigate={onNavigate}
    />
  );
}
