import { StateIcon, type TStateType } from "@/components/shared/state-icon";
import { cn } from "@/components/ui/utils";
import type { TProjectStatusType } from "@/lib/queries/projects";

export const PROJECT_STATUS_TYPES: TProjectStatusType[] = [
  "backlog",
  "planned",
  "started",
  "paused",
  "completed",
  "canceled",
];

export const projectStatusTypeLabels: Record<TProjectStatusType, string> = {
  backlog: "Backlog",
  planned: "Planned",
  started: "In progress",
  paused: "Paused",
  completed: "Completed",
  canceled: "Canceled",
};

const iconTypes: Record<Exclude<TProjectStatusType, "paused">, TStateType> = {
  backlog: "backlog",
  planned: "unstarted",
  started: "started",
  completed: "completed",
  canceled: "canceled",
};

type TProps = {
  type: TProjectStatusType;
  color?: string | null;
  name?: string;
  className?: string;
};

export function ProjectStatusIcon({ type, color, name, className }: TProps) {
  const label = name ?? projectStatusTypeLabels[type];
  if (type !== "paused") {
    return <StateIcon type={iconTypes[type]} color={color} name={label} className={className} />;
  }
  return (
    <svg
      viewBox="0 0 14 14"
      role="img"
      aria-label={label}
      className={cn("size-3.5 shrink-0", !color && "text-muted-foreground", className)}
      style={color ? { color } : undefined}
    >
      <circle cx="7" cy="7" r="6" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path d="M5.5 4.75v4.5M8.5 4.75v4.5" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}
