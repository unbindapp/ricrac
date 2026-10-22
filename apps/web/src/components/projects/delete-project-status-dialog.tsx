import { useState } from "react";
import { Trash2Icon } from "lucide-react";

import type { ProjectStatus } from "@/api/ops.gen";
import { ProjectStatusIcon } from "@/components/projects/project-status-icon";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useDeleteProjectStatusMutation } from "@/lib/queries/projects";

type TProps = {
  status: ProjectStatus;
  statuses: ProjectStatus[];
};

export function DeleteProjectStatusDialog({ status, statuses }: TProps) {
  const { workspace } = useWorkspace();
  const others = statuses.filter(
    (candidate) => candidate.id !== status.id && !candidate.archivedAt,
  );
  const fallback = others.find((candidate) => candidate.type === status.type) ?? others[0];
  const [targetId, setTargetId] = useState<string | null>(fallback?.id ?? null);
  const { mutateAsync: deleteStatus } = useDeleteProjectStatusMutation(workspace.id);
  const items = others.map((candidate) => ({ value: candidate.id, label: candidate.name }));
  const needsTarget = status.projectCount > 0;

  if (others.length === 0) {
    return (
      <SimpleTooltip content="The workspace keeps at least one status">
        <span className="flex">
          <Button variant="ghost" size="icon" aria-label={`Delete ${status.name}`} disabled>
            <Trash2Icon className="size-4" />
          </Button>
        </span>
      </SimpleTooltip>
    );
  }

  return (
    <ConfirmDialog
      trigger={
        <Button variant="ghost" size="icon" aria-label={`Delete ${status.name}`}>
          <Trash2Icon className="size-4" />
        </Button>
      }
      title={`Delete ${status.name}`}
      description={
        needsTarget
          ? `${status.projectCount} project${status.projectCount === 1 ? " uses" : "s use"} this status. Pick where they move.`
          : "No projects use this status."
      }
      confirmLabel="Delete status"
      confirmDisabled={needsTarget && !targetId}
      onConfirm={() =>
        deleteStatus({ id: status.id, moveProjectsTo: needsTarget ? targetId : undefined })
      }
    >
      {needsTarget && (
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium">Move projects to</p>
          <Select items={items} value={targetId} onValueChange={(value) => setTargetId(value)}>
            <SelectTrigger aria-label="Move projects to">
              <SelectValue placeholder="Pick a status" />
            </SelectTrigger>
            <SelectContent>
              {others.map((candidate) => (
                <SelectItem key={candidate.id} value={candidate.id}>
                  <ProjectStatusIcon type={candidate.type} color={candidate.color} />
                  <span className="truncate">{candidate.name}</span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
    </ConfirmDialog>
  );
}
