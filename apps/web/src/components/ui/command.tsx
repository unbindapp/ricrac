import * as React from "react";
import { type Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { Command as CommandPrimitive } from "cmdk";
import { LoaderIcon, SearchIcon } from "@/components/icons";
import { cn } from "@/components/ui/utils";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { duxMenuItemClassName } from "@/components/ui/dropdown-menu";
import { useUiTheme } from "@/themes/ui-theme-provider";
import { cva, VariantProps } from "class-variance-authority";

const commandVariants = cva(
  "flex h-full w-full flex-col overflow-hidden rounded-lg focus:outline-hidden",
  {
    variants: {
      variant: {
        default: "bg-popover text-popover-foreground dux:bg-transparent",
        modal: "bg-background text-foreground",
        searchView:
          "bg-popover text-popover-foreground dux:bg-popover dux:text-m3-on-surface dux:rounded-xl dux:border-0 dux:shadow-elevation-3 dux:[--tw-shadow-color:initial] dux:[&_[cmdk-list]]:p-2 dux:[&_[cmdk-item]]:min-h-14 dux:[&_[cmdk-item]>svg]:size-6 dux:[&_[cmdk-item][data-selected=true]]:rounded-lg dux:[&_[cmdk-group]_[cmdk-group-heading]]:type-title-small dux:[&_[cmdk-group]_[cmdk-group-heading]]:text-m3-on-surface-variant dux:[&_[cmdk-group]_[cmdk-group-heading]]:min-h-12 dux:[&_[cmdk-group]_[cmdk-group-heading]]:px-5",
      },
      wrapper: {
        default:
          "rounded-lg h-120 max-h-[min(calc(100vh-(100vh-3rem)*0.1-7rem),50vh)] sm:max-h-[calc(100vh-(100vh-3rem)*0.1-7rem)] border shadow-xl shadow-shadow-color/shadow-opacity",
        none: "",
      },
    },
    defaultVariants: {
      variant: "default",
      wrapper: "default",
    },
  },
);

export type TCommandVariants = VariantProps<typeof commandVariants>;

function Command({
  className,
  variant,
  wrapper,
  ...props
}: React.ComponentProps<typeof CommandPrimitive> & TCommandVariants) {
  return (
    <CommandPrimitive
      className={cn(
        commandVariants({
          variant,
          wrapper,
          className,
        }),
      )}
      {...props}
    />
  );
}

type CommandDialogProps = Omit<DialogPrimitive.Root.Props, "children"> & {
  children?: React.ReactNode;
};

function CommandDialog({ children, ...props }: CommandDialogProps) {
  return (
    <Dialog {...props}>
      <DialogContent className="overflow-hidden p-0">
        <Command className="[&_[cmdk-group-heading]]:text-muted-foreground [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group]]:px-2 [&_[cmdk-group]:not([hidden])_~[cmdk-group]]:pt-0 [&_[cmdk-input-wrapper]_svg]:h-5 [&_[cmdk-input-wrapper]_svg]:w-5 [&_[cmdk-input]]:h-12 [&_[cmdk-item]]:px-2 [&_[cmdk-item]]:py-3">
          {children}
        </Command>
      </DialogContent>
    </Dialog>
  );
}

function CommandInput({
  className,
  classNameWrapper,
  showSpinner = false,
  Icon,
  hideIcon,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Input> & {
  classNameWrapper?: string;
  showSpinner?: boolean;
  Icon?: React.ComponentType<{ className?: string }>;
  hideIcon?: boolean;
}) {
  const styles = useUiTheme().id === "dux" ? duxCommandInputStyles : commandInputStyles;
  return (
    <div className={cn(styles.wrapper, classNameWrapper)} cmdk-input-wrapper="">
      <div data-show-spinner={showSpinner || undefined} className={styles.icon}>
        {!showSpinner &&
          (Icon ? (
            <Icon className={styles.glyph} />
          ) : (
            !hideIcon && <SearchIcon className={styles.glyph} />
          ))}
        {showSpinner && <LoaderIcon className="absolute size-full animate-spin" />}
      </div>
      <CommandPrimitive.Input className={cn(styles.input, className)} {...props} />
    </div>
  );
}

const commandInputStyles = {
  wrapper: "relative flex items-center border-b",
  icon: "text-muted-foreground group pointer-events-none absolute left-3.5 size-4 shrink-0",
  glyph: "text-muted-foreground size-full",
  input:
    "placeholder:text-muted-foreground flex w-full rounded-md bg-transparent py-3 pr-4 pl-9.5 text-sm leading-tight outline-hidden disabled:cursor-not-allowed disabled:opacity-50",
};

const duxCommandInputStyles = {
  wrapper: "relative flex h-14 shrink-0 items-center border-b border-m3-outline",
  icon: "text-m3-on-surface group pointer-events-none absolute left-4 size-6 shrink-0",
  glyph: "size-full",
  input:
    "type-body-large text-m3-on-surface placeholder:text-m3-on-surface-variant caret-m3-primary flex h-full w-full bg-transparent pr-4 pl-14 outline-hidden disabled:cursor-not-allowed disabled:text-m3-on-surface/38",
};

function CommandList({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.List>) {
  return (
    <CommandPrimitive.List
      className={cn(
        "dux:p-2 dux:flex dux:flex-col dux:gap-0.5 p-1 [&_[cmdk-group]:not([hidden])~[cmdk-group]:not([hidden])]:mt-1",
        className,
      )}
      {...props}
    />
  );
}

function CommandEmpty(props: React.ComponentProps<typeof CommandPrimitive.Empty>) {
  return (
    <CommandPrimitive.Empty
      className="text-muted-foreground px-4 py-6 text-center text-sm"
      {...props}
    />
  );
}

function CommandGroup({
  className,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Group>) {
  return (
    <CommandPrimitive.Group
      className={cn(
        "group/command text-foreground [&_[cmdk-group-heading]]:text-muted-foreground overflow-hidden [&_[cmdk-group-heading]]:flex [&_[cmdk-group-heading]]:items-center [&_[cmdk-group-heading]]:gap-3 [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium",
        "dux:text-m3-on-surface dux:[&_[cmdk-group-heading]]:type-label-large dux:[&_[cmdk-group-heading]]:text-m3-on-surface dux:[&_[cmdk-group-heading]]:min-h-8 dux:[&_[cmdk-group-heading]]:py-1.5 dux:[&_[cmdk-group-heading]]:pr-1 dux:[&_[cmdk-group-heading]]:pl-3",
        className,
      )}
      {...props}
    />
  );
}

function CommandSeparator({
  className,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Separator>) {
  return (
    <CommandPrimitive.Separator
      className={cn(
        "bg-border dux:mx-3 dux:my-0.5 dux:bg-m3-outline-variant -mx-1 my-1 h-px",
        className,
      )}
      {...props}
    />
  );
}

const commandItemVariants = cva(
  `relative flex cursor-default leading-tight gap-2 select-none items-center rounded-md px-2 py-2 text-sm outline-hidden data-[disabled=true]:pointer-events-none data-[selected=true]:bg-border data-[disabled=true]:data-[selected=true]:bg-transparent data-[selected=true]:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 ${duxMenuItemClassName} dux:not-data-checked:data-[selected=true]:bg-transparent dux:data-[selected=true]:after:opacity-10 dux:data-checked:rounded-md dux:data-checked:bg-m3-tertiary-container dux:data-checked:text-m3-on-tertiary-container dux:data-checked:[--menu-icon:currentColor]`,
  {
    variants: {
      variant: {
        default: "",
      },
      state: {
        default: "",
        pending: "opacity-100 data-[disabled=true]:opacity-100",
      },
      fadeOnDisabled: {
        default:
          "disabled:opacity-50 data-[disabled=true]:opacity-50 dux:data-[disabled=true]:opacity-100 dux:data-[disabled=true]:text-m3-on-surface/38 dux:data-[disabled=true]:[--menu-icon:currentColor]",
        false: "",
      },
    },
    defaultVariants: {
      variant: "default",
      fadeOnDisabled: "default",
    },
  },
);

function CommandItem({
  className,
  state,
  disabled,
  variant,
  fadeOnDisabled,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Item> & VariantProps<typeof commandItemVariants>) {
  return (
    <CommandPrimitive.Item
      className={cn(commandItemVariants({ state, variant, fadeOnDisabled }), className)}
      disabled={state === "pending" ? true : disabled}
      {...props}
    />
  );
}

function CommandShortcut({ className, ...props }: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={cn(
        "text-muted-foreground dux:type-label-small dux:text-m3-on-surface-variant ml-auto text-xs tracking-widest",
        className,
      )}
      {...props}
    />
  );
}

export {
  Command,
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandShortcut,
  CommandSeparator,
};
