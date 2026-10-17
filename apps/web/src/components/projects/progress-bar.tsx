import { cn } from "@/components/ui/utils";

type TProps = {
  /** From 0 to 1. */
  value: number;
  label: string;
  showValue?: boolean;
  className?: string;
};

export function ProgressBar({ value, label, showValue = true, className }: TProps) {
  const percent = Math.min(100, Math.max(0, Math.round(value * 100)));
  return (
    <div className={cn("flex min-w-0 items-center gap-2", className)}>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
        className="bg-muted h-1.5 w-full min-w-12 overflow-hidden rounded-full"
      >
        <div
          data-done={percent === 100 || undefined}
          className="bg-primary data-done:bg-success h-full rounded-full transition-all"
          style={{ width: `${percent}%` }}
        />
      </div>
      {showValue && (
        <span className="text-muted-foreground w-9 shrink-0 text-right text-xs tabular-nums">
          {percent}%
        </span>
      )}
    </div>
  );
}
