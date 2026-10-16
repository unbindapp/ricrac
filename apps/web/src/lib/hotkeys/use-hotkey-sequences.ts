import {
  type HotkeyCallback,
  type HotkeySequence,
  useHotkeySequences,
} from "@tanstack/react-hotkeys";

import { useHotkeyScopeStore } from "@/lib/hotkeys/use-hotkey-scope";

export type THotkeySequence = { sequence: HotkeySequence; callback: HotkeyCallback };

/** Is a dialog or menu open, whose keys shouldn't reach the page under it? */
function isOverlayOpen() {
  return document.querySelector('[role="dialog"], [role="alertdialog"], [role="menu"]') !== null;
}

/** Sequences like G then I, held back while typing, in a dialog or in a modal scope. */
export function useHotkeySequenceScope(sequences: THotkeySequence[]) {
  const isModal = useHotkeyScopeStore((state) => state.scopes.some((scope) => scope.modal));
  useHotkeySequences(
    sequences.map(({ sequence, callback }) => ({
      sequence,
      callback: (event, context) => {
        if (isOverlayOpen()) return;
        callback(event, context);
      },
    })),
    { enabled: !isModal },
  );
}
