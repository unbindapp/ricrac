import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ChevronRightIcon, PlusIcon, TargetIcon } from "lucide-react";
import { useState } from "react";

import type { Initiative } from "@/api/ops.gen";
import { InitiativeListHeader, InitiativeRow } from "@/components/initiatives/initiative-row";
import {
  INITIATIVE_STATUSES,
  InitiativeStatusIcon,
  initiativeStatusLabels,
} from "@/components/initiatives/initiative-status";
import { nestInitiatives } from "@/components/initiatives/initiative-tree";
import { NewInitiativeDialog } from "@/components/initiatives/new-initiative-dialog";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { initiativesQuery, type TInitiativeStatus } from "@/lib/queries/initiatives";

export const Route = createFileRoute("/$workspace/_workspace/initiatives")({
  component: InitiativesPage,
});

function InitiativesPage() {
  const { workspace, canEdit } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(initiativesQuery(workspace.id));
  const initiatives = data ?? [];
  const newButton = (placement: "header" | "empty") => (
    <NewInitiativeDialog
      trigger={
        <Button
          size={placement === "header" ? "default" : "sm"}
          variant={placement === "header" ? "default" : "outline"}
        >
          <PlusIcon className="size-4" />
          New initiative
        </Button>
      }
    />
  );
  return (
    <PageContainer>
      <PageHeader
        title="Initiatives"
        icon={<TargetIcon />}
        actions={canEdit && newButton("header")}
      />
      <ListStates
        isPending={isPending}
        error={error}
        onRetry={() => void refetch()}
        isEmpty={initiatives.length === 0}
        empty={{
          icon: TargetIcon,
          title: "No initiatives yet",
          description:
            "Initiatives roll projects up into larger goals, and can hold sub-initiatives of their own.",
          action: canEdit && newButton("empty"),
        }}
      >
        <List>
          <InitiativeListHeader />
          {INITIATIVE_STATUSES.map((status) => (
            <StatusGroup
              key={status}
              status={status}
              initiatives={initiatives.filter((initiative) => initiative.status === status)}
            />
          ))}
        </List>
      </ListStates>
    </PageContainer>
  );
}

function StatusGroup({
  status,
  initiatives,
}: {
  status: TInitiativeStatus;
  initiatives: Initiative[];
}) {
  const [isCollapsed, setIsCollapsed] = useState(status === "completed");
  if (initiatives.length === 0) return null;
  const ids = new Set(initiatives.map((initiative) => initiative.id));
  return (
    <div role="presentation" className="contents">
      <div className="bg-muted flex h-9 items-center border-b px-1.5">
        <Button
          variant="ghost"
          size="sm"
          forceMinSize={false}
          aria-expanded={!isCollapsed}
          onClick={() => setIsCollapsed((value) => !value)}
        >
          <ChevronRightIcon
            data-open={!isCollapsed || undefined}
            className="text-muted-foreground size-3.5 transition-transform data-open:rotate-90"
          />
          <InitiativeStatusIcon status={status} />
          {initiativeStatusLabels[status]}
          <span className="text-muted-foreground text-xs font-normal tabular-nums">
            {initiatives.length}
          </span>
        </Button>
      </div>
      {!isCollapsed &&
        nestInitiatives(initiatives).map(({ item, depth }) => (
          <InitiativeRow
            key={item.id}
            initiative={item}
            depth={depth}
            showParent={Boolean(item.parent && !ids.has(item.parent.id))}
          />
        ))}
    </div>
  );
}
