import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import type { CycleDetail, Team } from "@/api/ops.gen";
import { CycleDialog } from "@/components/cycles/cycle-dialog";
import { CycleProgress } from "@/components/cycles/cycle-list";
import { CycleMenu } from "@/components/cycles/cycle-menu";
import {
  cycleDateRange,
  cycleStatus,
  cycleStatusLabels,
  daysLeft,
} from "@/components/cycles/cycle-status";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import { CircleDotIcon, RefreshCwIcon } from "@/components/icons";
import { IssuesByState } from "@/components/issues/issues-by-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { SectionTitle } from "@/components/layout/section-title";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceTeamGate } from "@/components/workspace/workspace-team-gate";
import { cycleName, cycleQuery } from "@/lib/queries/cycles";
import { isApiError } from "@/lib/server/errors";

export const Route = createFileRoute("/$workspace/_workspace/cycle/$number")({
  component: CyclePage,
});

function CyclePage() {
  return <WorkspaceTeamGate>{(team) => <Cycle team={team} />}</WorkspaceTeamGate>;
}

function Cycle({ team }: { team: Team }) {
  const { number } = Route.useParams();
  const { workspace, canEdit } = useWorkspace();
  const cycleNumber = Number(number);
  const isValidNumber = Number.isInteger(cycleNumber) && cycleNumber > 0;
  const {
    data: cycle,
    isPending,
    error,
    refetch,
  } = useQuery({
    ...cycleQuery(workspace.id, team.id, cycleNumber),
    enabled: isValidNumber,
  });
  const [isEditing, setIsEditing] = useState(false);
  const title = cycle ? cycleName(cycle) : `Cycle ${number}`;

  const body = () => {
    const isMissing = isApiError(error) && error.status >= 400 && error.status < 500;
    if (error && !isMissing) {
      return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
    }
    if (!isValidNumber || error) {
      return <EntityNotFound Icon={RefreshCwIcon} description={`There is no cycle ${number}.`} />;
    }
    if (isPending) return <ListSkeleton />;
    return <CycleBody cycle={cycle} />;
  };

  return (
    <PageContainer>
      <PageHeader
        title={title}
        icon={
          cycle ? (
            <CycleIcon progress={cycle.progress} status={cycleStatus(cycle)} className="size-5" />
          ) : (
            <RefreshCwIcon />
          )
        }
        breadcrumb={[
          {
            label: "Cycles",
            link: { to: "/$workspace/cycles", params: { workspace: workspace.urlKey } },
          },
        ]}
        actions={
          cycle && (
            <>
              <FavoriteButton kind="cycle" entityId={cycle.id} />
              {canEdit && <CycleMenu cycle={cycle} onEdit={() => setIsEditing(true)} />}
            </>
          )
        }
      />
      {body()}
      {cycle && (
        <CycleDialog
          team={team}
          target={isEditing ? { mode: "edit", cycle } : null}
          onClose={() => setIsEditing(false)}
        />
      )}
    </PageContainer>
  );
}

function CycleBody({ cycle }: { cycle: CycleDetail }) {
  const status = cycleStatus(cycle);
  const left = daysLeft(cycle);
  return (
    <>
      <Card className="flex w-full flex-col gap-4 p-4">
        <div className="flex w-full flex-wrap items-center gap-2">
          <Badge variant={status === "active" ? "process" : "muted"}>
            {cycleStatusLabels[status]}
          </Badge>
          {cycle.archivedAt && <Badge variant="muted">Archived</Badge>}
          <span className="text-muted-foreground text-sm">{cycleDateRange(cycle)}</span>
          {status === "active" && (
            <span className="text-muted-foreground ml-auto text-sm">
              {left === 0 ? "Ends today" : `${left} ${left === 1 ? "day" : "days"} left`}
            </span>
          )}
        </div>
        {cycle.description && <p className="text-sm whitespace-pre-wrap">{cycle.description}</p>}
        {cycle.scope.issues > 0 && <CycleProgress cycle={cycle} />}
      </Card>
      <section className="flex w-full flex-col gap-2">
        <SectionTitle title="Issues" />
        <IssuesByState
          issues={cycle.issues ?? []}
          empty={{
            icon: CircleDotIcon,
            title: "No issues in this cycle yet",
            description:
              "Add issues to the cycle from an issue's properties, or let started issues join it on their own in cycle settings.",
          }}
        />
      </section>
    </>
  );
}
