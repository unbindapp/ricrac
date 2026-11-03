import { useElementScrollRestoration } from "@tanstack/react-router";
import {
  defaultRangeExtractor,
  type Range,
  useVirtualizer,
  useWindowVirtualizer,
} from "@tanstack/react-virtual";
import { type RefObject, useLayoutEffect, useRef, useState } from "react";

import { useDeviceSize } from "@/components/providers/device-size-provider";

type TOptions = {
  count: number;
  estimateSize: (index: number) => number;
  getItemKey: (index: number) => string;
  overscan?: number;
  /** Space between items; read from the container's row gap when left out. */
  gap?: number;
  /** Group headers: the one whose group is on screen stays rendered, to pin it. */
  stickyIndexes?: number[];
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

/** Where pinned content starts: below the bars stuck to the top of the same scroller. */
function stickyBarsBottom(scroller: HTMLElement | null) {
  let bottom = 0;
  for (const bar of document.querySelectorAll<HTMLElement>("[data-sticky-bar]")) {
    if (scroller && !scroller.contains(bar)) continue;
    const style = getComputedStyle(bar);
    if (style.position !== "sticky" || style.display === "none") continue;
    bottom = Math.max(bottom, bar.offsetHeight + (Number.parseFloat(style.top) || 0));
  }
  return bottom;
}

/** The last group header that has reached the pinned position. */
function activeStickyIndex(
  stickyIndexes: number[],
  starts: (index: number) => number,
  line: number,
) {
  let active: number | undefined;
  for (const index of stickyIndexes) {
    if (starts(index) > line) break;
    active = index;
  }
  return active;
}

/**
 * Virtualizes against whatever scrolls the list: the window when the page scrolls the document
 * (phones on `PageContainer scroll="window"` pages), otherwise the pane around it.
 */
export function useListVirtualizer(
  containerRef: RefObject<HTMLElement | null>,
  { overscan = DEFAULT_OVERSCAN, gap: fixedGap, stickyIndexes, ...options }: TOptions,
) {
  const { size } = useDeviceSize();
  const [pane, setPane] = useState<HTMLElement | null>(null);
  const [followsWindow, setFollowsWindow] = useState(false);
  const [scrollMargin, setScrollMargin] = useState(0);
  const [gap, setGap] = useState(0);
  const [stickyTop, setStickyTop] = useState(0);
  const activeStickyRef = useRef<number | undefined>(undefined);

  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const scroller = findScrollParent(container);
    setPane(scroller);
    setFollowsWindow(!scroller);
    const measure = () => {
      setGap(Number.parseFloat(getComputedStyle(container).rowGap) || 0);
      setStickyTop(stickyBarsBottom(scroller));
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
    document.querySelectorAll("[data-sticky-bar]").forEach((bar) => observer.observe(bar));
    return () => observer.disconnect();
  }, [containerRef, size]);

  const rangeExtractor = (range: Range) => {
    const indexes = defaultRangeExtractor(range);
    const active = activeStickyRef.current;
    if (active === undefined || indexes.includes(active)) return indexes;
    return [active, ...indexes];
  };
  const shared = { ...options, overscan, gap: fixedGap ?? gap, scrollMargin, rangeExtractor };
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
  const virtualizer = followsWindow ? windowVirtualizer : paneVirtualizer;
  if (stickyIndexes?.length) virtualizer.getTotalSize();
  activeStickyRef.current = stickyIndexes?.length
    ? activeStickyIndex(
        stickyIndexes,
        (index) => virtualizer.measurementsCache[index]?.start ?? Infinity,
        (virtualizer.scrollOffset ?? 0) + stickyTop,
      )
    : undefined;
  return {
    virtualizer,
    scrollMargin,
    gap: fixedGap ?? gap,
    stickyTop,
    activeSticky: activeStickyRef.current,
  };
}
