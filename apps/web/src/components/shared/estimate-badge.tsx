import { TriangleIcon } from "@/components/icons";
import { formatEstimate, type TEstimateType } from "@/lib/helpers/estimates";

type TProps = {
  value: number | null | undefined;
  type: TEstimateType;
};

export function EstimateBadge({ value, type }: TProps) {
  if (value === null || value === undefined || type === "notUsed") return null;
  const text = formatEstimate(value, type);
  return (
    <span
      aria-label={`Estimate ${text}`}
      className="text-muted-foreground dank:h-8 dank:gap-2 dank:rounded-sm dank:border-m3-outline-variant dank:pr-3 dank:pl-2 dank:type-label-large dank:text-m3-on-surface-variant inline-flex h-5 shrink-0 items-center gap-1.5 rounded-full border px-2 text-xs leading-none"
    >
      <TriangleIcon className="dank:size-4.5 size-3.5" />
      {text}
    </span>
  );
}
