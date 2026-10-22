import type { ListIssuesFilters } from "@/components/issues/filters";
import { UNASSIGNED } from "@/components/issues/filters";
import type { TIssueDraft } from "@/components/issues/issue-draft";
import type { TNewIssueDefaults } from "@/components/issues/new-issue-store";
import type { TStateType } from "@/components/shared/state-icon";
import {
  useCycles,
  useLabels,
  useProjects,
  useTeamStates,
  useWorkspaceUsers,
} from "@/components/issues/picker-sources";
import { toLabelRef } from "@/components/issues/pickers";

function single<T>(values: T[] | undefined) {
  return values?.length === 1 ? values[0] : undefined;
}

/** A new issue made from a filtered page starts with what the filters pin down. */
export function useFilterDefaults(
  filters: ListIssuesFilters,
  teamId: string | undefined,
  /** The kind of status new issues get when the filters don't pin one, so they stay in view. */
  fallbackStateType?: TStateType,
): TNewIssueDefaults {
  const { users } = useWorkspaceUsers();
  const states = useTeamStates(teamId);
  const { labels } = useLabels(teamId);
  const { projects } = useProjects(teamId);
  const { cycles } = useCycles(teamId);

  const draft: Partial<TIssueDraft> = {};
  const stateId = single(filters.state_ids);
  const stateType = single(filters.state_types) ?? fallbackStateType;
  const state =
    states.find((item) => item.id === stateId) ??
    (stateType ? states.find((item) => item.type === stateType) : undefined);
  if (state) draft.state = { id: state.id, name: state.name, color: state.color, type: state.type };
  const priority = single(filters.priorities);
  if (priority !== undefined) draft.priority = priority;
  const assigneeId = single(filters.assignee_ids);
  const assignee =
    assigneeId && assigneeId !== UNASSIGNED ? users.find((u) => u.id === assigneeId) : undefined;
  if (assignee) {
    draft.assignee = {
      id: assignee.id,
      name: assignee.name,
      displayName: assignee.displayName,
      email: assignee.email,
    };
  }
  const labelIds = filters.label_ids ?? [];
  const filterLabels = labels.filter((label) => labelIds.includes(label.id));
  if (filterLabels.length > 0) draft.labels = filterLabels.map(toLabelRef);
  const project = projects.find((item) => item.id === single(filters.project_ids));
  if (project) draft.project = { id: project.id, name: project.name, slugId: project.slugId };
  const cycle = cycles.find((item) => item.id === single(filters.cycle_ids));
  if (cycle) {
    draft.cycle = {
      id: cycle.id,
      name: cycle.name,
      number: cycle.number,
      startsAt: cycle.startsAt,
      endsAt: cycle.endsAt,
    };
  }
  return { draft };
}
