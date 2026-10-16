import type { InitiativeDetail } from "@/api/ops.gen";
import { InitiativeParentPicker } from "@/components/initiatives/initiative-parent-picker";
import { InitiativeStatusPicker } from "@/components/initiatives/initiative-status";
import { PropertyRow } from "@/components/layout/property-row";
import { HealthBadge } from "@/components/projects/health-badge";
import { PersonPicker } from "@/components/projects/people";
import { fromResolution, toResolution } from "@/components/projects/project-dates";
import { EntityLinks } from "@/components/projects/project-links";
import { PlannedDatePicker } from "@/components/projects/project-pickers";
import { Separator } from "@/components/ui/separator";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSaveInitiativeMutation } from "@/lib/queries/initiatives";
import type { OpArgs } from "@/lib/server/client";

export function InitiativeProperties({ initiative }: { initiative: InitiativeDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save, mutateAsync: saveAsync } = useSaveInitiativeMutation(workspace.id);
  const update = (args: Omit<OpArgs<"save_initiative">, "id">) =>
    save({ id: initiative.id, ...args });
  const disabled = !canEdit;
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-0.5">
        <PropertyRow label="Status">
          <InitiativeStatusPicker
            status={initiative.status}
            disabled={disabled}
            onChange={(status) => update({ status })}
          />
        </PropertyRow>
        {initiative.health && (
          <PropertyRow label="Health">
            <span className="px-2">
              <HealthBadge health={initiative.health} />
            </span>
          </PropertyRow>
        )}
        <PropertyRow label="Owner">
          <PersonPicker
            role="Owner"
            user={initiative.owner}
            disabled={disabled}
            onChange={(owner) => update({ owner })}
          />
        </PropertyRow>
        <PropertyRow label="Target">
          <PlannedDatePicker
            label="Target date"
            edge="end"
            date={initiative.targetDate}
            resolution={toResolution(initiative.targetDateResolution)}
            disabled={disabled}
            onChange={({ date, resolution }) =>
              update({
                targetDate: date,
                targetDateResolution: date ? fromResolution(resolution) : null,
              })
            }
          />
        </PropertyRow>
        <PropertyRow label="Parent">
          <InitiativeParentPicker
            initiativeId={initiative.id}
            parentId={initiative.parent?.id ?? null}
            disabled={disabled}
            onChange={(parent) => update({ parent })}
          />
        </PropertyRow>
      </div>
      {(canEdit || initiative.links.length > 0) && <Separator />}
      <EntityLinks
        links={initiative.links}
        canEdit={canEdit}
        onAdd={(link) => saveAsync({ id: initiative.id, links: [link] })}
        onRemove={(url) => update({ removeLinks: [url] })}
      />
    </div>
  );
}
