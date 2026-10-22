import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

import {
  BoxIcon,
  CalendarIcon,
  CircleDashedIcon,
  CircleDotIcon,
  CircleUserIcon,
  HeartIcon,
  type LucideIcon,
  RefreshCwIcon,
  SignalHighIcon,
  TagIcon,
  UserIcon,
  UserPenIcon,
} from "@/components/icons";
import { type TFilterField, ME, NONE } from "@/components/views/view-filter";
import { Avatar } from "@/components/shared/avatar";
import { PriorityIcon, priorityLabel, PRIORITIES } from "@/components/shared/priority-icon";
import { STATE_TYPES, StateIcon, stateTypeLabels } from "@/components/shared/state-icon";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { labelsQuery } from "@/lib/queries/labels";
import { teamStatusesQuery } from "@/lib/queries/teams";
import { usersQuery } from "@/lib/queries/users";
import { projectOptionsQuery } from "@/lib/queries/views";

export type TFilterOption = { value: string; label: string; icon?: ReactNode; hint?: string };

export const fieldLabels: Record<TFilterField, string> = {
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
  projectPriority: "Priority",
  health: "Health",
};

export const fieldIcons: Record<TFilterField, LucideIcon> = {
  statusType: CircleDashedIcon,
  status: CircleDotIcon,
  assignee: CircleUserIcon,
  creator: UserPenIcon,
  priority: SignalHighIcon,
  labels: TagIcon,
  project: BoxIcon,
  cycle: RefreshCwIcon,
  dueDate: CalendarIcon,
  projectStatus: CircleDotIcon,
  lead: UserIcon,
  projectPriority: SignalHighIcon,
  health: HeartIcon,
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
  const team = useWorkspaceTeam();
  const { data: users = [] } = useQuery(usersQuery(workspace.id));
  const { data: labels = [] } = useQuery(labelsQuery(workspace.id));
  const { data: projects = [] } = useQuery(projectOptionsQuery(workspace.id));
  const { data: states = [] } = useQuery({
    ...teamStatusesQuery(workspace.id, team?.id ?? ""),
    enabled: !!team,
  });

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
  const priorities = PRIORITIES.map((priority) => ({
    value: String(priority),
    label: priorityLabel(priority),
    icon: <PriorityIcon priority={priority} />,
  }));

  return {
    statusType: STATE_TYPES.map((type) => ({
      value: type,
      label: stateTypeLabels[type],
      icon: <StateIcon type={type} />,
    })),
    status: states.map((state) => ({
      value: state.name,
      label: state.name,
      icon: <StateIcon type={state.type} color={state.color} />,
    })),
    assignee: people("No assignee"),
    creator: people(null),
    priority: priorities,
    labels: labels
      .filter((label) => !label.isGroup)
      .map((label) => ({
        value: label.id,
        label: label.parent ? `${label.parent.name}/${label.name}` : label.name,
        icon: dot(label.color),
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
    projectPriority: priorities,
    health: Object.entries(healthLabels).map(([value, label]) => ({ value, label })),
  };
}
