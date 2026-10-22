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
    <aside
      data-expanded={isExpanded || undefined}
      className="ease-emphasized hidden w-24 shrink-0 overflow-hidden transition-[width] duration-300 data-expanded:w-64 md:flex"
    >
      <NavigationRail
        isExpanded={isExpanded}
        menuLabel={isExpanded ? "Collapse navigation" : "Expand navigation"}
        onMenu={toggle}
      />
    </aside>
  );
}

/** The expanded rail, modal over the page on narrow screens. */
export function DankNavigationDrawer({ onNavigate }: { onNavigate: () => void }) {
  return (
    <NavigationRail
      isExpanded
      hasAccountLinks
      menuLabel="Close navigation"
      onMenu={onNavigate}
      onNavigate={onNavigate}
    />
  );
}
