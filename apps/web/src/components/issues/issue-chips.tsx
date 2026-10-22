import { parseISO } from "date-fns";

import type { Issue } from "@/api/ops.gen";
import { CalendarIcon, FlameIcon, MoonIcon } from "@/components/icons";
import { useNow } from "@/components/providers/now-provider";
import { ProjectIcon } from "@/components/shared/project-icon";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { formatDate, formatDateTime, formatRelative } from "@/lib/helpers/dates";

export function ProjectChip({
  project,
}: {
  project: { name: string; icon?: string | null; color?: string | null };
}) {
  return (
    <span className="bg-background text-foreground inline-flex h-5 max-w-40 shrink-0 items-center gap-1.5 rounded-full border px-2 text-xs leading-none">
      <ProjectIcon icon={project.icon} color={project.color} size="sm" />
      <span className="truncate">{project.name}</span>
    </span>
  );
}

const isClosed = (issue: Pick<Issue, "state">) =>
  issue.state.type === "completed" || issue.state.type === "canceled";

function isOverdue(dueDate: string, closed: boolean, now: number) {
  if (closed) return false;
  const due = parseISO(dueDate);
  due.setHours(23, 59, 59, 999);
  return due.getTime() < now;
}

export function DueDateText({ issue }: { issue: Pick<Issue, "dueDate" | "state"> }) {
  const now = useNow();
  if (!issue.dueDate) return null;
  const overdue = isOverdue(issue.dueDate, isClosed(issue), now);
  const text = formatDate(parseISO(issue.dueDate));
  return (
    <SimpleTooltip content={overdue ? `Overdue, due ${text}` : `Due ${text}`}>
      <span
        data-overdue={overdue || undefined}
        className="text-muted-foreground data-overdue:text-destructive inline-flex shrink-0 items-center gap-1.5 text-xs whitespace-nowrap"
      >
        <CalendarIcon className="size-3.5" />
        {text}
      </span>
    </SimpleTooltip>
  );
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** When an open issue's SLA breaches: red once breached, amber within a day. */
export function SlaChip({ issue }: { issue: Pick<Issue, "slaBreachesAt" | "state"> }) {
  const now = useNow();
  if (!issue.slaBreachesAt || isClosed(issue)) return null;
  const left = new Date(issue.slaBreachesAt).getTime() - now;
  const tone = left < 0 ? "breached" : left < DAY_MS ? "soon" : "later";
  const text = left < 0 ? "SLA breached" : `SLA ${formatRelative(issue.slaBreachesAt, now)}`;
  return (
    <SimpleTooltip content={`SLA breaches ${formatDateTime(issue.slaBreachesAt)}`}>
      <span
        data-tone={tone}
        className="text-muted-foreground data-[tone=breached]:text-destructive data-[tone=soon]:text-warning inline-flex shrink-0 items-center gap-1.5 text-xs whitespace-nowrap"
      >
        <FlameIcon className="size-3.5" />
        {text}
      </span>
    </SimpleTooltip>
  );
}

export function SnoozedChip({ issue }: { issue: Pick<Issue, "snoozedUntil"> }) {
  const now = useNow();
  if (!issue.snoozedUntil || new Date(issue.snoozedUntil).getTime() < now) return null;
  return (
    <SimpleTooltip content={`Snoozed until ${formatDateTime(issue.snoozedUntil)}`}>
      <span className="text-muted-foreground inline-flex shrink-0 items-center gap-1.5 text-xs whitespace-nowrap">
        <MoonIcon className="size-3.5" />
        Snoozed
      </span>
    </SimpleTooltip>
  );
}
