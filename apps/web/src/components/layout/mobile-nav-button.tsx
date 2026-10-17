import { PanelLeftIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useMobileNavStore } from "@/lib/stores/mobile-nav";

export function MobileNavButton() {
  const setOpen = useMobileNavStore((state) => state.setOpen);
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label="Open navigation"
      className="text-muted-foreground -ml-1 md:hidden"
      onClick={() => setOpen(true)}
    >
      <PanelLeftIcon className="size-4" />
    </Button>
  );
}
