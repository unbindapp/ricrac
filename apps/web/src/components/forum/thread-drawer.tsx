import { type ReactNode, useState } from "react";

import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";

type TProps = {
  /** The open thread's panel; the last one stays while the drawer slides away. */
  children: ReactNode;
  onClose: () => void;
};

export function ThreadDrawer({ children, onClose }: TProps) {
  const [shown, setShown] = useState(children);
  if (children && children !== shown) setShown(children);
  return (
    <Drawer
      direction="right"
      open={Boolean(children)}
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <DrawerContent className="dank:bg-sidebar dank:p-2 dank:ring-0 w-xl max-w-full">
        <DrawerTitle className="sr-only">Thread</DrawerTitle>
        {children ?? shown}
      </DrawerContent>
    </Drawer>
  );
}
