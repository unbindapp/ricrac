import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { useDebounceValue } from "usehooks-ts";
import { CircleDotIcon, PlusIcon, XIcon } from "lucide-react";

import type { ReleaseDetail, ReleaseIssue } from "@/api/ops.gen";
import { PropertyPicker } from "@/components/issues/property-picker";
import { EmptyState } from "@/components/layout/empty-state";
import { SectionTitle } from "@/components/layout/section-title";
import { Avatar } from "@/components/shared/avatar";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { issueOptionsQuery } from "@/lib/queries/entity-options";
import { useReleaseIssuesMutation } from "@/lib/queries/releases";

const SEARCH_DEBOUNCE_MS = 200;

/**
 * The issues a release ships. Rows are drawn here from the release's short issue shape rather
 * than with the issues area's IssueRow, which needs full issues.
 */
export function ReleaseIssues({ release }: { release: ReleaseDetail }) {
  const { canEdit } = useWorkspace();
  const issues = release.issues;
  const done = issues.filter((issue) => issue.state.type === "completed").length;
  const summary = issues.length === 0 ? undefined : `${done} of ${issues.length} done`;
  return (
    <section aria-label="Issues" className="flex flex-col gap-3">
      <SectionTitle
        title="Issues"
        description={summary}
        actions={canEdit && issues.length > 0 && <AddIssuePicker release={release} />}
      />
      {issues.length === 0 ? (
        <EmptyState
          icon={CircleDotIcon}
          title="No issues yet"
          description="Add the issues that ship in this release."
          action={canEdit && <AddIssuePicker release={release} variant="outline" />}
        />
      ) : (
        <div role="list" className="flex w-full flex-col overflow-hidden rounded-lg border">
          {issues.map((issue) => (
            <ReleaseIssueRow key={issue.id} release={release} issue={issue} />
          ))}
        </div>
      )}
    </section>
  );
}

function ReleaseIssueRow({ release, issue }: { release: ReleaseDetail; issue: ReleaseIssue }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: change, isPending } = useReleaseIssuesMutation(workspace.id);
  return (
    <div
      role="listitem"
      className="group/row has-hover:hover:bg-accent flex h-9 items-center gap-2 border-b px-3 text-sm last:border-b-0"
    >
      <StateIcon type={issue.state.type} color={issue.state.color} name={issue.state.name} />
      <Link
        to="/$workspace/issue/$identifier"
        params={{ workspace: workspace.urlKey, identifier: issue.identifier }}
        className="flex min-w-0 flex-1 items-center gap-2 outline-hidden focus-visible:underline"
      >
        <span className="text-muted-foreground w-16 shrink-0 truncate font-mono text-xs">
          {issue.identifier}
        </span>
        <span className="truncate">{issue.title}</span>
      </Link>
      {issue.assignee && <Avatar id={issue.assignee.id} name={issue.assignee.name} size="xs" />}
      {canEdit && (
        <SimpleTooltip content="Remove from release">
          <Button
            variant="ghost"
            size="icon"
            aria-label={`Remove ${issue.identifier} from the release`}
            isPending={isPending}
            className="opacity-0 group-hover/row:opacity-100 focus-visible:opacity-100"
            onClick={() => change({ release: release.id, issue: issue.id, action: "remove" })}
          >
            <XIcon className="size-4" />
          </Button>
        </SimpleTooltip>
      )}
    </div>
  );
}

function AddIssuePicker({
  release,
  variant = "ghost",
}: {
  release: ReleaseDetail;
  variant?: "ghost" | "outline";
}) {
  const { workspace } = useWorkspace();
  const [search, setSearch] = useState("");
  const [query] = useDebounceValue(search, SEARCH_DEBOUNCE_MS);
  const { data: issues = [], isFetching } = useQuery(issueOptionsQuery(workspace.id, query));
  const { mutate: change } = useReleaseIssuesMutation(workspace.id);
  const included = new Set(release.issues.map((issue) => issue.id));
  return (
    <PropertyPicker
      label="Add issue"
      align="end"
      searchPlaceholder="Search issues or type ENG-123…"
      emptyText="No matching issues"
      isPending={isFetching}
      onSearchChange={setSearch}
      value={[]}
      onChange={([id]) => {
        if (id) change({ release: release.id, issue: id, action: "add" });
      }}
      options={issues
        .filter((issue) => !included.has(issue.id))
        .map((issue) => ({
          value: issue.id,
          label: issue.title,
          description: issue.identifier,
          icon: <StateIcon type={issue.state.type} color={issue.state.color} />,
        }))}
      trigger={
        <Button variant={variant} size="sm">
          <PlusIcon className="size-4" />
          Add issue
        </Button>
      }
    />
  );
}
