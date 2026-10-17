import type { ProjectDetail } from "@/api/ops.gen";

const segments = [
  { types: ["completed"], label: "done", className: "bg-primary" },
  { types: ["started"], label: "in progress", className: "bg-primary/5-10" },
  {
    types: ["triage", "backlog", "unstarted"],
    label: "to do",
    className: "bg-muted-foreground/3-10",
  },
] as const;

/** How far the project's issues are, by where they stand, and its estimate in points. */
export function ProjectProgress({ project }: { project: ProjectDetail }) {
  const counts = segments.map((segment) => ({
    ...segment,
    count: segment.types.reduce((sum, type) => sum + (project.issueCountByStateType[type] ?? 0), 0),
  }));
  const total = counts.reduce((sum, segment) => sum + segment.count, 0);
  if (total === 0) return null;
  const percent = Math.round(project.progress * 100);
  return (
    <div className="bg-card flex w-full flex-col gap-3 rounded-lg border p-4">
      <div className="flex items-baseline gap-2">
        <span className="text-base font-semibold tabular-nums">{percent}%</span>
        <span className="text-muted-foreground text-sm">complete</span>
        <span className="flex-1" />
        {project.estimate.total > 0 && (
          <span className="text-muted-foreground text-xs tabular-nums">
            {project.estimate.completed} of {project.estimate.total} points
          </span>
        )}
      </div>
      <div
        role="img"
        aria-label={counts.map((segment) => `${segment.count} ${segment.label}`).join(", ")}
        className="bg-muted flex h-2 w-full overflow-hidden rounded-full"
      >
        {counts.map((segment) =>
          segment.count === 0 ? null : (
            <div
              key={segment.label}
              className={segment.className}
              style={{ width: `${(segment.count / total) * 100}%` }}
            />
          ),
        )}
      </div>
      <div className="text-muted-foreground flex flex-wrap items-center gap-3 text-xs">
        {counts.map((segment) => (
          <span key={segment.label} className="flex items-center gap-1.5 tabular-nums">
            <span aria-hidden className={`size-2 rounded-full ${segment.className}`} />
            {segment.count} {segment.label}
          </span>
        ))}
        {project.canceledIssueCount > 0 && (
          <span className="tabular-nums">{project.canceledIssueCount} canceled</span>
        )}
      </div>
    </div>
  );
}
