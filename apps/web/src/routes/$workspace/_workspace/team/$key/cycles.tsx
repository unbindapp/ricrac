import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { PlusIcon, RefreshCwIcon, SettingsIcon } from "lucide-react";
import { useState } from "react";

import { CycleDialog, type TCycleDialogTarget } from "@/components/cycles/cycle-dialog";
import { CycleList } from "@/components/cycles/cycle-list";
import { EmptyState } from "@/components/layout/empty-state";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { Button, LinkButton } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { teamCrumb } from "@/components/workspace/team-crumb";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { cyclesQuery } from "@/lib/queries/cycles";
import { teamQuery } from "@/lib/queries/teams";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/cycles")({
  component: TeamCyclesPage,
});

function TeamCyclesPage() {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const team = useRequiredTeamFromRoute();
  const params = { workspace: workspace.urlKey, key: team.key };
  const { data: cycles } = useQuery({
    ...cyclesQuery(workspace.id, team.id, true),
    enabled: team.cyclesEnabled,
  });
  const { data: detail } = useQuery({ ...teamQuery(workspace.id, team.id), enabled: canEdit });
  const [dialog, setDialog] = useState<TCycleDialogTarget | null>(null);

  const openNewCycle = () => {
    const lastEnd = Math.max(
      Date.now(),
      ...(cycles ?? []).map((cycle) => Date.parse(cycle.endsAt)),
    );
    setDialog({
      mode: "create",
      startsOn: new Date(lastEnd),
      weeks: detail?.cycleDurationWeeks ?? 2,
    });
  };

  return (
    <PageContainer>
      <PageHeader
        title="Cycles"
        icon={<RefreshCwIcon />}
        breadcrumb={[teamCrumb(team, workspace.urlKey), { label: "Cycles" }]}
        actions={
          team.cyclesEnabled && (
            <>
              {isAdmin && (
                <SimpleTooltip content="Cycle settings">
                  <LinkButton
                    to="/$workspace/settings/team/$key/cycles"
                    params={params}
                    variant="ghost"
                    size="icon"
                    aria-label="Cycle settings"
                  >
                    <SettingsIcon className="size-4" />
                  </LinkButton>
                </SimpleTooltip>
              )}
              {canEdit && (
                <Button size="sm" onClick={openNewCycle}>
                  <PlusIcon className="size-4" />
                  New cycle
                </Button>
              )}
            </>
          )
        }
      />
      {team.cyclesEnabled ? (
        <CycleList team={team} />
      ) : (
        <EmptyState
          icon={RefreshCwIcon}
          title="Cycles are off for this team"
          description="Cycles are repeating time boxes, like two-week sprints. Unfinished issues can roll over to the next one."
          action={
            isAdmin && (
              <LinkButton
                to="/$workspace/settings/team/$key/cycles"
                params={params}
                variant="outline"
                size="sm"
              >
                Turn on cycles
              </LinkButton>
            )
          }
        />
      )}
      <CycleDialog team={team} target={dialog} onClose={() => setDialog(null)} />
    </PageContainer>
  );
}
