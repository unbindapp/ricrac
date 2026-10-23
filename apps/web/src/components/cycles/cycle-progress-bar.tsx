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
      className="bg-muted dank:h-1 dank:gap-1 dank:bg-transparent flex h-1.5 w-full gap-0.5 overflow-hidden rounded-full"
    >
      {completed > 0 && (
        <div
          className="bg-chart-1 dank:bg-m3-primary dank:shrink-0 h-full rounded-full"
          style={{ width: `${completed}%` }}
        />
      )}
      {started > 0 && (
        <div
          className="bg-chart-4 dank:bg-m3-tertiary dank:shrink-0 h-full rounded-full"
          style={{ width: `${started}%` }}
        />
      )}
      <div className="bg-m3-secondary-container dank:flex hidden h-full min-w-0 flex-1 justify-end rounded-full">
        <span className="bg-m3-primary size-1 shrink-0 rounded-full" />
      </div>
    </div>
  );
}
