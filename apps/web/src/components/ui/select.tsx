import { Select as SelectPrimitive } from "@base-ui/react/select";
import * as React from "react";

import { CheckIcon, ChevronDownIcon, ChevronsUpDownIcon } from "@/components/icons";
import {
  dankMenuContainerClassName,
  dankMenuItemClassName,
  dankMenuItemDisabledClassName,
  dankMenuItemShapeClassName,
  dankPopupMotionClassName,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/components/ui/utils";

const Select = SelectPrimitive.Root;

function SelectTrigger({
  className,
  children,
  size = "default",
  ...props
}: SelectPrimitive.Trigger.Props & { size?: "sm" | "default" }) {
  return (
    <SelectPrimitive.Trigger
      data-slot="select-trigger"
      data-size={size}
      data-focus-indicator="child"
      className={cn(
        "group/select bg-input has-hover:hover:ring-border focus-visible:ring-primary/8-10 aria-invalid:border-destructive/8-10 flex h-8 w-full min-w-0 items-center justify-between gap-2 rounded-md border px-2.5 text-left text-sm select-none focus-visible:ring-1 focus-visible:outline-hidden disabled:cursor-not-allowed disabled:opacity-50 has-hover:hover:ring-1 data-[size=sm]:h-7 data-[size=sm]:px-2 [&_svg]:pointer-events-none [&_svg]:shrink-0",
        "dank:type-body-large dank:text-m3-on-surface dank:border-m3-outline dank:has-hover:hover:border-m3-on-surface dank:focus-visible:border-m3-primary dank:focus-visible:ring-m3-primary dank:aria-invalid:border-m3-error dank:aria-invalid:ring-m3-error dank:disabled:border-m3-on-surface/12 dank:disabled:text-m3-on-surface/38 dank:h-14 dank:gap-1 dank:rounded-xs dank:bg-transparent dank:pr-3 dank:pl-4 dank:transition-[border-color,box-shadow] dank:focus-visible:ring-1 dank:focus-visible:ring-inset dank:disabled:opacity-100 dank:has-hover:hover:ring-0 dank:has-hover:hover:focus-visible:ring-1 dank:data-[size=sm]:type-body-medium dank:data-[size=sm]:h-10 dank:data-[size=sm]:pr-2 dank:data-[size=sm]:pl-3",
        className,
      )}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon className="text-muted-foreground dank:text-m3-on-surface-variant dank:group-aria-invalid/select:text-m3-error">
        <ChevronsUpDownIcon className="dank:hidden size-3.5" />
        <ChevronDownIcon className="dank:block dank:group-data-[size=sm]/select:size-5 dank:group-data-popup-open/select:rotate-180 dank:transition-transform dank:duration-(--spring-fast-spatial-duration) dank:ease-spring-fast-spatial hidden size-6" />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  );
}

function SelectValue({ className, ...props }: SelectPrimitive.Value.Props) {
  return (
    <SelectPrimitive.Value
      data-slot="select-value"
      className={cn(
        "data-placeholder:text-muted-foreground flex min-w-0 flex-1 items-center gap-1.5 truncate",
        className,
      )}
      {...props}
    />
  );
}

function SelectContent({
  className,
  children,
  align = "start",
  sideOffset = 4,
  ...props
}: SelectPrimitive.Popup.Props &
  Pick<SelectPrimitive.Positioner.Props, "align" | "sideOffset" | "side">) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Positioner
        data-ui-popup=""
        align={align}
        sideOffset={sideOffset}
        alignItemWithTrigger={false}
        collisionPadding={{ top: 16, bottom: 16, left: 12, right: 12 }}
        className="isolate z-[1000] outline-none"
      >
        <SelectPrimitive.Popup
          data-slot="select-content"
          data-focus-indicator="child"
          className={cn(
            "bg-popover text-popover-foreground shadow-shadow-color/shadow-opacity flex max-h-[min(24rem,var(--available-height))] min-w-(--anchor-width) origin-(--transform-origin) flex-col overflow-hidden rounded-lg border shadow-lg outline-hidden",
            dankMenuContainerClassName,
            dankPopupMotionClassName,
            className,
          )}
          {...props}
        >
          <SelectPrimitive.List className="dank:px-0 dank:py-0.5 flex flex-col overflow-y-auto p-1">
            {children}
          </SelectPrimitive.List>
        </SelectPrimitive.Popup>
      </SelectPrimitive.Positioner>
    </SelectPrimitive.Portal>
  );
}

function SelectItem({ className, children, ...props }: SelectPrimitive.Item.Props) {
  return (
    <SelectPrimitive.Item
      data-slot="select-item"
      data-focus-indicator="child"
      className={cn(
        "data-highlighted:bg-accent data-highlighted:text-accent-foreground relative flex cursor-default items-center gap-2 rounded-md py-1.5 pr-8 pl-2 text-sm leading-tight outline-hidden select-none data-disabled:pointer-events-none data-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0",
        dankMenuItemClassName,
        dankMenuItemShapeClassName,
        dankMenuItemDisabledClassName,
        "dank:data-selected:bg-m3-tertiary-container dank:data-selected:text-m3-on-tertiary-container dank:data-selected:rounded-md dank:data-selected:[--menu-icon:currentColor] dank:pr-12",
        className,
      )}
      {...props}
    >
      <SelectPrimitive.ItemText className="dank:gap-3 dank:[&>svg]:size-5 dank:[&>svg]:text-(--menu-icon) flex min-w-0 flex-1 items-center gap-1.5">
        {children}
      </SelectPrimitive.ItemText>
      <SelectPrimitive.ItemIndicator className="dank:right-4 absolute right-2 flex items-center">
        <CheckIcon className="dank:size-5 size-4" />
      </SelectPrimitive.ItemIndicator>
    </SelectPrimitive.Item>
  );
}

function SelectGroupLabel({ className, ...props }: SelectPrimitive.GroupLabel.Props) {
  return (
    <SelectPrimitive.GroupLabel
      className={cn(
        "text-muted-foreground dank:min-h-8 dank:py-1.5 dank:pr-1 dank:pl-3 dank:type-label-large dank:text-m3-on-surface px-2 py-1 text-xs font-medium",
        className,
      )}
      {...props}
    />
  );
}

function SelectSeparator({ className, ...props }: SelectPrimitive.Separator.Props) {
  return (
    <SelectPrimitive.Separator
      className={cn(
        "bg-border dank:mx-3 dank:my-0.5 dank:bg-m3-outline-variant -mx-1 my-1 h-px",
        className,
      )}
      {...props}
    />
  );
}

const SelectGroup = SelectPrimitive.Group;

export type TSelectRootProps<Value> = React.ComponentProps<typeof SelectPrimitive.Root<Value>>;

export {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
  SelectGroup,
  SelectGroupLabel,
  SelectSeparator,
};
