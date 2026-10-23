import { type RefObject, useEffect, useState } from "react";

const POPUPS =
  "[data-slot=dropdown-menu-content],[data-slot=popover-content],[data-slot=suggestion-menu]";

function isPopup(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest(POPUPS));
}

/** True from focus or a press landing in the element until focus or a press lands outside it and its menus. */
export function useIsActive(
  ref: RefObject<HTMLElement | null>,
  initial = false,
  alsoInside: HTMLElement | null = null,
) {
  const [isActive, setIsActive] = useState(initial);
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    let pressed: EventTarget | null = null;
    const settle = (target: EventTarget | null) => {
      if (isPopup(target)) return;
      const isInside = (node: Node) =>
        element.contains(node) || Boolean(alsoInside?.contains(node));
      setIsActive(target instanceof Node && isInside(target));
    };
    const onFocusIn = (event: Event) => settle(event.target);
    const onPointerDown = (event: Event) => {
      pressed = event.target;
    };
    // Where the press began: a selection dragged past the edge stays inside.
    const onPointerUp = () => {
      if (pressed === null) return;
      settle(pressed);
      pressed = null;
    };
    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("pointerup", onPointerUp);
    document.addEventListener("pointercancel", onPointerUp);
    return () => {
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("pointerup", onPointerUp);
      document.removeEventListener("pointercancel", onPointerUp);
    };
  }, [ref, alsoInside]);
  return isActive;
}
