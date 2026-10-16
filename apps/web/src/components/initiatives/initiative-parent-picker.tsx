import { useQuery } from "@tanstack/react-query";
import { CornerLeftUpIcon } from "lucide-react";

import { InitiativeIcon } from "@/components/initiatives/initiative-status";
import { descendantIds } from "@/components/initiatives/initiative-tree";
import { PickerTrigger, type TPickerAppearance } from "@/components/issues/pickers";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { initiativesQuery } from "@/lib/queries/initiatives";

const NONE = "__none__";

/** The initiative this one is part of; itself and its sub-initiatives aren't offered. */
export function InitiativeParentPicker({
  initiativeId,
  parentId,
  onChange,
  appearance = "row",
  disabled,
}: {
  initiativeId?: string;
  parentId: string | null;
  onChange: (parentId: string | null) => void;
  appearance?: TPickerAppearance;
  disabled?: boolean;
}) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(initiativesQuery(workspace.id));
  const initiatives = data ?? [];
  const excluded = initiativeId ? descendantIds(initiatives, initiativeId) : new Set<string>();
  const parent = initiatives.find((initiative) => initiative.id === parentId);
  const options: TPickerOption[] = [
    {
      value: NONE,
      label: "No parent",
      icon: <CornerLeftUpIcon className="text-muted-foreground size-4" />,
    },
    ...initiatives
      .filter((initiative) => !excluded.has(initiative.id))
      .map((initiative) => ({
        value: initiative.id,
        label: initiative.name,
        icon: <InitiativeIcon initiative={initiative} />,
      })),
  ];
  return (
    <PropertyPicker
      label="Set parent initiative"
      options={options}
      value={[parentId ?? NONE]}
      isPending={isPending}
      disabled={disabled}
      onChange={([id]) => {
        const next = id === NONE ? null : (id ?? null);
        if (next !== parentId) onChange(next);
      }}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={parent ? `Parent: ${parent.name}` : "Set parent initiative"}
          icon={
            parent ? (
              <InitiativeIcon initiative={parent} />
            ) : (
              <CornerLeftUpIcon className="text-muted-foreground size-4" />
            )
          }
          text={parent?.name ?? (appearance === "row" ? "No parent" : "Parent")}
          muted={!parent}
        />
      }
    />
  );
}
