import { type RefObject, useEffect, useState } from "react";

const POPUPS =
  "[data-slot=dropdown-menu-content],[data-slot=popover-content],[data-slot=suggestion-menu]";

function isPopup(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest(POPUPS));
}

/** True from focus landing in the element until focus or a click lands outside it and its menus. */
export function useIsActive(ref: RefObject<HTMLElement | null>, initial = false) {
  const [isActive, setIsActive] = useState(initial);
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const settle = (target: EventTarget | null) => {
      if (isPopup(target)) return;
      setIsActive(target instanceof Node && element.contains(target));
    };
    const onEvent = (event: Event) => settle(event.target);
    document.addEventListener("focusin", onEvent);
    // On click, not pointerdown: what hides would otherwise move the button being clicked.
    document.addEventListener("click", onEvent);
    return () => {
      document.removeEventListener("focusin", onEvent);
      document.removeEventListener("click", onEvent);
    };
  }, [ref]);
  return isActive;
}
