import {
  closestCorners,
  DndContext,
  type DragEndEvent,
  DragOverlay,
  type DragStartEvent,
  KeyboardSensor,
  PointerSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type MouseEvent, useState } from "react";

import type { BoardColumn, Discussion, Issue } from "@/api/ops.gen";
import { DiscussionBoardCard } from "@/components/boards/discussion-board-card";
import { DiscussionSearchPicker } from "@/components/forum/discussion-search-picker";
import { MessagesSquareIcon, MinusIcon, PlusIcon } from "@/components/icons";
import { DEFAULT_DISPLAY } from "@/components/issues/display";
import { IssueCard } from "@/components/issues/issue-card";
import { useIsPeekable, useIssuePeek } from "@/components/issues/issue-peek";
import { IssueSearchPicker } from "@/components/issues/pickers";
import { sortOrderAtIndex } from "@/components/issues/sort-order";
import { useOpenIssuePage } from "@/components/issues/use-issue-actions";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { Button } from "@/components/ui/button";
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "@/components/ui/context-menu";
import { DropdownMenuGroup, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import {
  boardCardsQuery,
  boardItemKey,
  type TBoard,
  type TBoardItem,
  useAddIdeaMutation,
  usePlaceOnBoardMutation,
  useRemoveFromBoardMutation,
} from "@/lib/queries/boards";

const DRAG_ACTIVATION_DISTANCE = 6;
const NEW_ISSUE = "new";

type TCardData = { columnId: string; key: string };

function cardId(columnId: string, key: string) {
  return `${columnId}::${key}`;
}

function columnDropId(columnId: string) {
  return `column::${columnId}`;
}

export function BoardColumns({ board }: { board: TBoard }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(boardCardsQuery(workspace.id, board.id));

  if (isPending) return <ListSkeleton rows={8} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  return <Columns board={board} cards={data} />;
}

function Columns({ board, cards }: { board: TBoard; cards: TBoardItem[] }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: place } = usePlaceOnBoardMutation(workspace.id, board.id);
  const [activeCard, setActiveCard] = useState<TCardData | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: DRAG_ACTIVATION_DISTANCE } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const cardsIn = (columnId: string) =>
    cards.filter((card) => card.column.id === columnId).sort((a, b) => a.sortOrder - b.sortOrder);
  const activeItem = activeCard
    ? cards.find((card) => boardItemKey(card) === activeCard.key)
    : undefined;

  const handleDragStart = (event: DragStartEvent) => {
    setActiveCard((event.active.data.current as TCardData | undefined) ?? null);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveCard(null);
    const source = event.active.data.current as TCardData | undefined;
    if (!source || !event.over) return;
    const overCard = event.over.data.current as Partial<TCardData> | undefined;
    const targetId = overCard?.columnId ?? String(event.over.id).replace(/^column::/, "");
    const target = board.columns.find((column) => column.id === targetId);
    if (!target) return;
    const sourceCards = cardsIn(source.columnId);
    const targetCards = cardsIn(target.id);
    const card = sourceCards.find((item) => boardItemKey(item) === source.key);
    if (!card) return;

    const isCard = (item: TBoardItem) => boardItemKey(item) === source.key;
    const isSameColumn = source.columnId === target.id;
    const overIndex = overCard?.key
      ? targetCards.findIndex((item) => boardItemKey(item) === overCard.key)
      : targetCards.length;
    const reordered = isSameColumn
      ? arrayMove(targetCards, targetCards.findIndex(isCard), overIndex)
      : [...targetCards.slice(0, overIndex), card, ...targetCards.slice(overIndex)];
    const newIndex = reordered.findIndex(isCard);
    if (isSameColumn && newIndex === sourceCards.indexOf(card)) return;
    const neighbors = reordered.filter((item) => !isCard(item)).map((item) => item.sortOrder);
    place({
      subject: card,
      column: target.id,
      columnName: target.name,
      sortOrder: sortOrderAtIndex(neighbors, newIndex),
    });
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragCancel={() => setActiveCard(null)}
      onDragEnd={handleDragEnd}
    >
      <div className="flex w-full min-w-0 flex-1 items-start gap-3 overflow-x-auto pb-2">
        {board.columns.map((column) => (
          <Column
            key={column.id}
            board={board}
            column={column}
            cards={cardsIn(column.id)}
            canEdit={canEdit}
            onBoard={cards}
          />
        ))}
      </div>
      <DragOverlay>
        {activeItem?.kind === "issue" && (
          <IssueCard
            issue={activeItem.issue}
            properties={DEFAULT_DISPLAY.properties}
            canEdit={false}
            isOverlay
          />
        )}
        {activeItem?.kind === "discussion" && (
          <DiscussionBoardCard discussion={activeItem.discussion} isOverlay />
        )}
      </DragOverlay>
    </DndContext>
  );
}

