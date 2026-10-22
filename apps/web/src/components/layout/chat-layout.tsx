import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from "react";

import { PageSlotsContext } from "@/components/layout/page-container";
import { PageBar } from "@/components/layout/page-header";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";

const NEAR_END_PX = 96;

type TChatScroll = { scrollToEnd: () => void };

const ChatScrollContext = createContext<TChatScroll>({ scrollToEnd: () => undefined });

export const useChatScroll = () => useContext(ChatScrollContext);

type TProps = {
  /** What the page bar holds, when the chat is the page. */
  bar?: ReactNode;
  /** A panel's own header, when the chat sits beside the page. */
  header?: ReactNode;
  width?: "full" | "narrow";
  children: ReactNode;
  composer?: ReactNode;
  /** Opens on the last message; off when the chat follows other content on the page. */
  startAtEnd?: boolean;
};

/** A conversation that scrolls above its composer and follows new messages while you read the end. */
export function ChatLayout({
  bar,
  header,
  width = "full",
  children,
  composer,
  startAtEnd = true,
}: TProps) {
  const { separatesBars } = useUiTheme();
  const [barSlot, setBarSlot] = useState<HTMLElement | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const isAtEndRef = useRef(startAtEnd);

  useEffect(() => {
    const scroller = scrollRef.current;
    const content = contentRef.current;
    if (!scroller || !content) return;
    if (isAtEndRef.current) scroller.scrollTop = scroller.scrollHeight;
    const observer = new ResizeObserver(() => {
      if (isAtEndRef.current) scroller.scrollTop = scroller.scrollHeight;
    });
    observer.observe(content);
    return () => observer.disconnect();
  }, []);

  const onScroll = () => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    const distance = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight;
    isAtEndRef.current = distance < NEAR_END_PX;
  };

  const scrollToEnd = () => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    isAtEndRef.current = true;
    scroller.scrollTop = scroller.scrollHeight;
  };

  const column = cn("w-full", width === "narrow" ? "mx-auto max-w-3xl px-4 sm:px-6" : "px-4");

  return (
    <ChatScrollContext value={{ scrollToEnd }}>
      <div
        data-slot="chat-layout"
        className="dank:gap-2 dank:bg-sidebar flex min-h-0 w-full flex-1 flex-col"
      >
        {bar && separatesBars && (
          <div
            ref={setBarSlot}
            className="bg-background shrink-0 overflow-hidden rounded-lg empty:hidden"
          />
        )}
        {bar && (
          <PageSlotsContext value={{ bar: separatesBars ? barSlot : null, fab: null }}>
            {separatesBars ? (
              <PageBar>{bar}</PageBar>
            ) : (
              <div className="shrink-0 px-4 pt-4">
                <PageBar>{bar}</PageBar>
              </div>
            )}
          </PageSlotsContext>
        )}
        {header}
        <div className="dank:rounded-lg dank:bg-background flex min-h-0 flex-1 flex-col overflow-hidden">
          <div
            ref={scrollRef}
            data-width="narrow"
            onScroll={onScroll}
            className="group/page flex min-h-0 flex-1 flex-col overflow-y-auto"
          >
            <div ref={contentRef} className={cn(column, "flex flex-col gap-6 py-6")}>
              {children}
            </div>
          </div>
          {composer && <div className={cn(column, "shrink-0 pb-4")}>{composer}</div>}
        </div>
      </div>
    </ChatScrollContext>
  );
}
