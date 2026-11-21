import * as React from "react";
import { Menu as MenuPrimitive } from "@base-ui/react/menu";

import { CheckIcon, ChevronRightIcon, CircleIcon } from "@/components/icons";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";
import { useUiTheme } from "@/themes/ui-theme-provider";
import { cva, VariantProps } from "class-variance-authority";

function DropdownMenu({ highlightItemOnHover, ...props }: MenuPrimitive.Root.Props) {
  // On touch devices a tap fires synthesized mouse events at the tap point,
  // which would highlight whichever item happens to land under the finger as
  // the menu opens; there is no real hover to highlight there anyway
  const highlightOnHoverDefault = !window.matchMedia("(pointer: coarse)").matches;
  return (
    <MenuPrimitive.Root
      data-slot="dropdown-menu"
      highlightItemOnHover={highlightItemOnHover ?? highlightOnHoverDefault}
      {...props}
    />
  );
}

function DropdownMenuPortal({ ...props }: MenuPrimitive.Portal.Props) {
  return <MenuPrimitive.Portal data-slot="dropdown-menu-portal" {...props} />;
}

function DropdownMenuTrigger({ ...props }: MenuPrimitive.Trigger.Props) {
  return <MenuPrimitive.Trigger data-slot="dropdown-menu-trigger" {...props} />;
}

export const duxMenuItemClassName =
  "dux:relative dux:mx-1 dux:min-h-11 dux:gap-3 dux:rounded-xs dux:px-4 dux:py-2 dux:type-body-large dux:state-layer dux:[--accent:transparent] dux:[--menu-icon:var(--m3-on-surface-variant)] dux:[&.text-destructive]:[--menu-icon:currentColor] dux:[&>svg]:size-5 dux:[&>svg]:text-(--menu-icon) dux:transition-[border-radius,background-color] dux:duration-(--spring-fast-spatial-duration) dux:ease-spring-fast-spatial dux:focus-visible:outline-solid dux:focus-visible:outline-3 dux:focus-visible:-outline-offset-3 dux:focus-visible:outline-m3-secondary";

export const duxMenuItemShapeClassName =
  "dux:[&:not(:is([role^=menuitem],[role=option])~&)]:rounded-t-md dux:[&:not(:has(~:is([role^=menuitem],[role=option])))]:rounded-b-md";

export const duxMenuItemDisabledClassName =
  "dux:data-disabled:opacity-100 dux:data-disabled:text-m3-on-surface/38 dux:data-disabled:[--menu-icon:currentColor]";

export const duxMenuContainerClassName =
  "dux:bg-m3-surface-container-low dux:text-m3-on-surface dux:rounded-lg dux:border-0 dux:shadow-elevation-2 dux:[--tw-shadow-color:initial]";

export const duxPopupMotionClassName =
  "dux:popup-motion dux:data-open:animate-none dux:data-closed:animate-none";

export const dropdownContentVariants = cva(
  "bg-popover text-popover-foreground shadow-shadow-color/shadow-opacity flex max-h-[min(30rem,var(--available-height))] max-w-(--available-width) origin-(--transform-origin) flex-col overflow-hidden rounded-lg border p-0 shadow-lg outline-none dux:w-max dux:min-w-28 dux:max-w-[min(17.5rem,var(--available-width))] dux:gap-0.5 dux:overflow-visible dux:rounded-none dux:border-0 dux:bg-transparent dux:p-0 dux:text-m3-on-surface dux:shadow-none",
  {
    variants: {
      animate: {
        default: `data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-95 data-open:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 ${duxPopupMotionClassName}`,
        false: "",
      },
    },
    defaultVariants: {
      animate: "default",
    },
  },
);

type TDropdownContentVariants = VariantProps<typeof dropdownContentVariants>;

