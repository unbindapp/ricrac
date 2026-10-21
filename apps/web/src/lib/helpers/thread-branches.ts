type TBranchThread = { id: string; createdAt: string; resolvedAt: string | null };

export type TBranchLane<T> = { lane: number; thread: T };

export type TBranchRow<T> = {
  kind: "fork" | "merge";
  thread: T;
  lane: number;
  /** The other threads' lanes running past the row. */
  through: TBranchLane<T>[];
};

export type TBranchGraph<T> = {
  rows: TBranchRow<T>[];
  /** Lanes still running at the end, the open threads. */
  open: TBranchLane<T>[];
  /** Lanes across the graph, the main line included. */
  width: number;
};

type TEvent<T> = { kind: "fork" | "merge"; thread: T; at: number; order: number };

/**
 * Threads as branches off the main line: each forks when it starts, on the lowest free lane,
 * and merges back when it is resolved, freeing its lane.
 */
export function branchGraph<T extends TBranchThread>(threads: T[]): TBranchGraph<T> {
  const active = new Map<number, T>();
  const laneById = new Map<string, number>();
  const rows: TBranchRow<T>[] = [];
  let width = 1;
  for (const event of branchEvents(threads)) {
    if (event.kind === "fork") {
      const lane = lowestFreeLane(active);
      rows.push({ kind: "fork", thread: event.thread, lane, through: lanesOf(active) });
      active.set(lane, event.thread);
      laneById.set(event.thread.id, lane);
      width = Math.max(width, lane + 1);
      continue;
    }
    const lane = laneById.get(event.thread.id) ?? 0;
    active.delete(lane);
    rows.push({ kind: "merge", thread: event.thread, lane, through: lanesOf(active) });
  }
  return { rows, open: lanesOf(active), width };
}

function branchEvents<T extends TBranchThread>(threads: T[]) {
  const events = threads.flatMap((thread, order): TEvent<T>[] => {
    const forkedAt = Date.parse(thread.createdAt);
    const fork = { kind: "fork" as const, thread, at: forkedAt, order };
    if (!thread.resolvedAt) return [fork];
    const mergedAt = Math.max(forkedAt, Date.parse(thread.resolvedAt));
    return [fork, { kind: "merge", thread, at: mergedAt, order }];
  });
  return events.sort(
    (a, b) => a.at - b.at || kindRank(a.kind) - kindRank(b.kind) || a.order - b.order,
  );
}

function kindRank(kind: TEvent<unknown>["kind"]) {
  return kind === "fork" ? 0 : 1;
}

function lowestFreeLane(active: Map<number, unknown>) {
  let lane = 1;
  while (active.has(lane)) lane++;
  return lane;
}

function lanesOf<T>(active: Map<number, T>): TBranchLane<T>[] {
  return [...active].map(([lane, thread]) => ({ lane, thread })).sort((a, b) => a.lane - b.lane);
}
