import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import type { Milestone } from "@/api/ops.gen";
import { CircleCheckIcon, DiamondIcon, PlusIcon } from "@/components/icons";
import { CreateButton } from "@/components/layout/create-button";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { MilestoneListHeader, MilestoneRow } from "@/components/milestones/milestone-row";
import { MilestoneDialog } from "@/components/projects/milestone-dialog";
import { GroupToggle } from "@/components/shared/group-toggle";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { milestonesQuery } from "@/lib/queries/milestones";

export const Route = createFileRoute("/$workspace/_workspace/milestones")({
  component: MilestonesPage,
});

function MilestonesPage() {
  const { workspace, canEdit } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(milestonesQuery(workspace.id));
  const milestones = data ?? [];
  const newButton = (placement: "header" | "empty") => (
    <MilestoneDialog
      trigger={
        placement === "header" ? (
          <CreateButton label="New milestone" />
        ) : (
          <Button size="sm" variant="outline">
            <PlusIcon className="size-4" />
            New milestone
          </Button>
        )
      }
    />
  );
  return (
    <PageContainer>
      <PageHeader
        title="Milestones"
        icon={<DiamondIcon />}
        actions={canEdit && newButton("header")}
      />
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={() => void refetch()}
        isEmpty={milestones.length === 0}
        empty={{
          icon: DiamondIcon,
          title: "No milestones yet",
          description:
            "Milestones mark what the workspace is working toward across projects. Milestones of synced GitHub repositories show here too.",
          action: canEdit && newButton("empty"),
        }}
      >
        <List flush>
          <MilestoneListHeader />
          <MilestoneGroup
            label="Open"
            icon={<DiamondIcon className="text-muted-foreground size-4" />}
            milestones={milestones.filter((milestone) => !milestone.closedAt)}
          />
          <MilestoneGroup
            label="Closed"
            icon={<CircleCheckIcon className="text-muted-foreground size-4" />}
            milestones={milestones.filter((milestone) => milestone.closedAt)}
            collapsedByDefault
          />
        </List>
      </ListStates>
    </PageContainer>
  );
}

function MilestoneGroup({
  label,
  icon,
  milestones,
  collapsedByDefault = false,
}: {
  label: string;
  icon: ReactNode;
  milestones: Milestone[];
  collapsedByDefault?: boolean;
}) {
  const [isCollapsed, setIsCollapsed] = useState(collapsedByDefault);
  if (milestones.length === 0) return null;
  return (
    <div role="presentation" data-slot="list-group" className="contents">
      <div className="bg-muted dank:h-12 dank:gap-1 dank:bg-transparent dank:px-0 flex h-9 items-center px-1.5 group-data-flush/list:px-4">
        <GroupToggle
          icon={icon}
          label={label}
          count={milestones.length}
          isCollapsed={isCollapsed}
          onToggle={() => setIsCollapsed((value) => !value)}
        />
      </div>
      {!isCollapsed &&
        milestones.map((milestone) => <MilestoneRow key={milestone.id} milestone={milestone} />)}
    </div>
  );
}
