import type {
  CycleRef,
  Issue,
  IssueRef,
  LabelRef,
  MilestoneRef,
  ProjectRef,
  StateRef,
  UserRef,
} from "@/api/ops.gen";
import type { TIssueChanges } from "@/lib/queries/issues";

/** The properties an issue is edited through, in the shapes the server answers with. */
export type TIssueDraft = {
  state: StateRef | undefined;
  priority: number;
  assignee: UserRef | null;
  labels: LabelRef[];
  project: ProjectRef | null;
  projectMilestone: MilestoneRef | null;
  cycle: CycleRef | null;
  estimate: number | null;
  dueDate: string | null;
  parent: IssueRef | null;
};

export type TIssuePatch = Partial<TIssueDraft>;

export const EMPTY_DRAFT: TIssueDraft = {
  state: undefined,
  priority: 0,
  assignee: null,
  labels: [],
  project: null,
  projectMilestone: null,
  cycle: null,
  estimate: null,
  dueDate: null,
  parent: null,
};

export function draftFromIssue(issue: Issue): TIssueDraft {
  return {
    state: issue.state,
    priority: issue.priority,
    assignee: issue.assignee,
    labels: issue.labels,
    project: issue.project,
    projectMilestone: issue.projectMilestone,
    cycle: issue.cycle,
    estimate: issue.estimate,
    dueDate: issue.dueDate,
    parent: issue.parent,
  };
}

/** A milestone belongs to its project, so changing the project drops it. */
export function applyPatch(draft: TIssueDraft, patch: TIssuePatch): TIssueDraft {
  const next = { ...draft, ...patch };
  if ("project" in patch && patch.project?.id !== draft.project?.id) next.projectMilestone = null;
  return next;
}

export function patchToOptimistic(patch: TIssuePatch): Partial<Issue> {
  const { state, ...rest } = patch;
  const optimistic: Partial<Issue> = { ...rest };
  if (state) optimistic.state = state;
  if ("project" in patch) optimistic.projectMilestone = patch.projectMilestone ?? null;
  return optimistic;
}

const ids = (labels: LabelRef[]) => labels.map((label) => label.id);

/** save_issue's arguments for a patch; labels go as additions and removals when the current ones are known. */
export function patchToChanges(patch: TIssuePatch, currentLabels?: LabelRef[]): TIssueChanges {
  const changes: TIssueChanges = {};
  if (patch.state) changes.state = patch.state.id;
  if (patch.priority !== undefined) changes.priority = patch.priority;
  if (patch.assignee !== undefined) changes.assignee = patch.assignee?.id ?? null;
  if (patch.labels) {
    if (currentLabels) {
      const next = ids(patch.labels);
      const current = ids(currentLabels);
      const added = next.filter((id) => !current.includes(id));
      const removed = current.filter((id) => !next.includes(id));
      if (added.length > 0) changes.addLabels = added;
      if (removed.length > 0) changes.removeLabels = removed;
    } else {
      changes.labels = ids(patch.labels);
    }
  }
  if (patch.project !== undefined) changes.project = patch.project?.id ?? null;
  if (patch.projectMilestone !== undefined) {
    changes.milestone = patch.projectMilestone?.id ?? null;
  }
  if (patch.cycle !== undefined) changes.cycle = patch.cycle?.id ?? null;
  if (patch.estimate !== undefined) changes.estimate = patch.estimate;
  if (patch.dueDate !== undefined) changes.dueDate = patch.dueDate;
  if (patch.parent !== undefined) changes.parentId = patch.parent?.id ?? null;
  return changes;
}

export function draftToCreateArgs({
  teamId,
  title,
  description,
  draft,
}: {
  teamId: string;
  title: string;
  description: string;
  draft: TIssueDraft;
}): TIssueChanges {
  const args: TIssueChanges = { team: teamId, title: title.trim() };
  if (description.trim()) args.description = description;
  if (draft.state) args.state = draft.state.id;
  if (draft.priority) args.priority = draft.priority;
  if (draft.assignee) args.assignee = draft.assignee.id;
  if (draft.labels.length > 0) args.labels = ids(draft.labels);
  if (draft.project) args.project = draft.project.id;
  if (draft.projectMilestone) args.milestone = draft.projectMilestone.id;
  if (draft.cycle) args.cycle = draft.cycle.id;
  if (draft.estimate !== null) args.estimate = draft.estimate;
  if (draft.dueDate) args.dueDate = draft.dueDate;
  if (draft.parent) args.parentId = draft.parent.id;
  return args;
}
