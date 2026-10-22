import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";
import { TriangleIcon } from "lucide-react";

import type { Team } from "@/api/ops.gen";
import { filtersFromSearch, filtersToSearch, issueSearchSchema } from "@/components/issues/filters";
import { IssuesView } from "@/components/issues/issues-view";
import { TriageActions } from "@/components/issues/triage-actions";
import { EmptyState } from "@/components/layout/empty-state";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { LinkButton } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceTeamGate } from "@/components/workspace/workspace-team-gate";
import { FilterMenu } from "@/components/shared/filter-menu";

const searchSchema = issueSearchSchema.extend({
  tab: z.enum(["snoozed"]).optional().catch(undefined),
});

export const Route = createFileRoute("/$workspace/_workspace/triage")({
  validateSearch: zodValidator(searchSchema),
  component: TriagePage,
});

function TriagePage() {
  return <WorkspaceTeamGate>{(team) => <Triage team={team} />}</WorkspaceTeamGate>;
}

function Triage({ team }: { team: Team }) {
  const { tab, ...search } = Route.useSearch();
  const navigate = Route.useNavigate();
  const { workspace, isAdmin } = useWorkspace();
  if (!team.triageEnabled) {
    return (
      <PageContainer>
        <PageHeader title="Triage" icon={<TriangleIcon />} />
        <EmptyState
          icon={TriangleIcon}
          title="Triage is off"
          description="With triage on, new issues can land here first, to be accepted or declined."
          action={
            isAdmin && (
              <LinkButton
                to="/$workspace/settings/triage"
                params={{ workspace: workspace.urlKey }}
                variant="outline"
                size="sm"
              >
                Turn on triage
              </LinkButton>
            )
          }
        />
      </PageContainer>
    );
  }
  const isSnoozed = tab === "snoozed";
  return (
    <IssuesView
      key={`${team.id}:${tab}`}
      title="Triage"
      icon={<TriangleIcon />}
      storageKey={`team:${team.id}:triage`}
      teamId={team.id}
      defaultDisplay={{ group_by: "none", order_by: "created_at", order: "desc" }}
      fixedFilters={{
        state_types: ["triage"],
        snoozed: isSnoozed ? "only" : "hide",
      }}
      filters={filtersFromSearch(search)}
      onFiltersChange={(filters) =>
        void navigate({ search: (prev) => ({ ...prev, ...filtersToSearch(filters) }) })
      }
      rowActions={(issue) => <TriageActions issue={issue} />}
      tabs={
        <FilterMenu
          label="Show"
          value={tab ?? "inbox"}
          options={[
            { value: "inbox", label: "To triage" },
            { value: "snoozed", label: "Snoozed" },
          ]}
          onChange={(value) =>
            void navigate({
              search: (prev) => ({ ...prev, tab: value === "snoozed" ? "snoozed" : undefined }),
            })
          }
        />
      }
      emptyState={
        <EmptyState
          icon={TriangleIcon}
          title={isSnoozed ? "No snoozed issues yet" : "Triage is clear"}
          description={
            isSnoozed
              ? "Issues you snooze come back to triage when their time is up."
              : "New issues to accept or decline show up here."
          }
        />
      }
    />
  );
}
