import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { ViewDialog } from "@/components/views/view-dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TFilter, viewParam } from "@/lib/queries/views";

type TSaveViewArgs = {
  /** The list's filter, in the filter language of list_issues. */
  filter: TFilter;
  display?: Record<string, unknown>;
  /** The team (id or key) the list is of, if any. */
  team?: string | null;
};

/** Saves what an issue list shows as a view, then opens it. */
export function useSaveViewDialog({ filter, display, team }: TSaveViewArgs): {
  openSaveView: () => void;
  saveViewDialog: ReactNode;
} {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const [isOpen, setIsOpen] = useState(false);
  const saveViewDialog = (
    <ViewDialog
      target={isOpen ? { mode: "create", draft: { model: "issue", filter, display, team } } : null}
      onClose={() => setIsOpen(false)}
      onSaved={(view) =>
        void navigate({
          to: "/$workspace/view/$view_id",
          params: { workspace: workspace.urlKey, view_id: viewParam(view) },
        })
      }
    />
  );
  return { openSaveView: () => setIsOpen(true), saveViewDialog };
}