function DropdownMenuContent({
  className,
  align = "start",
  alignOffset = 0,
  side = "bottom",
  sideOffset = 4,
  collisionPadding = {
    top: 16,
    bottom: 16,
    left: 12,
    right: 12,
  },
  keepMounted,
  animate,
  children,
  ...props
}: MenuPrimitive.Popup.Props &
  Pick<
    MenuPrimitive.Positioner.Props,
    "align" | "alignOffset" | "side" | "sideOffset" | "collisionPadding"
  > &
  Pick<MenuPrimitive.Portal.Props, "keepMounted"> &
  TDropdownContentVariants) {
  return (
    <MenuPrimitive.Portal keepMounted={keepMounted}>
      <MenuPrimitive.Backdrop className="fixed inset-0 z-[1000]" />
      <MenuPrimitive.Positioner
        data-ui-popup=""
        align={align}
        alignOffset={alignOffset}
        side={side}
        sideOffset={sideOffset}
        collisionPadding={collisionPadding}
        className="isolate z-[1000] outline-none"
      >
        <MenuPrimitive.Popup
          data-slot="dropdown-menu-content"
          data-focus-indicator="child"
          className={cn(dropdownContentVariants({ animate, className }))}
          {...props}
        >
          <MenuPopupChildren>{children}</MenuPopupChildren>
        </MenuPrimitive.Popup>
      </MenuPrimitive.Positioner>
    </MenuPrimitive.Portal>
  );
}

function flatChildren(children: React.ReactNode, keyPrefix = ""): React.ReactNode[] {
  return React.Children.toArray(children).flatMap((child) => {
    if (!React.isValidElement<{ children?: React.ReactNode }>(child)) return [child];
    const key = `${keyPrefix}${child.key ?? ""}`;
    if (child.type === React.Fragment) return flatChildren(child.props.children, key);
    return [keyPrefix ? React.cloneElement(child, { key }) : child];
  });
}

function menuSegments(children: React.ReactNode) {
  const segments: React.ReactNode[][] = [[]];
  for (const child of flatChildren(children)) {
    const isSeparator = React.isValidElement(child) && child.type === DropdownMenuSeparator;
    if (isSeparator) segments.push([]);
    else segments[segments.length - 1].push(child);
  }
  return segments.filter((segment) => segment.length > 0);
}

function MenuPopupChildren({ children }: { children: React.ReactNode }) {
  const { id } = useUiTheme();
  if (id !== "dux") return children;
  return menuSegments(children).map((segment, index) => (
    <div
      key={index}
      role="none"
      data-slot="dropdown-menu-segment"
      className="bg-m3-surface-container-low shadow-elevation-2 flex min-h-0 flex-col overflow-y-auto rounded-sm py-0.5 first:rounded-t-lg last:rounded-b-lg"
    >
      {segment}
    </div>
  ));
}

function DropdownMenuGroup({ className, ...props }: MenuPrimitive.Group.Props) {
  return (
    <MenuPrimitive.Group
      data-slot="dropdown-menu-group"
      className={cn("dux:p-0 p-1", className)}
      {...props}
    />
  );
}

function DropdownMenuLabel({
  className,
  inset,
  ...props
}: MenuPrimitive.GroupLabel.Props & {
  inset?: boolean;
}) {
  return (
    <MenuPrimitive.GroupLabel
      data-slot="dropdown-menu-label"
      data-inset={inset}
      className={cn(
        "text-muted-foreground dux:min-h-8 dux:py-1.5 dux:pr-1 dux:pl-3 dux:type-label-large dux:text-m3-on-surface px-2 py-1 text-xs font-medium data-inset:pl-8",
        className,
      )}
      {...props}
    />
  );
}

