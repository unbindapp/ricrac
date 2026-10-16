import { TargetIcon } from "lucide-react";

import { PickerTrigger, type TPickerAppearance } from "@/components/projects/picker-trigger";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { EntityIcon } from "@/components/shared/entity-icon";
import { StateIcon, type TStateType } from "@/components/shared/state-icon";
import type { TInitiativeStatus } from "@/lib/queries/initiatives";

export const INITIATIVE_STATUSES: TInitiativeStatus[] = ["active", "planned", "completed"];

export const initiativeStatusLabels: Record<TInitiativeStatus, string> = {
  planned: "Planned",
  active: "Active",
  completed: "Completed",
};

const iconTypes: Record<TInitiativeStatus, TStateType> = {
  planned: "unstarted",
  active: "started",
  completed: "completed",
};

export function InitiativeStatusIcon({ status }: { status: TInitiativeStatus }) {
  return (
    <StateIcon
      type={iconTypes[status]}
      name={initiativeStatusLabels[status]}
      className={status === "planned" ? undefined : "text-foreground"}
    />
  );
}

export function InitiativeIcon({
  initiative,
  size,
}: {
  initiative: { icon?: string | null; color?: string | null };
  size?: "sm" | "md" | "lg";
}) {
  return (
    <EntityIcon icon={initiative.icon} color={initiative.color} fallback={TargetIcon} size={size} />
  );
}

export function InitiativeStatusPicker({
  status,
  onChange,
  appearance = "row",
  disabled,
}: {
  status: TInitiativeStatus;
  onChange: (status: TInitiativeStatus) => void;
  appearance?: TPickerAppearance;
  disabled?: boolean;
}) {
  const options: TPickerOption[] = INITIATIVE_STATUSES.map((option) => ({
    value: option,
    label: initiativeStatusLabels[option],
    icon: <InitiativeStatusIcon status={option} />,
  }));
  return (
    <PropertyPicker
      label="Change status"
      options={options}
      value={[status]}
      disabled={disabled}
      onChange={([next]) => {
        const picked = INITIATIVE_STATUSES.find((option) => option === next);
        if (picked && picked !== status) onChange(picked);
      }}
      trigger={
        <PickerTrigger
          appearance={appearance}
          label={`Status: ${initiativeStatusLabels[status]}`}
          icon={<InitiativeStatusIcon status={status} />}
          text={initiativeStatusLabels[status]}
        />
      }
    />
  );
}
