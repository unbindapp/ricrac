import { useState } from "react";
import { Trash2Icon } from "lucide-react";

import type { IssueStatus } from "@/api/ops.gen";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { StateIcon } from "@/components/shared/state-icon";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useDeleteIssueStatusMutation } from "@/lib/queries/teams";

type TProps = {
  state: IssueStatus;
  states: IssueStatus[];
};

export function DeleteWorkflowStateDialog({ state, states }: TProps) {
  const others = states.filter((candidate) => candidate.id !== state.id);
  const fallback = others.find((candidate) => candidate.type === state.type) ?? others[0];
  const [targetId, setTargetId] = useState<string | null>(fallback?.id ?? null);
  const { workspace } = useWorkspace();
  const { mutateAsync: deleteStatus } = useDeleteIssueStatusMutation(workspace.id);
  const items = others.map((candidate) => ({ value: candidate.id, label: candidate.name }));

  return (
    <ConfirmDialog
      trigger={
        <Button
          variant="ghost"
          size="icon"
          aria-label={`Delete ${state.name}`}
          disabled={others.length === 0}
        >
          <Trash2Icon className="size-4" />
        </Button>
      }
      title={`Delete ${state.name}`}
      description="Issues in this status move to the status you pick."
      confirmLabel="Delete status"
      confirmDisabled={!targetId}
      onConfirm={async () => {
        if (!targetId) return;
        await deleteStatus({ id: state.id, moveIssuesTo: targetId });
      }}
    >
      <div className="flex flex-col gap-1.5">
        <p className="text-sm font-medium">Move issues to</p>
        <Select items={items} value={targetId} onValueChange={(value) => setTargetId(value)}>
          <SelectTrigger aria-label="Move issues to">
            <SelectValue placeholder="Pick a status" />
          </SelectTrigger>
          <SelectContent>
            {others.map((candidate) => (
              <SelectItem key={candidate.id} value={candidate.id}>
                <StateIcon type={candidate.type} color={candidate.color} />
                <span className="truncate">{candidate.name}</span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </ConfirmDialog>
  );
}
