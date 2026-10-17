import type { CycleScope } from "@/api/ops.gen";

type TProps = {
  scope: CycleScope;
  label: string;
};

/** Completed and started shares of a cycle's issues, side by side. */
export function CycleProgressBar({ scope, label }: TProps) {
  const total = Math.max(1, scope.issues);
  const completed = (scope.completed / total) * 100;
  const started = (scope.started / total) * 100;
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(completed)}
      className="bg-muted flex h-1.5 w-full gap-0.5 overflow-hidden rounded-full"
    >
      {completed > 0 && (
        <div className="bg-chart-1 h-full rounded-full" style={{ width: `${completed}%` }} />
      )}
      {started > 0 && (
        <div className="bg-chart-4 h-full rounded-full" style={{ width: `${started}%` }} />
      )}
    </div>
  );
}
