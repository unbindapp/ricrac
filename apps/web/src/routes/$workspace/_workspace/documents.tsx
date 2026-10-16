import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { FileTextIcon, PlusIcon, SearchIcon, Trash2Icon } from "lucide-react";
import { type ReactNode, useState } from "react";
import { z } from "zod";

import { DocumentRow } from "@/components/documents/document-row";
import { NewDocumentDialog } from "@/components/documents/new-document-dialog";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { documentsQuery } from "@/lib/queries/documents";

const searchSchema = z.object({ view: z.enum(["all", "trash"]).optional() });

export const Route = createFileRoute("/$workspace/_workspace/documents")({
  validateSearch: zodValidator(searchSchema),
  component: DocumentsPage,
});

function DocumentsPage() {
  const { workspace, canEdit } = useWorkspace();
  const { view = "all" } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [query, setQuery] = useState("");
  const isTrash = view === "trash";
  const { data, isPending, error, refetch } = useQuery(
    documentsQuery(workspace.id, isTrash ? { includeTrashed: true } : {}),
  );
  const needle = query.trim().toLowerCase();
  const documents = (data ?? []).filter(
    (document) =>
      document.trashed === isTrash && (!needle || document.title.toLowerCase().includes(needle)),
  );
  const newButton = (placement: "header" | "empty") => (
    <NewDocumentDialog
      trigger={
        <Button
          size={placement === "header" ? "default" : "sm"}
          variant={placement === "header" ? "default" : "outline"}
        >
          <PlusIcon className="size-4" />
          New document
        </Button>
      }
    />
  );

  return (
    <PageContainer>
      <PageHeader
        title="Documents"
        icon={<FileTextIcon />}
        actions={canEdit && newButton("header")}
        toolbar={
          <PageToolbar
            actions={
              <Tabs
                value={view}
                onValueChange={(value) =>
                  void navigate({ search: { view: value === "trash" ? "trash" : undefined } })
                }
              >
                <TabsList className="border-b-0">
                  <TabsTab value="all">All</TabsTab>
                  <TabsTab value="trash">
                    <Trash2Icon />
                    Trash
                  </TabsTab>
                </TabsList>
              </Tabs>
            }
          >
            <div className="relative w-full max-w-xs">
              <SearchIcon className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                aria-label="Filter documents"
                placeholder="Filter documents…"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") setQuery("");
                }}
                className="h-8 pl-8"
              />
            </div>
          </PageToolbar>
        }
      />
      {isPending && <ListSkeleton />}
      {error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && documents.length === 0 && (
        <DocumentsEmpty
          isTrash={isTrash}
          query={needle}
          action={canEdit && !isTrash && newButton("empty")}
        />
      )}
      {documents.length > 0 && (
        <List>
          {documents.map((document) => (
            <DocumentRow key={document.id} document={document} />
          ))}
        </List>
      )}
    </PageContainer>
  );
}

function DocumentsEmpty({
  isTrash,
  query,
  action,
}: {
  isTrash: boolean;
  query: string;
  action: ReactNode;
}) {
  if (query) {
    return (
      <EmptyState
        icon={SearchIcon}
        title="No matching documents"
        description={`Nothing matches “${query}”.`}
      />
    );
  }
  if (isTrash) {
    return (
      <EmptyState
        icon={Trash2Icon}
        title="No trashed documents yet"
        description="Documents moved to the trash stay here for 30 days."
      />
    );
  }
  return (
    <EmptyState
      icon={FileTextIcon}
      title="No documents yet"
      description="Write specs, notes and decisions next to the team, project or initiative they belong to."
      action={action}
    />
  );
}
