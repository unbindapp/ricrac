import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { LayersIcon } from "@/components/icons";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { CreateButton } from "@/components/layout/create-button";
import { ViewDialog } from "@/components/views/view-dialog";
import { ViewList } from "@/components/views/view-list";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { viewParam } from "@/lib/queries/views";

export const Route = createFileRoute("/$workspace/_workspace/views")({
  component: ViewsPage,
});

function ViewsPage() {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const [isCreating, setIsCreating] = useState(false);
  const create = canEdit ? () => setIsCreating(true) : undefined;

  return (
    <PageContainer>
      <PageHeader
        title="Views"
        icon={<LayersIcon />}
        actions={create && <CreateButton label="New view" onClick={create} />}
      />
      <ViewList onCreate={create} />
      <ViewDialog
        target={isCreating ? { mode: "create", draft: { model: "issue" } } : null}
        onClose={() => setIsCreating(false)}
        onSaved={(view) =>
          void navigate({
            to: "/$workspace/view/$view_id",
            params: { workspace: workspace.urlKey, view_id: viewParam(view) },
          })
        }
      />
    </PageContainer>
  );
}
