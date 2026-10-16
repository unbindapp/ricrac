import { useQuery } from "@tanstack/react-query";
import { createContext, ReactNode, useContext, useMemo } from "react";

import type { Team } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamsQuery } from "@/lib/queries/teams";

type TTeamsContext = {
  /** The teams you can see. */
  teams: Team[];
  /** The teams you are a member of. */
  myTeams: Team[];
  isPending: boolean;
  error: Error | null;
  refetch: () => void;
  teamByKey: (key: string) => Team | undefined;
};

const TeamsContext = createContext<TTeamsContext | null>(null);

export function TeamsProvider({ children }: { children: ReactNode }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(teamsQuery(workspace.id));
  const value = useMemo<TTeamsContext>(() => {
    const teams = data ?? [];
    return {
      teams,
      myTeams: teams.filter((team) => team.isMember),
      isPending,
      error,
      refetch: () => void refetch(),
      teamByKey: (key: string) =>
        teams.find((team) => team.key.toLowerCase() === key.toLowerCase()),
    };
  }, [data, isPending, error, refetch]);
  return <TeamsContext.Provider value={value}>{children}</TeamsContext.Provider>;
}

export function useTeams() {
  const context = useContext(TeamsContext);
  if (!context) throw new Error("useTeams must be used within a TeamsProvider");
  return context;
}
