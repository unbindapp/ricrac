import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import { UserCircleIcon } from "@/components/icons/symbols.gen";
import {
  filtersFromSearch,
  filtersToSearch,
  issueSearchSchema,
  type ListIssuesFilters,
  type TFilterField,
} from "@/components/issues/filters";
import { IssuesView } from "@/components/issues/issues-view";
import { EmptyState } from "@/components/layout/empty-state";
import { FilterMenu } from "@/components/shared/filter-menu";
import { meQuery } from "@/lib/queries/me";

const TABS = ["assigned", "created", "subscribed"] as const;
type TMyIssuesTab = (typeof TABS)[number];

const tabLabels: Record<TMyIssuesTab, string> = {
  assigned: "Assigned",
  created: "Created",
  subscribed: "Subscribed",
};

const searchSchema = issueSearchSchema.extend({
  tab: z.enum(TABS).optional().catch(undefined),
});

export const Route = createFileRoute("/$workspace/_workspace/my-issues")({
  validateSearch: zodValidator(searchSchema),
  component: MyIssuesPage,
});

function fixedFiltersFor(tab: TMyIssuesTab, userId: string): ListIssuesFilters {
  switch (tab) {
    case "assigned":
      return { assignee_ids: [userId] };
    case "created":
      return { creator_ids: [userId] };
    case "subscribed":
      return { subscriber_id: userId };
  }
}

const emptyTitles: Record<TMyIssuesTab, string> = {
  assigned: "No issues assigned to you yet",
  created: "No issues created by you yet",
  subscribed: "No subscribed issues yet",
};

const emptyDescriptions: Record<TMyIssuesTab, string> = {
  assigned: "Issues assigned to you show up here.",
  created: "Issues you create show up here.",
  subscribed:
    "Issues you follow (you created, were assigned or mentioned, or subscribed) show up here.",
};

const hiddenFieldsFor: Record<TMyIssuesTab, TFilterField[]> = {
  assigned: ["assignee"],
  created: ["creator"],
  subscribed: ["subscriber"],
};

function MyIssuesPage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const { data: me } = useQuery(meQuery);
  const { tab: selected, ...rest } = search;
  const tab = selected ?? "assigned";
  if (!me) return null;
  return (
    <IssuesView
      key={tab}
      title="My issues"
      icon={<UserCircleIcon />}
      storageKey={`my-issues:${tab}`}
      fixedFilters={fixedFiltersFor(tab, me.id)}
      hiddenFilterFields={hiddenFieldsFor[tab]}
      defaultDisplay={{ group_by: "state" }}
      filters={filtersFromSearch(rest)}
      emptyState={
        <EmptyState
          icon={UserCircleIcon}
          title={emptyTitles[tab]}
          description={emptyDescriptions[tab]}
        />
      }
      onFiltersChange={(filters) =>
        void navigate({ search: (prev) => ({ ...prev, ...filtersToSearch(filters) }) })
      }
      tabs={
        <FilterMenu
          label="Issues"
          value={tab}
          options={TABS.map((item) => ({ value: item, label: tabLabels[item] }))}
          onChange={(value) => void navigate({ search: (prev) => ({ ...prev, tab: value }) })}
        />
      }
    />
  );
}
