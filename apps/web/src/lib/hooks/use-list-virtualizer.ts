import { useElementScrollRestoration } from "@tanstack/react-router";
import { useVirtualizer, useWindowVirtualizer } from "@tanstack/react-virtual";
import { type RefObject, useLayoutEffect, useState } from "react";

import { useDeviceSize } from "@/components/providers/device-size-provider";

type TOptions = {
  count: number;
  estimateSize: (index: number) => number;
  getItemKey: (index: number) => string;
  overscan?: number;
  /** Space between items; read from the container's row gap when left out. */
  gap?: number;
};

const DEFAULT_OVERSCAN = 8;

function findScrollParent(element: HTMLElement): HTMLElement | null {
  let current = element.parentElement;
  while (current) {
    const overflowY = getComputedStyle(current).overflowY;
    if (overflowY === "auto" || overflowY === "scroll") return current;
    current = current.parentElement;
  }
  return null;
}

/**
 * Virtualizes against whatever scrolls the list: the window when the page scrolls the document
 * (phones on `PageContainer scroll="window"` pages), otherwise the pane around it.
 */
export function useListVirtualizer(
  containerRef: RefObject<HTMLElement | null>,
  { overscan = DEFAULT_OVERSCAN, gap: fixedGap, ...options }: TOptions,
) {
  const { size } = useDeviceSize();
  const [pane, setPane] = useState<HTMLElement | null>(null);
  const [followsWindow, setFollowsWindow] = useState(false);
  const [scrollMargin, setScrollMargin] = useState(0);
  const [gap, setGap] = useState(0);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const scroller = findScrollParent(container);
    setPane(scroller);
    setFollowsWindow(!scroller);
    const measure = () => {
      setGap(Number.parseFloat(getComputedStyle(container).rowGap) || 0);
      setScrollMargin(
        scroller
          ? container.getBoundingClientRect().top -
              scroller.getBoundingClientRect().top +
              scroller.scrollTop
          : container.getBoundingClientRect().top + window.scrollY,
      );
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(scroller?.firstElementChild ?? document.body);
    return () => observer.disconnect();
  }, [containerRef, size]);

  const shared = { ...options, overscan, gap: fixedGap ?? gap, scrollMargin };
  const windowEntry = useElementScrollRestoration({ getElement: () => window });
  const windowVirtualizer = useWindowVirtualizer({
    ...shared,
    enabled: followsWindow,
    initialOffset: windowEntry?.scrollY,
  });
  const paneVirtualizer = useVirtualizer({
    ...shared,
    enabled: Boolean(pane),
    getScrollElement: () => pane,
  });
  return { virtualizer: followsWindow ? windowVirtualizer : paneVirtualizer, scrollMargin };
}
