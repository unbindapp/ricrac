import { cn } from "@/components/ui/utils";

const glyph =
  "M15 0a1 1 0 0 1 1 1v22a1 1 0 1 1-2 0V1a1 1 0 0 1 1-1m4 0a1 1 0 0 1 1 1v22a1 1 0 1 1-2 0V1a1 1 0 0 1 1-1m4 0a1 1 0 0 1 1 1v22a1 1 0 1 1-2 0V1a1 1 0 0 1 1-1M10.91.049a1 1 0 0 1 .18 1.992C5.994 2.501 2 6.784 2 12s3.994 9.5 9.09 9.959a1 1 0 0 1-.18 1.992C4.793 23.4 0 18.261 0 12 0 5.74 4.793.6 10.91.049m-.052 4.032a1 1 0 0 1 .284 1.98 6.001 6.001 0 0 0 0 11.878 1 1 0 0 1-.284 1.98 8.003 8.003 0 0 1 0-15.838m-.191 4.148a1 1 0 1 1 .666 1.884 2.002 2.002 0 0 0 0 3.774 1 1 0 0 1-.666 1.886 4.002 4.002 0 0 1 0-7.544";

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="-8 -8 40 40" aria-hidden className={cn("size-5 shrink-0", className)}>
      <rect x="-8" y="-8" width="40" height="40" rx="10" className="fill-logo" />
      <path d={glyph} className="fill-logo-foreground" />
    </svg>
  );
}

export function Logo() {
  return (
    <span className="inline-flex items-center gap-2">
      <LogoMark className="size-8" />
      <span className="text-xl leading-tight font-semibold">Ricrac</span>
    </span>
  );
}
