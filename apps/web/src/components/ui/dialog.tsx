import * as React from "react";
import { XIcon } from "@/components/icons";
import { pressStartedInExemptElement } from "@/components/ui/press-origin";
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";

import { cn } from "@/components/ui/utils";
import { cva, VariantProps } from "class-variance-authority";
import { Button, ButtonVariantMapContext, type TButtonVariants } from "@/components/ui/button";
import { returnFocusTarget } from "@/components/ui/focus-origin";
import { useUiTheme } from "@/themes/ui-theme-provider";

const createDialogHandle = DialogPrimitive.createHandle;
export type TDialogHandle = DialogPrimitive.Handle<unknown>;

function Dialog({ onOpenChange, ...props }: DialogPrimitive.Root.Props) {
  const handleOpenChange: DialogPrimitive.Root.Props["onOpenChange"] = (open, eventDetails) => {
    if (!open && eventDetails.reason === "outside-press") {
      if (pressStartedInExemptElement(eventDetails.event)) {
        eventDetails.cancel();
        return;
      }
    }
    onOpenChange?.(open, eventDetails);
  };
  return <DialogPrimitive.Root data-slot="dialog" onOpenChange={handleOpenChange} {...props} />;
}

function DialogTrigger({ ...props }: DialogPrimitive.Trigger.Props) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />;
}

function DialogPortal({ ...props }: DialogPrimitive.Portal.Props) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />;
}

function DialogClose({ ...props }: DialogPrimitive.Close.Props) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />;
}

const dialogLayerVariants = {
  default: "z-[1000]",
};

const dialogOverlayVariants = cva("bg-barrier/barrier fixed inset-0", {
  variants: {
    layer: dialogLayerVariants,
    animate: {
      default:
        "data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 duration-200 data-closed:duration-200 data-open:duration-200 dank:data-open:duration-(--spring-default-effects-duration) dank:data-closed:duration-(--spring-default-effects-duration) dank:ease-spring-default-effects",
      false: "",
    },
  },
  defaultVariants: {
    layer: "default",
    animate: "default",
  },
});

type TDialogOverlayVariants = VariantProps<typeof dialogOverlayVariants>;

function DialogOverlay({
  className,
  animate,
  layer,
  ...props
}: DialogPrimitive.Backdrop.Props & TDialogOverlayVariants) {
  return (
    <DialogPrimitive.Backdrop
      forceRender
      data-slot="dialog-overlay"
      className={cn(dialogOverlayVariants({ animate, layer, className }))}
      {...props}
    />
  );
}

const dialogViewportVariants = cva(
  "fixed inset-0 flex w-full justify-center overflow-auto px-2 pt-[var(--dialog-top-padding)] sm:pt-[var(--dialog-top-padding-sm)]",
  {
    variants: {
      layer: dialogLayerVariants,
      avoidKeyboard: {
        true: "pb-[calc(var(--dialog-bottom-padding)+var(--keyboard-inset-height))] sm:pb-[calc(var(--dialog-bottom-padding-sm)+var(--keyboard-inset-height))]",
        false: "pb-[var(--dialog-bottom-padding)] sm:pb-[var(--dialog-bottom-padding-sm)]",
      },
    },
    defaultVariants: {
      layer: "default",
      avoidKeyboard: false,
    },
  },
);

export const dialogContentVariants = cva(
  "relative z-50 my-auto flex w-auto max-w-full flex-col gap-5 outline-hidden focus:outline-hidden",
  {
    variants: {
      variant: {
        default:
          "bg-background shadow-dialog shadow-shadow-color/shadow-opacity rounded-lg border p-5 pt-4 dank:bg-m3-surface-container-high dank:text-m3-on-surface dank:rounded-xl dank:border-0 dank:p-6 dank:shadow-elevation-3 dank:[--tw-shadow-color:initial]",
        styleless: "",
      },
      animate: {
        default:
          "duration-200 data-open:duration-200 data-closed:duration-200 data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-[95%] data-open:zoom-in-[95%] data-closed:slide-out-to-bottom-[5%] data-open:slide-in-from-bottom-[5%] dank:data-open:duration-(--spring-default-spatial-duration) dank:data-open:ease-spring-default-spatial dank:data-closed:duration-(--spring-fast-effects-duration) dank:data-closed:ease-spring-fast-effects",
        false: "",
      },
    },
    defaultVariants: {
      variant: "default",
      animate: "default",
    },
  },
);

export type TDialogContentVariants = VariantProps<typeof dialogContentVariants>;

function DialogContent({
  className,
  classNameInnerWrapper,
  variant,
  animate,
  children,
  hideXButton,
  headerActions,
  avoidKeyboard,
  layer,
  ...props
}: DialogPrimitive.Popup.Props &
  TDialogContentVariants & {
    classNameInnerWrapper?: string;
    hideXButton?: boolean;
    /** Icon buttons in the top right corner, beside the close button. */
    headerActions?: React.ReactNode;
    avoidKeyboard?: boolean;
    layer?: keyof typeof dialogLayerVariants;
  }) {
  return (
    <DialogPortal>
      <DialogOverlay animate={animate} layer={layer} />
      <DialogPrimitive.Viewport
        data-slot="dialog-viewport"
        className={cn(dialogViewportVariants({ avoidKeyboard, layer }))}
      >
        <DialogPrimitive.Popup
          data-slot="dialog-content"
          data-focus-indicator="child"
          className={cn(dialogContentVariants({ variant, animate }), className)}
          finalFocus={returnFocusTarget}
          {...props}
        >
          <div className={cn("flex w-full flex-col gap-4", classNameInnerWrapper)}>
            {children}
            {(headerActions || (!hideXButton && variant !== "styleless")) && (
              <div className="dank:top-5 dank:right-4 absolute top-3 right-3 flex items-center gap-1">
                {headerActions}
                {!hideXButton && variant !== "styleless" && (
                  <DialogPrimitive.Close
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="Close"
                        className="text-muted-foreground"
                      >
                        <XIcon className="size-4" />
                      </Button>
                    }
                  />
                )}
              </div>
            )}
          </div>
        </DialogPrimitive.Popup>
      </DialogPrimitive.Viewport>
    </DialogPortal>
  );
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("dank:gap-4 flex w-full flex-col items-start gap-2", className)}
      {...props}
    />
  );
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  const isDank = useUiTheme().id === "dank";
  return (
    <ButtonVariantMapContext value={isDank ? asTextAction : null}>
      <div
        data-slot="dialog-footer"
        className={cn(
          "dank:mt-2 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
          className,
        )}
        {...props}
      />
    </ButtonVariantMapContext>
  );
}

const textActionVariants: Partial<Record<string, TButtonVariants["variant"]>> = {
  default: "ghost",
  secondary: "ghost",
  tonal: "ghost",
  outline: "ghost",
  destructive: "ghost-destructive",
};

/** Material dialogs act with text buttons. */
function asTextAction(variant: NonNullable<TButtonVariants["variant"]>) {
  return textActionVariants[variant] ?? variant;
}

function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn(
        "dank:type-headline-small dank:text-m3-on-surface max-w-full pr-8 text-base leading-tight font-semibold wrap-break-word",
        className,
      )}
      {...props}
    />
  );
}

function DialogDescription({ className, ...props }: DialogPrimitive.Description.Props) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn(
        "text-muted-foreground dank:type-body-medium dank:text-m3-on-surface-variant max-w-full text-sm",
        className,
      )}
      {...props}
    />
  );
}

export {
  createDialogHandle,
  Dialog,
  DialogPortal,
  DialogOverlay,
  DialogTrigger,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
};
