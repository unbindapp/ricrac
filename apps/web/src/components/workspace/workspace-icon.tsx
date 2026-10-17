import { cn } from "@/components/ui/utils";
import { initials } from "@/lib/helpers/colors";

type TProps = {
  name: string;
  logoUrl?: string | null;
  size?: "md" | "lg";
};

export function WorkspaceIcon({ name, logoUrl, size = "md" }: TProps) {
  const className = cn(
    "shrink-0 rounded-md",
    size === "md" && "size-5",
    size === "lg" && "size-10 rounded-lg",
  );
  if (logoUrl) {
    return <img aria-hidden src={logoUrl} alt="" className={cn(className, "object-cover")} />;
  }
  return (
    <span
      aria-hidden
      className={cn(
        className,
        "bg-foreground text-background inline-flex items-center justify-center leading-none font-semibold",
        size === "md" ? "text-xs" : "text-base",
      )}
    >
      {initials(name).slice(0, 1)}
    </span>
  );
}
