import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { ReactNode, useRef, useState } from "react";

import { MaximizeIcon, MinimizeIcon, XIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogOverlay, DialogPortal } from "@/components/ui/dialog";
import { SimpleTooltip } from "@/components/ui/tooltip";

type TProps = {
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
};

export function SettingsWindow({ isOpen, onClose, children }: TProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const popupRef = useRef<HTMLDivElement>(null);

  const handleOpenChange: DialogPrimitive.Root.Props["onOpenChange"] = (open, details) => {
    if (open) return;
    if (details.reason === "escape-key" && !isEventFrom(popupRef.current, details.event)) {
      details.cancel();
      return;
    }
    onClose();
  };

  return (
    <Dialog open={isOpen} disablePointerDismissal onOpenChange={handleOpenChange}>
      <DialogPortal>
        <DialogOverlay
          onClick={onClose}
          className="ease-emphasized fill-mode-forwards duration-300 data-closed:duration-300 data-open:duration-300"
        />
        <DialogPrimitive.Popup
          ref={popupRef}
          initialFocus={popupRef}
          data-slot="settings-window"
          data-focus-indicator="child"
          data-fullscreen={isFullscreen ? "" : undefined}
          className="bg-m3-surface-container-high text-m3-on-surface shadow-elevation-3 ease-emphasized data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95 fill-mode-forwards fixed inset-0 z-[1000] m-auto flex h-[min(1020px,calc(100dvh-48px))] w-[min(1100px,calc(100vw-48px))] flex-col overflow-hidden rounded-xl outline-hidden transition-[width,height,border-radius] duration-300 data-fullscreen:size-full data-fullscreen:rounded-none max-md:size-full max-md:rounded-none"
        >
          <header className="flex h-16 shrink-0 items-center gap-2 pr-4 pl-6">
            <DialogPrimitive.Title className="type-headline-small min-w-0 flex-1 truncate">
              Settings
            </DialogPrimitive.Title>
            <SimpleTooltip content={isFullscreen ? "Exit full screen" : "Full screen"}>
              <Button
                variant="ghost"
                size="icon"
                aria-label={isFullscreen ? "Exit full screen" : "Full screen"}
                className="max-md:hidden"
                onClick={() => setIsFullscreen(!isFullscreen)}
              >
                {isFullscreen ? <MinimizeIcon /> : <MaximizeIcon />}
              </Button>
            </SimpleTooltip>
            <DialogClose
              render={
                <Button variant="ghost" size="icon" aria-label="Close settings">
                  <XIcon />
                </Button>
              }
            />
          </header>
          <div className="flex min-h-0 flex-1 overflow-hidden px-6 pb-6 max-md:p-0">{children}</div>
        </DialogPrimitive.Popup>
      </DialogPortal>
    </Dialog>
  );
}

function isEventFrom(element: HTMLElement | null, event: Event | undefined) {
  const target = event?.target;
  if (!(target instanceof Node)) return true;
  return target === document.body || (element?.contains(target) ?? false);
}
