import type { Issue } from "@/api/ops.gen";
import { ArchiveIcon, CircleDashedIcon, Trash2Icon, XIcon } from "@/components/icons";
import { patchToOptimistic, type TIssuePatch } from "@/components/issues/issue-draft";
import {
  AssigneePicker,
  CyclePicker,
  LabelsPicker,
  PickerTrigger,
  PriorityPicker,
  ProjectPicker,
  StatePicker,
} from "@/components/issues/pickers";
import { PropertyPicker } from "@/components/issues/property-picker";
import { useIssueLifecycle } from "@/components/issues/use-issue-actions";
import { StateIcon, type TStateType, stateTypeLabels } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TIssueChanges, useUpdateIssuesMutation } from "@/lib/queries/issues";

type TProps = {
  issues: Issue[];
  onClear: () => void;
};

const BULK_STATE_TYPES: TStateType[] = ["backlog", "unstarted", "started", "completed", "canceled"];

function shared<T>(issues: Issue[], pick: (issue: Issue) => T, key: (value: T) => string) {
  const first = issues[0] ? pick(issues[0]) : undefined;
  if (first === undefined) return undefined;
  const firstKey = key(first);
  return issues.every((issue) => key(pick(issue)) === firstKey) ? first : undefined;
}

/** Shown while issues are selected (X, shift-click); changes them all with update_issues. */
export function BulkActionBar({ issues, onClear }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: updateIssues } = useUpdateIssuesMutation(workspace.id);
  const lifecycle = useIssueLifecycle();
  if (issues.length === 0 || !canEdit) return null;

  const ids = issues.map((issue) => issue.id);
  const teamId = shared(
    issues,
    (issue) => issue.team.id,
    (id) => id,
  );
  const state = shared(
    issues,
    (issue) => issue.state,
    (value) => value.id,
  );
  const priority = shared(issues, (issue) => issue.priority, String);
  const assignee = shared(
    issues,
    (issue) => issue.assignee,
    (user) => user?.id ?? "",
  );
  const project = shared(
    issues,
    (issue) => issue.project,
    (item) => item?.id ?? "",
  );
  const cycle = shared(
    issues,
    (issue) => issue.cycle,
    (item) => item?.id ?? "",
  );
  const commonLabels = issues[0].labels.filter((label) =>
    issues.every((issue) => issue.labels.some((item) => item.id === label.id)),
  );

  const apply = (changes: TIssueChanges, patch?: TIssuePatch) =>
    updateIssues({ ids, changes, optimistic: patch ? patchToOptimistic(patch) : undefined });

  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-4 z-40 flex justify-center px-4">
      <div
        role="toolbar"
        aria-label="Bulk actions"
        className="bg-popover pointer-events-auto flex max-w-full flex-wrap items-center gap-2 rounded-lg border p-2 shadow-lg"
      >
        <span className="px-1 text-sm font-medium whitespace-nowrap">{issues.length} selected</span>
        <Button variant="ghost" size="sm" aria-label="Clear selection" onClick={onClear}>
          <XIcon className="size-4" />
          <Kbd>Esc</Kbd>
        </Button>
        {teamId ? (
          <StatePicker
            appearance="compact"
            teamId={teamId}
            state={state}
            onChange={(next) => apply({ state: next.id }, { state: next })}
          />
        ) : (
          <PropertyPicker
            label="Change status"
            options={BULK_STATE_TYPES.map((type) => ({
              value: type,
              label: stateTypeLabels[type],
              description: "First status of this kind",
              icon: <StateIcon type={type} />,
            }))}
            value={[]}
            onChange={([type]) => apply({ state: type })}
            trigger={
              <PickerTrigger
                appearance="compact"
                label="Change status"
                icon={<CircleDashedIcon className="text-muted-foreground size-4" />}
                text="Status"
                muted
              />
            }
          />
        )}
        <PriorityPicker
          appearance="compact"
          priority={priority ?? 0}
          onChange={(next) => apply({ priority: next }, { priority: next })}
        />
        <AssigneePicker
          appearance="compact"
          assignee={assignee}
          onChange={(next) => apply({ assignee: next?.id ?? null }, { assignee: next })}
        />
        <LabelsPicker
          appearance="compact"
          teamId={teamId}
          labels={commonLabels}
          onChange={(next) => {
            const nextIds = next.map((label) => label.id);
            const commonIds = commonLabels.map((label) => label.id);
            const added = nextIds.filter((id) => !commonIds.includes(id));
            const removed = commonIds.filter((id) => !nextIds.includes(id));
            apply({
              ...(added.length > 0 ? { addLabels: added } : {}),
              ...(removed.length > 0 ? { removeLabels: removed } : {}),
            });
          }}
        />
        <ProjectPicker
          appearance="compact"
          teamId={teamId}
          project={project}
          onChange={(next) => apply({ project: next?.id ?? null }, { project: next })}
        />
        {teamId && (
          <CyclePicker
            appearance="compact"
            teamId={teamId}
            cycle={cycle}
            onChange={(next) => apply({ cycle: next?.id ?? null }, { cycle: next })}
          />
        )}
        <Button
          variant="outline"
          size="sm"
          isPending={lifecycle.isPending}
          onClick={() => void lifecycle.run(ids, "archive").then(onClear, () => undefined)}
        >
          <ArchiveIcon className="size-4" />
          Archive
        </Button>
        <Button
          variant="outline"
          size="sm"
          isPending={lifecycle.isPending}
          onClick={() => void lifecycle.run(ids, "delete").then(onClear, () => undefined)}
        >
          <Trash2Icon className="size-4" />
          Delete
        </Button>
      </div>
    </div>
  );
}
