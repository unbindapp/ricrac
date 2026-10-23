import { ContextMenu as ContextMenuPrimitive } from "@base-ui/react/context-menu";
import type { MouseEvent } from "react";

import { dropdownContentVariants, MenuPopupChildren } from "@/components/ui/dropdown-menu";
import { cn } from "@/components/ui/utils";

/** A menu opened by right click or a long press; its items are the dropdown menu's. */
function ContextMenu(props: ContextMenuPrimitive.Root.Props) {
  return <ContextMenuPrimitive.Root data-slot="context-menu" {...props} />;
}

function ContextMenuTrigger(props: ContextMenuPrimitive.Trigger.Props) {
  return <ContextMenuPrimitive.Trigger data-slot="context-menu-trigger" {...props} />;
}

function ContextMenuContent({ className, children, ...props }: ContextMenuPrimitive.Popup.Props) {
  return (
    <ContextMenuPrimitive.Portal>
      <ContextMenuPrimitive.Backdrop className="fixed inset-0 z-[1000]" />
      <ContextMenuPrimitive.Positioner
        data-ui-popup=""
        collisionPadding={12}
        className="isolate z-[1000] outline-none"
      >
        <ContextMenuPrimitive.Popup
          data-slot="context-menu-content"
          data-focus-indicator="child"
          className={cn(dropdownContentVariants({ className }))}
          {...props}
        >
          <MenuPopupChildren>{children}</MenuPopupChildren>
        </ContextMenuPrimitive.Popup>
      </ContextMenuPrimitive.Positioner>
    </ContextMenuPrimitive.Portal>
  );
}

/** On an element inside a trigger: links, media and selected text keep the browser's own menu. */
function keepNativeMenu(event: MouseEvent) {
  const target = event.target;
  const isNative = target instanceof Element && Boolean(target.closest("a, img, video"));
  if (isNative || window.getSelection()?.toString()) event.stopPropagation();
}

export { ContextMenu, ContextMenuContent, ContextMenuTrigger, keepNativeMenu };
