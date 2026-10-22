import { CircleUserIcon } from "lucide-react";
import { type TFilterField, UNASSIGNED } from "@/components/issues/filters";
import { cycleName, PRIORITY_ORDER, userName } from "@/components/issues/grouping";
import {
  useCycles,
  useLabels,
  useProjects,
  useTeamStates,
  useWorkspaceUsers,
} from "@/components/issues/picker-sources";
import { LabelDot, UserAvatar } from "@/components/issues/pickers";
import type { TPickerOption } from "@/components/issues/property-picker";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { PriorityIcon, priorityLabel } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { StateIcon } from "@/components/shared/state-icon";

export function useFilterOptions(teamId: string | undefined) {
  const { users } = useWorkspaceUsers();
  const { labels } = useLabels(teamId);
  const { projects } = useProjects(teamId);
  const { cycles, enabled: cyclesEnabled } = useCycles(teamId);

  const stateOptions: TPickerOption[] = useTeamStates(teamId).map((state) => ({
    value: state.id,
    label: state.name,
    icon: <StateIcon type={state.type} color={state.color} name={state.name} />,
  }));
  const userOptions: TPickerOption[] = users.map((user) => ({
    value: user.id,
    label: userName(user),
    keywords: [user.displayName, user.email],
    icon: <UserAvatar user={user} size="xs" />,
  }));

  const options: Record<TFilterField, TPickerOption[]> = {
    status: stateOptions,
    assignee: [
      {
        value: UNASSIGNED,
        label: "Unassigned",
        icon: <CircleUserIcon className="text-muted-foreground size-4" />,
      },
      ...userOptions,
    ],
    creator: userOptions,
    subscriber: userOptions,
    priority: PRIORITY_ORDER.map((priority) => ({
      value: String(priority),
      label: priorityLabel(priority),
      icon: <PriorityIcon priority={priority} />,
    })),
    labels: labels.map((label) => ({
      value: label.id,
      label: label.name,
      group: label.parent?.name,
      icon: <LabelDot color={label.color} />,
    })),
    project: [
      { value: UNASSIGNED, label: "No project", icon: <ProjectIcon /> },
      ...projects.map((project) => ({
        value: project.id,
        label: project.name,
        icon: <ProjectIcon icon={project.icon} color={project.color} />,
      })),
    ],
    cycle: cycles.map((cycle) => ({
      value: cycle.id,
      label: cycleName(cycle),
      icon: <CycleIcon status={cycle.isActive ? "active" : "upcoming"} />,
    })),
    due_date: [],
  };
  return { options, cyclesEnabled };
}
