import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import type { Milestone } from "@/api/ops.gen";
import { CircleCheckIcon, DiamondIcon, PlusIcon } from "@/components/icons";
import { CreateButton } from "@/components/layout/create-button";
import { VirtualList } from "@/components/layout/list";
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
    <PageContainer scroll="window">
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
        <MilestoneList milestones={milestones} />
      </ListStates>
    </PageContainer>
  );
}

type TMilestoneGroup = "open" | "closed";

type TMilestoneListItem =
  | { kind: "columns" }
  | { kind: "header"; group: TMilestoneGroup; count: number; isCollapsed: boolean }
  | { kind: "row"; milestone: Milestone };

const milestoneGroups = {
  open: { label: "Open", icon: DiamondIcon, collapsedByDefault: false },
  closed: { label: "Closed", icon: CircleCheckIcon, collapsedByDefault: true },
} satisfies Record<TMilestoneGroup, unknown>;

function MilestoneList({ milestones }: { milestones: Milestone[] }) {
  const [toggled, setToggled] = useState<TMilestoneGroup[]>([]);
  const toggle = (group: TMilestoneGroup) =>
    setToggled((current) =>
      current.includes(group) ? current.filter((value) => value !== group) : [...current, group],
    );

  const items: TMilestoneListItem[] = [{ kind: "columns" }];
  for (const group of ["open", "closed"] as const) {
    const members = milestones.filter(
      (milestone) => Boolean(milestone.closedAt) === (group === "closed"),
    );
    if (members.length === 0) continue;
    const isCollapsed = milestoneGroups[group].collapsedByDefault !== toggled.includes(group);
    items.push({ kind: "header", group, count: members.length, isCollapsed });
    if (isCollapsed) continue;
    for (const milestone of members) items.push({ kind: "row", milestone });
  }

  return (
    <VirtualList
      flush
      items={items}
      getKey={milestoneListKey}
      estimateSize={milestoneListItemSize}
      renderItem={(item) => {
        if (item.kind === "columns") return <MilestoneListHeader />;
        if (item.kind === "row") return <MilestoneRow milestone={item.milestone} />;
        const { label, icon: Icon } = milestoneGroups[item.group];
        return (
          <div className="bg-muted dank:h-12 dank:gap-1 dank:bg-transparent dank:px-0 flex h-9 items-center px-1.5 group-data-flush/list:px-4">
            <GroupToggle
              icon={<Icon className="text-muted-foreground size-4" />}
              label={label}
              count={item.count}
              isCollapsed={item.isCollapsed}
              onToggle={() => toggle(item.group)}
            />
          </div>
        );
      }}
    />
  );
}

function milestoneListKey(item: TMilestoneListItem) {
  if (item.kind === "columns") return "columns";
  if (item.kind === "header") return `header:${item.group}`;
  return item.milestone.id;
}

function milestoneListItemSize(item: TMilestoneListItem) {
  if (item.kind === "columns") return 32;
  if (item.kind === "header") return 36;
  return 44;
}
