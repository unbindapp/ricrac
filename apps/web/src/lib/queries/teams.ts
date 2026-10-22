import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";

import { teamKeyPattern } from "@/lib/helpers/identifiers";
import { queryKeyUsers } from "@/lib/queries/users";
import { queryKeyWorkspace } from "@/lib/queries/workspaces";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export const queryKeyTeams = {
  all: (workspaceId: string) => [workspaceId, "teams"] as const,
  list: (workspaceId: string) => [workspaceId, "teams", "list"] as const,
  detail: (workspaceId: string, team: string) => [workspaceId, "teams", "detail", team] as const,
  statuses: (workspaceId: string, team: string) =>
    [workspaceId, "teams", "statuses", team] as const,
};

export const teamsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyTeams.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_teams", {}),
  });

/** A team by its key (or id), with its members and statuses. */
export const teamQuery = (workspaceId: string, team: string) =>
  queryOptions({
    queryKey: queryKeyTeams.detail(workspaceId, team),
    queryFn: () => call(workspaceId, "get_team", { id: team }),
  });

export const teamStatusesQuery = (workspaceId: string, team: string) =>
  queryOptions({
    queryKey: queryKeyTeams.statuses(workspaceId, team),
    queryFn: () => callAll(workspaceId, "list_issue_statuses", { team }),
  });

/** The team's settings show up in the workspace and on people, so all three refresh. */
function useInvalidateTeams(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeyTeams.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyWorkspace.detail(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyUsers.all(workspaceId) }),
    ]);
}

export function useSaveTeamMutation(workspaceId: string) {
  const invalidate = useInvalidateTeams(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_team">) => call(workspaceId, "save_team", args),
    meta: { errorTitle: "Could not save settings" },
    onSuccess: invalidate,
  });
}

export function useSaveIssueStatusMutation(workspaceId: string) {
  const invalidate = useInvalidateTeams(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_issue_status">) => call(workspaceId, "save_issue_status", args),
    meta: { errorTitle: "Could not save status" },
    onSuccess: invalidate,
  });
}

export function useDeleteIssueStatusMutation(workspaceId: string) {
  const invalidate = useInvalidateTeams(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"delete_issue_status">) =>
      call(workspaceId, "delete_issue_status", args),
    meta: { errorTitle: "Could not delete status" },
    onSuccess: invalidate,
  });
}

export const teamNameMaxLength = 80;

export const TeamFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(teamNameMaxLength, `Name should be at most ${teamNameMaxLength} characters.`),
  key: z
    .string()
    .trim()
    .regex(teamKeyPattern, "Start with a letter, then up to 6 letters or digits."),
});