function DropdownMenuItem({
  className,
  inset,
  fadeOnDisabled = true,
  // Wrapping triggers (e.g. DialogTrigger render={...}) inject tabIndex={0},
  // which makes the item tabbable so it steals focus and the highlight when
  // the menu opens; the menu's own roving tabIndex must stay in charge
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  tabIndex: _tabIndex,
  ...props
}: MenuPrimitive.Item.Props & {
  inset?: boolean;
  fadeOnDisabled?: boolean;
}) {
  return (
    <MenuPrimitive.Item
      data-slot="dropdown-menu-item"
      data-inset={inset}
      data-focus-indicator="child"
      className={cn(
        "active:bg-accent data-highlighted:bg-accent data-highlighted:text-accent-foreground relative flex cursor-default items-center justify-start gap-2 rounded-md px-2 py-1.5 text-sm leading-tight outline-hidden select-none data-disabled:pointer-events-none data-inset:pl-8 [&>svg]:shrink-0",
        duxMenuItemClassName,
        duxMenuItemShapeClassName,
        "dux:data-inset:pl-12 dux:data-checked:rounded-md dux:data-checked:bg-m3-tertiary-container dux:data-checked:text-m3-on-tertiary-container dux:data-checked:[--menu-icon:currentColor]",
        fadeOnDisabled && `data-disabled:opacity-50 ${duxMenuItemDisabledClassName}`,
        className,
      )}
      {...props}
    />
  );
}

function DropdownMenuSub({ ...props }: MenuPrimitive.SubmenuRoot.Props) {
  return <MenuPrimitive.SubmenuRoot data-slot="dropdown-menu-sub" {...props} />;
}

function DropdownMenuSubTrigger({
  className,
  inset,
  children,
  ...props
}: MenuPrimitive.SubmenuTrigger.Props & {
  inset?: boolean;
}) {
  return (
    <MenuPrimitive.SubmenuTrigger
      data-slot="dropdown-menu-sub-trigger"
      data-inset={inset}
      data-focus-indicator="child"
      className={cn(
        "data-highlighted:bg-accent data-popup-open:bg-accent flex cursor-default items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-hidden select-none data-inset:pl-8 [&_svg]:pointer-events-none [&_svg]:shrink-0",
        duxMenuItemClassName,
        duxMenuItemShapeClassName,
        "dux:data-inset:pl-12",
        className,
      )}
      {...props}
    >
      {children}
      <ChevronRightIcon className="ml-auto" />
    </MenuPrimitive.SubmenuTrigger>
  );
}

function DropdownMenuSubContent({
  align = "start",
  alignOffset = -3,
  side = "right",
  sideOffset = 0,
  className,
  ...props
}: React.ComponentProps<typeof DropdownMenuContent>) {
  return (
    <DropdownMenuContent
      data-slot="dropdown-menu-sub-content"
      className={cn("w-auto min-w-24 p-1", className)}
      align={align}
      alignOffset={alignOffset}
      side={side}
      sideOffset={sideOffset}
      {...props}
    />
  );
}

function DropdownMenuCheckboxItem({
  className,
  children,
  checked,
  ...props
}: MenuPrimitive.CheckboxItem.Props) {
  return (
    <MenuPrimitive.CheckboxItem
      data-slot="dropdown-menu-checkbox-item"
      data-focus-indicator="child"
      className={cn(
        "group/checkbox data-highlighted:bg-accent data-highlighted:text-accent-foreground relative flex cursor-default items-center justify-start rounded-md py-1.5 pr-2 pl-8 text-sm leading-tight outline-hidden select-none data-disabled:pointer-events-none data-disabled:opacity-50",
        duxMenuItemClassName,
        duxMenuItemShapeClassName,
        duxMenuItemDisabledClassName,
        className,
      )}
      checked={checked}
      {...props}
    >
      <span className="border-muted-more-foreground group-data-checked/checkbox:bg-foreground group-data-checked/checkbox:text-background group-data-checked/checkbox:border-foreground dux:static dux:mx-px dux:size-4.5 dux:rounded-2px dux:border-2 dux:border-m3-on-surface-variant dux:p-0 dux:group-data-checked/checkbox:border-m3-primary dux:group-data-checked/checkbox:bg-m3-primary dux:group-data-checked/checkbox:text-m3-on-primary absolute left-2 flex size-4 shrink-0 items-center justify-center rounded-sm border p-px">
        <MenuPrimitive.CheckboxItemIndicator className="size-full">
          <CheckIcon strokeWidth={3} className="size-full" />
        </MenuPrimitive.CheckboxItemIndicator>
      </span>
      {children}
    </MenuPrimitive.CheckboxItem>
  );
}

