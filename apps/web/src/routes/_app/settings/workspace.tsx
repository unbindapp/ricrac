import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { LockIcon } from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { isMissingOp, op } from "@/api/client"
import { meQuery, workspaceQuery } from "@/api/queries"
import type { Workspace } from "@/api/v1"
import { LoadError, Loading, PageHeader, Properties, Property } from "@/components/common"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { isAdmin, useWorkspace } from "@/lib/workspace"

export const Route = createFileRoute("/_app/settings/workspace")({ component: WorkspacePage })

function WorkspacePage() {
  const workspace = useWorkspace()
  const details = useQuery(workspaceQuery(workspace.id))
  return (
    <>
      <PageHeader
        title="Workspace"
        description={`Settings and teams of ${workspace.name}. Your role: ${workspace.role}.`}
      />
      {details.isPending ? (
        <Loading />
      ) : isMissingOp(details.error) ? (
        <Card className="max-w-2xl">
          <CardHeader>
            <CardTitle>General</CardTitle>
            <CardDescription>Changing these and listing teams arrive with a server update.</CardDescription>
          </CardHeader>
          <CardContent>
            <Properties>
              <Property label="Name">{workspace.name}</Property>
              <Property label="URL key">
                <span className="font-mono">{workspace.urlKey}</span>
              </Property>
              <Property label="Your role">
                <span className="capitalize">{workspace.role}</span>
              </Property>
            </Properties>
          </CardContent>
        </Card>
      ) : details.isError ? (
        <LoadError error={details.error} what="Workspace settings" />
      ) : (
        <div className="space-y-6">
          <GeneralCard workspace={details.data} />
          <Card>
            <CardHeader>
              <CardTitle>Teams</CardTitle>
              <CardDescription>
                {details.data.memberCount} {details.data.memberCount === 1 ? "member" : "members"} ·{" "}
                {details.data.issueCount} {details.data.issueCount === 1 ? "issue" : "issues"}. Agents manage teams
                through the API.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="rounded-lg border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="pl-4">Name</TableHead>
                      <TableHead>Key</TableHead>
                      <TableHead className="pr-4">Visibility</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {details.data.teams.map((team) => (
                      <TableRow key={team.id}>
                        <TableCell className="pl-4 font-medium">{team.name}</TableCell>
                        <TableCell className="font-mono text-xs">{team.key}</TableCell>
                        <TableCell className="pr-4">
                          {team.private ? (
                            <Badge variant="outline">
                              <LockIcon />
                              Private
                            </Badge>
                          ) : (
                            <span className="text-sm text-muted-foreground">Everyone</span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </>
  )
}

function GeneralCard({ workspace }: { workspace: Workspace }) {
  const queryClient = useQueryClient()
  const admin = isAdmin(workspace.role)
  const [name, setName] = useState(workspace.name)
  const [urlKey, setUrlKey] = useState(workspace.urlKey)
  const changed = name.trim() !== workspace.name || urlKey.trim() !== workspace.urlKey

  const save = useMutation({
    mutationFn: () =>
      op<Workspace>(
        "update_workspace",
        {
          name: name.trim() !== workspace.name ? name.trim() : undefined,
          urlKey: urlKey.trim() !== workspace.urlKey ? urlKey.trim() : undefined,
        },
        workspace.id,
      ),
    onSuccess: async () => {
      toast.success("Workspace saved")
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: meQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: ["workspace", workspace.id] }),
      ])
    },
    onError: (error) => toast.error(error.message),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    save.mutate()
  }

  return (
    <Card className="max-w-2xl">
      <form onSubmit={submit} className="contents">
        <CardHeader>
          <CardTitle>General</CardTitle>
          <CardDescription>
            {admin ? "Admins and owners can change these." : "Only admins can change these."}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="ws-name">Name</FieldLabel>
              <Input
                id="ws-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={!admin}
                required
                maxLength={80}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="ws-key">URL key</FieldLabel>
              <Input
                id="ws-key"
                value={urlKey}
                onChange={(e) => setUrlKey(e.target.value.toLowerCase())}
                disabled={!admin}
                required
                className="font-mono"
              />
              <FieldDescription>
                Links look like <span className="font-mono break-all">{workspace.url}</span>. Changing the key changes
                them.
              </FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
        {admin && (
          <CardFooter className="justify-end">
            <Button type="submit" disabled={!changed || save.isPending}>
              {save.isPending && <Spinner />}
              Save
            </Button>
          </CardFooter>
        )}
      </form>
    </Card>
  )
}