function Column({
  board,
  column,
  cards,
  canEdit,
  onBoard,
}: {
  board: TBoard;
  column: BoardColumn;
  cards: TBoardItem[];
  canEdit: boolean;
  onBoard: TBoardItem[];
}) {
  const { workspace } = useWorkspace();
  const { setNodeRef, isOver } = useDroppable({ id: columnDropId(column.id) });
  const { mutate: place } = usePlaceOnBoardMutation(workspace.id, board.id);
  const [isWriting, setIsWriting] = useState(false);
  const ids = cards.map((card) => cardId(column.id, boardItemKey(card)));
  const idsOf = (kind: TBoardItem["kind"]) =>
    onBoard.filter((card) => card.kind === kind).map((card) => card.id);
  return (
    <section
      aria-label={column.name}
      data-over={isOver || undefined}
      className="bg-muted dank:border-transparent dank:data-over:border-m3-primary data-over:border-primary flex w-72 shrink-0 flex-col rounded-lg border"
    >
      <header className="dank:h-12 dank:pl-4 flex h-10 items-center gap-2 pr-1 pl-3 text-sm">
        <span className="dank:type-title-small min-w-0 truncate font-medium">{column.name}</span>
        <span className="text-muted-foreground dank:type-label-large text-xs">{cards.length}</span>
        <span className="flex-1" />
        {canEdit && (
          <DiscussionSearchPicker
            label={`Add a discussion to ${column.name}`}
            exclude={idsOf("discussion")}
            onSelect={(discussion) =>
              place({ subject: { kind: "discussion", id: discussion.id }, column: column.id })
            }
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Add a discussion to ${column.name}`}>
                <MessagesSquareIcon className="size-4" />
              </Button>
            }
          />
        )}
        {canEdit && (
          <IssueSearchPicker
            label={`Add to ${column.name}`}
            exclude={idsOf("issue")}
            extraOptions={[
              {
                value: NEW_ISSUE,
                label: "Write a new issue…",
                icon: <PlusIcon className="size-4" />,
              },
            ]}
            onOption={() => setIsWriting(true)}
            onSelect={(issue) =>
              issue && place({ subject: { kind: "issue", id: issue.id }, column: column.id })
            }
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Add an issue to ${column.name}`}>
                <PlusIcon className="size-4" />
              </Button>
            }
          />
        )}
      </header>
      <SortableContext id={column.id} items={ids} strategy={verticalListSortingStrategy}>
        <div ref={setNodeRef} role="list" className="flex min-h-16 flex-col gap-2 px-2 pb-2">
          {cards.map((card) => (
            <SortableCard
              key={boardItemKey(card)}
              board={board}
              columnId={column.id}
              card={card}
              canEdit={canEdit}
            />
          ))}
          {isWriting && (
            <NewIssueInput board={board} column={column} onDone={() => setIsWriting(false)} />
          )}
        </div>
      </SortableContext>
    </section>
  );
}

function NewIssueInput({
  board,
  column,
  onDone,
}: {
  board: TBoard;
  column: BoardColumn;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const { mutate: addIdea } = useAddIdeaMutation(workspace.id, board.id);
  const [title, setTitle] = useState("");
  const submit = () => {
    const trimmed = title.trim();
    if (!trimmed) {
      onDone();
      return;
    }
    setTitle("");
    addIdea({ title: trimmed, column: column.id }, { onError: () => setTitle(trimmed) });
  };
  return (
    <div className="flex flex-col gap-1.5">
      <Input
        autoFocus
        aria-label={`New issue in ${column.name}`}
        placeholder="Title"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        onBlur={() => !title.trim() && onDone()}
        onKeyDown={(event) => {
          if (event.key === "Enter") submit();
          if (event.key === "Escape") onDone();
        }}
      />
      <p className="text-muted-foreground text-xs">Enter adds it. It stays off GitHub.</p>
    </div>
  );
}

function SortableCard({
  board,
  columnId,
  card,
  canEdit,
}: {
  board: TBoard;
  columnId: string;
  card: TBoardItem;
  canEdit: boolean;
}) {
  const { workspace } = useWorkspace();
  const { mutate: remove } = useRemoveFromBoardMutation(workspace.id, board.id);
  const key = boardItemKey(card);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: cardId(columnId, key),
    data: { columnId, key } satisfies TCardData,
    disabled: !canEdit,
  });
  const sortable = {
    ref: setNodeRef,
    isDragging,
    style: {
      transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
      transition,
    },
    dragProps: { ...attributes, ...listeners },
  };
  const cardElement =
    card.kind === "issue" ? (
      <SortableIssueCard issue={card.issue} canEdit={canEdit} sortable={sortable} />
    ) : (
      <SortableDiscussionCard discussion={card.discussion} sortable={sortable} />
    );
  if (!canEdit) return cardElement;
  return (
    <ContextMenu>
      <ContextMenuTrigger render={<div />}>{cardElement}</ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={() => remove(card)}>
            <MinusIcon className="size-4" />
            Remove from board
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </ContextMenuContent>
    </ContextMenu>
  );
}

type TSortableProps = Pick<
  Parameters<typeof IssueCard>[0],
  "ref" | "isDragging" | "style" | "dragProps"
>;

function SortableIssueCard({
  issue,
  canEdit,
  sortable,
}: {
  issue: Issue;
  canEdit: boolean;
  sortable: TSortableProps;
}) {
  const peek = useIssuePeek();
  const isPeekable = useIsPeekable();
  const openPage = useOpenIssuePage();
  const handleClick = (event: MouseEvent) => {
    if (event.metaKey || event.ctrlKey || !peek || !isPeekable) {
      void openPage(issue.identifier);
      return;
    }
    peek.openPeek(issue.identifier);
  };
  return (
    <IssueCard
      {...sortable}
      issue={issue}
      properties={DEFAULT_DISPLAY.properties}
      canEdit={canEdit}
      isActive={peek?.peek?.toUpperCase() === issue.identifier}
      onClick={handleClick}
    />
  );
}

function SortableDiscussionCard({
  discussion,
  sortable,
}: {
  discussion: Discussion;
  sortable: TSortableProps;
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const handleClick = (event: MouseEvent) => {
    if (event.metaKey || event.ctrlKey) {
      window.open(discussion.url, "_blank", "noopener");
      return;
    }
    void navigate({
      to: "/$workspace/discussion/$slug",
      params: { workspace: workspace.urlKey, slug: entitySlug(discussion) },
    });
  };
  return <DiscussionBoardCard {...sortable} discussion={discussion} onClick={handleClick} />;
}
