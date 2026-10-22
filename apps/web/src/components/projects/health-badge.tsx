import { CircleAlertIcon, CircleCheckIcon, CircleXIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { THealth } from "@/lib/queries/projects";

export const HEALTHS: THealth[] = ["onTrack", "atRisk", "offTrack"];

export const healthLabels: Record<THealth, string> = {
  onTrack: "On track",
  atRisk: "At risk",
  offTrack: "Off track",
};

const healthVariants = {
  onTrack: "success",
  atRisk: "warning",
  offTrack: "destructive",
} as const;

export function HealthIcon({ health }: { health: THealth }) {
  switch (health) {
    case "onTrack":
      return <CircleCheckIcon className="text-success size-4 shrink-0" aria-hidden />;
    case "atRisk":
      return <CircleAlertIcon className="text-warning size-4 shrink-0" aria-hidden />;
    case "offTrack":
      return <CircleXIcon className="text-destructive size-4 shrink-0" aria-hidden />;
  }
}

/** The health of the latest status update; nothing until someone posts one. */
export function HealthBadge({ health }: { health: THealth | null | undefined }) {
  if (!health) return null;
  return (
    <Badge variant={healthVariants[health]}>
      <HealthIcon health={health} />
      {healthLabels[health]}
    </Badge>
  );
}
