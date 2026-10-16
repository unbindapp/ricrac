import { type ReactElement } from "react";

import type { ReleaseStage, ReleaseStageRef } from "@/api/ops.gen";
import { PropertyPicker } from "@/components/issues/property-picker";
import { ReleaseStageIcon } from "@/components/releases/release-stage-icon";
import { PropertyTrigger } from "@/components/shared/property-trigger";

type TProps = {
  stages: ReleaseStage[];
  value: ReleaseStageRef | null;
  disabled?: boolean;
  onChange: (stageId: string) => void;
  trigger?: ReactElement;
};

export function ReleaseStagePicker({ stages, value, disabled, onChange, trigger }: TProps) {
  return (
    <PropertyPicker
      label="Stage"
      disabled={disabled}
      value={value ? [value.id] : []}
      onChange={([id]) => {
        if (id && id !== value?.id) onChange(id);
      }}
      options={[...stages]
        .sort((a, b) => a.position - b.position)
        .map((stage) => ({
          value: stage.id,
          label: stage.name,
          icon: <ReleaseStageIcon type={stage.type} color={stage.color} />,
        }))}
      trigger={
        trigger ?? (
          <PropertyTrigger
            label={value ? `Stage: ${value.name}` : "Set stage"}
            icon={<ReleaseStageIcon type={value?.type ?? "planned"} color={value?.color} />}
            text={value?.name ?? "No stage"}
            muted={!value}
          />
        )
      }
    />
  );
}
