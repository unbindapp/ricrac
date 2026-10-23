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
        forceMinSize={false}
        className="bg-m3-primary-container text-m3-on-primary-container shadow-elevation-3 has-hover:hover:shadow-elevation-4 dank:h-14 dank:rounded-lg dank:px-4 dank:active:rounded-lg dank:text-base dank:leading-6 dank:tracking-[0.009375rem] dank:[&_svg]:size-6 gap-2"
      >
        <Icon />
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
