import { FlameIcon } from "lucide-react";

import { cn } from "@/components/ui/utils";
import { colorFromId, initials } from "@/lib/helpers/colors";

type TLogoProps = {
  id: string;
  name: string;
  logoUrl?: string | null;
  size?: "sm" | "md" | "lg";
};

/** The customer's logo, or its initials on a color of its own. */
export function CustomerLogo({ id, name, logoUrl, size = "md" }: TLogoProps) {
  const className = cn(
    "inline-flex shrink-0 items-center justify-center overflow-hidden rounded-sm font-semibold leading-none text-white",
    size === "sm" && "size-4 text-xs",
    size === "md" && "size-5 text-xs",
    size === "lg" && "size-10 rounded-lg text-base",
  );
  if (logoUrl) {
    return <img src={logoUrl} alt="" className={cn(className, "bg-muted object-cover")} />;
  }
  return (
    <span aria-hidden className={className} style={{ backgroundColor: colorFromId(id) }}>
      {initials(name).slice(0, size === "lg" ? 2 : 1)}
    </span>
  );
}

/** A status or tier: a dot of its color and its name. */
export function OptionDot({ color, className }: { color: string; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn("size-2.5 shrink-0 rounded-full", className)}
      style={{ backgroundColor: color }}
    />
  );
}

export function OptionLabel({ name, color }: { name: string; color: string }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <OptionDot color={color} />
      <span className="truncate">{name}</span>
    </span>
  );
}

/** Marks a need the customer said is important. */
export function ImportantIcon({ className }: { className?: string }) {
  return (
    <FlameIcon aria-label="Important" className={cn("text-warning size-4 shrink-0", className)} />
  );
}