function DropdownMenuRadioGroup({ className, ...props }: MenuPrimitive.RadioGroup.Props) {
  return (
    <MenuPrimitive.RadioGroup
      data-slot="dropdown-menu-radio-group"
      className={cn(
        "bg-background dux:gap-0 dux:rounded-none dux:border-0 dux:bg-transparent dux:p-0 gap-0.5 rounded-lg border p-1",
        className,
      )}
      {...props}
    />
  );
}

function DropdownMenuRadioItem({
  className,
  children,
  hideIndicator,
  ...props
}: MenuPrimitive.RadioItem.Props & { hideIndicator?: boolean }) {
  return (
    <MenuPrimitive.RadioItem
      data-slot="dropdown-menu-radio-item"
      data-focus-indicator="child"
      className={cn(
        "group/radio data-highlighted:bg-accent data-highlighted:text-accent-foreground relative flex cursor-default items-center gap-2 rounded-sm py-1.5 pr-2 pl-8 text-sm outline-hidden select-none data-disabled:pointer-events-none data-disabled:opacity-50",
        duxMenuItemClassName,
        duxMenuItemShapeClassName,
        duxMenuItemDisabledClassName,
        hideIndicator && "pl-2",
        className,
      )}
      {...props}
    >
      {!hideIndicator && (
        <span className="dux:static dux:size-5 dux:rounded-full dux:border-2 dux:border-m3-on-surface-variant dux:group-data-checked/radio:border-m3-primary absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
          <MenuPrimitive.RadioItemIndicator className="dux:flex dux:items-center dux:justify-center">
            <CircleIcon className="dux:hidden h-2 w-2 fill-current" />
            <span className="bg-m3-primary dux:block hidden size-2.5 rounded-full" />
          </MenuPrimitive.RadioItemIndicator>
        </span>
      )}
      {children}
    </MenuPrimitive.RadioItem>
  );
}

/** A radio item styled as a pill button, for grids of presets. */
function DropdownMenuRadioButton({
  className,
  value,
  children,
  ...props
}: React.ComponentProps<typeof Button> & { value: MenuPrimitive.RadioItem.Props["value"] }) {
  return (
    <Button
      size="sm"
      variant="outline"
      render={<DropdownMenuRadioItem hideIndicator value={value} />}
      className={cn(
        "has-hover:hover:border-border active:border-border data-checked:border-border data-checked:data-highlighted:border-border data-checked:bg-popover text-muted-foreground data-checked:text-foreground data-checked:data-highlighted:bg-popover data-checked:data-highlighted:text-foreground data-highlighted:text-foreground shadow-shadow-color/shadow-opacity w-full justify-center rounded-[4px] border-transparent px-2 py-1.5 font-medium shadow-none active:bg-transparent has-hover:hover:bg-transparent data-checked:z-1 data-checked:shadow-md data-highlighted:z-1 data-highlighted:bg-transparent",
        className,
      )}
      {...props}
    >
      {children}
    </Button>
  );
}

function DropdownMenuSeparator({ className, ...props }: MenuPrimitive.Separator.Props) {
  return (
    <MenuPrimitive.Separator
      data-slot="dropdown-menu-separator"
      className={cn(
        "bg-border dux:mx-3 dux:my-0.5 dux:bg-m3-outline-variant pointer-events-none h-px shrink-0 rounded-full in-data-[slot=dropdown-menu-group]:-mx-1 in-data-[slot=dropdown-menu-group]:my-1 in-data-[slot=dropdown-menu-sub-content]:-mx-1 in-data-[slot=dropdown-menu-sub-content]:my-1",
        className,
      )}
      {...props}
    />
  );
}

function DropdownMenuShortcut({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="dropdown-menu-shortcut"
      className={cn(
        "dux:type-label-small dux:text-m3-on-surface-variant dux:opacity-100 ml-auto text-xs tracking-widest opacity-60",
        className,
      )}
      {...props}
    />
  );
}

export {
  MenuPopupChildren,
  DropdownMenu,
  DropdownMenuPortal,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuRadioButton,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
};
