import { cn } from "cn"

const GLYPH =
  "M6 0a.5.5 0 0 1 0 1 5 5 0 0 0 0 10 .5.5 0 0 1 0 1A6 6 0 1 1 6 0zm2 0a.5.5 0 0 1 .5.5v11a.5.5 0 0 1-1 0V.5A.5.5 0 0 1 8 0zm2 0a.5.5 0 0 1 .5.5v11a.5.5 0 0 1-1 0V.5A.5.5 0 0 1 10 0zm2 0a.5.5 0 0 1 .5.5v11a.5.5 0 0 1-1 0V.5A.5.5 0 0 1 12 0zm2 0a6 6 0 0 1 0 12 .5.5 0 0 1 0-1 5 5 0 0 0 0-10 .5.5 0 0 1 0-1zM6 2a.5.5 0 0 1 0 1 3 3 0 0 0 0 6 .5.5 0 0 1 0 1 4 4 0 1 1 0-8zm8 0a4 4 0 0 1 0 8 .5.5 0 0 1 0-1 3 3 0 1 0 0-6 .5.5 0 0 1 0-1zM6 4a.5.5 0 0 1 0 1 1 1 0 0 0 0 2 .5.5 0 0 1 0 1 2 2 0 1 1 0-4zm8 0a2 2 0 1 1 0 4 .5.5 0 0 1 0-1 1 1 0 1 0 0-2 .5.5 0 0 1 0-1z"

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden className={cn("size-6", className)}>
      <rect width="32" height="32" rx="8" className="fill-foreground" />
      <path transform="translate(3.2 8.32) scale(1.28)" d={GLYPH} className="fill-background" />
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
