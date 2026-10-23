import { Toast as ToastPrimitive } from "@base-ui/react/toast";
import * as React from "react";

import {
  CheckCircleIcon,
  CircleAlertIcon,
  InfoIcon,
  LoaderIcon,
  TriangleAlertIcon,
  XIcon,
} from "@/components/icons";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";

type TToastData = {
  icon?: React.ReactNode;
  action?: React.ReactNode;
};

const toast = ToastPrimitive.createToastManager<TToastData>();

function ToastPortal({ ...props }: ToastPrimitive.Portal.Props) {
  return <ToastPrimitive.Portal data-slot="toast-portal" {...props} />;
}

function ToastViewport({ className, ...props }: ToastPrimitive.Viewport.Props) {
  return (
    <ToastPrimitive.Viewport
      data-slot="toast-viewport"
      className={cn(
        "pointer-events-none fixed inset-x-4 bottom-4 z-2000 w-auto font-sans outline-none sm:right-4 sm:left-auto sm:w-full sm:max-w-sm",
        "dank:inset-x-3 dank:bottom-3 dank:mx-auto dank:max-w-150 dank:sm:inset-x-3 dank:sm:w-auto dank:sm:max-w-150",
        className,
      )}
      {...props}
    />
  );
}

function Toast({ className, ...props }: ToastPrimitive.Root.Props) {
  return (
    <ToastPrimitive.Root
      data-slot="toast"
      className={cn(
        "group/toast border-border bg-background text-foreground shadow-shadow-color/shadow-opacity pointer-events-auto absolute right-0 bottom-0 z-[calc(1000-var(--toast-index))] w-full origin-bottom rounded-lg border shadow-lg will-change-transform outline-none select-none",
        "[--dir:-1]",
        "[--gap:0.75rem] [--height:var(--toast-frontmost-height,var(--toast-height))] [--offset-y:calc(var(--dir)*(var(--toast-offset-y)+var(--toast-index)*var(--gap))+var(--toast-swipe-movement-y))] [--peek:0.75rem] [--scale:calc(max(0,1-(var(--toast-index)*0.1)))] [--shrink:calc(1-var(--scale))]",
        "h-(--height) [transform:translateX(var(--toast-swipe-movement-x))_translateY(calc(var(--toast-swipe-movement-y)+var(--dir)*(var(--toast-index)*var(--peek)+var(--shrink)*var(--height))))_scale(var(--scale))] [transition:transform_500ms_cubic-bezier(0.22,1,0.36,1),opacity_500ms,height_150ms]",
        "after:absolute after:top-full after:left-0 after:h-[calc(var(--gap)+1px)] after:w-full after:content-['']",
        "data-expanded:h-(--toast-height) data-expanded:[transform:translateX(var(--toast-swipe-movement-x))_translateY(var(--offset-y))]",
        "data-limited:opacity-0 data-starting-style:[transform:translateY(calc(var(--dir)*-150%))]",
        "[&[data-ending-style]:not([data-limited]):not([data-swipe-direction])]:[transform:translateY(calc(var(--dir)*-150%))]",
        "data-ending-style:data-[swipe-direction=down]:[transform:translateY(calc(var(--toast-swipe-movement-y)+150%))]",
        "data-ending-style:data-[swipe-direction=left]:[transform:translateX(calc(var(--toast-swipe-movement-x)-150%))_translateY(var(--offset-y))]",
        "data-ending-style:data-[swipe-direction=right]:[transform:translateX(calc(var(--toast-swipe-movement-x)+150%))_translateY(var(--offset-y))]",
        "data-ending-style:data-[swipe-direction=up]:[transform:translateY(calc(var(--toast-swipe-movement-y)-150%))]",
        "data-expanded:data-ending-style:data-[swipe-direction=down]:[transform:translateY(calc(var(--toast-swipe-movement-y)+150%))]",
        "data-expanded:data-ending-style:data-[swipe-direction=left]:[transform:translateX(calc(var(--toast-swipe-movement-x)-150%))_translateY(var(--offset-y))]",
        "data-expanded:data-ending-style:data-[swipe-direction=right]:[transform:translateX(calc(var(--toast-swipe-movement-x)+150%))_translateY(var(--offset-y))]",
        "data-expanded:data-ending-style:data-[swipe-direction=up]:[transform:translateY(calc(var(--toast-swipe-movement-y)-150%))]",
        "dank:bg-m3-inverse-surface dank:text-m3-inverse-on-surface dank:shadow-elevation-3 dank:rounded-xs dank:border-0 dank:[--tw-shadow-color:initial]",
        "dank:[transition:transform_var(--spring-fast-spatial-duration)_var(--ease-spring-fast-spatial),scale_var(--spring-fast-spatial-duration)_var(--ease-spring-fast-spatial),opacity_var(--spring-fast-effects-duration)_var(--ease-spring-fast-effects),height_150ms]",
        "dank:data-starting-style:[transform:none] dank:data-starting-style:scale-80 dank:data-starting-style:opacity-0 dank:data-ending-style:scale-80 dank:data-ending-style:opacity-0",
        "dank:[&[data-ending-style]:not([data-limited]):not([data-swipe-direction])]:[transform:translateX(var(--toast-swipe-movement-x))_translateY(calc(var(--toast-swipe-movement-y)+var(--dir)*(var(--toast-index)*var(--peek)+var(--shrink)*var(--height))))_scale(var(--scale))]",
        "dank:[&[data-expanded][data-ending-style]:not([data-limited]):not([data-swipe-direction])]:[transform:translateX(var(--toast-swipe-movement-x))_translateY(var(--offset-y))]",
        className,
      )}
      {...props}
    />
  );
}

