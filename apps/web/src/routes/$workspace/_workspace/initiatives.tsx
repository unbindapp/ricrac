import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import type { Initiative } from "@/api/ops.gen";
import { PlusIcon, TargetIcon } from "@/components/icons";
import { InitiativeListHeader, InitiativeRow } from "@/components/initiatives/initiative-row";
import {
  INITIATIVE_STATUSES,
  InitiativeIcon,
  InitiativeStatusIcon,
  initiativeStatusLabels,
} from "@/components/initiatives/initiative-status";
import { nestInitiatives } from "@/components/initiatives/initiative-tree";
import { NewInitiativeDialog } from "@/components/initiatives/new-initiative-dialog";
import { VirtualList, isHeaderItem } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { CreateButton } from "@/components/layout/create-button";
import { inScope, scopeArgs, type TProjectScope } from "@/components/projects/project-groups";
import { TrashedRow } from "@/components/projects/entity-menu";
import { ScopeEmpty, ScopeTabs } from "@/components/projects/scope-tabs";
import { GroupToggle } from "@/components/shared/group-toggle";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  initiativesQuery,
  type TInitiativeStatus,
  useInitiativeLifecycleMutation,
} from "@/lib/queries/initiatives";

export const Route = createFileRoute("/$workspace/_workspace/initiatives")({
  component: InitiativesPage,
});

function InitiativesPage() {
  const { workspace, canEdit } = useWorkspace();
  const [scope, setScope] = useState<TProjectScope>("active");
  const { data, isPending, error, refetch } = useQuery(
    initiativesQuery(workspace.id, scopeArgs(scope)),
  );
  const initiatives = (data ?? []).filter((initiative) => inScope(initiative, scope));
  const newButton = (placement: "header" | "empty") => (
    <NewInitiativeDialog
      trigger={
        placement === "header" ? (
          <CreateButton label="New initiative" />
        ) : (
          <Button size="sm" variant="outline">
            <PlusIcon className="size-4" />
            New initiative
          </Button>
        )
      }
    />
  );
  return (
    <PageContainer scroll="window">
      <PageHeader
        title="Initiatives"
        icon={<TargetIcon />}
        actions={canEdit && newButton("header")}
        toolbar={<PageToolbar actions={<ScopeTabs scope={scope} onChange={setScope} />} />}
      />
      {scope !== "active" && !isPending && !error && initiatives.length === 0 ? (
        <ScopeEmpty scope={scope} noun="initiative" />
      ) : (
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
          {scope === "trash" ? (
            <TrashedInitiatives initiatives={initiatives} />
          ) : (
            <InitiativeList key={scope} initiatives={initiatives} scope={scope} />
          )}
        </ListStates>
      )}
    </PageContainer>
  );
}

function TrashedInitiatives({ initiatives }: { initiatives: Initiative[] }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: change, isPending, variables } = useInitiativeLifecycleMutation(workspace.id);
  return (
    <VirtualList
      flush
      items={initiatives}
      getKey={(initiative) => initiative.id}
      renderItem={(initiative) => (
        <TrashedRow
          icon={<InitiativeIcon initiative={initiative} />}
          name={initiative.name}
          trashedAt={initiative.trashedAt}
          isPending={isPending && variables?.id === initiative.id}
          onRestore={
            canEdit
              ? () =>
                  change(
                    { id: initiative.id, action: "restore" },
                    {
                      onSuccess: () => toast.add({ type: "success", title: "Initiative restored" }),
                    },
                  )
              : undefined
          }
        />
      )}
    />
  );
}

type TInitiativeListItem =
  | { kind: "columns" }
  | { kind: "header"; status: TInitiativeStatus; count: number; isCollapsed: boolean }
  | { kind: "row"; initiative: Initiative; depth: number; showParent: boolean };

function InitiativeList({
  initiatives,
  scope,
}: {
  initiatives: Initiative[];
  scope: TProjectScope;
}) {
  const [toggled, setToggled] = useState<TInitiativeStatus[]>([]);
  const toggle = (status: TInitiativeStatus) =>
    setToggled((current) =>
      current.includes(status) ? current.filter((value) => value !== status) : [...current, status],
    );

  const items: TInitiativeListItem[] = [{ kind: "columns" }];
  for (const status of INITIATIVE_STATUSES) {
    const group = initiatives.filter((initiative) => initiative.status === status);
    if (group.length === 0) continue;
    const collapsedByDefault = scope === "active" && status === "completed";
    const isCollapsed = collapsedByDefault !== toggled.includes(status);
    items.push({ kind: "header", status, count: group.length, isCollapsed });
    if (isCollapsed) continue;
    const ids = new Set(group.map((initiative) => initiative.id));
    for (const { item, depth } of nestInitiatives(group)) {
      const showParent = Boolean(item.parent && !ids.has(item.parent.id));
      items.push({ kind: "row", initiative: item, depth, showParent });
    }
  }

  return (
    <VirtualList
      isSticky={isHeaderItem}
      flush
      items={items}
      getKey={initiativeListKey}
      estimateSize={initiativeListItemSize}
      renderItem={(item) => {
        if (item.kind === "columns") return <InitiativeListHeader />;
        if (item.kind === "row") {
          return (
            <InitiativeRow
              initiative={item.initiative}
              depth={item.depth}
              showParent={item.showParent}
            />
          );
        }
        return (
          <div className="bg-muted dank:h-12 dank:gap-1 dank:bg-transparent dank:px-0 flex h-9 items-center px-1.5 group-data-flush/list:px-1">
            <GroupToggle
              icon={<InitiativeStatusIcon status={item.status} />}
              label={initiativeStatusLabels[item.status]}
              count={item.count}
              isCollapsed={item.isCollapsed}
              onToggle={() => toggle(item.status)}
            />
          </div>
        );
      }}
    />
  );
}

function initiativeListKey(item: TInitiativeListItem) {
  if (item.kind === "columns") return "columns";
  if (item.kind === "header") return `header:${item.status}`;
  return item.initiative.id;
}

function initiativeListItemSize(item: TInitiativeListItem) {
  if (item.kind === "columns") return 32;
  if (item.kind === "header") return 36;
  return 44;
}
