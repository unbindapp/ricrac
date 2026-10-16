import { useQueries, useQuery } from "@tanstack/react-query";
import { BoxIcon, CalendarIcon, RefreshCwIcon, UserIcon } from "lucide-react";
import type { ReactNode } from "react";

import { type TFilterField, ME, NONE } from "@/components/views/view-filter";
import { Avatar } from "@/components/shared/avatar";
import { PriorityIcon, priorityLabel, PRIORITIES } from "@/components/shared/priority-icon";
import { STATE_TYPES, StateIcon, stateTypeLabels } from "@/components/shared/state-icon";
import { TeamIcon } from "@/components/shared/team-icon";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { labelsQuery } from "@/lib/queries/labels";
import { teamStatusesQuery } from "@/lib/queries/teams";
import { usersQuery } from "@/lib/queries/users";
import { projectOptionsQuery } from "@/lib/queries/views";

export type TFilterOption = { value: string; label: string; icon?: ReactNode; hint?: string };

export const fieldLabels: Record<TFilterField, string> = {
  team: "Team",
  statusType: "Status type",
  status: "Status",
  assignee: "Assignee",
  creator: "Creator",
  priority: "Priority",
  labels: "Labels",
  project: "Project",
  cycle: "Cycle",
  dueDate: "Due date",
  projectStatus: "Status",
  lead: "Lead",
  projectTeam: "Team",
  projectPriority: "Priority",
  health: "Health",
};

const projectStatusTypes = ["backlog", "planned", "started", "paused", "completed", "canceled"];
const healthLabels: Record<string, string> = {
  onTrack: "On track",
  atRisk: "At risk",
  offTrack: "Off track",
};

const dot = (color: string) => (
  <span className="flex size-4 shrink-0 items-center justify-center">
    <span className="size-2.5 rounded-full" style={{ backgroundColor: color }} />
  </span>
);

const capitalize = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

/** What each field of the filter builder can pick, loaded once for the whole bar. */
export function useFilterOptions(): Record<TFilterField, TFilterOption[]> {
  const { workspace } = useWorkspace();
  const { teams } = useTeams();
  const { data: users = [] } = useQuery(usersQuery(workspace.id));
  const { data: labels = [] } = useQuery(labelsQuery(workspace.id));
  const { data: projects = [] } = useQuery(projectOptionsQuery(workspace.id));
  const statusLists = useQueries({
    queries: teams.map((team) => teamStatusesQuery(workspace.id, team.id)),
  });

  const teamOptions = teams.map((team) => ({
    value: team.key,
    label: team.name,
    icon: <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />,
  }));
  const people = (noneLabel: string | null) => [
    { value: ME, label: "Me", icon: <UserIcon className="size-4" />, hint: "Whoever looks" },
    ...(noneLabel ? [{ value: NONE, label: noneLabel }] : []),
    ...users
      .filter((user) => user.active)
      .map((user) => ({
        value: user.id,
        label: user.displayName,
        icon: <Avatar id={user.id} name={user.displayName} size="xs" />,
      })),
  ];
  const statuses = new Map<string, TFilterOption>();
  statusLists.forEach((list, index) => {
    for (const state of list.data ?? []) {
      const existing = statuses.get(state.name);
      const hint = existing?.hint ? `${existing.hint}, ${teams[index].key}` : teams[index].key;
      statuses.set(state.name, {
        value: state.name,
        label: state.name,
        icon: existing?.icon ?? <StateIcon type={state.type} color={state.color} />,
        hint,
      });
    }
  });
  const priorities = PRIORITIES.map((priority) => ({
    value: String(priority),
    label: priorityLabel(priority),
    icon: <PriorityIcon priority={priority} />,
  }));

  return {
    team: teamOptions,
    statusType: STATE_TYPES.map((type) => ({
      value: type,
      label: stateTypeLabels[type],
      icon: <StateIcon type={type} />,
    })),
    status: [...statuses.values()],
    assignee: people("No assignee"),
    creator: people(null),
    priority: priorities,
    labels: labels
      .filter((label) => !label.isGroup)
      .map((label) => ({
        value: label.id,
        label: label.parent ? `${label.parent.name}/${label.name}` : label.name,
        icon: dot(label.color),
        hint: label.team?.key,
      })),
    project: [
      { value: NONE, label: "No project" },
      ...projects.map((project) => ({
        value: project.id,
        label: project.name,
        icon: <BoxIcon className="size-4" />,
      })),
    ],
    cycle: [
      { value: "current", label: "Current cycle", icon: <RefreshCwIcon className="size-4" /> },
      { value: NONE, label: "No cycle" },
    ],
    dueDate: [
      { value: "overdue", label: "Overdue", icon: <CalendarIcon className="size-4" /> },
      { value: "week", label: "Due within a week", icon: <CalendarIcon className="size-4" /> },
      { value: NONE, label: "No due date" },
    ],
    projectStatus: projectStatusTypes.map((type) => ({ value: type, label: capitalize(type) })),
    lead: people("No lead").filter((option) => option.value !== ME),
    projectTeam: teamOptions,
    projectPriority: priorities,
    health: Object.entries(healthLabels).map(([value, label]) => ({ value, label })),
  };
}
