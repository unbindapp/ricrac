import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { AppWindowIcon } from "lucide-react"
import { toast } from "sonner"

import { api } from "@/api/client"
import { connectedAppsQuery } from "@/api/queries"
import { LoadError, Loading, PageHeader, Scopes, Time } from "@/components/common"
import { Confirm } from "@/components/confirm"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { WorkspaceAccess } from "@/components/workspace-access"

export const Route = createFileRoute("/_app/settings/connected-apps")({ component: ConnectedAppsPage })

function ConnectedAppsPage() {
  const apps = useQuery(connectedAppsQuery)
  const queryClient = useQueryClient()
  const revoke = useMutation({
    mutationFn: (id: string) => api("DELETE", `/connected-apps/${id}`),
    onSuccess: () => {
      toast.success("Access revoked")
      return queryClient.invalidateQueries({ queryKey: connectedAppsQuery.queryKey })
    },
    onError: (error) => toast.error(error.message),
  })

  return (
    <>
      <PageHeader
        title="Connected apps"
        description="Agents and apps you authorized with OAuth, in all your workspaces."
      />
      {apps.isPending ? (
        <Loading />
      ) : apps.isError ? (
        <LoadError error={apps.error} what="Connected apps" />
      ) : apps.data.length === 0 ? (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <AppWindowIcon />
            </EmptyMedia>
            <EmptyTitle>No connected apps</EmptyTitle>
            <EmptyDescription>
              When you connect an agent to Ricrac over MCP or OAuth, it shows up here.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">App</TableHead>
                <TableHead>Workspaces</TableHead>
                <TableHead>Scopes</TableHead>
                <TableHead>Authorized</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead className="pr-4">
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {apps.data.map((app) => (
                <TableRow key={app.id}>
                  <TableCell className="pl-4 font-medium">{app.name}</TableCell>
                  <TableCell><WorkspaceAccess all={app.allWorkspaces} workspaces={app.workspaces} /></TableCell>
                  <TableCell>
                    <Scopes scopes={app.scopes} />
                  </TableCell>
                  <TableCell>
                    <Time at={app.createdAt} />
                  </TableCell>
                  <TableCell>
                    <Time at={app.lastUsedAt} />
                  </TableCell>
                  <TableCell className="pr-4 text-right">
                    <Confirm
                      trigger={
                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                          Revoke
                        </Button>
                      }
                      title={`Disconnect ${app.name}?`}
                      description={`It loses access to ${app.allWorkspaces ? "your workspaces" : app.workspaces.join(", ")} right away. You can connect it again later.`}
                      action="Revoke access"
                      onConfirm={() => revoke.mutateAsync(app.id)}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  )
}
