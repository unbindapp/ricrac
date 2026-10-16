import { format, parseISO } from "date-fns";

export type TStageType = "planned" | "started" | "completed" | "canceled";

export const STAGE_TYPES: TStageType[] = ["planned", "started", "completed", "canceled"];

export const stageTypeLabels: Record<TStageType, string> = {
  planned: "Planned",
  started: "In progress",
  completed: "Released",
  canceled: "Canceled",
};

export function isStageType(value: string): value is TStageType {
  return (STAGE_TYPES as string[]).includes(value);
}

/** A calendar day (YYYY-MM-DD) as "Oct 20, 2026", read in local time. */
export function formatDay(day: string) {
  return format(parseISO(day), "MMM d, yyyy");
}

const SHORT_SHA = 7;

export function shortSha(sha: string) {
  return sha.slice(0, SHORT_SHA);
}

type TStage = { id: string; position: number };
type TStaged = { stage: { id: string } | null };

/** Releases grouped by stage in pipeline order; empty stages left out, releases without one last. */
export function groupByStage<S extends TStage, R extends TStaged>(stages: S[], releases: R[]) {
  const ordered = [...stages].sort((a, b) => a.position - b.position);
  const groups = ordered
    .map((stage) => ({
      stage: stage as S | null,
      releases: releases.filter((release) => release.stage?.id === stage.id),
    }))
    .filter((group) => group.releases.length > 0);
  const known = new Set(ordered.map((stage) => stage.id));
  const unstaged = releases.filter((release) => !release.stage || !known.has(release.stage.id));
  if (unstaged.length > 0) groups.push({ stage: null, releases: unstaged });
  return groups;
}

/** The date that says most about a release in its stage. */
export function releaseDateText(release: {
  completedAt: string | null;
  canceledAt: string | null;
  targetDate: string | null;
  startedAt: string | null;
}) {
  if (release.completedAt) return `Released ${format(parseISO(release.completedAt), "MMM d")}`;
  if (release.canceledAt) return `Canceled ${format(parseISO(release.canceledAt), "MMM d")}`;
  if (release.targetDate) return `Target ${format(parseISO(release.targetDate), "MMM d")}`;
  if (release.startedAt) return `Started ${format(parseISO(release.startedAt), "MMM d")}`;
  return null;
}
