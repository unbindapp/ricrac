import { ShortcutKeys } from "@/components/command-panel/shortcut-keys";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useCommandPanelStore } from "@/components/command-panel/command-panel-store";
import { shortcutGroups } from "@/lib/hotkeys/shortcuts";

export function ShortcutsDialog() {
  const isOpen = useCommandPanelStore((state) => state.isShortcutsOpen);
  const setOpen = useCommandPanelStore((state) => state.setShortcutsOpen);
  return (
    <Dialog open={isOpen} onOpenChange={setOpen}>
      <DialogContent className="w-xl">
        <DialogHeader>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
        </DialogHeader>
        <div className="gap-6 sm:columns-2">
          {shortcutGroups.map((group) => (
            <section
              key={group.title}
              className="mb-6 flex break-inside-avoid flex-col gap-2 last:mb-0"
            >
              <h3 className="text-muted-foreground text-xs font-medium">{group.title}</h3>
              {group.shortcuts.map((shortcut) => (
                <div key={shortcut.label} className="flex items-center gap-2 text-sm">
                  <span className="min-w-0 flex-1 truncate">{shortcut.label}</span>
                  <ShortcutKeys keys={shortcut.keys} then={shortcut.then} />
                </div>
              ))}
            </section>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
