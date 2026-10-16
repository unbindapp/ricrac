import type { TDateResolution } from "@/lib/queries/projects";

/** How precise a planned date is; the server stores the exact day as null. */
export type TResolution = "day" | NonNullable<TDateResolution>;

export const RESOLUTIONS: TResolution[] = ["day", "month", "quarter", "halfYear", "year"];

export const resolutionLabels: Record<TResolution, string> = {
  day: "Day",
  month: "Month",
  quarter: "Quarter",
  halfYear: "Half year",
  year: "Year",
};

export const toResolution = (value: TDateResolution | undefined): TResolution => value ?? "day";
export const fromResolution = (value: TResolution): TDateResolution =>
  value === "day" ? null : value;

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

function parts(date: string) {
  const [year, month, day] = date.split("-").map(Number);
  return { year, month, day };
}

export function formatProjectDate(date: string, resolution: TDateResolution | TResolution = null) {
  const { year, month, day } = parts(date);
  switch (resolution ?? "day") {
    case "day":
      return `${MONTHS[month - 1]} ${day}, ${year}`;
    case "month":
      return `${MONTHS[month - 1]} ${year}`;
    case "quarter":
      return `Q${Math.ceil(month / 3)} ${year}`;
    case "halfYear":
      return `H${month <= 6 ? 1 : 2} ${year}`;
    case "year":
      return String(year);
  }
}

function pad(value: number) {
  return String(value).padStart(2, "0");
}

const periodMonths: Record<Exclude<TResolution, "day">, number> = {
  month: 1,
  quarter: 3,
  halfYear: 6,
  year: 12,
};

/** Moves a date to the first (start dates) or last (target dates) day of its period. */
export function snapToResolution(date: string, resolution: TResolution, edge: "start" | "end") {
  if (resolution === "day") return date;
  const { year, month } = parts(date);
  const length = periodMonths[resolution];
  const firstMonth = Math.floor((month - 1) / length) * length + 1;
  if (edge === "start") return `${year}-${pad(firstMonth)}-01`;
  const lastMonth = firstMonth + length - 1;
  const lastDay = new Date(Date.UTC(year, lastMonth, 0)).getUTCDate();
  return `${year}-${pad(lastMonth)}-${pad(lastDay)}`;
}

/** Whether a target date has passed, counting the whole period of a coarse one. */
export function isPastTarget(date: string, today = new Date().toISOString().slice(0, 10)) {
  return date < today;
}
