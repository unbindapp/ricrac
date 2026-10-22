import { CircleUserIcon, CornerLeftUpIcon, ListIcon } from "lucide-react";
import type { TGroupMeta } from "@/components/issues/grouping";
import { useProjectLookup } from "@/components/issues/picker-sources";
import { LabelDot, UserAvatar } from "@/components/issues/pickers";
import { CycleIcon } from "@/components/shared/cycle-icon";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { StateIcon } from "@/components/shared/state-icon";

function GroupProjectIcon({ projectId }: { projectId: string | undefined }) {
  const project = useProjectLookup().get(projectId ?? "");
  return <ProjectIcon icon={project?.icon} color={project?.color} />;
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
  }
}
