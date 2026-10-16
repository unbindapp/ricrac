import { format } from "date-fns";

import type { CycleRef } from "@/api/ops.gen";

export type TCycleStatus = "active" | "upcoming" | "past";

type TCycleDates = Pick<CycleRef, "startsAt" | "endsAt">;

export function cycleStatus(cycle: TCycleDates, now = Date.now()): TCycleStatus {
  if (Date.parse(cycle.startsAt) > now) return "upcoming";
  if (Date.parse(cycle.endsAt) > now) return "active";
  return "past";
}

export const cycleStatusLabels: Record<TCycleStatus, string> = {
  active: "Current",
  upcoming: "Upcoming",
  past: "Completed",
};

/** The last day a cycle runs: it ends at the start of the day after. */
export function cycleLastDay(cycle: Pick<CycleRef, "endsAt">) {
  return new Date(Date.parse(cycle.endsAt) - 1);
}

/** "Mar 3 – 16", "Mar 24 – Apr 6", with the year when it isn't this one. */
export function cycleDateRange(cycle: TCycleDates, now = new Date()) {
  const start = new Date(cycle.startsAt);
  const end = cycleLastDay(cycle);
  const sameYear = start.getFullYear() === end.getFullYear();
  const year = sameYear && end.getFullYear() === now.getFullYear() ? "" : ", yyyy";
  const startText = format(start, sameYear ? "MMM d" : `MMM d${year}`);
  const sameMonth = sameYear && start.getMonth() === end.getMonth();
  return `${startText} – ${format(end, sameMonth ? `d${year}` : `MMM d${year}`)}`;
}

/** Whole days from now until the cycle ends (at least 0). */
export function daysLeft(cycle: Pick<CycleRef, "endsAt">, now = Date.now()) {
  return Math.max(0, Math.ceil((Date.parse(cycle.endsAt) - now) / 86_400_000));
}

export function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

/** "1 pt", "2.5 pts". */
export function pointsText(value: number) {
  return `${formatNumber(value)} ${value === 1 ? "pt" : "pts"}`;
}
