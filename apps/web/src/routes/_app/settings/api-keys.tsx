import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createFileRoute } from "@tanstack/react-router"
import { KeyRoundIcon, PlusIcon, TriangleAlertIcon } from "lucide-react"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { api } from "@/api/client"
import { apiKeysQuery } from "@/api/queries"
import type { CreateApiKey } from "@/api/types/CreateApiKey"
import type { CreatedApiKey } from "@/api/types/CreatedApiKey"
import { CopyValue, LoadError, Loading, PageHeader, Scopes, Time } from "@/components/common"
import { Confirm } from "@/components/confirm"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { isAdmin, useWorkspace } from "@/lib/workspace"

export const Route = createFileRoute("/_app/settings/api-keys")({ component: ApiKeysPage })

function ApiKeysPage() {
  const workspace = useWorkspace()
  const keys = useQuery(apiKeysQuery(workspace.id))
  const queryClient = useQueryClient()
  const [created, setCreated] = useState<CreatedApiKey | null>(null)
  const revoke = useMutation({
    mutationFn: (id: string) => api("DELETE", `/api-keys/${id}`, { workspace: workspace.id }),
    onSuccess: () => {
      toast.success("API key revoked")
      return queryClient.invalidateQueries({ queryKey: ["api-keys"] })
    },
    onError: (error) => toast.error(error.message),
  })

  return (
    <>
      <PageHeader
        title="API keys"
        description={`Your personal keys for the REST API and MCP in ${workspace.name}. A key acts as you.`}
        actions={<CreateKeyDialog onCreated={setCreated} />}
      />
      {keys.isPending ? (
        <Loading />
      ) : keys.isError ? (
        <LoadError error={keys.error} what="API keys" />
      ) : keys.data.length === 0 ? (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <KeyRoundIcon />
            </EmptyMedia>
            <EmptyTitle>No API keys</EmptyTitle>
            <EmptyDescription>
              Create one for scripts and agents that can't sign in with OAuth. Send it as{" "}
              <code className="font-mono">Authorization: Bearer …</code>.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="pl-4">Label</TableHead>
                <TableHead>Key</TableHead>
                <TableHead>Scopes</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead className="pr-4">
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.data.map((key) => (
                <TableRow key={key.id}>
                  <TableCell className="pl-4 font-medium">{key.label}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{key.keyHint}</TableCell>
                  <TableCell>
                    <Scopes scopes={key.scopes} />
                  </TableCell>
                  <TableCell>
                    <Time at={key.createdAt} />
                  </TableCell>
                  <TableCell>
                    <Time at={key.lastUsedAt} />
                  </TableCell>
                  <TableCell className="pr-4 text-right">
                    <Confirm
                      trigger={
                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                          Revoke
                        </Button>
                      }
                      title={`Revoke “${key.label}”?`}
                      description="Anything using this key stops working right away. This can't be undone."
                      action="Revoke key"
                      onConfirm={() => revoke.mutateAsync(key.id)}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <SecretDialog created={created} onClose={() => setCreated(null)} />
    </>
  )
}

const scopeOptions = [
  { scope: "read", label: "Read", description: "Read everything the key's owner can see." },
  { scope: "write", label: "Write", description: "Create and change issues, projects, comments and more." },
  { scope: "admin", label: "Admin", description: "Workspace administration: members, teams, settings." },
] as const

function CreateKeyDialog({ onCreated }: { onCreated: (key: CreatedApiKey) => void }) {
  const workspace = useWorkspace()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [scopes, setScopes] = useState<string[]>(["read", "write"])
  const options = scopeOptions.filter((o) => o.scope !== "admin" || isAdmin(workspace.role))

  const create = useMutation({
    mutationFn: (body: CreateApiKey) => api<CreatedApiKey>("POST", "/api-keys", { body, workspace: workspace.id }),
    onSuccess: (created) => {
      setOpen(false)
      setLabel("")
      setScopes(["read", "write"])
      onCreated(created)
      return queryClient.invalidateQueries({ queryKey: ["api-keys"] })
    },
    onError: (error) => toast.error(error.message),
  })

  const toggle = (scope: string, on: boolean) =>
    setScopes((current) => (on ? [...current, scope] : current.filter((s) => s !== scope)))

  const submit = (event: FormEvent) => {
    event.preventDefault()
    create.mutate({ label: label.trim(), scopes })
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <PlusIcon />
          Create key
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form onSubmit={submit} className="space-y-6">
          <DialogHeader>
            <DialogTitle>Create an API key</DialogTitle>
            <DialogDescription>For {workspace.name}. You'll see the key once.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="key-label">Label</FieldLabel>
              <Input
                id="key-label"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="CI agent"
                required
                maxLength={80}
                autoFocus
              />
            </Field>
            <FieldSet>
              <FieldLegend variant="label">Scopes</FieldLegend>
              <FieldGroup className="gap-3">
                {options.map((option) => (
                  <Field key={option.scope} orientation="horizontal">
                    <Checkbox
                      id={`scope-${option.scope}`}
                      checked={option.scope === "read" || scopes.includes(option.scope)}
                      disabled={option.scope === "read"}
                      onCheckedChange={(checked) => toggle(option.scope, checked === true)}
                    />
                    <FieldContent>
                      <FieldLabel htmlFor={`scope-${option.scope}`}>{option.label}</FieldLabel>
                      <FieldDescription>{option.description}</FieldDescription>
                    </FieldContent>
                  </Field>
                ))}
              </FieldGroup>
            </FieldSet>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending || !label.trim()}>
              {create.isPending && <Spinner />}
              Create key
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function SecretDialog({ created, onClose }: { created: CreatedApiKey | null; onClose: () => void }) {
  return (
    <Dialog open={created !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>Your new API key</DialogTitle>
          <DialogDescription>
            “{created?.key.label}”, with {created?.key.scopes.join(", ")} access.
          </DialogDescription>
        </DialogHeader>
        {created && <CopyValue value={created.secret} label="API key" />}
        <Alert>
          <TriangleAlertIcon />
          <AlertTitle>Copy it now</AlertTitle>
          <AlertDescription>
            You won't see this key again. Store it somewhere safe; anyone with it can act as you.
          </AlertDescription>
        </Alert>
        <DialogFooter>
          <Button onClick={onClose}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
