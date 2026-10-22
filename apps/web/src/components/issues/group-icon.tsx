import { CircleUserIcon, CornerLeftUpIcon, ListIcon } from "@/components/icons/symbols.gen";
import type { TGroupMeta } from "@/components/issues/grouping";
import { useProjectLookup } from "@/components/issues/picker-sources";
import { LabelDot, UserAvatar } from "@/components/issues/pickers";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { StateIcon } from "@/components/shared/state-icon";
import { TeamIcon } from "@/components/shared/team-icon";
import { useTeams } from "@/components/workspace/teams-provider";

function GroupProjectIcon({ projectId }: { projectId: string | undefined }) {
  const project = useProjectLookup().get(projectId ?? "");
  return <ProjectIcon icon={project?.icon} color={project?.color} />;
}

function GroupTeamIcon({ teamKey }: { teamKey: string }) {
  const team = useTeams().teamByKey(teamKey);
  return <TeamIcon teamKey={teamKey} icon={team?.icon} color={team?.color} size="sm" />;
}

export function GroupIcon({ meta }: { meta: TGroupMeta }) {
  switch (meta.kind) {
    case "none":
      return <ListIcon className="text-muted-foreground size-4" />;
    case "state":
      return <StateIcon type={meta.state.type} color={meta.state.color} name={meta.state.name} />;
    case "priority":
      return <PriorityIcon priority={meta.priority} />;
    case "assignee":
      return meta.user ? (
        <UserAvatar user={meta.user} size="xs" />
      ) : (
        <CircleUserIcon className="text-muted-foreground size-4" />
      );
    case "project":
      return <GroupProjectIcon projectId={meta.project?.id} />;
    case "cycle":
      return <CycleIcon status={meta.cycle ? "active" : "upcoming"} />;
    case "label":
      return <LabelDot color={meta.label?.color} />;
    case "parent":
      return <CornerLeftUpIcon className="text-muted-foreground size-4" />;
    case "team":
      return <GroupTeamIcon teamKey={meta.team.key} />;
  }
}
