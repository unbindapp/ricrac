import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { Favorite } from "@/api/ops.gen";
import { call, callAll } from "@/lib/server/client";

export type TFavorite = Favorite;
export type TFavoriteKind = Favorite["type"];

export const queryKeyFavorites = {
  all: (workspaceId: string) => [workspaceId, "favorites"] as const,
  list: (workspaceId: string) => [workspaceId, "favorites", "list"] as const,
};

/** Your favorites in their sidebar order. */
export const favoritesQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyFavorites.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_favorites", {}),
  });

/** The favorite pointing at a thing, by the thing's id. */
export function findFavorite(favorites: Favorite[] | undefined, kind: TFavoriteKind, id: string) {
  return favorites?.find((favorite) => favorite.type === kind && favorite.target.id === id);
}

function useInvalidateFavorites(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyFavorites.all(workspaceId) });
}

export function useAddFavoriteMutation(workspaceId: string) {
  const invalidate = useInvalidateFavorites(workspaceId);
  return useMutation({
    mutationFn: ({ kind, id }: { kind: TFavoriteKind; id: string }) =>
      call(workspaceId, "add_favorite", { type: kind, target: id }),
    meta: { errorTitle: "Could not add favorite" },
    onSuccess: invalidate,
  });
}

export function useRemoveFavoriteMutation(workspaceId: string) {
  const invalidate = useInvalidateFavorites(workspaceId);
  return useMutation({
    mutationFn: (favoriteId: string) => call(workspaceId, "remove_favorite", { id: favoriteId }),
    meta: { errorTitle: "Could not remove favorite" },
    onSuccess: invalidate,
  });
}

/** Moves a favorite: adding it again with a new sortOrder keeps it and changes its place. */
export function useMoveFavoriteMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateFavorites(workspaceId);
  return useMutation({
    mutationFn: ({ favorite, sortOrder }: { favorite: Favorite; sortOrder: number }) =>
      call(workspaceId, "add_favorite", {
        type: favorite.type,
        target: favorite.target.id,
        sortOrder,
      }),
    meta: { errorTitle: "Could not move favorite" },
    onMutate: ({ favorite, sortOrder }) => {
      queryClient.setQueryData(queryKeyFavorites.list(workspaceId), (favorites?: Favorite[]) =>
        favorites
          ?.map((item) => (item.id === favorite.id ? { ...item, sortOrder } : item))
          .sort(
            (a, b) => (a.folder ?? "").localeCompare(b.folder ?? "") || a.sortOrder - b.sortOrder,
          ),
      );
    },
    onSettled: invalidate,
  });
}
