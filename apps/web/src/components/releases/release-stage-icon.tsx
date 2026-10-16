import { StateIcon, type TStateType } from "@/components/shared/state-icon";
import { isStageType, type TStageType } from "@/lib/helpers/releases";

const iconTypes: Record<TStageType, TStateType> = {
  planned: "unstarted",
  started: "started",
  completed: "completed",
  canceled: "canceled",
};

/** A release stage drawn like the issue status of the same kind. */
export function ReleaseStageIcon({
  type,
  color,
  name,
}: {
  type: string;
  color?: string | null;
  name?: string;
}) {
  return (
    <StateIcon type={isStageType(type) ? iconTypes[type] : "backlog"} color={color} name={name} />
  );
}
