import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import {
  BlocksIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  RotateCwIcon,
  Trash2Icon,
  TriangleAlertIcon,
} from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { api } from "@/api/client"
import { configQuery, oauthAppsQuery } from "@/api/queries"
import type { OAuthApp } from "@/api/types/OAuthApp"
import type { OAuthAppSecret } from "@/api/types/OAuthAppSecret"
import type { SaveOAuthApp } from "@/api/types/SaveOAuthApp"
import { CopyButton, CopyValue, LoadError, Loading, PageHeader, Time } from "@/components/common"
import { Confirm } from "@/components/confirm"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { useWorkspace } from "@/lib/workspace"

export const Route = createFileRoute("/_app/settings/oauth-apps")({ component: OAuthAppsPage })

type Editing = { app: OAuthApp | null } | null

function OAuthAppsPage() {
  const workspace = useWorkspace()
  const apps = useQuery(oauthAppsQuery(workspace.id))
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<Editing>(null)
  const [deleting, setDeleting] = useState<OAuthApp | null>(null)
  const [rotating, setRotating] = useState<OAuthApp | null>(null)
  const [secret, setSecret] = useState<OAuthAppSecret | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["oauth-apps"] })

  const rotate = useMutation({
    mutationFn: (id: string) => api<OAuthAppSecret>("POST", `/oauth-apps/${id}/secret`, { workspace: workspace.id }),
    onSuccess: (result) => {
      setRotating(null)
      setSecret(result)
      return refresh()
    },
    onError: (error) => toast.error(error.message),
  })
  const remove = useMutation({
    mutationFn: (id: string) => api("DELETE", `/oauth-apps/${id}`, { workspace: workspace.id }),
    onSuccess: () => {
      toast.success("App deleted")
      return refresh()
    },
    onError: (error) => toast.error(error.message),
  })

  return (
    <>
      <PageHeader
        title="OAuth apps"
        description={`Apps that integrate with ${workspace.name} the way apps integrate with Linear: people authorize them and they act on their behalf.`}
        actions={
          <Button onClick={() => setEditing({ app: null })}>
            <PlusIcon />
            New app
          </Button>
        }
      />
      <div className="space-y-6">
        {apps.isPending ? (
          <Loading />
        ) : apps.isError ? (
          <LoadError error={apps.error} what="OAuth apps" />
        ) : apps.data.length === 0 ? (
          <Empty className="border">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <BlocksIcon />
              </EmptyMedia>
              <EmptyTitle>No OAuth apps</EmptyTitle>
              <EmptyDescription>
                Register an app to get a client ID and secret. MCP clients don't need one: they register themselves.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">Name</TableHead>
                  <TableHead>Client ID</TableHead>
                  <TableHead>Users</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="pr-4">
                    <span className="sr-only">Actions</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {apps.data.map((app) => (
                  <TableRow key={app.id}>
                    <TableCell className="max-w-72 pl-4">
                      <div className="truncate font-medium">{app.name}</div>
                      <div className="truncate text-xs text-muted-foreground" title={app.redirectUris.join("\n")}>
                        {app.redirectUris.length === 1
                          ? app.redirectUris[0]
                          : `${app.redirectUris.length} callback URLs`}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <code className="font-mono text-xs">{app.id}</code>
                        <CopyButton value={app.id} label="Copy client ID" />
                      </div>
                    </TableCell>
                    <TableCell>{app.users}</TableCell>
                    <TableCell>
                      <Time at={app.createdAt} />
                    </TableCell>
                    <TableCell className="pr-4 text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon-sm" aria-label={`Actions for ${app.name}`}>
                            <MoreHorizontalIcon />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem onSelect={() => setEditing({ app })}>
                            <PencilIcon />
                            Edit
                          </DropdownMenuItem>
                          <DropdownMenuItem onSelect={() => setRotating(app)}>
                            <RotateCwIcon />
                            Rotate secret
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem variant="destructive" onSelect={() => setDeleting(app)}>
                            <Trash2Icon />
                            Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
        <Endpoints />
      </div>

      {editing && (
        <AppDialog
          app={editing.app}
          onClose={() => setEditing(null)}
          onCreated={(result) => {
            setEditing(null)
            setSecret(result)
          }}
        />
      )}
      <SecretDialog secret={secret} onClose={() => setSecret(null)} />
      <Confirm
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
        title={`Delete “${deleting?.name}”?`}
        description="Everyone who authorized it is disconnected and its tokens stop working. This can't be undone."
        action="Delete app"
        onConfirm={() => remove.mutateAsync(deleting!.id)}
      />
      <Confirm
        open={rotating !== null}
        onOpenChange={(open) => !open && setRotating(null)}
        title={`Rotate the secret of “${rotating?.name}”?`}
        description="The current secret stops working right away; update your app with the new one."
        action="Rotate secret"
        onConfirm={() => rotate.mutateAsync(rotating!.id)}
      />
    </>
  )
}

function AppDialog({
  app,
  onClose,
  onCreated,
}: {
  app: OAuthApp | null
  onClose: () => void
  onCreated: (secret: OAuthAppSecret) => void
}) {
  const workspace = useWorkspace()
  const queryClient = useQueryClient()
  const [name, setName] = useState(app?.name ?? "")
  const [description, setDescription] = useState(app?.description ?? "")
  const [homepage, setHomepage] = useState(app?.homepageUrl ?? "")
  const [callbacks, setCallbacks] = useState(app?.redirectUris.join("\n") ?? "")

  const save = useMutation({
    mutationFn: (body: SaveOAuthApp): Promise<OAuthApp | OAuthAppSecret> =>
      app
        ? api<OAuthApp>("PATCH", `/oauth-apps/${app.id}`, { body, workspace: workspace.id })
        : api<OAuthAppSecret>("POST", "/oauth-apps", { body, workspace: workspace.id }),
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ["oauth-apps"] })
      if ("clientSecret" in result) return onCreated(result)
      toast.success("App saved")
      onClose()
    },
    onError: (error) => toast.error(error.message),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    save.mutate({
      name: name.trim(),
      description: description.trim(),
      homepageUrl: homepage.trim(),
      redirectUris: callbacks
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean),
    })
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <form onSubmit={submit} className="space-y-6">
          <DialogHeader>
            <DialogTitle>{app ? `Edit ${app.name}` : "New OAuth app"}</DialogTitle>
            <DialogDescription>People see the name and description when they authorize it.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="app-name">Name</FieldLabel>
              <Input
                id="app-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                maxLength={80}
                autoFocus
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="app-description">Description</FieldLabel>
              <Textarea
                id="app-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={2}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="app-homepage">Homepage URL</FieldLabel>
              <Input
                id="app-homepage"
                type="url"
                value={homepage}
                onChange={(e) => setHomepage(e.target.value)}
                placeholder="https://example.com"
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="app-callbacks">Callback URLs</FieldLabel>
              <Textarea
                id="app-callbacks"
                value={callbacks}
                onChange={(e) => setCallbacks(e.target.value)}
                rows={3}
                required
                placeholder="https://example.com/oauth/callback"
                className="font-mono text-sm"
                aria-describedby="app-callbacks-help"
              />
              <FieldDescription id="app-callbacks-help">
                One per line, up to 10: https, http on localhost, or an app scheme.
              </FieldDescription>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending || !name.trim() || !callbacks.trim()}>
              {save.isPending && <Spinner />}
              {app ? "Save" : "Create app"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function SecretDialog({ secret, onClose }: { secret: OAuthAppSecret | null; onClose: () => void }) {
  return (
    <Dialog open={secret !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>Credentials for {secret?.app.name}</DialogTitle>
          <DialogDescription>Configure your app with these.</DialogDescription>
        </DialogHeader>
        {secret && (
          <FieldGroup className="gap-4">
            <Field>
              <FieldLabel>Client ID</FieldLabel>
              <CopyValue value={secret.app.id} label="client ID" />
            </Field>
            <Field>
              <FieldLabel>Client secret</FieldLabel>
              <CopyValue value={secret.clientSecret} label="client secret" />
            </Field>
          </FieldGroup>
        )}
        <Alert>
          <TriangleAlertIcon />
          <AlertTitle>Copy the secret now</AlertTitle>
          <AlertDescription>
            You won't see it again. Rotate it to get a new one; the old one stops working.
          </AlertDescription>
        </Alert>
        <DialogFooter>
          <Button onClick={onClose}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Endpoints() {
  const { data: config } = useQuery(configQuery)
  if (!config) return null
  const base = config.publicUrl
  const rows = [
    ["Authorize", `${base}/oauth/authorize`],
    ["Token", `${base}/oauth/token`],
    ["Revoke", `${base}/oauth/revoke`],
    ["API base", `${base}/v1`],
  ]
  return (
    <Card>
      <CardHeader>
        <CardTitle>Endpoints</CardTitle>
        <CardDescription>
          Standard OAuth 2.1 authorization code flow. Scopes: <code className="font-mono">read write admin</code>{" "}
          (space-separated). PKCE (S256) is supported, and required when the client secret isn't sent. Each
          authorization is for one workspace, chosen by the person authorizing.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="grid gap-3 sm:grid-cols-[6rem_1fr] sm:items-center">
          {rows.map(([label, url]) => (
            <div key={label} className="contents">
              <dt className="text-sm text-muted-foreground">{label}</dt>
              <dd className="min-w-0">
                <CopyValue value={url} label={`${label} URL`} />
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}
