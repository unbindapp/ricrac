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
      className="text-muted-foreground dux:h-8 dux:gap-2 dux:rounded-sm dux:border-m3-outline-variant dux:pr-3 dux:pl-2 dux:type-label-large dux:text-m3-on-surface-variant inline-flex h-5 shrink-0 items-center gap-1.5 rounded-full border px-2 text-xs leading-none"
    >
      <TriangleIcon className="dux:size-4.5 size-3.5" />
      {text}
    </span>
  );
}
