import { format, isSameDay, isSameYear, subDays } from "date-fns";

const MINUTE = 60 * 1000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

type TDateInput = string | number | Date;

function toDate(value: TDateInput) {
  return value instanceof Date ? value : new Date(value);
}

function compactDistance(ms: number) {
  if (ms < HOUR) return `${Math.floor(ms / MINUTE)}m`;
  if (ms < DAY) return `${Math.floor(ms / HOUR)}h`;
  if (ms < WEEK) return `${Math.floor(ms / DAY)}d`;
  if (ms < MONTH) return `${Math.floor(ms / WEEK)}w`;
  if (ms < YEAR) return `${Math.floor(ms / MONTH)}mo`;
  return `${Math.floor(ms / YEAR)}y`;
}

export function formatRelative(value: TDateInput, now: TDateInput = Date.now()) {
  const diff = toDate(now).getTime() - toDate(value).getTime();
  if (Math.abs(diff) < MINUTE) return "just now";
  if (diff > 0) return `${compactDistance(diff)} ago`;
  return `in ${compactDistance(-diff)}`;
}

export function formatDate(value: TDateInput) {
  return format(toDate(value), "MMM d, yyyy");
}

export function formatDateTime(value: TDateInput) {
  return format(toDate(value), "MMM d, yyyy, h:mm a");
}

export function formatTime(value: TDateInput) {
  return format(toDate(value), "h:mm a");
}

/** A day as a chat divides messages: Today, Yesterday, or its date. */
export function formatDay(value: TDateInput, now: TDateInput = Date.now()) {
  const date = toDate(value);
  if (isSameDay(date, now)) return "Today";
  if (isSameDay(date, subDays(toDate(now), 1))) return "Yesterday";
  if (isSameYear(date, now)) return format(date, "EEEE, MMMM d");
  return format(date, "MMMM d, yyyy");
}

/** When a chat message was sent: Today at 3:09 PM, Yesterday at 3:09 PM, or 10/18/2026 3:09 PM. */
export function formatMessageTime(value: TDateInput, now: TDateInput = Date.now()) {
  const date = toDate(value);
  if (isSameDay(date, now)) return `Today at ${formatTime(date)}`;
  if (isSameDay(date, subDays(toDate(now), 1))) return `Yesterday at ${formatTime(date)}`;
  return format(date, "M/d/yyyy h:mm a");
}
