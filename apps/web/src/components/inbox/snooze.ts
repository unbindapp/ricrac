import { addDays, addHours, nextMonday, set } from "date-fns";

export type TSnoozeOption = {
  id: "hour" | "later" | "tomorrow" | "next-week";
  label: string;
  until: Date;
};

const MORNING = { hours: 9, minutes: 0, seconds: 0, milliseconds: 0 };

export function snoozeOptions(now: Date): TSnoozeOption[] {
  return [
    { id: "hour", label: "In 1 hour", until: addHours(now, 1) },
    { id: "later", label: "In 3 hours", until: addHours(now, 3) },
    { id: "tomorrow", label: "Tomorrow morning", until: set(addDays(now, 1), MORNING) },
    { id: "next-week", label: "Next Monday", until: set(nextMonday(now), MORNING) },
  ];
}
