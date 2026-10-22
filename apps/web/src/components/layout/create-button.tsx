import type { ComponentProps } from "react";
import { createPortal } from "react-dom";

import { type LucideIcon, PlusIcon } from "@/components/icons";
import { usePageSlots } from "@/components/layout/page-container";
import { Button } from "@/components/ui/button";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TProps = Omit<ComponentProps<typeof Button>, "children" | "variant"> & {
  label: string;
  icon?: LucideIcon;
};

/** A page's button for creating its things: in its header, or floating bottom right. */
export function CreateButton({ label, icon: Icon = PlusIcon, size, ...props }: TProps) {
  const { createButton } = useUiTheme();
  const { fab } = usePageSlots();
  if (createButton === "fab") {
    if (!fab) return null;
    return createPortal(
      <Button
        {...props}
        variant="tonal"
        className="shadow-shadow-color/shadow-opacity dank:h-14 dank:rounded-[1rem] dank:pr-5 dank:pl-4 dank:active:rounded-[0.75rem] gap-3 text-base shadow-lg"
      >
        <Icon className="size-6" />
        {label}
      </Button>,
      fab,
    );
  }
  return (
    <Button size={size} {...props}>
      <Icon className="size-4" />
      {label}
    </Button>
  );
}
