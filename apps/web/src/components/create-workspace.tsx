import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState, type FormEvent } from "react"
import { toast } from "sonner"

import { api } from "@/api/client"
import { meQuery } from "@/api/queries"
import type { CreateWorkspace } from "@/api/types/CreateWorkspace"
import type { WorkspaceSummary } from "@/api/types/WorkspaceSummary"
import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldSet, FieldLegend } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { teamKeyFrom, urlKeyFrom } from "@/lib/slug"
import { selectWorkspace } from "@/lib/workspace"

/** Name, URL key and first team. Keys follow the names until they're edited. */
export function CreateWorkspaceForm({
  onCreated,
  onCancel,
}: {
  onCreated?: (workspace: WorkspaceSummary) => void
  onCancel?: () => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState("")
  const [urlKey, setUrlKey] = useState<string | null>(null)
  const [teamName, setTeamName] = useState("")
  const [teamKey, setTeamKey] = useState<string | null>(null)
  const derivedUrlKey = urlKey ?? urlKeyFrom(name)
  const derivedTeamKey = teamKey ?? teamKeyFrom(teamName.trim() || name)

  const create = useMutation({
    mutationFn: (body: CreateWorkspace) => api<WorkspaceSummary>("POST", "/workspaces", { body }),
    onSuccess: async (workspace) => {
      selectWorkspace(workspace.id)
      await queryClient.invalidateQueries({ queryKey: meQuery.queryKey })
      toast.success(`Created ${workspace.name}`)
      onCreated?.(workspace)
    },
    onError: (error) => toast.error(error.message),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    create.mutate({
      name: name.trim(),
      urlKey: derivedUrlKey,
      teamName: teamName.trim() || undefined,
      teamKey: derivedTeamKey || undefined,
    })
  }

  return (
    <form onSubmit={submit}>
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="ws-name">Workspace name</FieldLabel>
          <Input
            id="ws-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Acme"
            required
            maxLength={80}
            autoFocus
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="ws-key">URL key</FieldLabel>
          <Input
            id="ws-key"
            value={derivedUrlKey}
            onChange={(e) => setUrlKey(e.target.value.toLowerCase())}
            placeholder="acme"
            required
            minLength={2}
            maxLength={32}
            pattern="[a-z0-9]([a-z0-9\-]*[a-z0-9])?"
            className="font-mono"
            aria-describedby="ws-key-help"
          />
          <FieldDescription id="ws-key-help">
            Used in links, like <span className="font-mono">/{derivedUrlKey || "acme"}/issue/ENG-1</span>. Lowercase
            letters, digits and dashes.
          </FieldDescription>
        </Field>
        <FieldSet>
          <FieldLegend variant="label">First team (optional)</FieldLegend>
          <div className="grid grid-cols-[1fr_7rem] gap-3">
            <Field>
              <FieldLabel htmlFor="team-name" className="sr-only">
                Team name
              </FieldLabel>
              <Input
                id="team-name"
                value={teamName}
                onChange={(e) => setTeamName(e.target.value)}
                placeholder={name.trim() || "Engineering"}
                maxLength={80}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="team-key" className="sr-only">
                Team key
              </FieldLabel>
              <Input
                id="team-key"
                value={derivedTeamKey}
                onChange={(e) => setTeamKey(e.target.value.toUpperCase())}
                placeholder="ENG"
                maxLength={7}
                className="font-mono"
                aria-label="Team key"
              />
            </Field>
          </div>
          <FieldDescription>
            Issues are numbered per team, like {derivedTeamKey || "ENG"}-1. Named after the workspace when left empty.
          </FieldDescription>
        </FieldSet>
        <div className="flex justify-end gap-2">
          {onCancel && (
            <Button type="button" variant="outline" onClick={onCancel}>
              Cancel
            </Button>
          )}
          <Button type="submit" disabled={create.isPending || !name.trim()}>
            {create.isPending && <Spinner />}
            Create workspace
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
