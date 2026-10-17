import { initials } from "@/lib/helpers/colors";

type TProps = {
  name: string;
  logoUrl?: string | null;
};

export function WorkspaceIcon({ name, logoUrl }: TProps) {
  if (logoUrl) {
    return (
      <img aria-hidden src={logoUrl} alt="" className="size-5 shrink-0 rounded-md object-cover" />
    );
  }
  return (
    <span
      aria-hidden
      className="bg-foreground text-background inline-flex size-5 shrink-0 items-center justify-center rounded-md text-xs leading-none font-semibold"
    >
      {initials(name).slice(0, 1)}
    </span>
  );
}
