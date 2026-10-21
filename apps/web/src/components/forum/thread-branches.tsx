import { Link } from "@tanstack/react-router";
import {
  CircleCheckIcon,
  CircleDotIcon,
  MessageSquareIcon,
  PlusIcon,
  SplitIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import type { DiscussionDetail, DiscussionThread } from "@/api/ops.gen";
import { NewThreadDialog } from "@/components/forum/new-thread-dialog";
import { threadKindOf, threadKinds } from "@/components/forum/thread-kinds";
import { EmptyState } from "@/components/layout/empty-state";
import { SectionTitle } from "@/components/layout/section-title";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { branchGraph, type TBranchLane, type TBranchRow } from "@/lib/helpers/thread-branches";

const LANE_GAP = 16;
const LANE_INSET = 12;
const TALL_ROW = 48;
const SHORT_ROW = 36;
const NODE_RADIUS = 5;
const MERGE_RADIUS = 7;

type TLane = TBranchLane<DiscussionThread>;

type TProps = {
  discussion: DiscussionDetail;
  openThreadId?: string;
  isMainChatOpen: boolean;
};

/** The discussion's threads drawn as branches off its main chat, like a git graph. */
export function ThreadBranches({ discussion, openThreadId, isMainChatOpen }: TProps) {
  const { canEdit } = useWorkspace();
  const { threads } = discussion;
  const graph = branchGraph(threads);
  const openCount = graph.open.length;
  const summary =
    threads.length > 0 ? `${openCount} open · ${threads.length - openCount} resolved` : undefined;

  return (
    <section aria-label="Branches" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Branches"
        description={summary}
        actions={
          canEdit && (
            <NewThreadDialog
              discussion={discussion}
              trigger={
                <Button variant="outline" size="sm">
                  <PlusIcon className="size-4" />
                  New thread
                </Button>
              }
            />
          )
        }
      />
      {threads.length === 0 ? (
        <EmptyState
          icon={SplitIcon}
          title="No branches yet"
          description="Split out a bug, a design question or an idea to settle it on its own."
        />
      ) : (
        <ol aria-label="Branches" className="flex flex-col overflow-hidden rounded-lg border py-1">
          <li>
            <MainChatRow discussion={discussion} width={graph.width} isOpen={isMainChatOpen} />
          </li>
          {graph.rows.map((row) => (
            <li key={`${row.kind}-${row.thread.id}`}>
              <BranchRow
                discussion={discussion}
                row={row}
                width={graph.width}
                openThreadId={openThreadId}
                isMainChatOpen={isMainChatOpen}
              />
            </li>
          ))}
          <li>
            <NowRow
              open={graph.open}
              width={graph.width}
              openThreadId={openThreadId}
              isMainChatOpen={isMainChatOpen}
            />
          </li>
        </ol>
      )}
    </section>
  );
}

function MainChatRow({
  discussion,
  width,
  isOpen,
}: {
  discussion: DiscussionDetail;
  width: number;
  isOpen: boolean;
}) {
  const count = discussion.commentCount;
  return (
    <BranchLink discussion={discussion} height="tall">
      <BranchCell width={width} height={TALL_ROW}>
        <MainLine from={TALL_ROW / 2} to={TALL_ROW} isOpen={isOpen} />
        <circle
          cx={laneX(0)}
          cy={TALL_ROW / 2}
          r={NODE_RADIUS + 1}
          className={isOpen ? "fill-primary" : "fill-muted-foreground"}
        />
      </BranchCell>
      <RowText
        title={<span className="truncate font-medium">Main chat</span>}
        meta={
          <>
            <span className="flex items-center gap-1.5 tabular-nums">
              <MessageSquareIcon className="size-3.5" />
              {count} {count === 1 ? "message" : "messages"}
            </span>
            {discussion.creator && (
              <span className="truncate">
                Started by {discussion.creator.name} <RelativeTime value={discussion.createdAt} />
              </span>
            )}
          </>
        }
      />
    </BranchLink>
  );
}

function BranchRow({
  discussion,
  row,
  width,
  openThreadId,
  isMainChatOpen,
}: {
  discussion: DiscussionDetail;
  row: TBranchRow<DiscussionThread>;
  width: number;
  openThreadId?: string;
  isMainChatOpen: boolean;
}) {
  const { thread, lane } = row;
  const isOpen = thread.id === openThreadId;
  const isResolved = Boolean(thread.resolvedAt);
  const height = row.kind === "fork" ? TALL_ROW : SHORT_ROW;
  const cy = height / 2;
  const tone = laneTone(thread);
  const strokeWidth = isOpen ? 3 : 2;

  return (
    <BranchLink
      discussion={discussion}
      threadId={thread.id}
      height={row.kind === "fork" ? "tall" : "short"}
    >
      <BranchCell width={width} height={height}>
        <MainLine from={0} to={height} isOpen={isMainChatOpen} />
        <ThroughLanes lanes={row.through} height={height} openThreadId={openThreadId} />
        {row.kind === "fork" ? (
          <>
            <path
              d={curve(laneX(0), 0, laneX(lane), cy - NODE_RADIUS)}
              className={tone}
              strokeWidth={strokeWidth}
            />
            <path
              d={`M ${laneX(lane)} ${cy + NODE_RADIUS} V ${height}`}
              className={tone}
              strokeWidth={strokeWidth}
            />
            <ThreadNode x={laneX(lane)} y={cy} isResolved={isResolved} />
          </>
        ) : (
          <>
            <path
              d={curve(laneX(lane), 0, laneX(0), cy)}
              className={tone}
              strokeWidth={strokeWidth}
            />
            <MergeNode x={laneX(0)} y={cy} />
          </>
        )}
      </BranchCell>
      {row.kind === "fork" ? <ForkText thread={thread} /> : <MergeText thread={thread} />}
    </BranchLink>
  );
}

function NowRow({
  open,
  width,
  openThreadId,
  isMainChatOpen,
}: {
  open: TLane[];
  width: number;
  openThreadId?: string;
  isMainChatOpen: boolean;
}) {
  const cy = SHORT_ROW / 2;
  return (
    <div className="flex h-9 items-center gap-2 pr-3 pl-1">
      <BranchCell width={width} height={SHORT_ROW}>
        <MainLine from={0} to={cy - NODE_RADIUS} isOpen={isMainChatOpen} />
        <circle
          cx={laneX(0)}
          cy={cy}
          r={NODE_RADIUS}
          className={isMainChatOpen ? "stroke-primary" : "stroke-muted-foreground"}
        />
        {open.map(({ lane, thread }) => (
          <g key={thread.id}>
            <path
              d={`M ${laneX(lane)} 0 V ${cy - NODE_RADIUS}`}
              className="stroke-primary"
              strokeWidth={thread.id === openThreadId ? 3 : 2}
            />
            <circle
              cx={laneX(lane)}
              cy={cy}
              r={NODE_RADIUS + 3}
              className="fill-primary/20 animate-pulse stroke-none"
            />
            <circle cx={laneX(lane)} cy={cy} r={NODE_RADIUS} className="stroke-primary" />
          </g>
        ))}
      </BranchCell>
      <span className="text-muted-foreground text-xs">{stillOpen(open.length)}</span>
    </div>
  );
}

function ForkText({ thread }: { thread: DiscussionThread }) {
  const { label, icon: Icon } = threadKinds[threadKindOf(thread.kind)];
  const isResolved = Boolean(thread.resolvedAt);
  return (
    <>
      <RowText
        title={
          <>
            <Icon aria-label={label} role="img" className="text-muted-foreground size-4 shrink-0" />
            <span
              data-resolved={isResolved || undefined}
              className="data-resolved:text-muted-foreground truncate font-medium"
            >
              {thread.title}
            </span>
          </>
        }
        meta={
          <>
            {thread.issue && (
              <span className="text-foreground font-mono">{thread.issue.identifier}</span>
            )}
            <span className="flex items-center gap-1.5 tabular-nums">
              <MessageSquareIcon className="size-3.5" />
              {thread.commentCount}
            </span>
            {thread.creator && (
              <span className="truncate">
                {thread.creator.name} <RelativeTime value={thread.createdAt} />
              </span>
            )}
          </>
        }
      />
      <BranchStatusBadge isResolved={isResolved} />
    </>
  );
}

function MergeText({ thread }: { thread: DiscussionThread }) {
  return (
    <span className="text-muted-foreground flex min-w-0 flex-1 items-center gap-1.5 text-xs">
      <span className="text-success shrink-0 font-medium">
        Resolved{thread.resolvingUser && ` by ${thread.resolvingUser.name}`}
      </span>
      <span className="truncate">{thread.title}</span>
      {thread.resolvedAt && (
        <span className="ml-auto shrink-0">
          <RelativeTime value={thread.resolvedAt} />
        </span>
      )}
    </span>
  );
}

function RowText({ title, meta }: { title: ReactNode; meta: ReactNode }) {
  return (
    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
      <span className="flex min-w-0 items-center gap-1.5 text-sm">{title}</span>
      <span className="text-muted-foreground flex min-w-0 items-center gap-2 text-xs">{meta}</span>
    </span>
  );
}

function BranchStatusBadge({ isResolved }: { isResolved: boolean }) {
  if (isResolved) {
    return (
      <Badge variant="success">
        <CircleCheckIcon />
        Resolved
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="text-primary border-primary/40">
      <CircleDotIcon />
      Open
    </Badge>
  );
}

function BranchLink({
  discussion,
  threadId,
  height,
  children,
}: {
  discussion: DiscussionDetail;
  threadId?: string;
  height: "tall" | "short";
  children: ReactNode;
}) {
  const { workspace } = useWorkspace();
  return (
    <Link
      to="/$workspace/discussion/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
      search={threadId ? { thread: threadId } : {}}
      replace
      activeOptions={{ exact: true }}
      activeProps={{ className: "bg-muted" }}
      className={cn(
        "has-hover:hover:bg-muted focus-visible:bg-muted flex items-center gap-2 pr-3 pl-1 outline-hidden",
        height === "tall" ? "h-12" : "h-9",
      )}
    >
      {children}
    </Link>
  );
}

function BranchCell({
  width,
  height,
  children,
}: {
  width: number;
  height: number;
  children: ReactNode;
}) {
  const pixels = LANE_INSET * 2 + (width - 1) * LANE_GAP;
  return (
    <svg
      aria-hidden
      width={pixels}
      height={height}
      viewBox={`0 0 ${pixels} ${height}`}
      fill="none"
      strokeWidth={2}
      strokeLinecap="round"
      className="shrink-0"
    >
      {children}
    </svg>
  );
}

function MainLine({ from, to, isOpen }: { from: number; to: number; isOpen: boolean }) {
  return (
    <path
      d={`M ${laneX(0)} ${from} V ${to}`}
      className={isOpen ? "stroke-primary" : "stroke-muted-foreground"}
      strokeWidth={isOpen ? 3 : 2}
    />
  );
}

function ThroughLanes({
  lanes,
  height,
  openThreadId,
}: {
  lanes: TLane[];
  height: number;
  openThreadId?: string;
}) {
  return lanes.map(({ lane, thread }) => (
    <path
      key={thread.id}
      d={`M ${laneX(lane)} 0 V ${height}`}
      className={laneTone(thread)}
      strokeWidth={thread.id === openThreadId ? 3 : 2}
    />
  ));
}

function ThreadNode({ x, y, isResolved }: { x: number; y: number; isResolved: boolean }) {
  if (isResolved) return <circle cx={x} cy={y} r={NODE_RADIUS - 1} className="fill-success" />;
  return <circle cx={x} cy={y} r={NODE_RADIUS} className="stroke-primary" />;
}

function MergeNode({ x, y }: { x: number; y: number }) {
  return (
    <>
      <circle cx={x} cy={y} r={MERGE_RADIUS} className="fill-success" />
      <path
        d={`M ${x - 3} ${y} L ${x - 1} ${y + 2} L ${x + 3} ${y - 2}`}
        className="stroke-success-foreground"
        strokeWidth={1.5}
        strokeLinejoin="round"
      />
    </>
  );
}

function laneX(lane: number) {
  return LANE_INSET + lane * LANE_GAP;
}

/** A smooth S from one lane to another, leaving and arriving vertically. */
function curve(fromX: number, fromY: number, toX: number, toY: number) {
  const bend = (toY - fromY) / 2;
  return `M ${fromX} ${fromY} C ${fromX} ${fromY + bend}, ${toX} ${toY - bend}, ${toX} ${toY}`;
}

function laneTone(thread: DiscussionThread) {
  return thread.resolvedAt ? "stroke-success" : "stroke-primary";
}

function stillOpen(count: number) {
  switch (count) {
    case 0:
      return "Every branch is resolved";
    case 1:
      return "1 branch still open";
    default:
      return `${count} branches still open`;
  }
}
