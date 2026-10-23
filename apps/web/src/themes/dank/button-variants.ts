import { cva } from "class-variance-authority";

import type { buttonVariants } from "@/components/ui/button";

const minTouchTarget =
  "before:absolute before:top-1/2 before:left-1/2 before:z-[-1] before:h-full before:min-h-12 before:w-full before:min-w-12 before:-translate-1/2 before:bg-transparent";

const filled =
  "disabled:bg-m3-on-surface/10 disabled:text-m3-on-surface-variant/38 disabled:shadow-none";

const outlined =
  "border border-m3-outline-variant bg-transparent disabled:text-m3-on-surface-variant/38";

const text = "bg-transparent disabled:text-m3-on-surface-variant/38";

/** Material 3 Expressive buttons: filled, tonal, outlined and text, in the extra small and small sizes. */
export const dankButtonVariants: typeof buttonVariants = cva(
  "state-layer relative z-0 max-w-full touch-manipulation select-none group/button text-center text-sm leading-5 font-medium tracking-[0.00625rem] focus-visible:z-[1] ease-spring-fast-spatial duration-(--spring-fast-spatial-duration) transition-[border-radius,box-shadow] disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&>[data-slot=button-spinner]+svg]:hidden data-pending:[&:not(:has(>[data-slot=button-spinner]+svg))>*:not([data-slot=button-spinner])]:opacity-0",
  {
    variants: {
      variant: {
        default: `bg-m3-primary text-m3-on-primary has-hover:hover:shadow-elevation-1 ${filled}`,
        destructive: `bg-m3-error text-m3-on-error has-hover:hover:shadow-elevation-1 ${filled}`,
        warning: `bg-warning text-warning-foreground has-hover:hover:shadow-elevation-1 ${filled}`,
        success: `bg-success text-success-foreground has-hover:hover:shadow-elevation-1 ${filled}`,
        process: `bg-process text-process-foreground has-hover:hover:shadow-elevation-1 ${filled}`,
        change: `bg-change text-change-foreground has-hover:hover:shadow-elevation-1 ${filled}`,
        outline: `text-m3-on-surface-variant ${outlined}`,
        "outline-muted": `text-m3-on-surface-variant ${outlined}`,
        "outline-foreground": `text-m3-on-surface ${outlined}`,
        "outline-process": `text-process ${outlined}`,
        "outline-change": `text-change ${outlined}`,
        "warning-outline": `text-warning ${outlined}`,
        secondary: `bg-m3-secondary-container text-m3-on-secondary-container has-hover:hover:shadow-elevation-1 ${filled}`,
        tonal: `bg-m3-secondary-container text-m3-on-secondary-container has-hover:hover:shadow-elevation-1 ${filled}`,
        ghost: `text-m3-primary ${text}`,
        "ghost-destructive": `text-m3-error ${text}`,
        "ghost-success": `text-success ${text}`,
        "ghost-warning": `text-warning ${text}`,
        "ghost-warning-foreground": `text-m3-on-surface ${text}`,
        "ghost-process": `text-process ${text}`,
        "ghost-change": `text-change ${text}`,
        "ghost-change-foreground": `text-m3-on-surface ${text}`,
        "ghost-foreground": `text-m3-on-surface ${text}`,
        card: "bg-card text-m3-on-surface",
        link: "text-m3-primary underline-offset-4 after:hidden has-hover:hover:underline",
      },
      size: {
        default:
          "h-10 gap-2 rounded-[1.25rem] px-4 active:rounded-sm data-popup-open:rounded-sm [&>svg]:size-5",
        sm: "h-8 gap-2 rounded-[1rem] px-3 active:rounded-sm data-popup-open:rounded-sm [&>svg]:size-5",
        icon: "flex size-10 shrink-0 items-center justify-center rounded-[1.25rem] active:rounded-sm data-popup-open:rounded-sm [&>svg]:size-6",
      },
      loadingState: {
        default: "",
        loading: "",
      },
      fadeOnDisabled: {
        default: "",
        false: "",
      },
      forceMinSize: {
        default: minTouchTarget,
        medium: minTouchTarget,
        false: "",
      },
      focusVariant: {
        default: "",
        "input-like": "",
      },
      layout: {
        default: "",
        flex: "inline-flex items-center justify-center",
      },
    },
    compoundVariants: [
      {
        variant: "ghost",
        size: "icon",
        className: "text-m3-on-surface-variant",
      },
    ],
    defaultVariants: {
      variant: "default",
      size: "default",
      loadingState: "default",
      fadeOnDisabled: "default",
      focusVariant: "default",
      forceMinSize: "default",
      layout: "default",
    },
  },
);
