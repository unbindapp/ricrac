import { cn } from "cn"

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden className={cn("size-6", className)}>
      <rect width="32" height="32" rx="7" className="fill-foreground" />
      <path
        d="M5 20l5.5-8 5.5 8 5.5-8 5.5 8"
        fill="none"
        className="stroke-background"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("flex items-center gap-2 text-lg font-semibold tracking-tight", className)}>
      <LogoMark />
      Ricrac
    </span>
  )
}
