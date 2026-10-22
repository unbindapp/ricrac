import { useState } from "react";
import { ArchiveIcon, EllipsisIcon, PencilIcon } from "lucide-react";

import type { Cycle } from "@/api/ops.gen";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { cycleName, useArchiveCycleMutation } from "@/lib/queries/cycles";

type TProps = {
  cycle: Cycle;
  onEdit: () => void;
};

export function CycleMenu({ cycle, onEdit }: TProps) {
  const { workspace } = useWorkspace();
  const [isArchiving, setIsArchiving] = useState(false);
  const { mutateAsync: archive } = useArchiveCycleMutation(workspace.id);
  const name = cycleName(cycle);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" size="icon" aria-label={`Actions for ${name}`}>
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem onClick={onEdit}>
            <PencilIcon className="size-4" />
            Edit cycle
          </DropdownMenuItem>
          {!cycle.archivedAt && (
            <DropdownMenuItem onClick={() => setIsArchiving(true)}>
              <ArchiveIcon className="size-4" />
              Archive cycle
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={isArchiving}
        onOpenChange={setIsArchiving}
        title={`Archive ${name}`}
        description="It leaves the cycle list (shown again with Show archived). Its issues stay where they are."
        confirmLabel="Archive cycle"
        variant="default"
        onConfirm={async () => {
          await archive(cycle.id);
          toast.add({ type: "success", title: `Archived ${name}` });
        }}
      />
    </>
  );
}
