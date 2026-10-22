import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { ReactNode, useRef, useState } from "react";

import { MaximizeIcon, MinimizeIcon, XIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogOverlay, DialogPortal } from "@/components/ui/dialog";
import { SimpleTooltip } from "@/components/ui/tooltip";

type TProps = {
  onClose: () => void;
  children: ReactNode;
};

export function SettingsWindow({ onClose, children }: TProps) {
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
    <Dialog open disablePointerDismissal onOpenChange={handleOpenChange}>
      <DialogPortal>
        <DialogOverlay onClick={onClose} />
        <DialogPrimitive.Popup
          ref={popupRef}
          initialFocus={popupRef}
          data-slot="settings-window"
          data-fullscreen={isFullscreen ? "" : undefined}
          className="bg-surface shadow-dialog shadow-shadow-color/shadow-opacity border-primary ease-emphasized data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 fixed inset-0 z-[1000] m-auto flex h-[min(1020px,calc(100dvh-48px))] w-[min(1100px,calc(100vw-48px))] flex-col overflow-hidden rounded-[1.75rem] border-2 outline-hidden transition-[width,height,border-radius] duration-300 data-fullscreen:size-full data-fullscreen:rounded-none data-fullscreen:border-0 max-md:size-full max-md:rounded-none max-md:border-0"
        >
          <header className="relative flex h-16 shrink-0 items-center justify-center px-4">
            <DialogPrimitive.Title className="text-lg font-medium">Settings</DialogPrimitive.Title>
            <div className="absolute top-1/2 right-4 flex -translate-y-1/2 items-center gap-3">
              <SimpleTooltip content={isFullscreen ? "Exit full screen" : "Full screen"}>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={isFullscreen ? "Exit full screen" : "Full screen"}
                  className="text-muted-foreground size-10 max-md:hidden"
                  onClick={() => setIsFullscreen(!isFullscreen)}
                >
                  {isFullscreen ? (
                    <MinimizeIcon className="size-5" />
                  ) : (
                    <MaximizeIcon className="size-5" />
                  )}
                </Button>
              </SimpleTooltip>
              <DialogClose
                render={
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Close settings"
                    className="bg-surface-container-highest has-hover:hover:bg-destructive has-hover:hover:text-destructive-foreground size-10 rounded-full"
                  >
                    <XIcon className="size-5" />
                  </Button>
                }
              />
            </div>
          </header>
          <div className="bg-surface-container-low mx-2.5 mb-2.5 flex min-h-0 flex-1 overflow-hidden rounded-[1.375rem] max-md:mx-0 max-md:mb-0 max-md:rounded-none">
            {children}
          </div>
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
