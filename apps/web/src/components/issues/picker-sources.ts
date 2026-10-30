import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import type { IssueLabel, Milestone, UserRef, WorkspaceUser } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { cyclesQuery } from "@/lib/queries/cycles";
import { issueProjectsQuery, projectMilestonesQuery } from "@/lib/queries/issue-options";
import { labelsQuery } from "@/lib/queries/labels";
import { milestonesQuery } from "@/lib/queries/milestones";
import { teamQuery } from "@/lib/queries/teams";
import { usersQuery } from "@/lib/queries/users";

/** A team's settings, statuses and members (get_team). */
export function useTeamDetail(teamId: string | undefined) {
  const { workspace } = useWorkspace();
  const { data } = useQuery({
    ...teamQuery(workspace.id, teamId ?? ""),
    enabled: Boolean(teamId),
  });
  return data;
}

/** The team's active statuses, in board order. */
export function useTeamStates(teamId: string | undefined) {
  return useTeamDetail(teamId)?.states ?? [];
}

/** Active members of the workspace. */
export function useWorkspaceUsers() {
  const { workspace, isVisitor } = useWorkspace();
  const { data, isPending } = useQuery({ ...usersQuery(workspace.id), enabled: !isVisitor });
  const users = useMemo(() => (data ?? []).filter((user) => user.active), [data]);
  return { users, isPending };
}

/** The member's avatar, which refs in issues and comments don't carry. */
export function useAvatarUrl(user: Pick<UserRef, "id"> | null | undefined) {
  const { workspace, isVisitor } = useWorkspace();
  const { data } = useQuery({
    ...usersQuery(workspace.id),
    select: (users: WorkspaceUser[]) => users.find((item) => item.id === user?.id)?.avatarUrl,
    enabled: Boolean(user) && !isVisitor,
  });
  return data ?? null;
}

/** Labels an issue of the team can take (its own and the workspace's), groups left out. */
export function useLabels(teamId: string | undefined) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(labelsQuery(workspace.id, { team: teamId }));
  const labels = useMemo(
    () => (data ?? []).filter((label: IssueLabel) => !label.isGroup && !label.retiredAt),
    [data],
  );
  const groups = useMemo(() => (data ?? []).filter((label) => label.isGroup), [data]);
  return { labels, groups, isPending };
}

/** Open projects, the team's first. */
export function useProjects(teamId: string | undefined) {
  const { workspace, isVisitor } = useWorkspace();
  const { data, isPending } = useQuery({
    ...issueProjectsQuery(workspace.id),
    enabled: !isVisitor,
  });
  const projects = useMemo(() => {
    const open = (data ?? []).filter(
      (project) =>
        !project.archivedAt && !project.trashed && !project.completedAt && !project.canceledAt,
    );
    const isTeams = (project: (typeof open)[number]) =>
      Boolean(teamId && project.teams.some((team) => team.id === teamId));
    return [...open.filter(isTeams), ...open.filter((project) => !isTeams(project))];
  }, [data, teamId]);
  return { projects, isPending };
}

/** Every project by id, for icons and colors that project refs don't carry. */
export function useProjectLookup() {
  const { workspace, isVisitor } = useWorkspace();
  const { data } = useQuery({ ...issueProjectsQuery(workspace.id), enabled: !isVisitor });
  return useMemo(() => new Map((data ?? []).map((project) => [project.id, project])), [data]);
}

/** Open milestones of the project and of the workspace's own, and the current one even when closed. */
export function useMilestones(projectId: string | undefined, currentId: string | undefined) {
  const { workspace, isVisitor } = useWorkspace();
  const hasProject = Boolean(projectId) && !isVisitor;
  const own = useQuery({ ...milestonesQuery(workspace.id), enabled: !isVisitor });
  const project = useQuery({
    ...projectMilestonesQuery(workspace.id, projectId ?? ""),
    enabled: hasProject,
  });
  const workspaceMilestones = useMemo(() => pickable(own.data, currentId), [own.data, currentId]);
  const projectMilestones = useMemo(
    () => (hasProject ? pickable(project.data, currentId) : []),
    [hasProject, project.data, currentId],
  );
  return {
    workspaceMilestones,
    projectMilestones,
    isPending: (!isVisitor && own.isPending) || (hasProject && project.isPending),
  };
}

function pickable(milestones: Milestone[] | undefined, currentId: string | undefined) {
  return (milestones ?? []).filter(
    (milestone) => !milestone.closedAt || milestone.id === currentId,
  );
}

/** The team's current and upcoming cycles, when it uses cycles. */
export function useCycles(teamId: string | undefined) {
  const { workspace, isVisitor } = useWorkspace();
  const team = useTeamDetail(teamId);
  const enabled = Boolean(teamId && team?.cyclesEnabled) && !isVisitor;
  const { data, isPending } = useQuery({
    ...cyclesQuery(workspace.id, teamId ?? ""),
    enabled,
  });
  const cycles = useMemo(
    () =>
      (data ?? [])
        .filter((cycle) => !cycle.isPast && !cycle.archivedAt)
        .sort((a, b) => a.startsAt.localeCompare(b.startsAt)),
    [data],
  );
  return { cycles, isPending: enabled && isPending, enabled };
}
