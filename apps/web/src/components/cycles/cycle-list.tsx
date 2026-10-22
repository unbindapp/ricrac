import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import type { Cycle, Team } from "@/api/ops.gen";
import { BurnUpChart } from "@/components/cycles/burn-up-chart";
import { CycleProgressBar } from "@/components/cycles/cycle-progress-bar";
import {
  cycleDateRange,
  cycleStatus,
  daysLeft,
  formatNumber,
  pointsText,
  type TCycleStatus,
} from "@/components/cycles/cycle-status";
import { RefreshCwIcon } from "@/components/icons/symbols.gen";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SectionTitle } from "@/components/layout/section-title";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { cycleName, cyclesQuery } from "@/lib/queries/cycles";

/** "9 issues", or "21 of 30 points" once issues are estimated. */
export function scopeText(cycle: Cycle) {
  const { scope } = cycle;
  if (scope.estimate > 0) {
    return `${formatNumber(scope.completedEstimate)} of ${formatNumber(scope.estimate)} points`;
  }
  return `${scope.completed} of ${scope.issues} ${scope.issues === 1 ? "issue" : "issues"}`;
}

export function CycleList({ team }: { team: Team }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(cyclesQuery(workspace.id, team.id, true));
  const [showArchived, setShowArchived] = useState(false);
  const cycles = data ?? [];
  const archivedCount = cycles.filter((cycle) => cycle.archivedAt).length;
  const shown = cycles.filter((cycle) => showArchived || !cycle.archivedAt);
  const inStatus = (status: TCycleStatus) => shown.filter((cycle) => cycleStatus(cycle) === status);
  const current = inStatus("active");
  const upcoming = inStatus("upcoming").reverse();
  const past = inStatus("past");

  return (
    <ListStates
      isPending={isPending}
      error={error}
      onRetry={() => void refetch()}
      isEmpty={cycles.length === 0}
      empty={{
        icon: RefreshCwIcon,
        title: "No cycles yet",
        description: "The next cycles are planned within a minute of turning cycles on.",
      }}
    >
      <div className="flex w-full flex-col gap-6">
        {current.map((cycle) => (
          <CurrentCycleCard key={cycle.id} cycle={cycle} />
        ))}
        {upcoming.length > 0 && <CycleSection title="Upcoming" cycles={upcoming} />}
        {past.length > 0 && (
          <CycleSection
            title="Completed"
            cycles={past}
            actions={
              archivedCount > 0 && (
                <Button
                  variant="ghost"
                  size="sm"
                  aria-pressed={showArchived}
                  onClick={() => setShowArchived((value) => !value)}
                  className="text-muted-foreground"
                >
                  {showArchived ? "Hide archived" : `Show ${archivedCount} archived`}
                </Button>
              )
            }
          />
        )}
      </div>
    </ListStates>
  );
}

function CurrentCycleCard({ cycle }: { cycle: Cycle }) {
  const { workspace } = useWorkspace();
  const left = daysLeft(cycle);
  return (
    <section className="flex w-full flex-col gap-2">
      <SectionTitle title="Current" />
      <Card className="flex w-full flex-col gap-4 p-4">
        <div className="flex w-full flex-wrap items-center gap-2">
          <CycleIcon progress={cycle.progress} status="active" className="size-4" />
          <Link
            to="/$workspace/cycle/$number"
            params={{ workspace: workspace.urlKey, number: String(cycle.number) }}
            className="truncate text-sm font-medium hover:underline"
          >
            {cycleName(cycle)}
          </Link>
          <span className="text-muted-foreground text-sm">{cycleDateRange(cycle)}</span>
          <Badge variant="process" className="ml-auto">
            {left === 0 ? "Ends today" : `${left} ${left === 1 ? "day" : "days"} left`}
          </Badge>
        </div>
        {cycle.scope.issues > 0 ? (
          <CycleProgress cycle={cycle} />
        ) : (
          <p className="text-muted-foreground text-sm">No issues in this cycle yet.</p>
        )}
      </Card>
    </section>
  );
}

/** Progress bar, issue counts (with points once estimated) and the burn-up chart. */
export function CycleProgress({ cycle }: { cycle: Cycle }) {
  const { scope } = cycle;
  const estimated = scope.estimate > 0;
  return (
    <>
      <div className="flex w-full items-center gap-3">
        <CycleProgressBar scope={scope} label={`${cycleName(cycle)} progress`} />
        <span className="shrink-0 text-sm font-medium tabular-nums">
          {Math.round(cycle.progress * 100)}%
        </span>
      </div>
      <dl className="grid w-full grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat
          label="Scope"
          value={String(scope.issues)}
          detail={estimated ? pointsText(scope.estimate) : null}
        />
        <Stat
          label="Started"
          value={String(scope.started)}
          detail={estimated ? pointsText(scope.startedEstimate) : null}
          swatch="bg-chart-4"
        />
        <Stat
          label="Completed"
          value={String(scope.completed)}
          detail={estimated ? pointsText(scope.completedEstimate) : null}
          swatch="bg-chart-1"
        />
        <Stat label="Not started" value={String(scope.unstarted)} />
      </dl>
      <BurnUpChart cycle={cycle} />
    </>
  );
}

function Stat({
  label,
  value,
  detail,
  swatch,
}: {
  label: string;
  value: string;
  detail?: string | null;
  swatch?: string;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <dt className="text-muted-foreground flex items-center gap-1.5 text-xs">
        {swatch && <span aria-hidden className={`size-2 rounded-full ${swatch}`} />}
        {label}
      </dt>
      <dd className="flex items-baseline gap-1.5 truncate text-sm font-medium tabular-nums">
        {value}
        {detail && <span className="text-muted-foreground text-xs font-normal">{detail}</span>}
      </dd>
    </div>
  );
}

function CycleSection({
  title,
  cycles,
  actions,
}: {
  title: string;
  cycles: Cycle[];
  actions?: ReactNode;
}) {
  return (
    <section className="flex w-full flex-col gap-2">
      <SectionTitle title={title} actions={actions} />
      <List>
        {cycles.map((cycle) => (
          <CycleRow key={cycle.id} cycle={cycle} />
        ))}
      </List>
    </section>
  );
}

function CycleRow({ cycle }: { cycle: Cycle }) {
  const { workspace } = useWorkspace();
  const status = cycleStatus(cycle);
  const hasIssues = cycle.scope.issues > 0;
  return (
    <Link
      to="/$workspace/cycle/$number"
      params={{ workspace: workspace.urlKey, number: String(cycle.number) }}
      role="listitem"
      className="has-hover:hover:bg-accent focus-visible:bg-accent flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0"
    >
      <CycleIcon progress={cycle.progress} status={status} className="size-4" />
      <span className="min-w-0 truncate">{cycleName(cycle)}</span>
      {cycle.archivedAt && <Badge variant="muted">Archived</Badge>}
      <span className="text-muted-foreground min-w-0 flex-1 truncate text-xs">
        {cycleDateRange(cycle)}
      </span>
      {hasIssues && (
        <>
          <span className="text-muted-foreground hidden shrink-0 text-xs sm:inline">
            {scopeText(cycle)}
          </span>
          <span className="hidden w-24 shrink-0 md:flex">
            <CycleProgressBar scope={cycle.scope} label={`${cycleName(cycle)} progress`} />
          </span>
          <span className="text-muted-foreground w-9 shrink-0 text-right text-xs tabular-nums">
            {Math.round(cycle.progress * 100)}%
          </span>
        </>
      )}
    </Link>
  );
}
