import { formatForDisplay } from "@tanstack/react-hotkeys";
import { Fragment } from "react";

import { Kbd, KbdGroup } from "@/components/ui/kbd";

/** Keys as the platform names them (⌘K on a Mac, Ctrl+K elsewhere), "then" between steps. */
export function ShortcutKeys({ keys, then = false }: { keys: string[]; then?: boolean }) {
  return (
    <KbdGroup>
      {keys.map((key, index) => (
        <Fragment key={key}>
          {index > 0 && then && <span className="text-muted-foreground px-0.5 text-xs">then</span>}
          <Kbd>{formatForDisplay(key)}</Kbd>
        </Fragment>
      ))}
    </KbdGroup>
  );
}