function ToastContent({ className, ...props }: ToastPrimitive.Content.Props) {
  return (
    <ToastPrimitive.Content
      data-slot="toast-content"
      className={cn(
        "flex h-full gap-2 overflow-hidden px-4 py-3.5 pr-10 leading-tight transition-opacity duration-250 ease-[cubic-bezier(0.22,1,0.36,1)] data-behind:opacity-0 data-expanded:opacity-100",
        "dank:type-body-medium dank:items-center dank:gap-0 dank:py-0 dank:pr-12 dank:pl-4",
        className,
      )}
      {...props}
    />
  );
}

function ToastTitle({ className, ...props }: ToastPrimitive.Title.Props) {
  return (
    <ToastPrimitive.Title
      data-slot="toast-title"
      className={cn(
        "text-foreground group-data-[type=error]/toast:text-destructive group-data-[type=success]/toast:text-success group-data-[type=warning]/toast:text-warning font-semibold",
        "dank:text-m3-inverse-on-surface dank:group-data-[type]/toast:text-m3-inverse-on-surface dank:type-body-medium",
        className,
      )}
      {...props}
    />
  );
}

function ToastDescription({ className, ...props }: ToastPrimitive.Description.Props) {
  return (
    <ToastPrimitive.Description
      data-slot="toast-description"
      className={cn(
        "text-muted-foreground group-data-[type=error]/toast:text-foreground group-data-[type=success]/toast:text-foreground group-data-[type=warning]/toast:text-foreground text-sm leading-snug",
        "dank:text-m3-inverse-on-surface dank:group-data-[type]/toast:text-m3-inverse-on-surface dank:type-body-medium",
        className,
      )}
      {...props}
    />
  );
}

function ToastClose({ className, ...props }: ToastPrimitive.Close.Props) {
  return (
    <ToastPrimitive.Close
      data-slot="toast-close"
      aria-label="Close toast"
      className={cn(
        "text-muted-more-foreground dank:top-1/2 dank:right-1 dank:-translate-y-1/2 dank:text-m3-inverse-on-surface absolute top-1 right-1",
        className,
      )}
      render={
        <Button variant="ghost" size="icon" className="dank:size-10 size-8">
          <XIcon className="dank:size-6 size-4" />
        </Button>
      }
      {...props}
    ></ToastPrimitive.Close>
  );
}

const toastIcons: Record<string, React.ReactNode> = {
  success: <CheckCircleIcon className="size-4" />,
  info: <InfoIcon className="size-4" />,
  warning: <CircleAlertIcon className="size-4" />,
  error: <TriangleAlertIcon className="size-4" />,
  loading: <LoaderIcon className="size-4 animate-spin" />,
};

function ToastIcon({ icon, type }: { icon: React.ReactNode; type: string | undefined }) {
  const resolvedIcon = icon ?? (type ? toastIcons[type] : null);
  if (!resolvedIcon) return null;

  return (
    <span
      data-slot="toast-icon"
      className="text-foreground group-data-[type=error]/toast:text-destructive group-data-[type=success]/toast:text-success group-data-[type=warning]/toast:text-warning line-icon dank:hidden [&_svg]:pointer-events-none"
    >
      {resolvedIcon}
    </span>
  );
}

const swipeDirectionMobile: ToastPrimitive.Root.Props["swipeDirection"] = ["down"];
const swipeDirectionDesktop: ToastPrimitive.Root.Props["swipeDirection"] = ["down", "right"];

function ToastList() {
  const { toasts } = ToastPrimitive.useToastManager<TToastData>();
  const { isExtraSmall } = useDeviceSize();
  const swipeDirection = isExtraSmall ? swipeDirectionMobile : swipeDirectionDesktop;

  return toasts.map((toastItem) => (
    <Toast key={toastItem.id} toast={toastItem} swipeDirection={swipeDirection}>
      <ToastContent>
        <ToastIcon icon={toastItem.data?.icon} type={toastItem.type} />
        <div className="dank:flex-row dank:items-center dank:gap-2 flex w-full min-w-0 shrink flex-col items-start justify-start gap-2.5">
          <div className="dank:flex-1 dank:gap-0 dank:py-3.5 flex w-full min-w-0 shrink flex-col gap-1">
            <ToastTitle />
            <ToastDescription />
          </div>
          {toastItem.data?.action && (
            <div className="dank:w-auto dank:shrink-0 dank:pb-0 dank:*:h-10 dank:*:border-0 dank:*:bg-transparent dank:*:px-3 dank:*:text-m3-inverse-primary flex w-full pb-0.5">
              {toastItem.data?.action}
            </div>
          )}
        </div>
        <ToastClose />
      </ToastContent>
    </Toast>
  ));
}

function Toaster({
  toastManager = toast,
  timeout = 5000,
  ...props
}: ToastPrimitive.Provider.Props) {
  return (
    <ToastPrimitive.Provider toastManager={toastManager} timeout={timeout} {...props}>
      <ToastPortal>
        <ToastViewport>
          <ToastList />
        </ToastViewport>
      </ToastPortal>
    </ToastPrimitive.Provider>
  );
}

export { toast, Toaster };
