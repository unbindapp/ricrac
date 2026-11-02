import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { Board, BoardCard } from "@/api/ops.gen";
import { invalidateIssues } from "@/lib/queries/issues";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TBoard = Board;
export type TBoardCard = BoardCard;

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

export const boardCardsQuery = (workspaceId: string, boardId: string) =>
  queryOptions({
    queryKey: queryKeyBoards.cards(workspaceId, boardId),
    queryFn: () => callAll(workspaceId, "list_board_issues", { board: boardId }),
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

export function usePlaceOnBoardMutation(workspaceId: string, boardId: string) {
  const queryClient = useQueryClient();
  const key = queryKeyBoards.cards(workspaceId, boardId);
  return useMutation({
    mutationFn: (args: Omit<OpArgs<"place_on_board">, "board"> & { columnName?: string }) =>
      call(workspaceId, "place_on_board", {
        board: boardId,
        issue: args.issue,
        column: args.column,
        sortOrder: args.sortOrder,
      }),
    meta: { errorTitle: "Could not move the card" },
    onMutate: async ({ issue, column, columnName, sortOrder }) => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<TBoardCard[]>(key);
      if (previous && columnName !== undefined && sortOrder != null) {
        queryClient.setQueryData<TBoardCard[]>(
          key,
          previous.map((card) =>
            card.issue.id === issue
              ? { ...card, column: { id: column, name: columnName }, sortOrder }
              : card,
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
    mutationFn: (issue: string) =>
      call(workspaceId, "remove_from_board", { board: boardId, issue }),
    meta: { errorTitle: "Could not remove the card" },
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
