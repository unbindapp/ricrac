import { format } from "date-fns";
import { type PointerEvent, useState } from "react";

import type { Cycle, CycleSnapshot } from "@/api/ops.gen";
import { cycleLastDay } from "@/components/cycles/cycle-status";

const DAY_MS = 86_400_000;
const WIDTH = 600;
const HEIGHT = 160;
const PAD = { top: 8, right: 8, bottom: 4, left: 8 };

type TKey = "scope" | "started" | "completed";
type TDay = { date: Date; point?: CycleSnapshot };

const SERIES: { key: TKey; label: string; swatch: string; stroke: string }[] = [
  {
    key: "scope",
    label: "Scope",
    swatch: "bg-muted-foreground",
    stroke: "stroke-muted-foreground",
  },
  { key: "started", label: "Started", swatch: "bg-chart-4", stroke: "stroke-chart-4" },
  { key: "completed", label: "Completed", swatch: "bg-chart-1", stroke: "stroke-chart-1" },
];

function cycleDays(cycle: Cycle): TDay[] {
  const byDate = new Map(cycle.history.map((point) => [point.date, point]));
  const start = new Date(cycle.startsAt);
  const count = Math.max(1, Math.round((Date.parse(cycle.endsAt) - start.getTime()) / DAY_MS));
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(start.getTime() + index * DAY_MS);
    return { date, point: byDate.get(date.toISOString().slice(0, 10)) };
  });
}

/** The cycle's issues each day it has run: in scope, started and completed, with a hover readout. */
export function BurnUpChart({ cycle }: { cycle: Cycle }) {
  const [hovered, setHovered] = useState<number | null>(null);
  const days = cycleDays(cycle);
  const recorded = days.flatMap((day, index) => (day.point ? [{ index, point: day.point }] : []));
  if (recorded.length === 0) return null;

  const max = Math.max(1, ...recorded.map((day) => day.point.scope));
  const step = (WIDTH - PAD.left - PAD.right) / Math.max(1, days.length - 1);
  const x = (index: number) => PAD.left + index * step;
  const y = (value: number) => PAD.top + (1 - value / max) * (HEIGHT - PAD.top - PAD.bottom);
  const line = (key: TKey) =>
    recorded.map((day, n) => `${n ? "L" : "M"}${x(day.index)},${y(day.point[key])}`).join(" ");
  const first = recorded[0];
  const last = recorded[recorded.length - 1];
  const completedArea = `${line("completed")} L${x(last.index)},${y(0)} L${x(first.index)},${y(0)} Z`;
  const shown = recorded.find((day) => day.index === hovered) ?? null;

  const onPointerMove = (event: PointerEvent<SVGRectElement>) => {
    const box = event.currentTarget.getBoundingClientRect();
    const index = Math.round((((event.clientX - box.left) / box.width) * WIDTH - PAD.left) / step);
    const nearest = recorded.reduce((best, day) =>
      Math.abs(day.index - index) < Math.abs(best.index - index) ? day : best,
    );
    setHovered(nearest.index);
  };

  return (
    <figure className="flex w-full flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3 text-xs">
        <figcaption className="text-muted-foreground flex-1">
          {shown ? format(days[shown.index].date, "EEE, MMM d") : "Issues each day"}
        </figcaption>
        {SERIES.map((series) => (
          <span key={series.key} className="flex items-center gap-1.5">
            <span aria-hidden className={`h-0.5 w-3 rounded-full ${series.swatch}`} />
            <span className="text-muted-foreground">{series.label}</span>
            <span className="font-medium tabular-nums">{(shown ?? last).point[series.key]}</span>
          </span>
        ))}
      </div>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        preserveAspectRatio="none"
        role="img"
        aria-label={`${last.point.scope} issues, ${last.point.started} started, ${last.point.completed} completed`}
        className="h-40 w-full overflow-visible"
      >
        {[0, 0.5, 1].map((ratio) => (
          <line
            key={ratio}
            x1={PAD.left}
            x2={WIDTH - PAD.right}
            y1={y(max * ratio)}
            y2={y(max * ratio)}
            vectorEffect="non-scaling-stroke"
            className="stroke-border"
            strokeWidth={1}
          />
        ))}
        <path d={completedArea} className="fill-chart-1/10" />
        {SERIES.map((series) => (
          <path
            key={series.key}
            d={line(series.key)}
            vectorEffect="non-scaling-stroke"
            className={`${series.stroke} fill-none`}
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        ))}
        {shown && (
          <line
            x1={x(shown.index)}
            x2={x(shown.index)}
            y1={PAD.top}
            y2={HEIGHT - PAD.bottom}
            vectorEffect="non-scaling-stroke"
            className="stroke-foreground/30"
            strokeWidth={1}
          />
        )}
        <rect
          width={WIDTH}
          height={HEIGHT}
          className="fill-transparent"
          onPointerMove={onPointerMove}
          onPointerLeave={() => setHovered(null)}
        />
      </svg>
      <div className="text-muted-foreground flex justify-between text-xs">
        <span>{format(new Date(cycle.startsAt), "MMM d")}</span>
        <span>{format(cycleLastDay(cycle), "MMM d")}</span>
      </div>
    </figure>
  );
}
