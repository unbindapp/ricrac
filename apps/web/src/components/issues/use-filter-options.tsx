import { useQueries } from "@tanstack/react-query";
import { CircleUserIcon } from "lucide-react";

import { type TFilterField, UNASSIGNED } from "@/components/issues/filters";
import { cycleName, PRIORITY_ORDER, userName } from "@/components/issues/grouping";
import {
  useCycles,
  useLabels,
  useProjects,
  useWorkspaceUsers,
} from "@/components/issues/picker-sources";
import { LabelDot, UserAvatar } from "@/components/issues/pickers";
import type { TPickerOption } from "@/components/issues/property-picker";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { PriorityIcon, priorityLabel } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { StateIcon } from "@/components/shared/state-icon";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamQuery } from "@/lib/queries/teams";

/** The values each filter offers: the team's when the page has one, else those of your teams. */
export function useFilterOptions(teamId: string | undefined) {
  const { workspace } = useWorkspace();
  const { teams, myTeams } = useTeams();
  const { users } = useWorkspaceUsers();
  const { labels } = useLabels(teamId);
  const { projects } = useProjects(teamId);
  const { cycles, enabled: cyclesEnabled } = useCycles(teamId);

  const scopedTeams = teamId ? teams.filter((team) => team.id === teamId) : myTeams;
  const details = useQueries({
    queries: scopedTeams.map((team) => teamQuery(workspace.id, team.id)),
  });
  const stateOptions: TPickerOption[] = details.flatMap(({ data: team }) =>
    (team?.states ?? []).map((state) => ({
      value: state.id,
      label: state.name,
      group: scopedTeams.length > 1 ? team?.name : undefined,
      keywords: team ? [team.key] : [],
      icon: <StateIcon type={state.type} color={state.color} name={state.name} />,
    })),
  );
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
