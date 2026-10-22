import { useQuery } from "@tanstack/react-query";
import { createContext, ReactNode, useContext, useMemo } from "react";

import type { Team } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamsQuery } from "@/lib/queries/teams";

type TTeamsContext = {
  /** The workspace's one team, which holds its issues, statuses, labels and cycles. */
  team: Team | undefined;
  isPending: boolean;
  error: Error | null;
  refetch: () => void;
};

const TeamsContext = createContext<TTeamsContext | null>(null);

export function TeamsProvider({ children }: { children: ReactNode }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(teamsQuery(workspace.id));
  const value = useMemo<TTeamsContext>(() => {
    const teams = data ?? [];
    return {
      team: teams.find((team) => !team.archivedAt) ?? teams[0],
      isPending,
      error,
      refetch: () => void refetch(),
    };
  }, [data, isPending, error, refetch]);
  return <TeamsContext.Provider value={value}>{children}</TeamsContext.Provider>;
}

export function useTeams() {
  const context = useContext(TeamsContext);
  if (!context) throw new Error("useTeams must be used within a TeamsProvider");
  return context;
}

/** The workspace's team; undefined only while it loads or when it failed to. */
export function useWorkspaceTeam() {
  return useTeams().team;
}
