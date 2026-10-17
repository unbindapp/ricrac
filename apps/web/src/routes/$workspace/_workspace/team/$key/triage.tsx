import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { TriangleIcon } from "lucide-react";
import { z } from "zod";

import { filtersFromSearch, filtersToSearch, issueSearchSchema } from "@/components/issues/filters";
import { IssuesView } from "@/components/issues/issues-view";
import { TriageActions } from "@/components/issues/triage-actions";
import { EmptyState } from "@/components/layout/empty-state";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { LinkButton } from "@/components/ui/button";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { teamCrumb } from "@/components/workspace/team-crumb";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";

const searchSchema = issueSearchSchema.extend({
  tab: z.enum(["snoozed"]).optional().catch(undefined),
});

export const Route = createFileRoute("/$workspace/_workspace/team/$key/triage")({
  validateSearch: zodValidator(searchSchema),
  component: TeamKeyTriagePage,
});

function TeamKeyTriagePage() {
  const team = useRequiredTeamFromRoute();
  const { tab, ...search } = Route.useSearch();
  const navigate = Route.useNavigate();
  const { workspace, isAdmin } = useWorkspace();
  const breadcrumb = [teamCrumb(team, workspace.urlKey)];
  if (!team.triageEnabled) {
    return (
      <PageContainer>
        <PageHeader title="Triage" icon={<TriangleIcon />} breadcrumb={breadcrumb} />
        <EmptyState
          icon={TriangleIcon}
          title="Triage is off for this team"
          description="With triage on, issues from outside the team land here first, to be accepted or declined."
          action={
            isAdmin && (
              <LinkButton
                to="/$workspace/settings/team/$key/triage"
                params={{ workspace: workspace.urlKey, key: team.key }}
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
      breadcrumb={breadcrumb}
      storageKey={`team:${team.id}:triage`}
      teamId={team.id}
      defaultDisplay={{ group_by: "none", order_by: "created_at", order: "desc" }}
      fixedFilters={{
        team_ids: [team.id],
        state_types: ["triage"],
        snoozed: isSnoozed ? "only" : "hide",
      }}
      filters={filtersFromSearch(search)}
      onFiltersChange={(filters) =>
        void navigate({ search: (prev) => ({ ...prev, ...filtersToSearch(filters) }) })
      }
      rowActions={(issue) => <TriageActions issue={issue} />}
      tabs={
        <Tabs
          value={tab ?? "inbox"}
          onValueChange={(value) =>
            void navigate({
              search: (prev) => ({ ...prev, tab: value === "snoozed" ? "snoozed" : undefined }),
            })
          }
        >
          <TabsList>
            <TabsTab value="inbox">To triage</TabsTab>
            <TabsTab value="snoozed">Snoozed</TabsTab>
          </TabsList>
        </Tabs>
      }
      emptyState={
        <EmptyState
          icon={TriangleIcon}
          title={isSnoozed ? "No snoozed issues yet" : "Triage is clear"}
          description={
            isSnoozed
              ? "Issues you snooze come back to triage when their time is up."
              : "New issues from outside the team show up here to be accepted or declined."
          }
        />
      }
    />
  );
}
