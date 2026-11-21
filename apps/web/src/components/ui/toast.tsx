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
        "dux:inset-x-3 dux:bottom-3 dux:mx-auto dux:max-w-150 dux:sm:inset-x-3 dux:sm:w-auto dux:sm:max-w-150",
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
        "dux:bg-m3-inverse-surface dux:text-m3-inverse-on-surface dux:shadow-elevation-3 dux:rounded-xs dux:border-0 dux:[--tw-shadow-color:initial]",
        "dux:[transition:transform_var(--spring-fast-spatial-duration)_var(--ease-spring-fast-spatial),scale_var(--spring-fast-spatial-duration)_var(--ease-spring-fast-spatial),opacity_var(--spring-fast-effects-duration)_var(--ease-spring-fast-effects),height_150ms]",
        "dux:data-starting-style:[transform:none] dux:data-starting-style:scale-80 dux:data-starting-style:opacity-0 dux:data-ending-style:scale-80 dux:data-ending-style:opacity-0",
        "dux:[&[data-ending-style]:not([data-limited]):not([data-swipe-direction])]:[transform:translateX(var(--toast-swipe-movement-x))_translateY(calc(var(--toast-swipe-movement-y)+var(--dir)*(var(--toast-index)*var(--peek)+var(--shrink)*var(--height))))_scale(var(--scale))]",
        "dux:[&[data-expanded][data-ending-style]:not([data-limited]):not([data-swipe-direction])]:[transform:translateX(var(--toast-swipe-movement-x))_translateY(var(--offset-y))]",
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
        "dux:type-body-medium dux:items-center dux:gap-0 dux:py-0 dux:pr-12 dux:pl-4",
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
        "dux:text-m3-inverse-on-surface dux:group-data-[type]/toast:text-m3-inverse-on-surface dux:type-body-medium",
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
        "dux:text-m3-inverse-on-surface dux:group-data-[type]/toast:text-m3-inverse-on-surface dux:type-body-medium",
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
        "text-muted-more-foreground dux:top-1/2 dux:right-1 dux:-translate-y-1/2 dux:text-m3-inverse-on-surface absolute top-1 right-1",
        className,
      )}
      render={
        <Button variant="ghost" size="icon" className="dux:size-10 size-8">
          <XIcon className="dux:size-6 size-4" />
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
      className="text-foreground group-data-[type=error]/toast:text-destructive group-data-[type=success]/toast:text-success group-data-[type=warning]/toast:text-warning line-icon dux:hidden [&_svg]:pointer-events-none"
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
        <div className="dux:flex-row dux:items-center dux:gap-2 flex w-full min-w-0 shrink flex-col items-start justify-start gap-2.5">
          <div className="dux:flex-1 dux:gap-0 dux:py-3.5 flex w-full min-w-0 shrink flex-col gap-1">
            <ToastTitle />
            <ToastDescription />
          </div>
          {toastItem.data?.action && (
            <div className="dux:w-auto dux:shrink-0 dux:pb-0 dux:*:h-10 dux:*:border-0 dux:*:bg-transparent dux:*:px-3 dux:*:text-m3-inverse-primary flex w-full pb-0.5">
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
