import type { WorkspaceSummary } from "@/api/types/WorkspaceSummary"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"

/** The workspaces a key or app reaches, or null for all of them, including ones joined later. */
export type WorkspaceChoice = string[] | null

export function WorkspaceAccess({ all, workspaces }: { all: boolean; workspaces: string[] }) {
  return <>{all ? "All workspaces" : workspaces.join(", ")}</>
}

function allDescription(workspaces: WorkspaceSummary[]) {
  const names = workspaces.length <= 3 ? workspaces.map((w) => w.name).join(", ") : `Your ${workspaces.length} workspaces`
  return `${names} and any you join later.`
}

export function WorkspaceAccessField({
  id,
  workspaces,
  value,
  preferred,
  onChange,
}: {
  id: string
  workspaces: WorkspaceSummary[]
  value: WorkspaceChoice
  /** Ticked first when switching to picking workspaces. */
  preferred?: string
  onChange: (value: WorkspaceChoice) => void
}) {
  const toggle = (workspace: string, on: boolean) =>
    onChange(on ? [...(value ?? []), workspace] : (value ?? []).filter((w) => w !== workspace))

  return (
    <FieldSet>
      <FieldLegend variant="label">Workspaces</FieldLegend>
      <RadioGroup
        value={value === null ? "all" : "some"}
        onValueChange={(choice) => onChange(choice === "all" ? null : preferred ? [preferred] : [])}
      >
        <Field orientation="horizontal">
          <RadioGroupItem value="all" id={`${id}-all`} />
          <FieldContent>
            <FieldLabel htmlFor={`${id}-all`}>All workspaces</FieldLabel>
            <FieldDescription>{allDescription(workspaces)}</FieldDescription>
          </FieldContent>
        </Field>
        <Field orientation="horizontal">
          <RadioGroupItem value="some" id={`${id}-some`} />
          <FieldLabel htmlFor={`${id}-some`}>Only the ones I pick</FieldLabel>
        </Field>
      </RadioGroup>
      {value !== null && (
        <FieldGroup className="gap-3 pl-7">
          {workspaces.map((workspace) => (
            <Field key={workspace.id} orientation="horizontal">
              <Checkbox
                id={`${id}-${workspace.id}`}
                checked={value.includes(workspace.id)}
                onCheckedChange={(checked) => toggle(workspace.id, checked === true)}
              />
              <FieldLabel htmlFor={`${id}-${workspace.id}`} className="font-normal">
                {workspace.name}
              </FieldLabel>
            </Field>
          ))}
        </FieldGroup>
      )}
    </FieldSet>
  )
}
