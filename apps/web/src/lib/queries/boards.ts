import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type {
  Board,
  BoardCard,
  BoardColumnRef,
  BoardDiscussionCard,
  Discussion,
  Issue,
  RemovedDiscussionFromBoard,
  RemovedFromBoard,
} from "@/api/ops.gen";
import { invalidateIssues } from "@/lib/queries/issues";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TBoard = Board;

export type TBoardSubject = { kind: "issue" | "discussion"; id: string };

type TPlacement = { column: BoardColumnRef; sortOrder: number };

export type TBoardItem = TPlacement &
  (
    | { kind: "issue"; id: string; issue: Issue }
    | { kind: "discussion"; id: string; discussion: Discussion }
  );

export function boardItemKey(subject: TBoardSubject) {
  return `${subject.kind}:${subject.id}`;
}

const isSubject = (item: TBoardItem, subject: TBoardSubject) =>
  item.kind === subject.kind && item.id === subject.id;

export const queryKeyBoards = {
  all: (workspaceId: string) => [workspaceId, "boards"] as const,
  list: (workspaceId: string) => [workspaceId, "boards", "list"] as const,
  detail: (workspaceId: string, board: string) => [workspaceId, "boards", "detail", board] as const,
  cards: (workspaceId: string, boardId: string) =>
    [workspaceId, "boards", "cards", boardId] as const,
};

export const boardsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyBoards.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_boards", {}),
  });

/** A board by id, slug or link tail (its route param). */
export const boardQuery = (workspaceId: string, board: string) =>
  queryOptions({
    queryKey: queryKeyBoards.detail(workspaceId, board),
    queryFn: () => call(workspaceId, "get_board", { id: board }),
    retry: false,
  });

/** The board's issues and discussions together, ordered by column through sortOrder. */
export const boardCardsQuery = (workspaceId: string, boardId: string) =>
  queryOptions({
    queryKey: queryKeyBoards.cards(workspaceId, boardId),
    queryFn: async (): Promise<TBoardItem[]> => {
      const [issues, discussions] = await Promise.all([
        callAll(workspaceId, "list_board_issues", { board: boardId }),
        callAll(workspaceId, "list_board_discussions", { board: boardId, includeBody: true }),
      ]);
      return [
        ...issues.map(({ column, sortOrder, issue }) => ({
          kind: "issue" as const,
          id: issue.id,
          column,
          sortOrder,
          issue,
        })),
        ...discussions.map(({ column, sortOrder, discussion }) => ({
          kind: "discussion" as const,
          id: discussion.id,
          column,
          sortOrder,
          discussion,
        })),
      ];
    },
  });

/** The part of a board's link after /board/: its name and slug id, which get_board takes. */
export function boardParam(board: Pick<TBoard, "url" | "slugId">) {
  return board.url.split("/board/")[1] ?? board.slugId;
}

function useInvalidateBoards(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyBoards.all(workspaceId) });
}

export function useSaveBoardMutation(workspaceId: string) {
  const invalidate = useInvalidateBoards(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_board">) => call(workspaceId, "save_board", args),
    meta: { errorTitle: "Could not save board" },
    onSuccess: invalidate,
  });
}

export function useDeleteBoardMutation(workspaceId: string) {
  const invalidate = useInvalidateBoards(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_board", { id }),
    meta: { errorTitle: "Could not delete board" },
    onSuccess: invalidate,
  });
}

type TPlaceArgs = {
  subject: TBoardSubject;
  column: string;
  columnName?: string;
  sortOrder?: number;
};

function placeOnBoard(
  workspaceId: string,
  boardId: string,
  args: TPlaceArgs,
): Promise<BoardCard | BoardDiscussionCard> {
  const { subject, column, sortOrder } = args;
  if (subject.kind === "discussion") {
    return call(workspaceId, "place_discussion_on_board", {
      board: boardId,
      discussion: subject.id,
      column,
      sortOrder,
    });
  }
  return call(workspaceId, "place_on_board", {
    board: boardId,
    issue: subject.id,
    column,
    sortOrder,
  });
}

export function usePlaceOnBoardMutation(workspaceId: string, boardId: string) {
  const queryClient = useQueryClient();
  const key = queryKeyBoards.cards(workspaceId, boardId);
  return useMutation({
    mutationFn: (args: TPlaceArgs) => placeOnBoard(workspaceId, boardId, args),
    meta: { errorTitle: "Could not move the card" },
    onMutate: async ({ subject, column, columnName, sortOrder }) => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<TBoardItem[]>(key);
      if (previous && columnName !== undefined && sortOrder != null) {
        queryClient.setQueryData<TBoardItem[]>(
          key,
          previous.map((item) =>
            isSubject(item, subject)
              ? { ...item, column: { id: column, name: columnName }, sortOrder }
              : item,
          ),
        );
      }
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeyBoards.all(workspaceId) }),
  });
}

export function useRemoveFromBoardMutation(workspaceId: string, boardId: string) {
  const invalidate = useInvalidateBoards(workspaceId);
  return useMutation({
    mutationFn: (subject: TBoardSubject): Promise<RemovedFromBoard | RemovedDiscussionFromBoard> =>
      subject.kind === "discussion"
        ? call(workspaceId, "remove_discussion_from_board", {
            board: boardId,
            discussion: subject.id,
          })
        : call(workspaceId, "remove_from_board", { board: boardId, issue: subject.id }),
    meta: { errorTitle: "Could not remove the card" },
    onSuccess: invalidate,
  });
}

export function useAddToBoardMutation(workspaceId: string) {
  const invalidate = useInvalidateBoards(workspaceId);
  return useMutation({
    mutationFn: ({ board, ...args }: TPlaceArgs & { board: TBoard }) =>
      placeOnBoard(workspaceId, board.id, args),
    meta: { errorTitle: "Could not add to the board" },
    onSuccess: invalidate,
  });
}

export function useAddIdeaMutation(workspaceId: string, boardId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ title, column }: { title: string; column: string }) => {
      const issue = await call(workspaceId, "save_issue", { title, githubRepository: "none" });
      return call(workspaceId, "place_on_board", { board: boardId, issue: issue.id, column });
    },
    meta: { errorTitle: "Could not add the issue" },
    onSettled: () => invalidateIssues(queryClient, workspaceId),
  });
}
