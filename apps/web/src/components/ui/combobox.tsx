import { Combobox as ComboboxPrimitive } from "@base-ui/react/combobox";
import * as React from "react";

import {
  dankMenuContainerClassName,
  dankMenuItemClassName,
  dankMenuItemDisabledClassName,
  dankMenuItemShapeClassName,
  dankPopupMotionClassName,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/components/ui/utils";

const Combobox = ComboboxPrimitive.Root;

function ComboboxInput({ className, ...props }: ComboboxPrimitive.Input.Props) {
  return (
    <ComboboxPrimitive.Input
      className={cn(
        "bg-input placeholder:text-muted-foreground focus-visible:ring-primary/8-10 has-hover:hover:ring-border has-hover:hover:focus-visible:ring-primary/8-10 data-staged:focus-visible:ring-change/8-10 data-staged:has-hover:hover:ring-change/7-10 data-staged:has-hover:hover:focus-visible:ring-change/8-10 flex h-8 w-full rounded-md border px-2.5 py-1.5 text-sm leading-tight focus-visible:ring-1 focus-visible:outline-hidden disabled:cursor-not-allowed disabled:opacity-50 has-hover:hover:ring-1 disabled:has-hover:hover:ring-0",
        "dank:type-body-large dank:text-m3-on-surface dank:caret-m3-primary dank:placeholder:text-m3-on-surface-variant dank:border-m3-outline dank:has-hover:hover:border-m3-on-surface dank:focus-visible:border-m3-primary dank:focus-visible:ring-m3-primary dank:has-hover:hover:focus-visible:ring-m3-primary dank:aria-invalid:border-m3-error dank:aria-invalid:ring-m3-error dank:disabled:border-m3-on-surface/12 dank:disabled:text-m3-on-surface/38 dank:h-14 dank:rounded-xs dank:bg-transparent dank:px-4 dank:py-4 dank:transition-[border-color,box-shadow] dank:focus-visible:ring-1 dank:focus-visible:ring-inset dank:disabled:opacity-100 dank:has-hover:hover:ring-0 dank:has-hover:hover:focus-visible:ring-1",
        className,
      )}
      {...props}
    />
  );
}

function ComboboxContent({
  className,
  side = "bottom",
  sideOffset = 4,
  align = "start",
  collisionPadding = { top: 16, bottom: 16, left: 12, right: 12 },
  collisionAvoidance,
  children,
  ...props
}: ComboboxPrimitive.Popup.Props &
  Pick<
    ComboboxPrimitive.Positioner.Props,
    "side" | "sideOffset" | "align" | "collisionPadding" | "collisionAvoidance"
  >) {
  return (
    <ComboboxPrimitive.Portal>
      <ComboboxPrimitive.Positioner
        data-ui-popup=""
        side={side}
        sideOffset={sideOffset}
        align={align}
        collisionPadding={collisionPadding}
        collisionAvoidance={collisionAvoidance}
        className="isolate z-50"
      >
        <ComboboxPrimitive.Popup
          data-focus-indicator="child"
          className={cn(
            "shadow-shadow-color/shadow-opacity bg-popover text-popover-foreground flex max-h-[min(30rem,var(--available-height))] w-(--anchor-width) max-w-(--available-width) origin-(--transform-origin) flex-col overflow-hidden rounded-lg border shadow-lg outline-hidden",
            dankMenuContainerClassName,
            dankPopupMotionClassName,
            className,
          )}
          {...props}
        >
          {children}
        </ComboboxPrimitive.Popup>
      </ComboboxPrimitive.Positioner>
    </ComboboxPrimitive.Portal>
  );
}

function ComboboxList({ className, ...props }: ComboboxPrimitive.List.Props) {
  return (
    <ComboboxPrimitive.List
      className={cn(
        "dank:px-0 dank:py-0.5 flex min-h-0 flex-col overflow-y-auto p-1 outline-hidden",
        className,
      )}
      {...props}
    />
  );
}

function ComboboxItem({ className, ...props }: ComboboxPrimitive.Item.Props) {
  return (
    <ComboboxPrimitive.Item
      data-focus-indicator="child"
      className={cn(
        "data-highlighted:bg-border data-highlighted:text-foreground relative flex cursor-default items-center gap-2 rounded-md px-2 py-1.5 text-sm leading-tight outline-hidden select-none data-disabled:pointer-events-none data-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0",
        dankMenuItemClassName,
        dankMenuItemShapeClassName,
        dankMenuItemDisabledClassName,
        "dank:not-data-selected:data-highlighted:bg-transparent dank:data-selected:bg-m3-tertiary-container dank:data-selected:text-m3-on-tertiary-container dank:data-selected:rounded-md dank:data-selected:[--menu-icon:currentColor]",
        className,
      )}
      {...props}
    />
  );
}

function ComboboxEmpty({ className, ...props }: ComboboxPrimitive.Empty.Props) {
  return (
    <ComboboxPrimitive.Empty
      className={cn(
        "text-muted-foreground dank:type-body-large dank:px-4 dank:py-2.5 flex items-center gap-2 px-3.5 py-3.5 leading-tight empty:hidden",
        className,
      )}
      {...props}
    />
  );
}

function ComboboxChips({ className, ...props }: ComboboxPrimitive.Chips.Props) {
  return (
    <ComboboxPrimitive.Chips
      className={cn("flex w-full flex-wrap gap-1.5", className)}
      {...props}
    />
  );
}

function ComboboxChip({ className, ...props }: ComboboxPrimitive.Chip.Props) {
  return (
    <ComboboxPrimitive.Chip
      className={cn(
        "group/chip bg-background data-highlighted:ring-primary/8-10 data-staged:text-change data-staged:bg-change/2-10 data-staged:border-change/5-10 flex max-w-full min-w-0 items-stretch overflow-hidden rounded-lg border data-highlighted:ring-1",
        className,
      )}
      {...props}
    />
  );
}

function ComboboxChipRemove({ className, ...props }: ComboboxPrimitive.ChipRemove.Props) {
  return (
    <ComboboxPrimitive.ChipRemove
      className={cn(
        "text-muted-foreground has-hover:hover:bg-border has-hover:hover:text-foreground active:bg-border active:text-foreground focus-visible:ring-primary/8-10 group-data-staged/chip:text-change/8-10 group-data-staged/chip:border-change/5-10 group-data-staged/chip:has-hover:hover:bg-change/3-10 group-data-staged/chip:has-hover:hover:text-change group-data-staged/chip:active:bg-change/3-10 group-data-staged/chip:active:text-change dank:focus-visible:ring-0 flex shrink-0 items-center border-l px-2 focus-visible:ring-1 focus-visible:outline-hidden",
        className,
      )}
      {...props}
    />
  );
}

export {
  Combobox,
  ComboboxInput,
  ComboboxContent,
  ComboboxList,
  ComboboxItem,
  ComboboxEmpty,
  ComboboxChips,
  ComboboxChip,
  ComboboxChipRemove,
};

export type TComboboxChangeEventDetails = ComboboxPrimitive.Root.ChangeEventDetails;
export type TComboboxRootProps<
  Value,
  Multiple extends boolean | undefined = false,
> = React.ComponentProps<typeof ComboboxPrimitive.Root<Value, Multiple>>;
