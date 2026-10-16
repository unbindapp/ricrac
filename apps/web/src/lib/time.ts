const relative = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" })
const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
]

/** An ISO timestamp relative to now, like "in 6 hours" or "2 days ago". */
export function relativeTime(iso: string, now = Date.now()) {
  const seconds = (new Date(iso).getTime() - now) / 1000
  const [unit, size] = units.find(([, size]) => Math.abs(seconds) >= size) ?? ["second", 1]
  return Math.abs(seconds) < 60 ? "just now" : relative.format(Math.round(seconds / size), unit)
}

const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" })
const dateOnly = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeZone: "UTC" })

export const formatDateTime = (iso: string) => dateTime.format(new Date(iso))

/** A calendar date like 2026-10-16, shown without shifting it to the local time zone. */
export const formatDate = (date: string) => dateOnly.format(new Date(date.length === 10 ? `${date}T00:00:00Z` : date))
