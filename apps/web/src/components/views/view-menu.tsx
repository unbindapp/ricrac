import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import {
  BracesIcon,
  EllipsisIcon,
  LinkIcon,
  PencilIcon,
  Trash2Icon,
} from "@/components/icons/symbols.gen";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { ViewDialog } from "@/components/views/view-dialog";
import { ViewFilterJsonDialog } from "@/components/views/view-filter-json-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";
import { type TView, useDeleteViewMutation } from "@/lib/queries/views";

/** Owners change their views; admins also change shared ones. */
export function useCanEditView(view: TView) {
  const { isAdmin, canEdit } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  if (!canEdit) return false;
  return view.owner?.id === me?.id || (view.shared && isAdmin);
}

type TProps = {
  view: TView;
  onDeleted?: () => void;
};

export function ViewMenu({ view, onDeleted }: TProps) {
  const { workspace } = useWorkspace();
  const canEdit = useCanEditView(view);
  const [dialog, setDialog] = useState<"edit" | "json" | "delete" | null>(null);
  const { mutateAsync: deleteView } = useDeleteViewMutation(workspace.id);
  const close = () => setDialog(null);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" size="icon" aria-label={`Actions for ${view.name}`}>
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-52">
          {canEdit && (
            <DropdownMenuItem onClick={() => setDialog("edit")}>
              <PencilIcon className="size-4" />
              Edit view
            </DropdownMenuItem>
          )}
          {canEdit && (
            <DropdownMenuItem onClick={() => setDialog("json")}>
              <BracesIcon className="size-4" />
              Edit filter as JSON
            </DropdownMenuItem>
          )}
          <DropdownMenuItem
            onClick={() =>
              void navigator.clipboard
                .writeText(view.url)
                .then(() => toast.add({ type: "success", title: "Link copied" }))
            }
          >
            <LinkIcon className="size-4" />
            Copy link
          </DropdownMenuItem>
          {canEdit && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive" onClick={() => setDialog("delete")}>
                <Trash2Icon className="size-4" />
                Delete view
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <ViewDialog target={dialog === "edit" ? { mode: "edit", view } : null} onClose={close} />
      <ViewFilterJsonDialog view={dialog === "json" ? view : null} onClose={close} />
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={(open) => !open && close()}
        title={`Delete ${view.name}`}
        description={
          view.shared
            ? "The view is deleted for everyone in the workspace. The issues stay as they are."
            : "The view is deleted. The issues stay as they are."
        }
        confirmLabel="Delete view"
        onConfirm={async () => {
          await deleteView(view.id);
          onDeleted?.();
        }}
      />
    </>
  );
}
