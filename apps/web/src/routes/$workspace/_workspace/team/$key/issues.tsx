import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { filtersFromSearch, filtersToSearch, issueSearchSchema } from "@/components/issues/filters";
import { TeamIssues } from "@/components/issues/team-issues";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";

const searchSchema = issueSearchSchema.extend({
  tab: z.enum(["all"]).optional().catch(undefined),
});

export const Route = createFileRoute("/$workspace/_workspace/team/$key/issues")({
  validateSearch: zodValidator(searchSchema),
  component: TeamKeyIssuesPage,
});

function TeamKeyIssuesPage() {
  const team = useRequiredTeamFromRoute();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const { tab, ...rest } = search;
  return (
    <TeamIssues
      team={team}
      tab={tab ?? "active"}
      filters={filtersFromSearch(rest)}
      onFiltersChange={(filters) =>
        void navigate({ search: (prev) => ({ ...prev, ...filtersToSearch(filters) }) })
      }
    />
  );
}
