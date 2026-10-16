import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import type { IssueLabel, UserRef, WorkspaceUser } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { cyclesQuery } from "@/lib/queries/cycles";
import { issueProjectsQuery, projectMilestonesQuery } from "@/lib/queries/issue-options";
import { labelsQuery } from "@/lib/queries/labels";
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
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(usersQuery(workspace.id));
  const users = useMemo(() => (data ?? []).filter((user) => user.active), [data]);
  return { users, isPending };
}

/** The member's avatar, which refs in issues and comments don't carry. */
export function useAvatarUrl(user: Pick<UserRef, "id"> | null | undefined) {
  const { workspace } = useWorkspace();
  const { data } = useQuery({
    ...usersQuery(workspace.id),
    select: (users: WorkspaceUser[]) => users.find((item) => item.id === user?.id)?.avatarUrl,
    enabled: Boolean(user),
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
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(issueProjectsQuery(workspace.id));
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
  const { workspace } = useWorkspace();
  const { data } = useQuery(issueProjectsQuery(workspace.id));
  return useMemo(() => new Map((data ?? []).map((project) => [project.id, project])), [data]);
}

export function useMilestones(projectId: string | undefined) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery({
    ...projectMilestonesQuery(workspace.id, projectId ?? ""),
    enabled: Boolean(projectId),
  });
  return { milestones: data ?? [], isPending: Boolean(projectId) && isPending };
}

/** The team's current and upcoming cycles, when it uses cycles. */
export function useCycles(teamId: string | undefined) {
  const { workspace } = useWorkspace();
  const team = useTeamDetail(teamId);
  const enabled = Boolean(teamId && team?.cyclesEnabled);
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
