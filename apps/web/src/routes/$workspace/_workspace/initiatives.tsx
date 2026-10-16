import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ChevronRightIcon, PlusIcon, TargetIcon } from "lucide-react";
import { useState } from "react";

import type { Initiative } from "@/api/ops.gen";
import { InitiativeListHeader, InitiativeRow } from "@/components/initiatives/initiative-row";
import {
  INITIATIVE_STATUSES,
  InitiativeIcon,
  InitiativeStatusIcon,
  initiativeStatusLabels,
} from "@/components/initiatives/initiative-status";
import { nestInitiatives } from "@/components/initiatives/initiative-tree";
import { NewInitiativeDialog } from "@/components/initiatives/new-initiative-dialog";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { inScope, scopeArgs, type TProjectScope } from "@/components/projects/project-groups";
import { TrashedRow } from "@/components/projects/entity-menu";
import { ScopeEmpty, ScopeTabs } from "@/components/projects/scope-tabs";
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
            <List>
              <InitiativeListHeader />
              {INITIATIVE_STATUSES.map((status) => (
                <StatusGroup
                  key={`${scope}:${status}`}
                  status={status}
                  initiatives={initiatives.filter((initiative) => initiative.status === status)}
                  collapsedByDefault={scope === "active" && status === "completed"}
                />
              ))}
            </List>
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
    <List>
      {initiatives.map((initiative) => (
        <TrashedRow
          key={initiative.id}
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
      ))}
    </List>
  );
}

function StatusGroup({
  status,
  initiatives,
  collapsedByDefault,
}: {
  status: TInitiativeStatus;
  initiatives: Initiative[];
  collapsedByDefault: boolean;
}) {
  const [isCollapsed, setIsCollapsed] = useState(collapsedByDefault);
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
