import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";

import { filtersFromSearch, filtersToSearch, issueSearchSchema } from "@/components/issues/filters";
import { TeamIssues } from "@/components/issues/team-issues";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/backlog")({
  validateSearch: zodValidator(issueSearchSchema),
  component: TeamKeyBacklogPage,
});

function TeamKeyBacklogPage() {
  const team = useRequiredTeamFromRoute();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <TeamIssues
      team={team}
      tab="backlog"
      filters={filtersFromSearch(search)}
      onFiltersChange={(filters) =>
        void navigate({ search: (prev) => ({ ...prev, ...filtersToSearch(filters) }) })
      }
    />
  );
}
